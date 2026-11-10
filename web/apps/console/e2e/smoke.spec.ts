import { expect, signedInMe, test } from './fixtures';

// #33 smoke: the theme / global-style / title / guard-loading basics that the
// rest of the spec-32 effort builds on. Assertions target user-observable
// surfaces only: computed CSS variables, document.title, the loading state.

test('brand theme exposes the indigo color token as a CSS variable', async ({ page }) => {
  await page.goto('/login');
  const submit = page.getByRole('button', { name: '登 录' });

  // antd 5.29 scopes cssVar tokens under a themed wrapper class rather than
  // :root; custom properties inherit, so the computed value is read from a
  // themed component.
  const primaryVar = await submit.evaluate((el) =>
    getComputedStyle(el).getPropertyValue('--ant-color-primary'),
  );
  expect(primaryVar.trim().toLowerCase()).toBe('#2f54eb');

  // The theme must reach components, not just the token table — the primary
  // button is rendered in the brand indigo (#2F54EB → rgb(47, 84, 235)).
  const submitBg = await submit.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(submitBg).toBe('rgb(47, 84, 235)');
});

test('document.title follows the route using i18n copy', async ({ page, authMocks }) => {
  // Direct navigation to /login (zh-CN locale per playwright.config.ts).
  await page.goto('/login');
  await expect(page).toHaveTitle('登录控制台 · ModelGo 控制台');

  // Client-side route change: an authenticated visitor is bounced from /login
  // to the shell, and the title follows the new route.
  authMocks.me = signedInMe();
  await page.goto('/login');
  await expect(page).toHaveURL(/\/$/);
  await expect(page).toHaveTitle('控制台总览 · ModelGo 控制台');
});

test('guard bootstrap shows the themed loading state instead of a blank page', async ({
  page,
  authMocks,
}) => {
  // Hold /auth/me so the auth bootstrap stays pending long enough to observe
  // the guard's loading branch (deterministic stub latency, no test sleeps).
  authMocks.me = {
    status: 401,
    body: { code: 'SESSION_EXPIRED', message: 'no session' },
    delayMs: 1500,
  };
  await page.goto('/');

  // While the bootstrap request is pending, the guard renders the themed Spin.
  const spinner = page.locator('.ant-spin');
  await expect(spinner).toBeVisible();

  // Once /auth/me settles (unauthenticated), the guard redirects to /login
  // and the loading state is gone — no white flash at any point.
  await expect(page).toHaveURL(/\/login$/);
  await expect(spinner).toBeHidden();
  await expect(page.getByRole('button', { name: '登 录' })).toBeVisible();
});
