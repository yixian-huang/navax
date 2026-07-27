import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';
import { USER } from './accounts';

// 独立 fixture（非 user.spec.ts 用的 theme-lilac.zip）：目录 slug 全局唯一，
// 本测试会把它批准进官方目录并永久留在共享的服务端数据里（本套件单 worker、
// 不并行、不重置数据目录，spec 文件按顺序依次执行）；若复用同一个 slug，
// user.spec.ts 的导入测试之后再对同 slug 的私有主题发起目录申请会因
// ErrSlugConflict 直接 422。用独立主题名/slug 隔离，两边互不影响。
const THEME_ZIP = fileURLToPath(new URL('../fixtures/theme-mauve.zip', import.meta.url));

// 管理员关键路径：运营概览、用户管理、平台主题库（系统配置入口）。
test.describe('管理员', () => {
  test.use({ storageState: '.auth/admin.json' });

  test('运营概览展示统计', async ({ page }) => {
    await page.goto('/admin');
    await expect(page.getByRole('heading', { name: '运营概览' })).toBeVisible();
    await expect(page.getByText('总用户数')).toBeVisible();
  });

  test('用户列表包含受邀注册的用户', async ({ page }) => {
    await page.goto('/admin/users');
    await expect(page.getByText(USER.username).first()).toBeVisible();
  });

  test('系统配置可进入平台主题库', async ({ page }) => {
    await page.goto('/admin/settings');
    await page.getByRole('link', { name: '打开平台主题库' }).click();
    await expect(page.getByRole('heading', { name: '平台主题库' })).toBeVisible();
    await expect(page.getByText('Slate Dark').first()).toBeVisible();
  });

  test('管理员停用并恢复主题版本', async ({ page }) => {
    await page.goto('/admin/themes');
    // 官方目录分组存在。
    await expect(page.getByRole('heading', { name: '官方目录' })).toBeVisible();
    // sakura 非默认，可停用。用它专属的"设为默认主题"按钮（aria-label 含主题名，
    // 每张卡片唯一）锚定卡片容器，再在该容器内找停用/启用按钮——避免撞到目录里
    // 无版本的历史遗留主题（停用会因守卫 409）。
    const sakuraCard = page.getByRole('button', { name: '设为默认主题 Sakura' }).locator('xpath=../..');
    await sakuraCard.getByRole('button', { name: '停用版本' }).click();
    await expect(sakuraCard.getByText('已停用版本')).toBeVisible({ timeout: 10000 });
    // 恢复。
    await sakuraCard.getByRole('button', { name: '启用版本' }).click();
    await expect(sakuraCard.getByText('已停用版本')).toHaveCount(0, { timeout: 10000 });
  });

  test('管理员查看主题历史版本', async ({ page }) => {
    await page.goto('/admin/themes');
    // 同上范式：用"设为默认主题 Sakura" aria-label 锚定卡片容器，避免撞到目录里
    // 无版本的历史遗留主题（如 cyber）。「查看版本」按钮可见文案是"查看版本"，但
    // 设了 aria-label（展开/收起主题 {name} 版本列表），无障碍名以 aria-label 为准。
    const sakuraCard = page.getByRole('button', { name: '设为默认主题 Sakura' }).locator('xpath=../..');
    await sakuraCard.getByRole('button', { name: /展开主题 Sakura 版本列表/ }).click();
    // sakura 种子只有一个版本且为当前版本：面板展开后应看到该版本行，
    // 「当前」列渲染的文案是"是"（不是"当前"）。
    await expect(sakuraCard.getByText('是', { exact: true })).toBeVisible({ timeout: 10000 });
  });

  test('管理员批准目录审核申请', async ({ page }) => {
    // 用管理员自己的会话导入一个私有主题、提交审核——不依赖其它 spec 文件的
    // 状态(spec 文件默认并行 worker,不能假设执行顺序或共享服务端状态)。
    const imported = await page.request.post('/api/v1/me/themes/import', {
      multipart: { file: { name: 'mauve.zip', mimeType: 'application/zip', buffer: readFileSync(THEME_ZIP) } },
    });
    expect(imported.ok()).toBeTruthy();
    const themeId = (await imported.json()).data.id as string;

    const submitted = await page.request.post(`/api/v1/me/themes/${themeId}/catalog-request`);
    expect(submitted.ok()).toBeTruthy();

    await page.goto('/admin/themes');
    await expect(page.getByRole('heading', { name: '目录审核申请' })).toBeVisible();
    const row = page.getByRole('row').filter({ hasText: 'Mauve' });
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: '批准' }).click();
    // 确认对话框里的批准按钮：ReviewDialog 渲染为 `fixed inset-0` 遮罩层，不是
    // ARIA dialog role，且队列行自己的「批准」按钮仍在遮罩后面留在 DOM 里，
    // 用 `.fixed.inset-0` 把定位范围收窄到弹层内部，避免撞到队列行按钮。
    await page.locator('.fixed.inset-0').getByRole('button', { name: '批准', exact: true }).click();
    await expect(page.getByText('目录申请已批准')).toBeVisible({ timeout: 10000 });

    // 队列默认按「待审核」筛选，处理完的申请不再匹配该筛选、会从列表消失；
    // 切到「全部状态」才能看到该行刷新为已批准、操作列变为「已处理」。
    await page.getByRole('combobox').selectOption({ label: '全部状态' });
    await expect(page.getByRole('row').filter({ hasText: 'Mauve' }).getByText('已处理')).toBeVisible({ timeout: 10000 });
    // 主题网格失效重取,该主题应移入「官方目录」分组。
    const catalogSection = page.locator('h3', { hasText: '官方目录' }).locator('xpath=..');
    await expect(catalogSection.getByText('Mauve')).toBeVisible({ timeout: 10000 });
  });
});
