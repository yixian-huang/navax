// ============================================================
// nav.ax Mock API Handlers — intercepts fetch and returns mock data
// ============================================================

import { API_BASE } from '@/api/client';
import {
  mockAuthUser,
  mockAdminUser,
  mockNavigationPage,
  mockPublishedPage,
  mockThemes,
  mockAdminOverview,
  mockUsers,
  mockInvitations,
  mockPlatformSites,
  mockPlatformCategories,
  mockSystemSettings,
  mockAuditEntries,
  mockAllLinks,
  mockCategories,
  mockSites,
  mockSystemPage,
  mockSystemCategories,
  mockSystemSites,
  mockSystemWidgets,
  paginate,
  getMockSubdomain,
  setMockSubdomain,
  cancelMockSubdomain,
} from '@/mocks/data';
import { mockAnalyticsResponse, getDailyStatsForDays, getTopSitesForLimit } from '@/mocks/analytics';
import { mockDiscoveredPages } from '@/mocks/discover';
import type { MockPageState, MockPublishedPage } from '@/mocks/data';
import type { AuthSession, CreateCategoryRequest, UpdateSiteRequest, ReorderRequest, Category, Site, CreateSiteRequest, User } from '@/api/types';

type MockAuthenticatedSession = AuthSession & { authenticated: true; user: User };

let signedInMockUser: MockAuthenticatedSession | null = null;
let mockSessionClosed = false;

function getCurrentUser(): MockAuthenticatedSession | null {
  if (mockSessionClosed) return null;
  if (signedInMockUser) return signedInMockUser;
  return window.location.pathname.startsWith('/admin') ? mockAdminUser : mockAuthUser;
}

function getEditMode(): 'system' | 'personal' {
  return new URLSearchParams(window.location.search).get('scope') === 'system' ? 'system' : 'personal';
}

// ---- Helpers for dual data source (system vs personal) ----
function isEditingSystem(): boolean {
  const user = getCurrentUser();
  return user?.user.role === 'admin' && getEditMode() === 'system';
}

function activePage() {
  return isEditingSystem() ? mockSystemPage : mockNavigationPage;
}

function activeCategories() {
  return isEditingSystem() ? mockSystemCategories : mockCategories;
}

function activeSites() {
  return isEditingSystem() ? mockSystemSites : mockSites;
}

function activeWidgets() {
  return isEditingSystem() ? mockSystemWidgets : mockNavigationPage.widgets;
}

// 背景图不属于旧页面模型，单独按页面存储，避免每次合成时被重置
const pageBackgrounds = new Map<string, { type: string; value: string; opacity: number }>();

function activePageSettings() {
  const page = activePage();
  return {
    layout: {
      template: 'full',
      density: page.layout.density,
      columns: page.layout.columns,
      categoryStyle: page.layout.categoryStyle,
    },
    appearance: {
      themeId: page.themeId,
      background: pageBackgrounds.get(page.id) ?? { type: 'none', value: '', opacity: 1 },
    },
    search: { defaultEngine: 'google', showEngineSelector: true },
    display: { showClock: page.layout.showClock, showDate: page.layout.showDate, showGreeting: true },
    preferences: { locale: 'zh-CN', timezone: 'Asia/Shanghai', openLinksInNewTab: true },
  };
}

function activePublication() {
  const page = activePage();
  return {
    visibility: page.isPublished ? 'public' : 'private',
    slug: page.slug,
    showAuthor: page.publishSettings.showAuthor,
    seoTitle: page.publishSettings.title,
    seoDescription: page.publishSettings.description,
    published: page.isPublished,
    canonicalUrl: page.publishSettings.customDomain || null,
    robots: page.isPublished ? 'index,follow' : 'noindex,follow',
    snapshotId: page.isPublished ? `snapshot_${page.id}` : null,
    publishedRevision: page.isPublished ? 1 : null,
    publishedAt: page.publishedAt || null,
    hasUnpublishedChanges: page.hasUnpublishedChanges,
  };
}

// Keep system page categories in sync
function syncPageCategories() {
  if (isEditingSystem()) {
    mockSystemPage.categories = [...mockSystemCategories];
  } else {
    mockNavigationPage.categories = [...mockCategories];
  }
}

// 把扁平内部状态投影为草稿页契约（NavigationPageContract）：
// 分类去掉内嵌 sites，站点单独扁平列出，供前端 normalizePage 重新嵌套。
function contractPageResponse() {
  const page = activePage();
  const categories = activeCategories();
  const sites = categories.flatMap(category => category.sites);
  return {
    id: page.id,
    kind: isEditingSystem() ? 'system' : 'personal',
    ownerId: page.ownerId,
    ownerName: page.ownerName,
    title: page.title,
    description: page.description,
    draftRevision: 0,
    settings: activePageSettings(),
    categories: categories.map(({ sites: _sites, ...rest }) => rest),
    sites,
    publication: activePublication(),
    draftUpdatedAt: page.draftUpdatedAt,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: page.updatedAt ?? page.draftUpdatedAt,
  };
}

