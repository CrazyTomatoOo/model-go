import { test as base, expect } from '@playwright/test';
import type { Page, Route } from '@playwright/test';

/**
 * Shared E2E harness for the console (#33, spec #32 Testing Decisions):
 * every test runs against the production build served by `vite preview`
 * (see webServer in playwright.config.ts) and stubs the auth API at the
 * network layer, so no real apiserver is involved.
 */

/** Response template for one /api/v1/auth/* endpoint. */
export type AuthReply = {
  status?: number;
  body?: unknown;
  /** Hold the response this long before answering (loading-state tests). */
  delayMs?: number;
};

/**
 * Per-endpoint overrides. Endpoints left unmocked fall back to
 * contract-faithful defaults (see DEFAULT_REPLIES below).
 */
export type AuthMocks = Partial<Record<'me' | 'login' | 'password' | 'logout', AuthReply>>;

/**
 * A signed-in /auth/me response for an active user (no forced password
 * change). Pass displayName to exercise the shell avatar's display_name
 * derivation (#36); the current backend contract omits the field.
 */
export function signedInMe(username = 'sysadmin', displayName?: string): AuthReply {
  return {
    status: 200,
    body: {
      user: {
        id: 1,
        username,
        ...(displayName !== undefined ? { display_name: displayName } : {}),
        role: 'sysadmin',
        status: 'active',
        must_change_password: false,
      },
      csrf_token: 'e2e-csrf-token',
    },
  };
}

const DEFAULT_REPLIES: Record<string, AuthReply> = {
  me: { status: 401, body: { code: 'SESSION_EXPIRED', message: 'no session' } },
  login: {
    status: 401,
    body: { code: 'INVALID_CREDENTIALS', message: 'incorrect username or password' },
  },
  password: {
    status: 400,
    body: {
      code: 'VALIDATION_ERROR',
      message: 'validation failed',
      errors: [{ field: 'new_password', message: 'does not satisfy the password policy' }],
    },
  },
  logout: { status: 204 },
};

async function fulfillReply(route: Route, reply: AuthReply): Promise<void> {
  if (reply.delayMs !== undefined) {
    await new Promise((resolve) => setTimeout(resolve, reply.delayMs));
  }
  const status = reply.status ?? 200;
  if (reply.body === undefined) {
    await route.fulfill({ status, body: '' });
    return;
  }
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(reply.body),
  });
}

/**
 * Intercept every /api/v1/auth/* request and answer from `mocks`. The mocks
 * object is read at request time, so tests may reconfigure it mid-flight
 * (e.g. sign in after exercising the logged-out state).
 */
export async function mockAuthApi(page: Page, mocks: AuthMocks): Promise<void> {
  await page.route('**/api/v1/auth/**', (route) => {
    const endpoint = new URL(route.request().url()).pathname.replace('/api/v1/auth/', '');
    const reply =
      mocks[endpoint as keyof AuthMocks] ??
      DEFAULT_REPLIES[endpoint] ?? {
        status: 404,
        body: { code: 'NOT_FOUND', message: `unmocked auth endpoint: ${endpoint}` },
      };
    return fulfillReply(route, reply);
  });
}

type ConsoleFixtures = {
  /** Per-test overrides for the auth API stubs (see mockAuthApi). */
  authMocks: AuthMocks;
};

export const test = base.extend<ConsoleFixtures>({
  // A factory (not the shared `{}` literal) so every test gets its own mocks
  // object — a constant default would leak per-test overrides across specs
  // running on the same worker. Playwright requires the object-destructuring
  // first argument (plain `({})` trips no-empty-pattern), so we destructure
  // the context fixture and void it to keep both tools happy.
  authMocks: [
    async ({ context: _context }, use) => {
      void _context;
      await use({});
    },
    { option: true },
  ],
  // The network layer is the API boundary: every test gets the auth stubs.
  page: async ({ page, authMocks }, use) => {
    await mockAuthApi(page, authMocks);
    await use(page);
  },
});

export { expect };
