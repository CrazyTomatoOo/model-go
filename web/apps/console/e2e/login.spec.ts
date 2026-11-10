import { expect, test } from '@playwright/test';

// 真浏览器回归（issue #22 ⑥）：登录→强制改密→外壳→登出。
// 说明：这条回归驱动真实 apiserver（经 vite dev proxy）。spec-32 的 E2E harness
// 改为 vite preview 起构建产物并在网络层拦 /api/v1/auth/*，preview 没有 /api
// 代理，因此在 CI job 改造回可直连后端之前保持跳过（见 #32 Testing Decisions）。
test.skip(
  !process.env.E2E_REAL_APISERVER,
  'requires a real apiserver; the vite-preview harness stubs /api at the network layer',
);
const username = process.env.E2E_USERNAME ?? 'sysadmin';
const oldPassword = process.env.E2E_OLD_PASSWORD ?? 'Bootstrap@Passw0rd';
const newPassword = process.env.E2E_NEW_PASSWORD ?? 'E2E@Strong#2026';

test('logs in, changes the forced password, reaches the shell, and logs out', async ({
  page,
}) => {
  await page.goto('/login');
  await expect(page.getByRole('heading', { name: '登录控制台' })).toBeVisible();
  await page.screenshot({ path: 'e2e-artifacts/01-login-zh.png' });

  // 登录：初始密码 → 强制改密
  await page.getByLabel('用户名').fill(username);
  await page.getByLabel('密码').fill(oldPassword);
  await page.getByRole('button', { name: '登 录' }).click();
  await expect(page).toHaveURL(/\/change-password/);
  await page.screenshot({ path: 'e2e-artifacts/02-change-password.png' });

  // 改密：策略 + 两次一致
  await page.getByLabel('初始密码').fill(oldPassword);
  await page.getByLabel('新密码', { exact: true }).fill(newPassword);
  await page.getByLabel('确认新密码').fill(newPassword);
  await page.getByRole('button', { name: '确认修改' }).click();
  await expect(page).toHaveURL(/\//);
  await page.screenshot({ path: 'e2e-artifacts/03-shell.png' });

  // 外壳占位 + 用户菜单
  await expect(page.getByText(username)).toBeVisible();
  await page.getByRole('button', { name: username }).click();
  await page.getByRole('menuitem', { name: /退出登录/ }).click();

  // 登出后回登录页
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole('heading', { name: '登录控制台' })).toBeVisible();
  await page.screenshot({ path: 'e2e-artifacts/04-logged-out.png' });
});
