import type { Page } from '@playwright/test';
import { expect, signedInMe, test } from './fixtures';

// #35 T3: the voluntary change-password modal from the shell header. Runs
// against the production build with the auth API stubbed at the network
// layer (e2e/fixtures.ts); zh-CN is the configured locale unless a test
// overrides it.

const CURRENT = 'old-pass-123';
const NEXT = 'new-pass-12345678';

/** Desktop/tablet entry (#36 chrome): header avatar → user menu entry. */
async function openChangePasswordModal(page: Page): Promise<void> {
  await page.locator('.ant-layout-header .ant-avatar').click();
  // Scoped to the dropdown overlay: the side menu renders its own 修改密码
  // menuitem next to it.
  await page.locator('.ant-dropdown-menu').getByRole('menuitem', { name: '修改密码' }).click();
}

async function fillAndSubmit(page: Page): Promise<void> {
  const dialog = page.getByRole('dialog', { name: '修改密码' });
  await dialog.getByLabel('当前密码', { exact: true }).fill(CURRENT);
  await dialog.getByLabel('新密码', { exact: true }).fill(NEXT);
  await dialog.getByLabel('确认新密码', { exact: true }).fill(NEXT);
  await dialog.getByRole('button', { name: '确认修改' }).click();
}

test('opens from the header user menu with labelled fields and closes on Esc', async ({
  page,
  authMocks,
}) => {
  authMocks.me = signedInMe();
  await page.goto('/');

  await openChangePasswordModal(page);
  const dialog = page.getByRole('dialog', { name: '修改密码' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel('当前密码', { exact: true })).toBeVisible();
  await expect(dialog.getByLabel('新密码', { exact: true })).toBeVisible();
  await expect(dialog.getByLabel('确认新密码', { exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: '确认修改' })).toBeVisible();
  // antd auto-inserts a space into two-character Chinese button copy.
  await expect(dialog.getByRole('button', { name: '取 消' })).toBeVisible();

  // a11y: opening the dialog moves focus into it (not onto the mask), and
  // Esc close is antd Modal default behavior; reopening must work too.
  await expect(dialog.getByLabel('当前密码', { exact: true })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await openChangePasswordModal(page);
  await expect(dialog).toBeVisible();
});

test('client-side validation blocks the submit without any API call', async ({
  page,
  authMocks,
}) => {
  authMocks.me = signedInMe();
  let passwordRequests = 0;
  page.on('request', (req) => {
    if (req.url().includes('/api/v1/auth/password')) {
      passwordRequests += 1;
    }
  });
  await page.goto('/');
  await openChangePasswordModal(page);
  const dialog = page.getByRole('dialog', { name: '修改密码' });

  // Empty submit: the required rule fires per field.
  await dialog.getByRole('button', { name: '确认修改' }).click();
  await expect(dialog.getByText('请填写此项').first()).toBeVisible();
  expect(passwordRequests).toBe(0);

  // Mismatched confirmation.
  await dialog.getByLabel('当前密码', { exact: true }).fill(CURRENT);
  await dialog.getByLabel('新密码', { exact: true }).fill(NEXT);
  await dialog.getByLabel('确认新密码', { exact: true }).fill('other-pass-123456');
  await dialog.getByRole('button', { name: '确认修改' }).click();
  await expect(dialog.getByText('两次输入的新密码不一致')).toBeVisible();
  expect(passwordRequests).toBe(0);

  // Policy-violating new password (client-side mirror of spec §6).
  await dialog.getByLabel('确认新密码', { exact: true }).fill('short');
  await dialog.getByLabel('新密码', { exact: true }).fill('short');
  await dialog.getByRole('button', { name: '确认修改' }).click();
  await expect(dialog.getByText('新密码不符合密码策略')).toBeVisible();
  expect(passwordRequests).toBe(0);
});

test('submit sends X-CSRF-Token and the JSON body; success auto-closes and stays signed in', async ({
  page,
  authMocks,
}) => {
  authMocks.me = signedInMe();
  authMocks.password = { status: 204 };
  const seen: { method?: string; csrf?: Promise<string | null>; body?: unknown } = {};
  page.on('request', (req) => {
    if (req.url().endsWith('/api/v1/auth/password')) {
      seen.method = req.method();
      seen.csrf = req.headerValue('X-CSRF-Token');
      seen.body = req.postDataJSON();
    }
  });
  await page.goto('/');
  await openChangePasswordModal(page);
  await fillAndSubmit(page);

  const dialog = page.getByRole('dialog', { name: '修改密码' });
  await expect(dialog.getByText('密码修改成功')).toBeVisible();
  // Success dwells briefly, then the modal auto-closes by itself.
  await expect(dialog).toBeHidden();

  expect(seen.method).toBe('POST');
  expect(await seen.csrf).toBe('e2e-csrf-token');
  expect(seen.body).toEqual({ old_password: CURRENT, new_password: NEXT });

  // Stay logged in on the shell, no redirect to /login.
  await expect(page).toHaveURL(/\/$/);
  await expect(page.locator('.ant-layout-header .ant-avatar')).toBeVisible();
});

test('server field errors map to the current and new password fields', async ({
  page,
  authMocks,
}) => {
  authMocks.me = signedInMe();
  authMocks.password = {
    status: 400,
    body: {
      code: 'VALIDATION_ERROR',
      message: '请求参数校验失败',
      errors: [{ field: 'old_password', message: '旧密码不正确' }],
    },
  };
  await page.goto('/');
  await openChangePasswordModal(page);
  await fillAndSubmit(page);

  const dialog = page.getByRole('dialog', { name: '修改密码' });
  await expect(dialog.getByText('当前密码错误')).toBeVisible();

  // Reuse lands on the new-password field per the §4.5 field contract;
  // mocks are read at request time, so reconfiguring mid-test is enough.
  authMocks.password = {
    status: 400,
    body: {
      code: 'VALIDATION_ERROR',
      message: '请求参数校验失败',
      errors: [{ field: 'new_password', message: '新密码不得与当前密码或最近 5 次历史密码相同' }],
    },
  };
  await dialog.getByRole('button', { name: '确认修改' }).click();
  await expect(dialog.getByText('新密码不符合密码策略或与近期历史密码重复')).toBeVisible();
});

test('429 RATE_LIMITED shows a countdown, disables submit, then recovers', async ({
  page,
  authMocks,
}) => {
  authMocks.me = signedInMe();
  authMocks.password = {
    status: 429,
    body: { code: 'RATE_LIMITED', message: '操作过于频繁' },
    headers: { 'Retry-After': '2' },
  };
  await page.goto('/');
  await openChangePasswordModal(page);
  await fillAndSubmit(page);

  const dialog = page.getByRole('dialog', { name: '修改密码' });
  const submit = dialog.getByRole('button', { name: '确认修改' });
  await expect(dialog.getByText(/尝试过于频繁/)).toBeVisible();
  await expect(submit).toBeDisabled();

  // The countdown expires on its own (Retry-After: 2) and the form recovers
  // without a page reload; a retry then goes through.
  await expect(submit).toBeEnabled({ timeout: 10_000 });
  authMocks.password = { status: 204 };
  await submit.click();
  await expect(dialog.getByText('密码修改成功')).toBeVisible();
});

test('modal fits a narrow viewport without horizontal scrolling', async ({ page, authMocks }) => {
  authMocks.me = signedInMe();
  await page.setViewportSize({ width: 360, height: 640 });
  await page.goto('/');

  // Mobile tier (#36): the user menu lives in the drawer; the drawer menu's
  // 修改密码 entry fires the same change-password bus as the header dropdown.
  await page.getByRole('button', { name: '打开导航菜单' }).click();
  const drawer = page.locator('.ant-drawer-open');
  await expect(drawer).toBeVisible();
  await drawer.getByRole('menuitem', { name: '修改密码' }).click();

  const dialog = page.getByRole('dialog', { name: '修改密码' });
  await expect(dialog).toBeVisible();
  // The dialog itself must stay inside the viewport. (The shell chrome owns
  // the rest of the narrow-width layout — T4 territory, not this modal.)
  const box = await dialog.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(360);
});

test.describe('en-US locale', () => {
  test.use({ locale: 'en-US' });

  test('renders the modal in English', async ({ page, authMocks }) => {
    authMocks.me = signedInMe();
    await page.goto('/');
    await page.locator('.ant-layout-header .ant-avatar').click();
    await page
      .locator('.ant-dropdown-menu')
      .getByRole('menuitem', { name: 'Change Password' })
      .click();

    const dialog = page.getByRole('dialog', { name: 'Change Password' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel('Current password', { exact: true })).toBeVisible();
    await expect(dialog.getByLabel('New password', { exact: true })).toBeVisible();
    await expect(dialog.getByLabel('Confirm new password', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Confirm Change' })).toBeVisible();
  });
});
