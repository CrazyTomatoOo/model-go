import { expect, signedInMe, test } from './fixtures';

// #36 T4 外壳品牌对齐：shared chrome (brand Header + side menu + user menu),
// responsive tiers (>=1200 full / 768-1199 icon-collapsed / <768 drawer), and
// the logout confirm flow. Assertions hit user-observable surfaces only:
// computed colors, landmarks, roles, URLs, and network effects.

/** Brand indigo #2F54EB as a computed rgb() value. */
const BRINDIGO = 'rgb(47, 84, 235)';

test.beforeEach(async ({ authMocks }) => {
  authMocks.me = signedInMe();
});

test('header and side menu use the brand indigo surface at full width', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');

  const header = page.locator('.ant-layout-header');
  await expect(header).toBeVisible();
  const headerBg = await header.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(headerBg).toBe(BRINDIGO);

  // Full side menu with the overview entry (zh locale per playwright.config).
  const sider = page.locator('.ant-layout-sider');
  await expect(sider).toBeVisible();
  await expect(sider).not.toHaveClass(/ant-layout-sider-collapsed/);
  const siderBg = await sider.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(siderBg).toBe(BRINDIGO);
  await expect(page.getByRole('menuitem', { name: '控制台' })).toBeVisible();

  // The content area is a <main> landmark (shell chrome owns the landmarks).
  await expect(page.locator('main')).toBeVisible();
});

test('side menu collapses to icons at a 900px viewport', async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 800 });
  await page.goto('/');

  const sider = page.locator('.ant-layout-sider');
  await expect(sider).toBeVisible();
  await expect(sider).toHaveClass(/ant-layout-sider-collapsed/);
  const box = await sider.boundingBox();
  expect(box?.width).toBe(80);

  // The overview entry is still reachable (icon affordance) in collapsed mode.
  await expect(page.getByRole('menuitem', { name: '控制台' })).toBeVisible();
});

test('a 390px viewport swaps the side menu for a drawer with header actions inside', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  // No inline sider; the hamburger toggle is the navigation entry point.
  await expect(page.locator('.ant-layout-sider')).toHaveCount(0);
  const hamburger = page.getByRole('button', { name: '打开导航菜单' });
  await expect(hamburger).toBeVisible();

  // Header actions (user menu) moved into the drawer, not the header.
  await expect(page.locator('.ant-layout-header .ant-avatar')).toHaveCount(0);

  await hamburger.click();
  const drawer = page.locator('.ant-drawer-open');
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole('menuitem', { name: '控制台' })).toBeVisible();
  await expect(drawer.locator('.ant-avatar')).toBeVisible();

  // Navigating from the drawer closes it again.
  await drawer.getByRole('menuitem', { name: '控制台' }).click();
  await expect(page.locator('.ant-drawer-open')).toHaveCount(0);
  await expect(page).toHaveURL(/\/$/);
});

test('user menu shows avatar initials derived from the /auth/me display_name', async ({
  page,
  authMocks,
}) => {
  // display_name wins over username for the initials derivation; the current
  // backend contract has no display_name, so the fallback is unit-tested.
  authMocks.me = signedInMe('sysadmin', 'Zhang San');
  await page.goto('/');

  const avatar = page.locator('.ant-layout-header .ant-avatar');
  await expect(avatar).toHaveText('ZS');

  // The dropdown carries the T3 change-password entry point and the logout.
  await avatar.click();
  const dropdownMenu = page.locator('.ant-dropdown-menu');
  await expect(dropdownMenu.getByRole('menuitem', { name: '修改密码' })).toBeVisible();
  await expect(dropdownMenu.getByRole('menuitem', { name: '退出登录' })).toBeVisible();
});

test('logout asks for confirmation, calls the API, and returns to /login', async ({ page }) => {
  await page.goto('/');

  // Count the real logout request at the network layer (overrides the stub).
  let logoutCalls = 0;
  await page.route('**/api/v1/auth/logout', (route) => {
    logoutCalls += 1;
    return route.fulfill({ status: 204, body: '' });
  });

  const avatar = page.locator('.ant-layout-header .ant-avatar');
  await avatar.click();
  await page.getByRole('menuitem', { name: '退出登录' }).click();

  // Confirm dialog appears; cancelling keeps the session.
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('确定要退出当前账号吗？');
  await page.getByRole('button', { name: '取 消' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page).toHaveURL(/\/$/);
  expect(logoutCalls).toBe(0);

  // Confirming signs out: the API is called and the app returns to /login.
  await avatar.click();
  await page.getByRole('menuitem', { name: '退出登录' }).click();
  await page.getByRole('button', { name: '确认退出' }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(logoutCalls).toBe(1);
});
