import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
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
        <Routes>
          <Route path="/change-password" element={<ChangePasswordPage />} />
          <Route path="/" element={<div>CONSOLE_HOME</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('blocks submit locally and skips the API call when the confirmation does not match', async () => {
    const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'new-pass-12345678', 'different-pass-123');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('两次输入的新密码不一致')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('posts {old_password,new_password} with X-CSRF-Token and navigates home on success', async () => {
    setCsrfToken('tok-csrf-1');
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(null, { status: 204 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'new-pass-12345678', 'new-pass-12345678');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('CONSOLE_HOME')).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    expect(String(input)).toBe('/api/v1/auth/password');
    expect(init).toMatchObject({ method: 'POST' });
    expect((init?.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok-csrf-1');
    expect(JSON.parse(String(init?.body))).toEqual({
      old_password: 'old-pass-123',
      new_password: 'new-pass-12345678',
    });
  });

  it('shows the initial-password error on a 401 response', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ message: 'bad credentials' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })) as typeof fetch;
    renderPage();

    fillPasswords('wrong-old-123', 'new-pass-12345678', 'new-pass-12345678');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('初始密码错误')).toBeTruthy();
  });

  it('shows the policy-violation error on a 400 response', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ message: 'policy' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      })) as typeof fetch;
    renderPage();

    fillPasswords('old-pass-123', 'weak', 'weak');
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    expect(await screen.findByText('新密码不符合密码策略')).toBeTruthy();
  });
});
