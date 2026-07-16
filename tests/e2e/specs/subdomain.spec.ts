import { test, expect } from '@playwright/test';

// 短域名申请→管理员审核的端到端闭环。
// 1–3 位标签属于稀缺短域名，提交后进入待审核；管理员在运维中心批准。
// 用户与管理员两段流程放在同一文件内，依赖单文件顺序执行保证「先申请后审核」。
// 域名申请入口位于「发布 & 域名」页（/app/publish）的「自定义域名」区块。
const SHORT_LABEL = 'zz1';

test.describe('用户申请短域名', () => {
  test.use({ storageState: '.auth/user.json' });

  test('提交短域名申请后进入待审核', async ({ page }) => {
    await page.goto('/app/publish');
    await page.getByPlaceholder('your-name').fill(SHORT_LABEL);
    await page.getByRole('button', { name: '申请域名' }).click();
    await expect(page.getByText('申请已提交，等待审核')).toBeVisible();
    await expect(page.getByText(/正在审核中/)).toBeVisible();
  });
});

test.describe('管理员审核短域名', () => {
  test.use({ storageState: '.auth/admin.json' });

  test('批准待审核的短域名申请', async ({ page }) => {
    await page.goto('/admin/operations?tab=subdomains');
    const row = page.getByRole('row').filter({ hasText: SHORT_LABEL });
    await expect(row).toBeVisible();

    await row.getByRole('button', { name: '批准' }).click();
    await expect(page.getByRole('heading', { name: '批准子域名申请' })).toBeVisible();
    // 表格行与弹窗内均有「批准」按钮，弹窗渲染在后，取最后一个即确认按钮。
    await page.getByRole('button', { name: '批准' }).last().click();
    await expect(page.getByText('子域名申请已批准')).toBeVisible();

    // 默认「待审核」筛选下，已批准的申请会移出列表。
    await expect(page.getByRole('row').filter({ hasText: SHORT_LABEL })).toHaveCount(0);
  });
});
