import { test, expect } from '@playwright/test';
import { USER } from './accounts';

// 管理员关键路径：运营概览、用户管理、主题管理。
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

  test('主题管理页展示主题包', async ({ page }) => {
    await page.goto('/admin/themes');
    await expect(page.getByText('Slate Dark').first()).toBeVisible();
  });
});
