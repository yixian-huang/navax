// ============================================================
// nav.ax Navigation API Service
// ============================================================

import { request, requestAttachment } from './client';
import type {
  ApiResponse,
  Category,
  CategoryContract,
  CreateCategoryRequest,
  CreateSiteRequest,
  DiscoveredPage,
  ExportFormat,
  ImportCommitRequest,
  ImportFormat,
  ImportPreview,
  ImportResult,
  LinkCheckResult,
  LayoutConfig,
  NavigationPage,
  NavigationPageContract,
  PageKind,
  PageSettings,
  PageSettingsUpdate,
  PaginatedResponse,
  PlatformSite,
  Publication,
  PublicationSettingsUpdate,
  PublishedNavigationPage,
  PublishedPageContract,
  PublishSettings,
  ReorderRequest,
  Site,
  SubdomainInfo,
  SubdomainRequest,
  Theme,
  UpdateSiteRequest,
  Widget,
} from './types';

type PageScope = PageKind;
type DeleteCategoryMode = 'reject-if-not-empty' | 'delete-sites' | 'move-to-uncategorized';

function withLegacyCategory(category: CategoryContract, sites: Site[] = []): Category {
  return { ...category, sites: sites.filter(site => site.categoryId === category.id) };
}

function defaultSettings(page: NavigationPage): PageSettings {
  return {
    layout: {
      template: 'full',
      density: page.layout.density,
      columns: page.layout.columns,
      categoryStyle: page.layout.categoryStyle,
    },
    appearance: {
      themeId: page.themeId,
      background: { type: 'none', value: '', opacity: 1 },
    },
    search: { defaultEngine: 'google', showEngineSelector: true },
    display: {
      showClock: page.layout.showClock,
      showDate: page.layout.showDate,
      showGreeting: true,
    },
    preferences: {
      locale: 'zh-CN',
      timezone: 'Asia/Shanghai',
      openLinksInNewTab: true,
    },
  };
}

function withLegacyPage(page: NavigationPageContract | NavigationPage): NavigationPage {
  if ('layout' in page && page.layout) return page as NavigationPage;

  const contract = page as NavigationPageContract;
  const categories = contract.categories.map(category => withLegacyCategory(category, contract.sites));
  const publication = contract.publication;
  const settings = contract.settings;

  return {
    ...contract,
    categories,
    ownerAvatar: '',
    slug: publication.slug,
    isPublished: publication.published,
    themeId: settings.appearance.themeId,
    layout: {
      density: settings.layout.density,
      showClock: settings.display.showClock,
      showDate: settings.display.showDate,
      columns: settings.layout.columns,
      categoryStyle: settings.layout.categoryStyle,
    },
    widgets: [],
    publishSettings: {
      title: contract.title,
      slug: publication.slug,
      description: contract.description,
      isPublished: publication.published,
      customDomain: publication.canonicalUrl ?? '',
      showAuthor: publication.showAuthor,
    },
    publishedAt: publication.publishedAt ?? '',
    hasUnpublishedChanges: publication.hasUnpublishedChanges,
  };
}

function withLegacyPublishedPage(page: PublishedPageContract | PublishedNavigationPage): PublishedNavigationPage {
  if ('ownerName' in page) return page;
  return {
    ...page,
    ownerName: page.owner.name,
    ownerAvatar: page.owner.avatarUrl,
    themeId: page.settings.appearance.themeId,
    layout: {
      density: page.settings.layout.density,
      showClock: page.settings.display.showClock,
      showDate: page.settings.display.showDate,
      columns: page.settings.layout.columns,
      categoryStyle: page.settings.layout.categoryStyle,
    },
    widgets: [],
    subdomain: page.subdomain ?? '',
    subdomainStatus: page.subdomain ? 'approved' : 'none',
    updatedAt: page.publishedAt,
  };
}

function legacyPublication(settings: PublishSettings, current: Publication): PublicationSettingsUpdate {
  return {
    visibility: settings.isPublished ? (current.visibility === 'private' ? 'unlisted' : current.visibility) : 'private',
    slug: settings.slug,
    showAuthor: settings.showAuthor,
    seoTitle: settings.title,
    seoDescription: settings.description,
  };
}

function toLegacyPublishSettings(page: NavigationPage, publication: Publication): PublishSettings {
  return {
    title: publication.seoTitle || page.title,
    slug: publication.slug,
    description: publication.seoDescription || page.description,
    isPublished: publication.published,
    customDomain: publication.canonicalUrl ?? '',
    showAuthor: publication.showAuthor,
  };
}

