import { test, expect } from '@playwright/test';
import { USER } from './accounts';

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
});
