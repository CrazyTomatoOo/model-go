import { afterEach, beforeEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { setCsrfToken } from '../lib/auth';
import '../i18n';
import LoginPage from './LoginPage';

function renderLoginPage() {
  render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/change-password" element={<div>CHANGE_PASSWORD_LANDING</div>} />
        <Route path="/" element={<div>ROOT_LANDING</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

function fillAndSubmit(username: string, password: string) {
  fireEvent.change(screen.getByLabelText('Username'), { target: { value: username } });
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } });
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));
}

function okBody(mustChangePassword: boolean): string {
  return JSON.stringify({
    user: {
      id: 1,
      username: 'alice',
      role: 'user',
      status: 'active',
      must_change_password: mustChangePassword,
    },
    csrf_token: 'tok-abc',
  });
}

describe('LoginPage', () => {
  const originalFetch = globalThis.fetch;

  beforeAll(() => {
    // antd's responsive observer requires matchMedia, which jsdom lacks.
    window.matchMedia = ((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;
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

  it('renders the username and password fields with a submit button', async () => {
    renderLoginPage();

    await screen.findByText('Sign in to Console');
    screen.getByLabelText('Username');
    screen.getByLabelText('Password');
    screen.getByRole('button', { name: 'Sign in' });
  });

  it('validates empty fields locally without calling the API', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      throw new Error('unexpected fetch call');
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderLoginPage();

    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText('Username is required')).toBeTruthy();
    expect(await screen.findByText('Password is required')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('POSTs /api/v1/auth/login with the JSON credentials on valid submit', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(okBody(false), { status: 200 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderLoginPage();

    fillAndSubmit('alice', 'secret');

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [input, init] = fetchMock.mock.calls[0];
    expect(String(input)).toBe('/api/v1/auth/login');
    expect(init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(JSON.parse(String(init?.body))).toEqual({ username: 'alice', password: 'secret' });
  });

  it('redirects to /change-password when must_change_password is true', async () => {
    globalThis.fetch = (async () => new Response(okBody(true), { status: 200 })) as typeof fetch;
    renderLoginPage();

    fillAndSubmit('alice', 'secret');

    await screen.findByText('CHANGE_PASSWORD_LANDING');
  });

  it('redirects to / when must_change_password is false', async () => {
    globalThis.fetch = (async () => new Response(okBody(false), { status: 200 })) as typeof fetch;
    renderLoginPage();

    fillAndSubmit('alice', 'secret');

    await screen.findByText('ROOT_LANDING');
  });

  it.each([
    ['INVALID_CREDENTIALS', 401, 'Incorrect username or password'],
    ['ACCOUNT_PENDING', 403, 'Account is pending approval by a security administrator'],
    ['ACCOUNT_DISABLED', 403, 'Account is disabled, contact the administrator'],
    ['ACCOUNT_LOCKED', 403, 'Too many failed login attempts, account locked for 15 minutes'],
    ['RATE_LIMITED', 429, 'Too many requests, please try again later'],
  ])('shows the mapped copy for the %s error', async (code, status, copy) => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ code, message: 'server message' }), { status })) as typeof fetch;
    renderLoginPage();

    fillAndSubmit('alice', 'secret');

    await screen.findByText(copy);
  });

  it('shows the network error copy when the request cannot reach the server', async () => {
    globalThis.fetch = (async () => {
      throw new TypeError('fetch failed');
    }) as typeof fetch;
    renderLoginPage();

    fillAndSubmit('alice', 'secret');

    await screen.findByText('Network error, please try again later');
  });
});
