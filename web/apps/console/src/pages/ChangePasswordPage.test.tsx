import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { AuthProvider } from '../auth/AuthProvider';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import i18n from '../i18n';
import { setCsrfToken } from '../lib/auth';
import ChangePasswordPage from './ChangePasswordPage';

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

function fillPasswords(oldPassword: string, newPassword: string, confirmPassword: string): void {
  fireEvent.change(screen.getByLabelText('初始密码'), { target: { value: oldPassword } });
  fireEvent.change(screen.getByLabelText('新密码'), { target: { value: newPassword } });
  fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: confirmPassword } });
}

describe('ChangePasswordPage', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(async () => {
    setCsrfToken(null);
    globalThis.fetch = (async () => {
      throw new Error('unexpected fetch call');
    }) as typeof fetch;
    await i18n.changeLanguage('zh');
  });

  afterEach(() => {
    setCsrfToken(null);
    globalThis.fetch = originalFetch;
    cleanup();
  });

  function renderPage(): void {
    render(
      <MemoryRouter initialEntries={['/change-password']}>
        <AuthProvider>
          <Routes>
            <Route path="/change-password" element={<ChangePasswordPage />} />
            <Route path="/" element={<div>CONSOLE_HOME</div>} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
  }

  it('blocks submit locally and skips the API call when the confirmation does not match', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'new-pass-12345678', 'different-pass-123');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('两次输入的新密码不一致')).toBeTruthy();
    const passwordCalls = fetchMock.mock.calls.filter(
      ([input]) => String(input) === '/api/v1/auth/password',
    );
    expect(passwordCalls).toHaveLength(0);
  });

  it('posts {old_password,new_password} with X-CSRF-Token and navigates home on success', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return new Response(
          JSON.stringify({
            user: {
              id: 1,
              username: 'alice',
              role: 'user',
              status: 'active',
              must_change_password: true,
            },
            csrf_token: 'tok-csrf-1',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }
      if (method === 'POST' && path === '/api/v1/auth/password') {
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected fetch: ${method} ${path}`);
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('CONSOLE_HOME')).toBeTruthy();
    const passwordCalls = fetchMock.mock.calls.filter(
      ([input]) => String(input) === '/api/v1/auth/password',
    );
    expect(passwordCalls).toHaveLength(1);
    const [input, init] = passwordCalls[0];
    expect(String(input)).toBe('/api/v1/auth/password');
    expect(init).toMatchObject({ method: 'POST' });
    expect((init?.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok-csrf-1');
    expect(JSON.parse(String(init?.body))).toEqual({
      old_password: 'old-pass-123',
      new_password: 'new-pass-12345678',
    });
  });

  it('shows the initial-password error on 400 VALIDATION_ERROR for field old_password', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return new Response(JSON.stringify({ code: 'SESSION_EXPIRED', message: 'no session' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response(
        JSON.stringify({
          code: 'VALIDATION_ERROR',
          message: '请求参数校验失败',
          errors: [{ field: 'old_password', message: '旧密码不正确' }],
        }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      );
    }) as typeof fetch;
    renderPage();

    fillPasswords('wrong-old-123', 'new-pass-12345678', 'new-pass-12345678');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('初始密码错误')).toBeTruthy();
  });

  it('shows the policy-violation error on 400 VALIDATION_ERROR for field new_password', async () => {
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = (init?.method ?? 'GET').toUpperCase();
      if (method === 'GET' && path === '/api/v1/auth/me') {
        return new Response(JSON.stringify({ code: 'SESSION_EXPIRED', message: 'no session' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response(
        JSON.stringify({
          code: 'VALIDATION_ERROR',
          message: '请求参数校验失败',
          errors: [{ field: 'new_password', message: '密码不符合密码策略要求' }],
        }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      );
    }) as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'weak', 'weak');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('新密码不符合密码策略')).toBeTruthy();
  });
});
