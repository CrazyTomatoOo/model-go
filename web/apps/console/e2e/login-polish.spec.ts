import { expect, signedInMe, test } from './fixtures';

// #34 T2 登录页产品化：提交加载态、错误 shake + 限流倒计时、403/404 页。
// 全部走网络层拦截的 auth stubs（见 fixtures.ts），无需真实 apiserver。

// The fixtures' authMocks default is one shared object per module instance,
// and this worker-sequential run lets the FIRST test of the next spec file
// execute on THIS file's registry. Mutations must not outlive their test —
// a leftover signed-in /auth/me override would render the shell instead of
// the login page in the next spec (observed against smoke.spec's first test).
test.afterEach(({ authMocks }) => {
  authMocks.me = undefined;
  authMocks.login = undefined;
});

test('submit button spins and disables while the login request is in flight', async ({
  page,
  authMocks,
}) => {
  // Hold the response so the in-flight window is observable without sleeps.
  authMocks.login = {
    status: 401,
    body: { code: 'INVALID_CREDENTIALS', message: 'incorrect username or password' },
    delayMs: 2000,
  };
  await page.goto('/login');
  await page.getByLabel('用户名').fill('sysadmin');
  await page.getByLabel('密码').fill('wrong-pass');

  const submit = page.getByRole('button', { name: '登 录' });
  await submit.click();

  // antd Button `loading` swaps in the LoadingOutlined spin for the whole
  // request window (antd swallows clicks while loading; it keeps the button
  // focusable, so the assertion targets the loading class, not `disabled`).
  await expect(submit).toHaveClass(/ant-btn-loading/);
  await expect(submit.locator('.ant-btn-loading-icon')).toBeVisible();

  // Once the stub answers, the copy keyed by the backend code appears.
  await expect(page.getByText('用户名或密码错误')).toBeVisible();
  await expect(submit).not.toHaveClass(/ant-btn-loading/);
  await expect(submit).toBeEnabled();
});

test('rate-limited login shakes the error and shows a retry countdown', async ({
  page,
  authMocks,
}) => {
  authMocks.login = { status: 429, body: { code: 'RATE_LIMITED', message: 'rate limited' } };
  await page.goto('/login');
  await page.getByLabel('用户名').fill('sysadmin');
  await page.getByLabel('密码').fill('pass');
  await page.getByRole('button', { name: '登 录' }).click();

  // The error surfaces in the shake-animated container (retriggered per error)…
  const alert = page.locator('.login-error-shake');
  await expect(alert).toBeVisible();
  await expect(alert).toContainText('操作过于频繁');

  // …carries a live countdown, and the submit button is disabled for it.
  await expect(page.getByText(/请 \d+ 秒后重试/)).toBeVisible();
  await expect(page.getByRole('button', { name: '登 录' })).toBeDisabled();
});

test('403 page explains the denial and auto-redirects home after 3s', async ({
  page,
  authMocks,
}) => {
  // Signed-in state (the harness equivalent of a store override): the
  // auto-redirect lands on the shell, not on the login bounce.
  authMocks.me = signedInMe();
  await page.goto('/403');
  await expect(page.getByText('无访问权限')).toBeVisible();
  await expect(page).toHaveTitle('无访问权限 · ModelGo 控制台');

  // toHaveURL polls: the redirect fires ~3s after mount, no test sleeps.
  await expect(page).toHaveURL(/\/$/, { timeout: 10_000 });
  await expect(page.getByText('sysadmin')).toBeVisible();
});

test('unknown routes render the 404 page with i18n title', async ({ page }) => {
  await page.goto('/definitely-not-a-console-route');
  // exact: the subTitle ('您访问的页面不存在或已被移除。') also contains the title text.
  await expect(page.getByText('页面不存在', { exact: true })).toBeVisible();
  await expect(page).toHaveTitle('页面不存在 · ModelGo 控制台');
});
