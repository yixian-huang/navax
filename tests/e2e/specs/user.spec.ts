import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';
import { USER } from './accounts';

const BACKGROUND_PNG = fileURLToPath(new URL('../fixtures/background.png', import.meta.url));

// 用户关键路径：登录、编辑导航、发布、查看公开页、切换主题。
test.describe('用户登录', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('邮箱密码登录后进入工作台', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('your@email.com').fill(USER.email);
    await page.getByPlaceholder('输入密码').fill(USER.password);
    await page.getByRole('button', { name: '登录' }).click();
    await page.waitForURL(/\/app/);
    await expect(page.getByText('工作台').first()).toBeVisible();
  });
});

test.describe('用户工作台', () => {
  test.use({ storageState: '.auth/user.json' });

  test('创建分类与站点', async ({ page }) => {
    await page.goto('/app/links');

    await page.getByRole('button', { name: '新建分类' }).click();
    await page.getByPlaceholder('例如：开发工具').fill('我的书签');
    await page.getByRole('button', { name: '创建' }).click();
    await expect(page.getByText('我的书签').first()).toBeVisible();

    await page.getByRole('button', { name: '添加站点' }).first().click();
    await page.getByRole('button', { name: '手动添加' }).click();
    await page.getByPlaceholder('GitHub', { exact: true }).fill('IETF');
    await page.getByPlaceholder('https://github.com').fill('https://www.ietf.org');
    await page.getByRole('combobox').selectOption({ label: '我的书签' });
    await page.getByRole('button', { name: '添加', exact: true }).click();
    await expect(page.getByText('IETF').first()).toBeVisible();
  });

  test('发布导航页并公开可见', async ({ page }) => {
    await page.goto('/app/publish');
    await expect(page.getByText('未发布').first()).toBeVisible();

    await page.getByRole('button', { name: '发布', exact: true }).click();
    await expect(page.getByText('发布成功！')).toBeVisible();
    await expect(page.getByText('已发布').first()).toBeVisible();

    const publication = await page.request.get('/api/v1/pages/current?scope=personal');
    const slug = (await publication.json()).data.publication.slug;

    await page.goto(`/u/${slug}`);
    await page.getByRole('tab', { name: /我的书签/ }).click();
    await expect(page.getByText('IETF').first()).toBeVisible();
  });

  test('切换主题为 Slate Dark', async ({ page }) => {
    await page.goto('/app/themes');
    await expect(page.getByText('Slate Dark').first()).toBeVisible();
    await page.getByRole('button', { name: /Slate Dark/ }).click();
    await expect(page.getByText('主题已切换为「Slate Dark」')).toBeVisible();
  });

  test('上传本地背景图', async ({ page }) => {
    await page.goto('/app/themes');
    // 隐藏的 file input 由「上传图片」按钮触发；直接对 input 设置文件。
    await page.locator('input[type="file"]').setInputFiles(BACKGROUND_PNG);
    await expect(page.getByText('背景图已更新')).toBeVisible();
    // 上传后返回真实 asset URL，预览图片指向 /api/v1/assets/…
    await expect(page.getByAltText('背景预览')).toHaveAttribute('src', /\/api\/v1\/assets\//);
  });
});
