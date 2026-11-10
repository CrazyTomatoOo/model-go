import { expect, signedInMe, test } from './fixtures';

// #37 T5 控制台 Overview：GPU 状态卡、资源水位卡、快捷入口卡在登录态下
// 渲染（后端尚无 overview API，卡片呈现设计过的空态），语言切换即时刷新
// 卡片文案与 document.title（无需导航）。断言只打用户可观察表面。

test.beforeEach(async ({ authMocks }) => {
  authMocks.me = signedInMe();
});

test.afterEach(({ authMocks }) => {
  authMocks.me = undefined;
});

test('overview cards render behind auth with designed empty states', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');

  await expect(page).toHaveTitle('控制台总览 · ModelGo 控制台');

  // GPU 使用卡片：标题 + 空态（暂无加速卡接入）。
  const gpuCard = page.locator('.ant-card', { hasText: 'GPU 加速卡' });
  await expect(gpuCard.getByText('GPU 加速卡', { exact: true })).toBeVisible();
  await expect(gpuCard.getByText('暂未接入 GPU 资源')).toBeVisible();

  // 资源水位卡片：模型 / 数据集 / 训练任务 + 空态。
  const statsCard = page.locator('.ant-card', { hasText: '资源水位' });
  await expect(statsCard.getByText('资源水位', { exact: true })).toBeVisible();
  await expect(statsCard.getByText('模型', { exact: true })).toBeVisible();
  await expect(statsCard.getByText('数据集', { exact: true })).toBeVisible();
  await expect(statsCard.getByText('训练任务', { exact: true })).toBeVisible();
  await expect(statsCard.getByText('暂无统计数据')).toBeVisible();

  // 快捷入口卡片：真实路由可点，未来入口禁用并标注即将上线。
  const shortcutsCard = page.locator('.ant-card', { hasText: '快捷入口' });
  await expect(shortcutsCard.getByRole('button', { name: '修改密码' })).toBeVisible();
  await expect(shortcutsCard.getByRole('button', { name: '训练任务' })).toBeDisabled();
  // One "coming soon" tag per upcoming section (training jobs, datasets, models).
  await expect(shortcutsCard.getByText('即将上线')).toHaveCount(3);
});

test('language toggle switches card copy without navigation', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');

  const languageSwitch = page.getByRole('radiogroup', { name: '语言' });
  await languageSwitch.getByText('EN', { exact: true }).click();

  // Copy flips to English in place — same URL, no reload/navigation.
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByText('GPU Accelerators', { exact: true })).toBeVisible();
  await expect(page.getByText('No GPU resources connected')).toBeVisible();
  await expect(page.getByText('Resource Levels', { exact: true })).toBeVisible();
  await expect(page.getByText('Quick Actions', { exact: true })).toBeVisible();
  await expect(page.getByText('Coming soon')).toHaveCount(3);
});

test('document.title refreshes when the language toggles, without navigation', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await expect(page).toHaveTitle('控制台总览 · ModelGo 控制台');

  await page.getByRole('radiogroup', { name: '语言' }).getByText('EN', { exact: true }).click();
  await expect(page).toHaveTitle('Console Overview · ModelGo Console');
  await expect(page).toHaveURL(/\/$/);

  // …and back to Chinese. The header switch's aria-label follows the active
  // language, so the second toggle locates the Segmented by its stable class.
  await page.locator('.shell-header-actions .ant-segmented').getByText('中', { exact: true }).click();
  await expect(page).toHaveTitle('控制台总览 · ModelGo 控制台');
});

test('overview cards stack cleanly at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  // The mobile tier (T4) swaps the sider for a drawer; the overview content
  // below the header must still render its three cards in a single column.
  await expect(page.getByText('GPU 加速卡', { exact: true })).toBeVisible();
  await expect(page.getByText('资源水位', { exact: true })).toBeVisible();
  await expect(page.getByText('快捷入口', { exact: true })).toBeVisible();

  const cards = page.locator('main .ant-card');
  await expect(cards).toHaveCount(3);
  const firstBox = await cards.nth(0).boundingBox();
  const secondBox = await cards.nth(1).boundingBox();
  // Stacked, not side by side: the second card starts below the first.
  expect(secondBox?.y).toBeGreaterThanOrEqual((firstBox?.y ?? 0) + (firstBox?.height ?? 0));
});
