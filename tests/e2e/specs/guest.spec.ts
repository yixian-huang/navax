import { test, expect } from '@playwright/test';

// 游客关键路径：公开首页、发现页、未知页面 404。
test.describe('游客', () => {
  test('公开首页展示已发布的系统内容', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: '登录' })).toBeVisible();
    // 站点按分类 tab 展示，切到目标分类后断言站点可见
    await page.getByRole('tab', { name: /精选工具/ }).click();
    await expect(page.getByText('Example 官网')).toBeVisible();
  });

  test('发现页可访问', async ({ page }) => {
    await page.goto('/discover');
    await expect(page.getByRole('heading', { name: '发现精选导航' })).toBeVisible();
  });

  test('访问不存在的分享页提示页面不可用', async ({ page }) => {
    await page.goto('/u/does-not-exist');
    await expect(page.getByText('该导航页不存在或已被设为私密')).toBeVisible();
  });

  test('公开首页键盘可聚焦分类标签', async ({ page }) => {
    await page.goto('/');
    const tab = page.getByRole('tab', { name: /精选工具/ });
    await tab.focus();
    await expect(tab).toBeFocused();
  });
});
