import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';
import { USER } from './accounts';

const BACKGROUND_PNG = fileURLToPath(new URL('../fixtures/background.png', import.meta.url));
const BOOKMARKS_HTML = fileURLToPath(new URL('../fixtures/bookmarks.html', import.meta.url));

// 用户关键路径：登录、编辑导航、发布、查看公开页、切换主题。
test.describe('用户登录', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('邮箱密码登录后进入工作台', async ({ page }) => {
    await page.goto('/login');
    // 登录页默认停在「密码登录」tab；提交按钮用 exact 区分「密码登录/验证码登录」等按钮。
    await page.getByLabel('邮箱或用户名').fill(USER.email);
    await page.getByLabel('密码', { exact: true }).fill(USER.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
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

    // 快速添加：填 URL，在「更多选项」里手动命名，避免依赖线上抓取结果。
    await page.getByRole('button', { name: '添加站点' }).first().click();
    await page.getByPlaceholder(/粘贴或输入 URL/).fill('https://www.ietf.org');
    await page.getByRole('button', { name: /更多选项/ }).click();
    await page.getByPlaceholder('留空则用自动识别').fill('IETF');
    await page.getByRole('combobox').selectOption({ label: '我的书签' });
    await page.getByRole('button', { name: '添加', exact: true }).click();
    await expect(page.getByText('IETF').first()).toBeVisible();
  });

  test('发布导航页并公开可见', async ({ page }) => {
    await page.goto('/app/publish');
    await expect(page.getByText('未发布').first()).toBeVisible();

    // Prefer page primary CTA over AppShell header toolbar (both label「发布」).
    await page
      .locator('div.space-y-5')
      .filter({ has: page.getByRole('heading', { name: '发布 & 域名' }) })
      .getByRole('button', { name: '发布', exact: true })
      .click();
    await expect(page.getByText('发布成功')).toBeVisible();
    await expect(page.getByText('已是最新').first()).toBeVisible();

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
    await expect(page.getByText(/主题已写入草稿：「Slate Dark」/)).toBeVisible();
  });

  test('导入书签并导出备份', async ({ page }) => {
    await page.goto('/app/import-export');

    // 隐藏 file input 由「选择文件」按钮触发；直接设置书签 HTML。
    await page.locator('input[type="file"]').setInputFiles(BOOKMARKS_HTML);
    await page.getByRole('button', { name: '预检文件' }).click();

    // 预检成功后展示来源分类，全部有效且非重复的站点默认勾选。
    await expect(page.getByText('E2E 导入分类')).toBeVisible();
    const commit = page.getByRole('button', { name: /导入已选 \d+ 项/ });
    await expect(commit).toBeEnabled();
    await commit.click();
    await expect(page.getByText(/已导入 \d+ 个站点/)).toBeVisible();

    // 导出 JSON 备份：应触发下载并提示成功。
    const download = page.waitForEvent('download');
    await page.getByRole('button', { name: 'nav.ax JSON' }).click();
    expect((await download).suggestedFilename()).toMatch(/\.json$/);
    await expect(page.getByText('导出文件已生成')).toBeVisible();
  });

  test('上传本地背景图', async ({ page }) => {
    await page.goto('/app/themes');
    // 隐藏的 file input 由「上传图片」按钮触发；直接对 input 设置文件。
    // 页面有两个隐藏 file input：第一个上传到「我的背景」，第二个是站长预设。
    await page.locator('input[type="file"]').first().setInputFiles(BACKGROUND_PNG);
    await expect(page.getByText(/背景已写入草稿/)).toBeVisible();
    // 上传后返回真实 asset URL，预览图片指向 /api/v1/assets/…
    await expect(page.getByAltText('背景预览')).toHaveAttribute('src', /\/api\/v1\/assets\//);
  });
});