function envelope<T, U>(response: ApiResponse<T>, data: U): ApiResponse<U> {
  return { ...response, data };
}

async function fetchPage(pageId: string): Promise<ApiResponse<NavigationPage>> {
  const response = await request<ApiResponse<NavigationPageContract | NavigationPage>>(`/pages/${encodeURIComponent(pageId)}`);
  return envelope(response, withLegacyPage(response.data));
}

/** 新代码的页面作用域 API；所有写操作显式绑定 pageId。 */
export function createPageNavigationApi(pageId: string) {
  const base = `/pages/${encodeURIComponent(pageId)}`;

  return {
    getPage: () => fetchPage(pageId),

    updatePage: (data: { expectedRevision: number; title?: string; description?: string }) =>
      request<ApiResponse<NavigationPageContract>>(base, { method: 'PATCH', body: data })
        .then(response => envelope(response, withLegacyPage(response.data))),

    getCategories: () =>
      request<ApiResponse<CategoryContract[]>>(`${base}/categories`),

    createCategory: (data: CreateCategoryRequest) =>
      request<ApiResponse<CategoryContract>>(`${base}/categories`, { method: 'POST', body: data }),

    updateCategory: (categoryId: string, data: Partial<CreateCategoryRequest>) =>
      request<ApiResponse<CategoryContract>>(`${base}/categories/${encodeURIComponent(categoryId)}`, { method: 'PATCH', body: data }),

    deleteCategory: (categoryId: string, mode: DeleteCategoryMode = 'reject-if-not-empty') =>
      request<ApiResponse<null>>(`${base}/categories/${encodeURIComponent(categoryId)}`, { method: 'DELETE', params: { mode } }),

    getSites: (categoryId?: string) =>
      request<ApiResponse<Site[]>>(`${base}/sites`, { params: { categoryId } }),

    createSite: (data: CreateSiteRequest) =>
      request<ApiResponse<Site>>(`${base}/sites`, { method: 'POST', body: data }),

    updateSite: (siteId: string, data: UpdateSiteRequest) =>
      request<ApiResponse<Site>>(`${base}/sites/${encodeURIComponent(siteId)}`, { method: 'PATCH', body: data }),

    deleteSite: (siteId: string) =>
      request<ApiResponse<null>>(`${base}/sites/${encodeURIComponent(siteId)}`, { method: 'DELETE' }),

    replaceContentOrder: (data: {
      expectedRevision: number;
      categories: { id: string; siteIds: string[] }[];
    }) => request<ApiResponse<{ draftRevision: number }>>(`${base}/content-order`, { method: 'PUT', body: data }),

    getSettings: () => request<ApiResponse<PageSettings>>(`${base}/settings`),

    replaceSettings: (data: PageSettingsUpdate) =>
      request<ApiResponse<PageSettings>>(`${base}/settings`, { method: 'PUT', body: data }),

    getPublication: () => request<ApiResponse<Publication>>(`${base}/publication`),

    replacePublication: (data: PublicationSettingsUpdate) =>
      request<ApiResponse<Publication>>(`${base}/publication`, { method: 'PUT', body: data }),

    publish: (expectedRevision: number, idempotencyKey = crypto.randomUUID()) =>
      request<ApiResponse<Publication>>(`${base}/publish`, {
        method: 'POST',
        headers: { 'Idempotency-Key': idempotencyKey },
        body: { expectedRevision },
      }),

    unpublish: () => request<ApiResponse<Publication>>(`${base}/publication`, { method: 'DELETE' }),

    previewImport: (format: ImportFormat, file: File) => {
      const body = new FormData();
      body.set('format', format);
      body.set('file', file);
      return request<ApiResponse<ImportPreview>>(`${base}/imports/preview`, { method: 'POST', body });
    },

    commitImport: (data: ImportCommitRequest, idempotencyKey: string) =>
      request<ApiResponse<ImportResult>>(`${base}/imports`, {
        method: 'POST',
        headers: { 'Idempotency-Key': idempotencyKey },
        body: data,
      }),

    exportPage: (format: ExportFormat) => requestAttachment(`${base}/export`, { format }),

    checkLinks: (siteIds: string[]) =>
      request<ApiResponse<LinkCheckResult[]>>(`${base}/link-checks`, {
        method: 'POST',
        body: { siteIds },
      }),
  };
}

let currentPageRequest: Promise<ApiResponse<NavigationPage>> | undefined;

