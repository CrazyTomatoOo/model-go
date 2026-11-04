import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { applyAuthToken, getCsrfToken, setCsrfToken } from './auth';
import { ApiError, apiFetch } from './request';

type LoginResponse = { token: string };

describe('apiFetch', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    setCsrfToken(null);
    globalThis.fetch = (async () => {
      throw new Error('unexpected fetch call');
    }) as typeof fetch;
  });

  afterEach(() => {
    setCsrfToken(null);
    globalThis.fetch = originalFetch;
  });

  it('sends the request and parses the JSON response', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      return new Response(JSON.stringify({ token: 't' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const res = await apiFetch<LoginResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: { username: 'alice', password: 'secret' },
    });

    expect(res).toEqual({ token: 't' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    expect(String(input)).toBe('/api/v1/auth/login');
    expect(init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(JSON.parse(String(init?.body))).toEqual({ username: 'alice', password: 'secret' });
  });

  it('omits Content-Type and body for a GET request', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response('{}', { status: 200 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await apiFetch('/api/v1/auth/me');

    const [, init] = fetchMock.mock.calls[0];
    expect(init?.headers).toEqual({});
    expect(init?.body).toBeUndefined();
  });

  it('throws ApiError with status and server message on 4xx/5xx', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ message: 'bad credentials' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })) as typeof fetch;

    const error = await apiFetch('/api/v1/auth/login', {
      method: 'POST',
      body: {},
    }).catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(401);
    expect((error as ApiError).message).toBe('bad credentials');
  });

  it('throws ApiError with a fallback message when the body is not JSON', async () => {
    globalThis.fetch = (async () => new Response('oops', { status: 500 })) as typeof fetch;

    const error = await apiFetch('/api/v1/auth/login').catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(500);
    expect((error as ApiError).message).toBe('Request failed: 500');
  });

  it('sends X-CSRF-Token on state-changing requests when csrfToken is passed', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response('{}', { status: 200 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await apiFetch('/api/v1/auth/logout', { method: 'POST', csrfToken: 'tok-123' });

    const [, init] = fetchMock.mock.calls[0];
    expect((init?.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok-123');
  });

  it('never sends X-CSRF-Token on GET requests', async () => {
    setCsrfToken('tok-get');
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response('{}', { status: 200 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await apiFetch('/api/v1/auth/me');

    const [, init] = fetchMock.mock.calls[0];
    expect('X-CSRF-Token' in (init?.headers as Record<string, string>)).toBe(false);
  });

  it('uses the stored token from applyAuthToken for subsequent requests', async () => {
    applyAuthToken({ user: { id: 1 }, csrf_token: 'tok-abc' });
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response('{}', { status: 200 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await apiFetch('/api/v1/auth/logout', { method: 'POST' });

    const [, init] = fetchMock.mock.calls[0];
    expect((init?.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok-abc');
    expect(getCsrfToken()).toBe('tok-abc');
  });
});
