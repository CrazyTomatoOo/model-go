import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getCsrfToken, setCsrfToken } from '../../lib/auth';
import { LoginError, login } from './login';

describe('login', () => {
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

  it('POSTs /api/v1/auth/login with the JSON credentials body', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
      return new Response(
        JSON.stringify({
          user: {
            id: 1,
            username: 'alice',
            role: 'user',
            status: 'active',
            must_change_password: false,
          },
          csrf_token: 'tok-123',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      );
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const res = await login('alice', 'secret');

    expect(res.user.username).toBe('alice');
    expect(res.csrf_token).toBe('tok-123');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    expect(String(input)).toBe('/api/v1/auth/login');
    expect(init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(JSON.parse(String(init?.body))).toEqual({ username: 'alice', password: 'secret' });
  });

  it('stores the returned csrf_token in memory for later requests', async () => {
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          user: {
            id: 1,
            username: 'alice',
            role: 'user',
            status: 'active',
            must_change_password: true,
          },
          csrf_token: 'tok-abc',
        }),
        { status: 200 },
      )) as typeof fetch;

    await login('alice', 'secret');

    expect(getCsrfToken()).toBe('tok-abc');
  });

  it('attaches X-CSRF-Token on POST when a token is already in memory', async () => {
    setCsrfToken('tok-existing');
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(
          JSON.stringify({
            user: {
              id: 1,
              username: 'alice',
              role: 'user',
              status: 'active',
              must_change_password: false,
            },
            csrf_token: 'tok-abc',
          }),
          { status: 200 },
        ),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await login('alice', 'secret');

    const [, init] = fetchMock.mock.calls[0];
    expect((init?.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok-existing');
  });

  it.each([
    ['INVALID_CREDENTIALS', 401, '用户名或密码错误'],
    ['ACCOUNT_PENDING', 403, '账号待安全管理员审核启用'],
    ['ACCOUNT_DISABLED', 403, '账号已停用，请联系管理员'],
    ['ACCOUNT_LOCKED', 403, '登录失败次数过多，账号已锁定，请 15 分钟后再试'],
    ['RATE_LIMITED', 429, '操作过于频繁，请稍后再试'],
  ])('maps a %s response to a LoginError with that code', async (code, status, message) => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ code, message }), { status })) as typeof fetch;

    const err = await login('alice', 'secret').catch((e) => e);

    expect(err).toBeInstanceOf(LoginError);
    expect((err as LoginError).code).toBe(code);
    expect((err as LoginError).message).toBe(message);
  });

  it('maps a network failure to NETWORK_ERROR', async () => {
    globalThis.fetch = (async () => {
      throw new TypeError('fetch failed');
    }) as typeof fetch;

    const err = await login('alice', 'secret').catch((e) => e);

    expect(err).toBeInstanceOf(LoginError);
    expect((err as LoginError).code).toBe('NETWORK_ERROR');
  });

  it('maps a non-JSON error body to the NETWORK_ERROR fallback', async () => {
    globalThis.fetch = (async () => new Response('oops', { status: 500 })) as typeof fetch;

    const err = await login('alice', 'secret').catch((e) => e);

    expect(err).toBeInstanceOf(LoginError);
    expect((err as LoginError).code).toBe('NETWORK_ERROR');
  });
});
