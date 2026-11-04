import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppRoutes } from './App';
import { AuthProvider } from './auth/AuthProvider';
import './i18n';
import { getCsrfToken, setCsrfToken } from './lib/auth';

type StubRoute = {
  method: string;
  path: string;
  status: number;
  body?: unknown;
};

const NO_SESSION = { code: 'SESSION_EXPIRED', message: 'no session' };

function user(mustChangePassword: boolean) {
  return {
    id: 1,
    username: 'alice',
    role: 'user',
    status: 'active',
    must_change_password: mustChangePassword,
  };
}

function stubRoutes(routes: StubRoute[]) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    const method = (init?.method ?? 'GET').toUpperCase();
    const route = routes.find((r) => r.method === method && r.path === path);
    if (route === undefined) {
      throw new Error(`unexpected fetch: ${method} ${path}`);
    }
    return new Response(route.body === undefined ? null : JSON.stringify(route.body), {
      status: route.status,
      headers: { 'Content-Type': 'application/json' },
    });
  });
}

function renderApp(initialPath: string) {
  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  );
}

function fillAndSubmitLogin(username: string, password: string) {
  fireEvent.change(screen.getByLabelText('Username'), { target: { value: username } });
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } });
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));
}

function fillAndSubmitPasswordChange() {
  fireEvent.change(screen.getByLabelText('Initial Password'), {
    target: { value: 'old-pass-123' },
  });
  fireEvent.change(screen.getByLabelText('New Password'), { target: { value: 'new-pass-12345678' } });
  fireEvent.change(screen.getByLabelText('Confirm New Password'), {
    target: { value: 'new-pass-12345678' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Confirm Change' }));
}

describe('App integration (AuthProvider + routes)', () => {
  const originalFetch = globalThis.fetch;

  beforeAll(() => {
    // antd's responsive observer requires matchMedia, which jsdom lacks.
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

  beforeEach(() => {
    setCsrfToken(null);
    globalThis.fetch = (async () => {
      throw new Error('unexpected fetch call');
    }) as typeof fetch;
  });

  afterEach(() => {
    cleanup();
    setCsrfToken(null);
    globalThis.fetch = originalFetch;
  });

  it('signs in, forces the password change, and lands on the shell', async () => {
    const fetchMock = stubRoutes([
      { method: 'GET', path: '/api/v1/auth/me', status: 401, body: NO_SESSION },
      {
        method: 'POST',
        path: '/api/v1/auth/login',
        status: 200,
        body: { user: user(true), csrf_token: 'tok-login' },
      },
      { method: 'POST', path: '/api/v1/auth/password', status: 204 },
    ]);
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderApp('/login');

    await screen.findByText('Sign in to Console');
    fillAndSubmitLogin('alice', 'secret');

    await screen.findByText('First sign-in: change your password');
    fillAndSubmitPasswordChange();

    await screen.findByText('Signed in. The workspace arrives in a later iteration.');
    screen.getByText('alice');
    const passwordCalls = fetchMock.mock.calls.filter(
      ([input]) => String(input) === '/api/v1/auth/password',
    );
    expect(passwordCalls).toHaveLength(1);
  });

  it('restores an authenticated session from GET /auth/me and renders the shell', async () => {
    const fetchMock = stubRoutes([
      {
        method: 'GET',
        path: '/api/v1/auth/me',
        status: 200,
        body: { user: user(false), csrf_token: 'tok-me' },
      },
    ]);
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderApp('/');

    await screen.findByText('Signed in. The workspace arrives in a later iteration.');
    screen.getByText('alice');
    expect(getCsrfToken()).toBe('tok-me');
  });

  it('clears the session and returns to /login on logout', async () => {
    const fetchMock = stubRoutes([
      {
        method: 'GET',
        path: '/api/v1/auth/me',
        status: 200,
        body: { user: user(false), csrf_token: 'tok-me' },
      },
      { method: 'POST', path: '/api/v1/auth/logout', status: 204 },
    ]);
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderApp('/');

    await screen.findByText('Signed in. The workspace arrives in a later iteration.');
    fireEvent.click(screen.getByText('alice'));
    const logoutItem = await screen.findByText('Log out');
    fireEvent.click(logoutItem);

    await screen.findByText('Sign in to Console');
    const logoutCalls = fetchMock.mock.calls.filter(
      ([input]) => String(input) === '/api/v1/auth/logout',
    );
    expect(logoutCalls).toHaveLength(1);
  });

  it('redirects an unauthenticated visit to / back to /login', async () => {
    const fetchMock = stubRoutes([
      { method: 'GET', path: '/api/v1/auth/me', status: 401, body: NO_SESSION },
    ]);
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderApp('/');

    await screen.findByText('Sign in to Console');
    expect(getCsrfToken()).toBeNull();
  });
});