// 把已发布页投影为公开契约（PublishedPageContract）：分类内嵌 sites，owner 独立对象。
// settings 从来源自身的 themeId/layout 构造，不依赖当前编辑作用域。
function contractPublishedResponse(source: MockPublishedPage | MockPageState, kind: 'system' | 'personal', subdomain: string) {
  // 与草稿一致：使用 pageBackgrounds，否则上传背景后公开页永远是空背景，看起来像“上传无效”。
  const background = pageBackgrounds.get(source.id) ?? { type: 'none', value: '', opacity: 1 };
  return {
    id: source.id,
    snapshotId: `snapshot_${source.id}`,
    kind,
    title: source.title,
    description: source.description,
    slug: source.slug,
    visibility: 'public',
    owner: { name: source.ownerName, avatarUrl: source.ownerAvatar, visible: true },
    settings: {
      layout: {
        template: 'full',
        density: source.layout.density,
        columns: source.layout.columns,
        categoryStyle: source.layout.categoryStyle,
      },
      appearance: { themeId: source.themeId, background },
      search: { defaultEngine: 'google', showEngineSelector: true },
      display: { showClock: source.layout.showClock, showDate: source.layout.showDate, showGreeting: true },
      preferences: { locale: 'zh-CN', timezone: 'Asia/Shanghai', openLinksInNewTab: true },
    },
    categories: source.categories,
    subdomain: subdomain || null,
    publishedAt: source.publishedAt,
    etag: `"mock-${source.id}-${background.type}-${background.value.slice(0, 24)}"`,
  };
}

// ---- Subdomain for system page ----
let mockSystemSubdomain: { subdomain: string; status: 'approved' } = { subdomain: 'nav', status: 'approved' };

type HandlerFn = (url: string, init?: { method?: string; body?: string }) => Promise<Response> | null;

const handlers: HandlerFn[] = [];

let mockRequestSeq = 0;