async function getCurrentPage(scope: PageScope = 'personal'): Promise<ApiResponse<NavigationPage>> {
  const response = await request<ApiResponse<NavigationPageContract | NavigationPage>>('/pages/current', { params: { scope } });
  return envelope(response, withLegacyPage(response.data));
}

function currentPage(scope: PageScope = 'personal'): Promise<ApiResponse<NavigationPage>> {
  currentPageRequest ??= getCurrentPage(scope).catch(error => {
    currentPageRequest = undefined;
    throw error;
  });
  return currentPageRequest;
}

async function currentScopedApi() {
  const page = await currentPage();
  return { page, api: createPageNavigationApi(page.data.id) };
}

/**
 * 兼容当前页面层的 API。新代码应使用 createPageNavigationApi(pageId)，
 * 避免系统页/个人页依赖内存中的隐式编辑模式。
 */
export const navigationApi = {
  forPage: createPageNavigationApi,
  getCurrentPage,

  getMyPage: () => currentPage(),

  updateLayout: async (layout: LayoutConfig) => {
    const { page, api } = await currentScopedApi();
    const settings = page.data.settings ?? defaultSettings(page.data);
    await api.replaceSettings({
      ...settings,
      layout: { ...settings.layout, density: layout.density, columns: layout.columns, categoryStyle: layout.categoryStyle },
      display: { ...settings.display, showClock: layout.showClock, showDate: layout.showDate },
      expectedRevision: page.data.draftRevision ?? 0,
    });
    currentPageRequest = undefined;
    return fetchPage(page.data.id);
  },

  getCategories: async () => {
    const { api } = await currentScopedApi();
    const response = await api.getCategories();
    return envelope(response, response.data.map(category => withLegacyCategory(category)));
  },
  createCategory: async (data: CreateCategoryRequest) => {
    const { api } = await currentScopedApi();
    const response = await api.createCategory(data);
    currentPageRequest = undefined;
    return envelope(response, withLegacyCategory(response.data));
  },
  updateCategory: async (id: string, data: Partial<CreateCategoryRequest>) => {
    const { api } = await currentScopedApi();
    const response = await api.updateCategory(id, data);
    currentPageRequest = undefined;
    return envelope(response, withLegacyCategory(response.data));
  },
  deleteCategory: async (id: string) => {
    const { api } = await currentScopedApi();
    const response = await api.deleteCategory(id);
    currentPageRequest = undefined;
    return response;
  },

  reorderCategories: async (data: ReorderRequest) => {
    const { page, api } = await currentScopedApi();
    const order = new Map(data.items.map(item => [item.id, item.sortOrder]));
    const categories = [...page.data.categories]
      .sort((a, b) => (order.get(a.id) ?? a.sortOrder) - (order.get(b.id) ?? b.sortOrder))
      .map(category => ({ id: category.id, siteIds: category.sites.map(site => site.id) }));
    const response = await api.replaceContentOrder({ expectedRevision: page.data.draftRevision ?? 0, categories });
    currentPageRequest = undefined;
    return envelope(response, null);
  },

  getSites: async (categoryId?: string) => {
    const { api } = await currentScopedApi();
    return api.getSites(categoryId);
  },
  createSite: async (data: CreateSiteRequest) => {
    const { api } = await currentScopedApi();
    const response = await api.createSite(data);
    currentPageRequest = undefined;
    return response;
  },
  updateSite: async (id: string, data: UpdateSiteRequest) => {
    const { api } = await currentScopedApi();
    const response = await api.updateSite(id, data);
    currentPageRequest = undefined;
    return response;
  },
  deleteSite: async (id: string) => {
    const { api } = await currentScopedApi();
    const response = await api.deleteSite(id);
    currentPageRequest = undefined;
    return response;
  },
  reorderSites: async (data: ReorderRequest) => {
    const { page, api } = await currentScopedApi();
    const order = new Map(data.items.map(item => [item.id, item.sortOrder]));
    const categories = page.data.categories.map(category => ({
      id: category.id,
      siteIds: [...category.sites]
        .sort((a, b) => (order.get(a.id) ?? a.sortOrder) - (order.get(b.id) ?? b.sortOrder))
        .map(site => site.id),
    }));
    const response = await api.replaceContentOrder({ expectedRevision: page.data.draftRevision ?? 0, categories });
    currentPageRequest = undefined;
    return envelope(response, null);
  },

  /** @deprecated OpenAPI v1 不再提供通用 widgets。 */
  getWidgets: async (): Promise<ApiResponse<Widget[]>> => {
    const page = await currentPage();
    return envelope(page, page.data.widgets);
  },
  /** @deprecated OpenAPI v1 不再提供通用 widgets。 */
  updateWidget: async (_id: string, _data: Partial<Widget>): Promise<ApiResponse<Widget>> => {
    throw new Error('OpenAPI v1 不支持通用小组件更新，请迁移到 PageSettings.display');
  },

  getPublishSettings: async () => {
    const { page, api } = await currentScopedApi();
    const response = await api.getPublication();
    return envelope(response, toLegacyPublishSettings(page.data, response.data));
  },
  updatePublishSettings: async (data: PublishSettings) => {
    const { page, api } = await currentScopedApi();
    const current = page.data.publication ?? (await api.getPublication()).data;
    const response = await api.replacePublication(legacyPublication(data, current));
    currentPageRequest = undefined;
    return envelope(response, toLegacyPublishSettings(page.data, response.data));
  },
  publish: async () => {
    const { page, api } = await currentScopedApi();
    await api.publish(page.data.draftRevision ?? 0);
    currentPageRequest = undefined;
    return fetchPage(page.data.id);
  },
  unpublish: async () => {
    const { page, api } = await currentScopedApi();
    await api.unpublish();
    currentPageRequest = undefined;
    return fetchPage(page.data.id);
  },

  getPublicPage: async (slug: string) => {
    const endpoint = slug === 'nav' ? '/public/home' : `/public/pages/${encodeURIComponent(slug)}`;
    const response = await request<ApiResponse<PublishedPageContract | PublishedNavigationPage>>(endpoint);
    return envelope(response, withLegacyPublishedPage(response.data));
  },

  getThemes: () => request<ApiResponse<Theme[]>>('/themes'),

  setTheme: async (themeId: string) => {
    const { page, api } = await currentScopedApi();
    const settings = page.data.settings ?? defaultSettings(page.data);
    await api.replaceSettings({
      ...settings,
      appearance: { ...settings.appearance, themeId },
      expectedRevision: page.data.draftRevision ?? 0,
    });
    currentPageRequest = undefined;
    return fetchPage(page.data.id);
  },

  getPlatformSites: async (params?: { category?: string; search?: string; page?: number; pageSize?: number }) => {
    const response = await request<ApiResponse<PlatformSite[]>>('/public/directory', {
      params: { categoryId: params?.category, search: params?.search, page: params?.page, pageSize: params?.pageSize },
    });
    const page = response.meta.page ?? params?.page ?? 1;
    const pageSize = response.meta.pageSize ?? params?.pageSize ?? response.data.length;
    const total = response.meta.total ?? response.data.length;
    const data: PaginatedResponse<PlatformSite> = {
      items: response.data,
      page,
      pageSize,
      total,
      totalPages: response.meta.totalPages ?? Math.ceil(total / Math.max(pageSize, 1)),
    };
    return envelope(response, data);
  },

  discoverPages: async (params?: { search?: string; tag?: string; sort?: 'latest' | 'popular' | 'featured'; page?: number; pageSize?: number }) => {
    const response = await request<ApiResponse<DiscoveredPage[]>>('/public/discover', { params });
    const page = response.meta.page ?? params?.page ?? 1;
    const pageSize = response.meta.pageSize ?? params?.pageSize ?? response.data.length;
    const total = response.meta.total ?? response.data.length;
    const data: PaginatedResponse<DiscoveredPage> = {
      items: response.data,
      page,
      pageSize,
      total,
      totalPages: response.meta.totalPages ?? Math.ceil(total / Math.max(pageSize, 1)),
    };
    return envelope(response, data);
  },

  getSubdomain: async () => {
    const response = await request<ApiResponse<SubdomainInfo | null>>('/me/subdomain');
    if (!response.data) return response;
    return envelope(response, { ...response.data, subdomain: response.data.subdomain ?? response.data.label });
  },
  applySubdomain: async (data: SubdomainRequest) => {
    const label = data.label ?? data.subdomain ?? '';
    const response = await request<ApiResponse<SubdomainInfo>>('/me/subdomain', { method: 'POST', body: { label } });
    return envelope(response, { ...response.data, subdomain: response.data.subdomain ?? response.data.label });
  },
  cancelSubdomainApplication: () =>
    request<ApiResponse<null>>('/me/subdomain', { method: 'DELETE' }),
};
