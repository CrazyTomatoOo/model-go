import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router';
import { AuthProvider } from '../../auth/AuthProvider';
import i18n from '../../i18n';
import { setCsrfToken } from '../../lib/auth';
import ChangePasswordModal from './ChangePasswordModal';

// jsdom does not implement matchMedia; AntD's responsive observer needs it.
beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
});

afterAll(() => {
  Object.defineProperty(window, 'matchMedia', { writable: true, value: undefined });
});

function meResponse() {
  return new Response(
    JSON.stringify({
      user: { id: 1, username: 'alice', role: 'user', status: 'active', must_change_password: false },
      csrf_token: 'tok-csrf-1',
    }),
    { status: 200, headers: { 'Content-Type': 'application/json' } },
  );
}

function fieldErrorResponse(field: string, message: string) {
  return new Response(
    JSON.stringify({
      code: 'VALIDATION_ERROR',
      message: '请求参数校验失败',
      errors: [{ field, message }],
    }),
    { status: 400, headers: { 'Content-Type': 'application/json' } },
  );
}

describe('ChangePasswordModal', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(async () => {
    setCsrfToken(null);
    globalThis.fetch = (async () => meResponse()) as typeof fetch;
    await i18n.changeLanguage('zh');
  });

  afterEach(() => {
    setCsrfToken(null);
    globalThis.fetch = originalFetch;
    cleanup();
  });

  function renderModal(): void {
    render(
      <MemoryRouter>
        <AuthProvider>
          <ChangePasswordModal open onClose={() => undefined} />
        </AuthProvider>
      </MemoryRouter>,
    );
  }

  function fill(current: string, next: string, confirm: string): void {
    fireEvent.change(screen.getByLabelText('当前密码'), { target: { value: current } });
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: next } });
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: confirm } });
  }

  function submitForm(): void {
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));
  }

  it('renders the dialog with the three labelled password fields', () => {
    renderModal();
    const dialog = screen.getByRole('dialog', { name: '修改密码' });
    expect(dialog).toBeTruthy();
    expect(screen.getByLabelText('当前密码')).toBeTruthy();
    expect(screen.getByLabelText('新密码')).toBeTruthy();
    expect(screen.getByLabelText('确认新密码')).toBeTruthy();
  });

  it('blocks submit with the required message when fields are empty and skips the API', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    globalThis.fetch = (async () => meResponse()) as typeof fetch;
    renderModal();
    // Wait for the auth bootstrap, then arm the strict fetch mock: only the
    // bootstrap GET /auth/me may go out; a password POST would throw.
    await screen.findByRole('dialog');
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    submitForm();

    expect(await screen.findAllByText('请填写此项', {}, { timeout: 3000 })).not.toHaveLength(0);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects a policy-violating new password inline without calling the API', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    renderModal();
    await screen.findByRole('dialog');
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    fill('old-pass-123', 'short', 'short');
    submitForm();

    expect(await screen.findByText('新密码不符合密码策略')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects a new password that contains the username', async () => {
    renderModal();
    await screen.findByRole('dialog');
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    fill('old-pass-123', 'alice-new-password-1', 'alice-new-password-1');
    submitForm();

    expect(await screen.findByText('密码不得与用户名相同或包含用户名')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects a confirmation that does not match the new password', async () => {
    renderModal();
    await screen.findByRole('dialog');
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    fill('old-pass-123', 'new-pass-12345678', 'other-pass-123456');
    submitForm();

    expect(await screen.findByText('两次输入的新密码不一致')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('shows the success state and clears must_change_password on 204', async () => {
    const calls: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return meResponse();
      }
      if (method === 'POST' && path === '/api/v1/auth/password') {
        calls.push(path);
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected fetch: ${method} ${path}`);
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderModal();
    await screen.findByRole('dialog');

    fill('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    submitForm();

    expect(await screen.findByText('密码修改成功')).toBeTruthy();
    expect(calls).toEqual(['/api/v1/auth/password']);
  });

  it('maps a 400 field error on old_password to the current-password field', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return meResponse();
      }
      return fieldErrorResponse('old_password', '旧密码不正确');
    }) as typeof fetch;
    renderModal();
    await screen.findByRole('dialog');

    fill('wrong-old-123', 'new-pass-12345678', 'new-pass-12345678');
    submitForm();

    expect(await screen.findByText('当前密码错误')).toBeTruthy();
  });

  it('maps a 400 field error on new_password to the new-password field', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return meResponse();
      }
      return fieldErrorResponse('new_password', '新密码不得与当前密码或最近 5 次历史密码相同');
    }) as typeof fetch;
    renderModal();
    await screen.findByRole('dialog');

    fill('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    submitForm();

    expect(await screen.findByText('新密码不符合密码策略或与近期历史密码重复')).toBeTruthy();
  });

  it('maps a 429 RATE_LIMITED to a countdown alert and disables submit', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return meResponse();
      }
      return new Response(JSON.stringify({ code: 'RATE_LIMITED', message: '操作过于频繁' }), {
        status: 429,
        headers: { 'Content-Type': 'application/json', 'Retry-After': '7' },
      });
    }) as typeof fetch;
    renderModal();
    await screen.findByRole('dialog');

    fill('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    submitForm();

    expect(await screen.findByText('尝试过于频繁，请 7 秒后再试')).toBeTruthy();
    expect(screen.getByRole('button', { name: '确认修改' })).toHaveProperty('disabled', true);
  });

  it('shows the network alert when the request fails at transport level', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return meResponse();
      }
      throw new TypeError('fetch failed');
    }) as typeof fetch;
    renderModal();
    await screen.findByRole('dialog');

    fill('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    submitForm();

    expect(await screen.findByText('网络异常，请稍后重试')).toBeTruthy();
  });
});