function jsonResponse(data: unknown, status = 200): Response {
  // 契约要求每个响应的 meta 携带 requestId；mock 统一在此补齐，避免逐处遗漏。
  const envelope = data as { meta?: Record<string, unknown> } | null;
  if (envelope && typeof envelope === 'object' && envelope.meta && typeof envelope.meta === 'object'
    && !('requestId' in envelope.meta)) {
    mockRequestSeq += 1;
    envelope.meta.requestId = `mock-req-${mockRequestSeq}`;
  }
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ---- Auth handlers ----
handlers.push((url, init) => {
  if (url === `${API_BASE}/auth/session`) {
    const current = getCurrentUser();
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: current
        ? { authenticated: true, user: current.user, expiresAt: new Date(Date.now() + 86400000).toISOString() }
        : { authenticated: false, user: null, expiresAt: null },
      meta: { message: '', detail: '' },
    }));
  }
  if (url === `${API_BASE}/auth/login`) {
    let body: { email?: string } = {};
    try { body = JSON.parse(init?.body || ''); } catch { /* ignore */ }
    const user = body.email === 'admin@nav.ax'
      ? { ...mockAdminUser, user: { ...mockAdminUser.user, email: 'admin@nav.ax' } }
      : mockAuthUser;
    signedInMockUser = user;
    mockSessionClosed = false;
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: { authenticated: true, user: user.user, expiresAt: new Date(Date.now() + 86400000).toISOString() },
      meta: { message: '登录成功', detail: '' },
    }));
  }
  if (url === `${API_BASE}/auth/logout`) {
    signedInMockUser = null;
    mockSessionClosed = true;
    return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '已退出', detail: '' } }));
  }
  if (url === `${API_BASE}/auth/password/forgot`) {
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: { message: '如果该邮箱对应有效账号，我们已发送密码重置邮件。' },
      meta: { message: '', detail: '' },
    }));
  }
  if (url === `${API_BASE}/auth/password/reset`) {
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: { message: '密码已重置，请使用新密码登录。' },
      meta: { message: '', detail: '' },
    }));
  }
  if (url.startsWith(`${API_BASE}/auth/invite/`) && url.endsWith('/validate')) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: { valid: true, inviterName: 'admin' }, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/auth/register`) {
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: { authenticated: true, user: mockAuthUser.user, expiresAt: new Date(Date.now() + 86400000).toISOString() },
      meta: { message: '注册成功', detail: '' },
    }));
  }
  if (url === `${API_BASE}/auth/profile`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: { ...mockAuthUser.user, bio: '前端开发者，开源爱好者。喜欢收集好用的工具和资源。' }, meta: { message: '', detail: '' } }));
  }
  return null;
});

// ---- Public catalog handlers ----
handlers.push(url => {
  if (url.startsWith(`${API_BASE}/navigation/directory`)) {
    const query = new URL(url, window.location.origin).searchParams;
    const search = query.get('search')?.trim().toLowerCase() ?? '';
    const categoryId = query.get('categoryId') ?? '';
    const items = mockPlatformSites.filter(site =>
      (!categoryId || site.categoryId === categoryId) &&
      (!search || `${site.title} ${site.description} ${site.url}`.toLowerCase().includes(search)),
    );
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: items,
      meta: { page: 1, pageSize: items.length, total: items.length, totalPages: items.length ? 1 : 0 },
    }));
  }
  if (url.startsWith(`${API_BASE}/public/discover`)) {
    const query = new URL(url, window.location.origin).searchParams;
    const search = query.get('search')?.trim().toLowerCase() ?? '';
    const tag = query.get('tag') ?? '';
    const sort = query.get('sort') ?? 'latest';
    const items = [...mockDiscoveredPages]
      .filter(page =>
        (!tag || page.tags.includes(tag)) &&
        (!search || `${page.title} ${page.description}`.toLowerCase().includes(search)),
      )
      .sort((a, b) => sort === 'popular'
        ? b.viewCount - a.viewCount
        : new Date(b.publishedAt).getTime() - new Date(a.publishedAt).getTime())
      .map(page => ({
        slug: page.slug,
        title: page.title,
        description: page.description,
        ownerName: page.ownerName,
        themeId: 'slate',
        tags: page.tags,
        featured: page.isVerified,
        viewCount: page.viewCount,
        publishedAt: page.publishedAt,
      }));
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: items,
      meta: { page: 1, pageSize: items.length, total: items.length, totalPages: items.length ? 1 : 0 },
    }));
  }
  return null;
});

// ---- Public config & asset upload ----
let mockAssetSeq = 0;
handlers.push(async (url, init) => {
  if (url === `${API_BASE}/public/config`) {
    return jsonResponse({
      code: 'OK',
      data: {
        instanceName: 'nav.ax',
        publicBaseUrl: window.location.origin,
        rootDomain: null,
        registrationMode: 'invite',
        features: { discover: true, analytics: true, subdomains: false, mail: true },
        limits: { maxCategoriesPerPage: 50, maxSitesPerPage: 1000, maxUploadBytes: 5 * 1024 * 1024 },
      },
      meta: { message: '', detail: '' },
    });
  }
  if (url === `${API_BASE}/assets` && (init?.method || 'GET') === 'POST') {
    mockAssetSeq += 1;
    // 开发 mock：把实际上传的文件转成 data URL，主题预览 / 公开页可立刻看到真实图片。
    let kind = 'background';
    let preview = 'data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%22320%22%20height=%22180%22%3E%3Crect%20width=%22100%25%22%20height=%22100%25%22%20fill=%22%234a6b52%22/%3E%3C/svg%3E';
    let mimeType = 'image/svg+xml';
    let size = 1024;
    const body = init?.body as unknown;
    if (typeof FormData !== 'undefined' && body instanceof FormData) {
      const kindValue = body.get('kind');
      if (typeof kindValue === 'string' && kindValue) kind = kindValue;
      const file = body.get('file');
      if (file instanceof Blob) {
        size = file.size;
        mimeType = file.type || 'application/octet-stream';
        preview = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(String(reader.result || ''));
          reader.onerror = () => reject(reader.error ?? new Error('read upload failed'));
          reader.readAsDataURL(file);
        });
      }
    }
    return jsonResponse({
      code: 'OK',
      data: {
        id: `ast_mock_${mockAssetSeq}`,
        kind,
        url: preview,
        mimeType,
        size,
        createdAt: new Date().toISOString(),
      },
      meta: { message: '', detail: '' },
    }, 201);
  }
  return null;
});

// ---- Navigation handlers ----
handlers.push((url, init) => {
  if (/\/pages\/[^/]+\/settings$/.test(url)) {
    const method = init?.method || 'GET';
    if (method === 'PUT') {
      const body = JSON.parse(init?.body || '{}');
      const page = activePage();
      page.layout = {
        density: body.layout?.density ?? page.layout.density,
        columns: body.layout?.columns ?? page.layout.columns,
        categoryStyle: body.layout?.categoryStyle ?? page.layout.categoryStyle,
        showClock: body.display?.showClock ?? page.layout.showClock,
        showDate: body.display?.showDate ?? page.layout.showDate,
      };
      page.themeId = body.appearance?.themeId ?? page.themeId;
      if (body.appearance?.background) pageBackgrounds.set(page.id, body.appearance.background);
      page.hasUnpublishedChanges = true;
      page.draftUpdatedAt = new Date().toISOString();
    }
    return Promise.resolve(jsonResponse({ code: 'OK', data: activePageSettings(), meta: { message: '', detail: '' } }));
  }
  if (/\/pages\/[^/]+\/content-order$/.test(url)) {
    const body = JSON.parse(init?.body || '{}') as { categories?: { id: string; siteIds: string[] }[] };
    const cats = activeCategories();
    const sites = activeSites();
    for (const [categoryOrder, item] of (body.categories ?? []).entries()) {
      const category = cats.find(cat => cat.id === item.id);
      if (!category) continue;
      category.sortOrder = categoryOrder;
      category.sites = item.siteIds
        .map((siteId, sortOrder) => {
          const site = sites.find(candidate => candidate.id === siteId);
          if (site) site.sortOrder = sortOrder;
          return site;
        })
        .filter((site): site is Site => Boolean(site));
    }
    cats.sort((a, b) => a.sortOrder - b.sortOrder);
    syncPageCategories();
    return Promise.resolve(jsonResponse({ code: 'OK', data: { draftRevision: Date.now() }, meta: { message: '', detail: '' } }));
  }
  if (/\/pages\/[^/]+\/publication$/.test(url)) {
    const method = init?.method || 'GET';
    const page = activePage();
    if (method === 'PUT') {
      const body = JSON.parse(init?.body || '{}');
      page.slug = body.slug ?? page.slug;
      page.publishSettings.slug = page.slug;
      page.publishSettings.showAuthor = body.showAuthor ?? page.publishSettings.showAuthor;
      page.publishSettings.title = body.seoTitle ?? page.publishSettings.title;
      page.publishSettings.description = body.seoDescription ?? page.publishSettings.description;
      page.isPublished = body.visibility !== 'private' && page.isPublished;
      page.publishSettings.isPublished = page.isPublished;
    } else if (method === 'DELETE') {
      page.isPublished = false;
      page.publishSettings.isPublished = false;
    }
    return Promise.resolve(jsonResponse({ code: 'OK', data: activePublication(), meta: { message: '', detail: '' } }));
  }
  if (/\/pages\/[^/]+\/publish$/.test(url)) {
    const page = activePage();
    page.isPublished = true;
    page.publishSettings.isPublished = true;
    page.publishedAt = new Date().toISOString();
    page.hasUnpublishedChanges = false;
    return Promise.resolve(jsonResponse({ code: 'OK', data: activePublication(), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/page`) {
    syncPageCategories();
    // 返回契约形状（NavigationPageContract），与真实后端一致，由前端 normalizePage 处理。
    return Promise.resolve(jsonResponse({ code: 'OK', data: contractPageResponse(), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/page/layout`) {
    const body = JSON.parse(init?.body || '');
    const page = activePage();
    if (body.density) page.layout.density = body.density;
    if (body.columns !== undefined) page.layout.columns = body.columns;
    if (body.categoryStyle) page.layout.categoryStyle = body.categoryStyle;
    page.hasUnpublishedChanges = true;
    page.draftUpdatedAt = new Date().toISOString();
    return Promise.resolve(jsonResponse({ code: 'OK', data: page, meta: { message: '布局已保存', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/categories`) {
    const method = init?.method || 'GET';
    if (method === 'GET') {
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: activePage().categories, meta: { message: '', detail: '' } }));
    }
    if (method === 'POST') {
      let body: CreateCategoryRequest;
      try { body = JSON.parse(init?.body || ''); } catch {
        return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400));
      }
      if (!body.name?.trim()) {
        return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '分类名称不能为空', detail: '' } }, 400));
      }
      const cats = activeCategories();
      const newCat: Category = {
        id: isEditingSystem() ? `sys_cat_${Date.now()}` : `cat_${Date.now()}`,
        pageId: isEditingSystem() ? 'page_system' : 'page_001',
        name: body.name.trim(),
        icon: body.icon || 'ri-folder-line',
        sortOrder: cats.length,
        sites: [],
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      cats.push(newCat);
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: newCat, meta: { message: '分类已创建', detail: '' } }));
    }
  }
  // Update / delete category
  if (url.match(/\/navigation\/categories\/[^/]+$/) && !url.includes('reorder')) {
    const id = url.split('/').pop()!;
    const method = init?.method || 'GET';
    const cats = activeCategories();
    const sites = activeSites();
    if (method === 'PATCH') {
      let body: Partial<CreateCategoryRequest>;
      try { body = JSON.parse(init?.body || ''); } catch { return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400)); }
      const cat = cats.find(c => c.id === id);
      if (!cat) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '分类不存在', detail: '' } }, 404));
      if (body.name) cat.name = body.name;
      if (body.icon) cat.icon = body.icon;
      cat.updatedAt = new Date().toISOString();
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: cat, meta: { message: '分类已更新', detail: '' } }));
    }
    if (method === 'DELETE') {
      const idx = cats.findIndex(c => c.id === id);
      if (idx === -1) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '分类不存在', detail: '' } }, 404));
      // Also remove associated sites
      const siteIds = cats[idx].sites.map(s => s.id);
      for (const sid of siteIds) {
        const si = sites.findIndex(s => s.id === sid);
        if (si !== -1) sites.splice(si, 1);
      }
      cats.splice(idx, 1);
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '分类已删除', detail: '' } }));
    }
  }
  // Reorder categories
  if (url === `${API_BASE}/navigation/categories/reorder` && (init?.method || 'POST') === 'POST') {
    let body: ReorderRequest;
    try { body = JSON.parse(init?.body || ''); } catch { return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400)); }
    const cats = activeCategories();
    for (const item of body.items) {
      const cat = cats.find(c => c.id === item.id);
      if (cat) cat.sortOrder = item.sortOrder;
    }
    cats.sort((a, b) => a.sortOrder - b.sortOrder);
    syncPageCategories();
    return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '排序已更新', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/sites`) {
    const method = init?.method || 'GET';
    if (method === 'GET') {
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: activePage().categories.flatMap(c => c.sites), meta: { message: '', detail: '' } }));
    }
    if (method === 'POST') {
      let body: CreateSiteRequest;
      try { body = JSON.parse(init?.body || ''); } catch { return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400)); }
      if (!body.title?.trim() || !body.url?.trim()) {
        return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '站点名称和网址不能为空', detail: '' } }, 400));
      }
      const cats = activeCategories();
      const cat = cats.find(c => c.id === body.categoryId);
      if (!cat) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '分类不存在', detail: '' } }, 404));
      const sites = activeSites();
      const newSite: Site = {
        id: isEditingSystem() ? `sys_site_${Date.now()}` : `site_${Date.now()}`,
        categoryId: body.categoryId,
        title: body.title.trim(),
        url: body.url.trim(),
        icon: body.icon || 'ri-link',
        description: body.description || '',
        sortOrder: cat.sites.length,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      sites.push(newSite);
      cat.sites.push(newSite);
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: newSite, meta: { message: '站点已添加', detail: '' } }));
    }
  }
  // Update site
  if (url.match(/\/navigation\/sites\/[^/]+$/) && !url.includes('reorder')) {
    const id = url.split('/').pop()!;
    const method = init?.method || 'GET';
    const sites = activeSites();
    const cats = activeCategories();
    if (method === 'PATCH') {
      let body: UpdateSiteRequest;
      try { body = JSON.parse(init?.body || ''); } catch { return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400)); }
      const site = sites.find(s => s.id === id);
      if (!site) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '站点不存在', detail: '' } }, 404));
      if (body.title) site.title = body.title;
      if (body.url) site.url = body.url;
      if (body.icon !== undefined) site.icon = body.icon;
      if (body.description !== undefined) site.description = body.description;
      if (body.categoryId && body.categoryId !== site.categoryId) {
        // Move to new category
        const oldCat = cats.find(c => c.id === site.categoryId);
        if (oldCat) oldCat.sites = oldCat.sites.filter(s => s.id !== id);
        site.categoryId = body.categoryId;
        const newCat = cats.find(c => c.id === body.categoryId);
        if (newCat) newCat.sites.push(site);
      }
      site.updatedAt = new Date().toISOString();
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: site, meta: { message: '站点已更新', detail: '' } }));
    }
    if (method === 'DELETE') {
      const idx = sites.findIndex(s => s.id === id);
      if (idx === -1) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '站点不存在', detail: '' } }, 404));
      const site = sites[idx];
      const cat = cats.find(c => c.id === site.categoryId);
      if (cat) cat.sites = cat.sites.filter(s => s.id !== id);
      sites.splice(idx, 1);
      syncPageCategories();
      return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '站点已删除', detail: '' } }));
    }
  }
  // Reorder sites
  if (url === `${API_BASE}/navigation/sites/reorder` && (init?.method || 'POST') === 'POST') {
    let body: ReorderRequest;
    try { body = JSON.parse(init?.body || ''); } catch { return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400)); }
    const sites = activeSites();
    for (const item of body.items) {
      const site = sites.find(s => s.id === item.id);
      if (site) site.sortOrder = item.sortOrder;
    }
    // Resort sites within each category
    for (const cat of activeCategories()) {
      cat.sites.sort((a, b) => a.sortOrder - b.sortOrder);
    }
    syncPageCategories();
    return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '排序已更新', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/widgets`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: activeWidgets(), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/publish`) {
    const method = init?.method || 'POST';
    const page = activePage();
    if (method === 'POST') {
      page.publishSettings.isPublished = true;
      page.isPublished = true;
      page.publishedAt = new Date().toISOString();
      page.hasUnpublishedChanges = false;
      return Promise.resolve(jsonResponse({ code: 'OK', data: page.publishSettings, meta: { message: '已发布', detail: '' } }));
    }
    if (method === 'DELETE') {
      page.publishSettings.isPublished = false;
      page.isPublished = false;
      return Promise.resolve(jsonResponse({ code: 'OK', data: page.publishSettings, meta: { message: '已取消发布', detail: '' } }));
    }
    return Promise.resolve(jsonResponse({ code: 'OK', data: page.publishSettings, meta: { message: '', detail: '' } }));
  }
  if (url.startsWith(`${API_BASE}/navigation/public/`)) {
    const slug = url.split('/').pop() || '';
    if (slug === 'nav') {
      // 系统页公开视图（契约形状）
      const sysPub = contractPublishedResponse(
        { ...mockSystemPage, ownerName: 'nav.ax', ownerAvatar: '' },
        'system',
        mockSystemSubdomain.subdomain,
      );
      return Promise.resolve(jsonResponse({ code: 'OK', data: sysPub, meta: { message: '', detail: '' } }));
    }
    // 个人页公开视图（契约形状）
    const currentSub = getMockSubdomain();
    const published = contractPublishedResponse(mockPublishedPage, 'personal', currentSub?.subdomain || '');
    return Promise.resolve(jsonResponse({ code: 'OK', data: published, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/navigation/themes`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: mockThemes, meta: { message: '', detail: '' } }));
  }
  // Subdomain
  if (url === `${API_BASE}/navigation/subdomain`) {
    const method = init?.method || 'GET';
    if (method === 'GET') {
      if (isEditingSystem()) {
        return Promise.resolve(jsonResponse({ code: 'OK', data: {
          id: 'sub_system',
          userId: 'system',
          subdomain: mockSystemSubdomain.subdomain,
          status: mockSystemSubdomain.status,
          fullDomain: `${mockSystemSubdomain.subdomain}.nav.ax`,
          appliedAt: '2026-01-01T00:00:00Z',
          reviewedAt: '2026-01-01T00:00:00Z',
        }, meta: { message: '', detail: '' } }));
      }
      const current = getMockSubdomain();
      return Promise.resolve(jsonResponse({ code: 'OK', data: current, meta: { message: '', detail: '' } }));
    }
    if (method === 'POST') {
      let body: { subdomain?: string; label?: string };
      try {
        body = JSON.parse(init?.body || '');
      } catch {
        return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '请求格式错误', detail: '' } }, 400));
      }
      const label = body.label ?? body.subdomain ?? '';
      if (!label || label.length > 30 || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(label)) {
        return Promise.resolve(jsonResponse({ code: 'InvalidParameter', data: null, meta: { message: '子域名格式无效：仅支持小写字母、数字和连字符，不能以连字符开头或结尾', detail: '' } }, 400));
      }
      if (isEditingSystem()) {
        mockSystemSubdomain = { subdomain: label, status: 'approved' };
        return Promise.resolve(jsonResponse({ code: 'OK', data: {
          id: 'sub_system',
          userId: 'system',
          subdomain: label,
          status: 'approved',
          fullDomain: `${label}.nav.ax`,
          appliedAt: new Date().toISOString(),
        }, meta: { message: '子域名已更新', detail: '' } }));
      }
      const existing = getMockSubdomain();
      if (existing && existing.status !== 'rejected') {
        return Promise.resolve(jsonResponse({ code: 'Conflict', data: null, meta: { message: '你已有一个正在处理中的子域名申请', detail: '' } }, 409));
      }
      const result = setMockSubdomain(label);
      return Promise.resolve(jsonResponse({
        code: 'OK',
        data: result,
        meta: { message: result.status === 'approved' ? '子域名已自动启用' : '短子域名申请已提交，请等待审核', detail: '' },
      }));
    }
    if (method === 'DELETE') {
      if (isEditingSystem()) {
        return Promise.resolve(jsonResponse({ code: 'Forbidden', data: null, meta: { message: '系统子域名不可取消', detail: '' } }, 403));
      }
      cancelMockSubdomain();
      return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '申请已取消', detail: '' } }));
    }
  }
  return null;
});

