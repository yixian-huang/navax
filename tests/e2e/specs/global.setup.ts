import { test as setup, expect, request } from '@playwright/test';
import { SETUP_TOKEN, ADMIN, USER } from './accounts';

// 通过 API 完成实例初始化与账号准备，浏览器测试专注 UI 关键路径：
// 1. bootstrap 创建管理员 → 保存 admin 会话
// 2. 为系统页植入内容并发布 → 游客首页可见
// 3. 创建邀请并注册普通用户 → 保存 user 会话（页面留待 UI 流程编辑与发布）

setup('初始化实例并准备账号', async ({ request: adminAPI, baseURL }) => {
  const origin = { Origin: baseURL! };

  const boot = await adminAPI.post('/api/v1/bootstrap', {
    headers: { ...origin, 'X-Setup-Token': SETUP_TOKEN },
    data: {
      adminUsername: 'admin',
      adminEmail: ADMIN.email,
      adminPassword: ADMIN.password,
      instanceName: 'nav.ax',
      publicBaseUrl: baseURL,
    },
  });
  expect(boot.status(), await boot.text()).toBe(201);

  // 系统页植入一条可见内容并公开发布
  const systemPage = await (await adminAPI.get('/api/v1/pages/current?scope=system')).json();
  const systemPageId: string = systemPage.data.id;

  const category = await adminAPI.post(`/api/v1/pages/${systemPageId}/categories`, {
    headers: origin,
    data: { name: '精选工具', icon: 'ri-tools-line' },
  });
  expect(category.status(), await category.text()).toBe(201);
  const categoryId = (await category.json()).data.id;

  const site = await adminAPI.post(`/api/v1/pages/${systemPageId}/sites`, {
    headers: origin,
    data: { categoryId, title: 'Example 官网', url: 'https://example.com' },
  });
  expect(site.status(), await site.text()).toBe(201);

  // 再加一个有站点的分类，确保发布投影里至少 2 个非空分类。
  // 公开页 CategoryTabs 仅在 categories.length > 1 时渲染（空「未分类」已不再进入快照）。
  const category2 = await adminAPI.post(`/api/v1/pages/${systemPageId}/categories`, {
    headers: origin,
    data: { name: '学习资源', icon: 'ri-book-line' },
  });
  expect(category2.status(), await category2.text()).toBe(201);
  const category2Id = (await category2.json()).data.id;
  const site2 = await adminAPI.post(`/api/v1/pages/${systemPageId}/sites`, {
    headers: origin,
    data: { categoryId: category2Id, title: 'MDN', url: 'https://developer.mozilla.org' },
  });
  expect(site2.status(), await site2.text()).toBe(201);

  const publication = await (await adminAPI.get(`/api/v1/pages/${systemPageId}/publication`)).json();
  const visibility = await adminAPI.put(`/api/v1/pages/${systemPageId}/publication`, {
    headers: origin,
    data: { visibility: 'public', slug: publication.data.slug, showAuthor: false, seoTitle: '', seoDescription: '' },
  });
  expect(visibility.status(), await visibility.text()).toBe(200);

  const refreshed = await (await adminAPI.get('/api/v1/pages/current?scope=system')).json();
  const published = await adminAPI.post(`/api/v1/pages/${systemPageId}/publish`, {
    headers: { ...origin, 'Idempotency-Key': 'e2e-publish-system-000001' },
    data: { expectedRevision: refreshed.data.draftRevision },
  });
  expect(published.status(), await published.text()).toBe(200);

  // 启用短域名功能，供子域名申请/审核 E2E 使用
  const domainSettings = await adminAPI.patch('/api/v1/admin/settings', {
    headers: origin,
    data: { domain: { rootDomain: 'nav.test', subdomainsEnabled: true } },
  });
  expect(domainSettings.status(), await domainSettings.text()).toBe(200);

  await adminAPI.storageState({ path: '.auth/admin.json' });

  // 邀请并注册普通用户
  const invitation = await adminAPI.post('/api/v1/admin/invitations', {
    headers: origin,
    data: { maxUses: 1, expiresInDays: 7 },
  });
  expect(invitation.status(), await invitation.text()).toBe(201);
  const inviteToken = (await invitation.json()).data.token;

  const userAPI = await request.newContext({ baseURL });
  const registered = await userAPI.post(`/api/v1/auth/invitations/${inviteToken}/register`, {
    headers: origin,
    data: { username: USER.username, email: USER.email, password: USER.password },
  });
  expect(registered.status(), await registered.text()).toBe(201);
  await userAPI.storageState({ path: '.auth/user.json' });
  await userAPI.dispose();
});
