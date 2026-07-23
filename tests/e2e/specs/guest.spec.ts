import { test, expect } from '@playwright/test';

// 游客关键路径：公开首页、发现页、未知页面 404。
test.describe('游客', () => {
  test('公开首页展示已发布的系统内容', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: '登录' })).toBeVisible();
    // 非空分类 ≥2 时渲染 tab；切到目标分类后断言站点可见
    await expect(page.getByRole('tab', { name: /精选工具/ })).toBeVisible();
    await page.getByRole('tab', { name: /精选工具/ }).click();
    await expect(page.getByText('Example 官网')).toBeVisible();
  });

  test('发现页可访问', async ({ page }) => {
    await page.goto('/discover');
    await expect(page.getByRole('heading', { name: '发现精选导航' })).toBeVisible();
  });

  test('访问不存在的分享页提示页面不可用', async ({ page }) => {
    await page.goto('/u/does-not-exist');
    await expect(page.getByText('导航页不存在')).toBeVisible();
  });

  test('公开首页键盘可聚焦分类标签', async ({ page }) => {
    await page.goto('/');
    const tab = page.getByRole('tab', { name: /精选工具/ });
    await tab.focus();
    await expect(tab).toBeFocused();
  });

  test('站点列表密度无整块毛玻璃容器', async ({ page }) => {
    await page.goto('/');
    // Switch to list density via aria-label
    const listBtn = page.getByRole('radio', { name: '列表' });
    if (await listBtn.count()) {
      await listBtn.click();
    }
    // List panel must not use material-card frosted slab
    const panel = page.locator('.site-card-list-panel');
    if (await panel.count()) {
      await expect(panel).not.toHaveClass(/material-card/);
      const filter = await panel.evaluate(el => getComputedStyle(el).backdropFilter || (getComputedStyle(el) as CSSStyleDeclaration & { webkitBackdropFilter?: string }).webkitBackdropFilter || 'none');
      expect(filter === 'none' || !filter).toBeTruthy();
    }
    // Comfortable cards keep side-by-side icon layout when selected
    const comfortBtn = page.getByRole('radio', { name: '舒适' });
    if (await comfortBtn.count()) {
      await comfortBtn.click();
      const card = page.locator('.site-card-comfortable').first();
      if (await card.count()) {
        await expect(card).toBeVisible();
        await expect(card).toHaveClass(/flex/);
      }
    }
  });

  test('主题作用域封闭在宿主 frame 内', async ({ page }) => {
    await page.goto('/');

    // data-theme 必须落在主题根上，而不是 <html>。设在 <html> 上意味着
    // 第三方 CSS 会作用于整个应用，包括已登录界面。
    await expect(page.locator('html')).not.toHaveAttribute('data-theme', /.+/);
    const root = page.locator('[data-nx="page-root"]');
    await expect(root).toHaveCount(1);

    // 隔离边界在宿主 wrapper 上：contain: paint 一次性提供包含块、
    // 层叠上下文与绘制裁剪，而主题在语法上选不到它。
    const frame = page.locator('[data-nx-frame]');
    await expect(frame).toHaveCount(1);
    const contain = await frame.evaluate(el => getComputedStyle(el).contain);
    expect(contain).toContain('paint');

    // 受保护的源码链接必须在 frame 之外，且实际可见可点。
    const protectedRegion = page.locator('[data-nx-protected]');
    await expect(protectedRegion).toBeVisible();
    const outsideFrame = await protectedRegion.evaluate(
      el => !el.closest('[data-nx-frame]'),
    );
    expect(outsideFrame).toBe(true);
    const sourceLink = protectedRegion.getByRole('link', { name: '源码' });
    await expect(sourceLink).toBeVisible();
  });

  test('主题样式经内容寻址的 link 供应', async ({ page }) => {
    await page.goto('/');
    const link = page.locator('link[data-theme-style]');
    if (await link.count()) {
      const href = await link.first().getAttribute('href');
      expect(href).toMatch(/\/api\/v1\/public\/themes\/v[0-9a-f]{32}\.css$/);
      const response = await page.request.get(href!);
      expect(response.status()).toBe(200);
      expect(response.headers()['cache-control']).toContain('immutable');
    }
  });
});
