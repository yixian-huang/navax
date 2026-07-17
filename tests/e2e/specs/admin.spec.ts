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
    await page.getByRole('link', { name: '管理主题库' }).click();
    await expect(page.getByRole('heading', { name: '平台主题库' })).toBeVisible();
    await expect(page.getByText('Slate Dark').first()).toBeVisible();
  });
});