// ---- Analytics handlers ----
handlers.push((url) => {
  if (url === `${API_BASE}/me/analytics/overview` || url.startsWith(`${API_BASE}/me/analytics/overview?`)) {
    const overview = mockAnalyticsResponse.overview;
    return Promise.resolve(jsonResponse({ code: 'OK', data: {
      totalPV: overview.totalPV,
      totalUV: overview.totalUV,
      todayPV: overview.todayPV,
      todayUV: overview.todayUV,
      pvChange: overview.pvChange,
      uvChange: overview.uvChange,
      bounceRate: overview.bounceRate > 1 ? overview.bounceRate / 100 : overview.bounceRate,
      averagePages: overview.avgSessionPages,
      avgSessionPages: overview.avgSessionPages,
      todayVisitors: overview.todayVisitors,
      visitorsChange: overview.visitorsChange,
    }, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/me/analytics/trends` || url.startsWith(`${API_BASE}/me/analytics/trends?`)) {
    const sp = new URL(url, 'http://x').searchParams;
    return Promise.resolve(jsonResponse({ code: 'OK', data: getDailyStatsForDays(Number(sp.get('period')) || 30), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/me/analytics/breakdown` || url.startsWith(`${API_BASE}/me/analytics/breakdown?`)) {
    const data = mockAnalyticsResponse;
    return Promise.resolve(jsonResponse({ code: 'OK', data: {
      topSites: data.topSites.map(item => ({ key: item.siteId, label: item.siteTitle, value: item.clicks, icon: item.siteIcon, categoryName: item.categoryName })),
      categories: data.categoryStats.map(item => ({ key: item.categoryName, label: item.categoryName, value: item.clicks, icon: item.categoryIcon })),
      devices: [],
      referrers: [],
      recentVisits: data.recentVisits.map(item => ({
        anonymousId: `visitor-${item.id}`,
        device: item.device,
        browser: item.browser,
        country: item.country,
        referrerDomain: item.referrer,
        visitedAt: item.visitedAt,
      })),
    }, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/analytics` || url.startsWith(`${API_BASE}/analytics?`)) {
    const sp = new URL(url, 'http://x').searchParams;
    const days = Number(sp.get('days')) || 30;
    const siteLimit = Number(sp.get('siteLimit')) || 15;
    const dailyStats = getDailyStatsForDays(days);
    const topSites = getTopSitesForLimit(siteLimit);
    const filteredOverview = {
      ...mockAnalyticsResponse.overview,
      totalPV: dailyStats.reduce((s, d) => s + d.pv, 0),
      totalUV: dailyStats.reduce((s, d) => s + d.uv, 0),
    };
    return Promise.resolve(jsonResponse({ code: 'OK', data: {
      ...mockAnalyticsResponse,
      overview: filteredOverview,
      dailyStats,
      topSites,
    }, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/analytics/daily` || url.startsWith(`${API_BASE}/analytics/daily?`)) {
    const sp = new URL(url, 'http://x').searchParams;
    const days = Number(sp.get('days')) || 30;
    return Promise.resolve(jsonResponse({ code: 'OK', data: getDailyStatsForDays(days), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/analytics/top-sites` || url.startsWith(`${API_BASE}/analytics/top-sites?`)) {
    const sp = new URL(url, 'http://x').searchParams;
    const limit = Number(sp.get('limit')) || 15;
    return Promise.resolve(jsonResponse({ code: 'OK', data: getTopSitesForLimit(limit), meta: { message: '', detail: '' } }));
  }
  return null;
});

// ---- Admin handlers ----
handlers.push((url, init) => {
  if (url === `${API_BASE}/admin/overview`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: mockAdminOverview, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/users` || url.startsWith(`${API_BASE}/admin/users?`)) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: paginate(mockUsers, 1, 10), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/invitations` || url.startsWith(`${API_BASE}/admin/invitations?`)) {
    const method = init?.method || 'GET';
    if (method === 'POST') {
      const token = `mock-invite-token-${Date.now()}`;
      const created = {
        id: `inv_${Date.now()}`,
        tokenPreview: `${token.slice(0, 8)}…`,
        creatorName: 'admin',
        maxUses: 10,
        usedCount: 0,
        expiresAt: new Date(Date.now() + 30 * 864e5).toISOString(),
        createdAt: new Date().toISOString(),
        token,
        inviteUrl: `${window.location.origin}/invite/${token}`,
        emailSent: false,
      };
      mockInvitations.unshift(created);
      return Promise.resolve(jsonResponse({ code: 'OK', data: created, meta: { message: '', detail: '' } }, 201));
    }
    return Promise.resolve(jsonResponse({ code: 'OK', data: paginate(mockInvitations, 1, 10), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/directory/sites` || url.startsWith(`${API_BASE}/admin/directory/sites?`)) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: paginate(mockPlatformSites, 1, 10), meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/directory/categories`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: mockPlatformCategories, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/themes`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: mockThemes, meta: { message: '', detail: '' } }));
  }
  if (url === `${API_BASE}/admin/settings`) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: mockSystemSettings, meta: { message: '', detail: '' } }));
  }
  if (url.startsWith(`${API_BASE}/admin/audit`)) {
    return Promise.resolve(jsonResponse({ code: 'OK', data: paginate(mockAuditEntries, 1, 20), meta: { message: '', detail: '' } }));
  }
  // Admin: all links management
  if (url === `${API_BASE}/admin/links` || url.startsWith(`${API_BASE}/admin/links?`)) {
    const sp = new URL(url, 'http://x').searchParams;
    const page = Number(sp.get('page')) || 1;
    const pageSize = Number(sp.get('pageSize')) || 15;
    const search = sp.get('search')?.toLowerCase() || '';
    const ownerId = sp.get('ownerId') || '';
    let filtered = mockAllLinks;
    if (search) {
      filtered = filtered.filter(l => l.title.toLowerCase().includes(search) || l.url.toLowerCase().includes(search) || l.ownerName.toLowerCase().includes(search));
    }
    if (ownerId) {
      filtered = filtered.filter(l => l.ownerId === ownerId);
    }
    return Promise.resolve(jsonResponse({ code: 'OK', data: paginate(filtered, page, pageSize), meta: { message: '', detail: '' } }));
  }
  // Admin: generate a password reset link for a user
  if (url.match(/\/admin\/users\/[^/]+\/password-reset$/)) {
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: {
        resetUrl: `${window.location.origin}/reset-password?token=mock-reset-token-abcdefghijklmnopqrstuvwxyz`,
        expiresAt: new Date(Date.now() + 3600000).toISOString(),
        emailSent: false,
      },
      meta: { message: '', detail: '' },
    }));
  }
  // Admin: delete link
  if (url.match(/\/admin\/links\/[^/]+$/)) {
    const id = url.split('/').pop()!;
    const idx = mockAllLinks.findIndex(l => l.id === id);
    if (idx === -1) return Promise.resolve(jsonResponse({ code: 'NotFound', data: null, meta: { message: '链接不存在', detail: '' } }, 404));
    mockAllLinks.splice(idx, 1);
    return Promise.resolve(jsonResponse({ code: 'OK', data: null, meta: { message: '链接已删除', detail: '' } }));
  }
  return null;
});

