import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setCsrfToken } from '../../lib/auth';
import { ChangePasswordError, changePassword } from './changePassword';

describe('changePassword', () => {
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

  it('POSTs {old_password,new_password} to /api/v1/auth/password with X-CSRF-Token', async () => {
    setCsrfToken('tok-csrf');
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(null, { status: 204 }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await changePassword({ oldPassword: 'old-pass-123', newPassword: 'new-pass-12345678' });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    expect(String(input)).toBe('/api/v1/auth/password');
    expect(init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'tok-csrf' },
    });
    expect(JSON.parse(String(init?.body))).toEqual({
      old_password: 'old-pass-123',
      new_password: 'new-pass-12345678',
    });
  });

  it('maps a 400 field error on old_password to invalidInitialPassword', async () => {
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          code: 'VALIDATION_ERROR',
          message: '请求参数校验失败',
          errors: [{ field: 'old_password', message: '旧密码不正确' }],
        }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      )) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'wrong',
      newPassword: 'new-pass-12345678',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('invalidInitialPassword');
  });

  it('maps a 400 field error on new_password to policyViolation (policy or reuse)', async () => {
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          code: 'VALIDATION_ERROR',
          message: '请求参数校验失败',
          errors: [{ field: 'new_password', message: '新密码不得与当前密码或最近 5 次历史密码相同' }],
        }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      )) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'old-pass-123',
      newPassword: 'reused-pass-123',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('policyViolation');
  });

  it('maps a 429 RATE_LIMITED to rateLimited carrying the Retry-After seconds', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ code: 'RATE_LIMITED', message: '操作过于频繁' }), {
        status: 429,
        headers: { 'Content-Type': 'application/json', 'Retry-After': '7' },
      })) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'old-pass-123',
      newPassword: 'new-pass-12345678',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('rateLimited');
    expect((err as ChangePasswordError).retryAfterSeconds).toBe(7);
  });

  it('maps a 429 without Retry-After to rateLimited with a null countdown', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ code: 'RATE_LIMITED', message: '操作过于频繁' }), {
        status: 429,
        headers: { 'Content-Type': 'application/json' },
      })) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'old-pass-123',
      newPassword: 'new-pass-12345678',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('rateLimited');
    expect((err as ChangePasswordError).retryAfterSeconds).toBeNull();
  });

  it('maps a non-validation API failure to unknown', async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ code: 'INTERNAL', message: '服务器内部错误' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      })) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'old-pass-123',
      newPassword: 'new-pass-12345678',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('unknown');
  });

  it('maps a transport failure to network', async () => {
    globalThis.fetch = (async () => {
      throw new TypeError('fetch failed');
    }) as typeof fetch;

    const err = await changePassword({
      oldPassword: 'old-pass-123',
      newPassword: 'new-pass-12345678',
    }).catch((e) => e);

    expect(err).toBeInstanceOf(ChangePasswordError);
    expect((err as ChangePasswordError).code).toBe('network');
  });
});