// ---- Install mock interceptor ----
const originalFetch = window.fetch;

function mapContractUrlToLegacy(url: string): string {
  const parsed = new URL(url, window.location.origin);
  const path = parsed.pathname;
  let legacyPath = path;
  let keepSearch = true;

  if (path.startsWith(`${API_BASE}/auth/invitations/`) && path.endsWith('/register')) {
    legacyPath = `${API_BASE}/auth/register`;
  } else if (path.startsWith(`${API_BASE}/auth/invitations/`)) {
    const token = path.split('/').pop();
    legacyPath = `${API_BASE}/auth/invite/${token}/validate`;
  } else if (path === `${API_BASE}/me/profile`) {
    legacyPath = `${API_BASE}/auth/profile`;
  } else if (path === `${API_BASE}/pages/current` || /^\/api\/v1\/pages\/[^/]+$/.test(path)) {
    legacyPath = `${API_BASE}/navigation/page`;
    keepSearch = false;
  } else if (/^\/api\/v1\/pages\/[^/]+\/categories(?:\/[^/]+)?$/.test(path)) {
    const categoryId = path.match(/\/categories\/([^/]+)$/)?.[1];
    legacyPath = `${API_BASE}/navigation/categories${categoryId ? `/${categoryId}` : ''}`;
    keepSearch = false;
  } else if (/^\/api\/v1\/pages\/[^/]+\/sites(?:\/[^/]+)?$/.test(path)) {
    const siteId = path.match(/\/sites\/([^/]+)$/)?.[1];
    legacyPath = `${API_BASE}/navigation/sites${siteId ? `/${siteId}` : ''}`;
  } else if (path === `${API_BASE}/public/home`) {
    legacyPath = `${API_BASE}/navigation/public/nav`;
  } else if (path.startsWith(`${API_BASE}/public/pages/`)) {
    legacyPath = `${API_BASE}/navigation/public/${path.split('/').pop()}`;
  } else if (path === `${API_BASE}/themes`) {
    legacyPath = `${API_BASE}/navigation/themes`;
  } else if (path === `${API_BASE}/public/directory`) {
    legacyPath = `${API_BASE}/navigation/directory`;
  } else if (path === `${API_BASE}/me/subdomain`) {
    legacyPath = `${API_BASE}/navigation/subdomain`;
  }

  return `${legacyPath}${keepSearch ? parsed.search : ''}`;
}

export function installMockApi() {
  window.fetch = async function (input: any, init?: any): Promise<Response> {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.startsWith(API_BASE)) {
      const mappedUrl = mapContractUrlToLegacy(url);
      for (const handler of handlers) {
        const result = await handler(mappedUrl, init);
        if (result) return result;
      }
      return jsonResponse({ code: 'MOCK_NOT_IMPLEMENTED', data: null, meta: { message: '该 Mock 路由未实现', detail: mappedUrl } }, 501);
    }

    return originalFetch.call(window, input, init);
  } as typeof fetch;
}

export function uninstallMockApi() {
  window.fetch = originalFetch;
}
