// ============================================================
// nav.ax — API TypeScript Types
// ============================================================

export type Role = 'user' | 'admin';
export type SupportedWidgetType = 'clock' | 'date';
/**
 * @deprecated OpenAPI v1 已将时钟和日期收敛到 PageSettings.display，不再提供通用小组件接口。
 * 这些类型仅供显式启用的开发 Mock 兼容，生产 API 不使用。
 */
export type LegacyWidgetType = 'notes' | 'weather' | 'quote';
export type WidgetType = SupportedWidgetType | LegacyWidgetType;
export type Density = 'list' | 'compact' | 'comfortable';
export type ThemeMode = 'light' | 'dark' | 'both';
export type PublishStatus = 'draft' | 'published' | 'failed';
export type UserStatus = 'active' | 'disabled';
export type ContractSubdomainStatus = 'pending' | 'approved' | 'rejected' | 'revoked';
/** @deprecated `none` 表示无申请，契约使用 data: null。 */
export type LegacySubdomainStatus = 'none';
/** @deprecated 旧页面状态；新 API 使用 ContractSubdomainStatus 并以 null 表示无申请。 */
export type SubdomainStatus = Exclude<ContractSubdomainStatus, 'revoked'> | LegacySubdomainStatus;
export type Visibility = 'private' | 'unlisted' | 'public';
export type PageKind = 'personal' | 'system';
export type LayoutTemplate = 'full' | 'search-focus' | 'browse-first' | 'sidebar';

export interface User {
  id: string;
  username: string;
  email: string;
  avatarUrl: string;
  /** OpenAPI v1 字段；旧 Mock 数据迁移期可缺省。 */
  bio?: string;
  role: Role;
  status: UserStatus;
  createdAt: string;
  updatedAt: string;
}

export interface UserProfile {
  id: string;
  username: string;
  email: string;
  avatarUrl: string;
  bio: string;
  role: Role;
  status: UserStatus;
  createdAt: string;
  updatedAt: string;
}

export interface Invitation {
  id: string;
  tokenPreview?: string;
  email?: string | null;
  creatorName?: string;
  maxUses: number;
  usedCount: number;
  expiresAt: string;
  revokedAt?: string | null;
  createdAt: string;
  /** @deprecated 旧管理页兼容字段。 */
  code?: string;
  /** @deprecated 旧管理页兼容字段。 */
  createdBy?: string;
  /** @deprecated 使用 revokedAt 判断。 */
  isRevoked?: boolean;
}

/** Create response only — full token is never stored server-side in plain form. */
export interface InvitationCreated extends Invitation {
  token: string;
  inviteUrl: string;
  emailSent?: boolean;
}

export interface Site {
  id: string;
  categoryId: string;
  title: string;
  url: string;
  icon: string;
  description: string;
  sortOrder: number;
  /**
   * Draft visibility. Present and required on draft contracts; omitted from
   * public/published projections (treat missing as visible).
   */
  enabled?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Category {
  id: string;
  pageId: string;
  name: string;
  icon: string;
  sortOrder: number;
  /**
   * Draft visibility. Present and required on draft contracts; omitted from
   * public/published projections (treat missing as visible).
   */
  enabled?: boolean;
  /** @deprecated 草稿契约通过 NavigationPage.sites 返回；仅公开快照嵌套 sites。 */
  sites: Site[];
  createdAt: string;
  updatedAt: string;
}

export type CategoryContract = Omit<Category, 'sites'>;

export interface Widget {
  id: string;
  pageId: string;
  type: WidgetType;
  config: Record<string, unknown>;
  position: { x: number; y: number };
  enabled: boolean;
  createdAt: string;
}

/** @deprecated 使用 PageSettings.layout 和 PageSettings.display。 */
export interface LayoutConfig {
  density: Density;
  showClock: boolean;
  showDate: boolean;
  columns: number;
  categoryStyle: 'tabs' | 'sidebar' | 'grid';
}

export interface PageSettings {
  layout: {
    template: LayoutTemplate;
    density: Density;
    columns: number;
    categoryStyle: 'tabs' | 'sidebar' | 'grid';
  };
  appearance: {
    themeId: string;
    background: {
      type: 'none' | 'color' | 'gradient' | 'image' | 'video';
      value: string;
      opacity: number;
      mediaId?: string | null;
      poster?: string | null;
    };
  };
  search: {
    defaultEngine: 'google' | 'bing' | 'duckduckgo' | 'baidu';
    showEngineSelector: boolean;
  };
  display: {
    showClock: boolean;
    showDate: boolean;
    showGreeting: boolean;
    subtitle?: string;
    clockFormat?: '24h' | '12h';
    dateFormat?: 'long' | 'short' | 'compact';
    showSeconds?: boolean;
  };
  preferences: {
    locale: string;
    timezone: string;
    openLinksInNewTab: boolean;
  };
}

export interface PageSettingsUpdate extends PageSettings {
  expectedRevision: number;
}

export interface Publication {
  visibility: Visibility;
  slug: string;
  showAuthor: boolean;
  seoTitle?: string;
  seoDescription?: string;
  /** Dedicated Open Graph image; empty falls back to background on publish. */
  seoImage?: string;
  published: boolean;
  canonicalUrl: string | null;
  robots?: 'noindex,follow' | 'index,follow';
  snapshotId: string | null;
  publishedRevision: number | null;
  publishedAt: string | null;
  hasUnpublishedChanges: boolean;
}

export interface PublicationSettingsUpdate {
  visibility: Visibility;
  slug: string;
  showAuthor: boolean;
  seoTitle?: string;
  seoDescription?: string;
  seoImage?: string;
}

/** OpenAPI v1 中的完整导航页草稿。 */
export interface NavigationPageContract {
  id: string;
  kind: PageKind;
  ownerId: string | null;
  ownerName: string;
  title: string;
  description: string;
  draftRevision: number;
  settings: PageSettings;
  categories: CategoryContract[];
  sites: Site[];
  publication: Publication;
  draftUpdatedAt: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * 迁移期页面模型。新代码应优先读取 settings/publication/sites。
 * API 适配层会将 OpenAPI 草稿模型归一化为此页面模型；兼容字段仅服务显式开发 Mock。
 */
// UI 草稿页模型：等价于契约，仅把 categories 换成内嵌 sites 的结构。
// 其余字段（settings、publication、draftRevision 等）直接来自契约。
export interface NavigationPage extends Omit<NavigationPageContract, 'categories'> {
  categories: Category[];
  /** 草稿契约不返回作者头像，规范化时补空串。 */
  ownerAvatar?: string;
}

/** @deprecated 使用 Publication/PublicationSettingsUpdate。 */
export interface PublishSettings {
  title: string;
  slug: string;
  description: string;
  isPublished: boolean;
  customDomain: string;
  showAuthor: boolean;
  seoImage?: string;
}

// UI 已发布页模型：契约字段 + owner 展平后的便捷字段。
export interface PublishedNavigationPage {
  id: string;
  snapshotId?: string;
  kind?: PageKind;
  visibility?: Exclude<Visibility, 'private'>;
  settings?: PageSettings;
  etag?: string;
  ownerName: string;
  ownerAvatar: string;
  title: string;
  description: string;
  categories: Category[];
  subdomain: string;
  subdomainStatus: SubdomainStatus | LegacySubdomainStatus;
  publishedAt: string;
  updatedAt: string;
}

export interface PublishedPageContract {
  id: string;
  snapshotId: string;
  kind: PageKind;
  title: string;
  description: string;
  seoTitle?: string;
  seoDescription?: string;
  ogImage?: string;
  slug: string;
  visibility: Exclude<Visibility, 'private'>;
  owner: { name: string; avatarUrl: string; visible: boolean };
  settings: PageSettings;
  categories: Category[];
  subdomain?: string | null;
  publishedAt: string;
  etag: string;
}

/** 前端使用的公开/预览页面模型（与契约一致）。 */
export type PublishedPage = PublishedPageContract;

export interface Theme {
  id: string;
  name: string;
  version?: string;
  author: string;
  description?: string;
  mode: ThemeMode;
  preview: string;
  enabled?: boolean;
  default?: boolean;
  /** @deprecated 使用 default。 */
  isDefault?: boolean;
  /** @deprecated 使用 enabled。 */
  isActive?: boolean;
}

export type AssetKind = 'avatar' | 'background' | 'site-icon';

export type BackgroundMediaScope = 'instance' | 'user';
export type BackgroundMediaKind = 'image' | 'video';

export interface BackgroundMedia {
  id: string;
  scope: BackgroundMediaScope;
  ownerUserId?: string | null;
  assetId: string;
  mediaKind: BackgroundMediaKind;
  mimeType: string;
  url: string;
  posterUrl?: string | null;
  width: number;
  height: number;
  durationMs?: number | null;
  sizeBytes: number;
  sortOrder: number;
  enabled: boolean;
  createdAt: string;
}

export interface Asset {
  id: string;
  kind: AssetKind;
  url: string;
  mimeType: string;
  size: number;
  createdAt: string;
}

export interface PublicConfig {
  instanceName: string;
  publicBaseUrl: string;
  rootDomain: string | null;
  registrationMode: 'invite' | 'closed' | 'open';
  features: {
    discover: boolean;
    analytics: boolean;
    subdomains: boolean;
    mail: boolean;
  };
  limits: {
    maxCategoriesPerPage: number;
    maxSitesPerPage: number;
    maxUploadBytes: number;
  };
}

export interface ThemeManifest {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  mode: ThemeMode;
  tokens: Record<string, Record<string, string>>;
  preview: string;
  license: string;
  homepage: string;
}

export interface AdminOverview {
  totalUsers: number;
  activeUsers: number;
  activeInvitations: number;
  publicPages: number;
  recentActions: AuditEntry[];
  health: SystemHealth;
}

export interface AuditEntry {
  id: string;
  actor: string;
  action: string;
  target: string;
  detail: string;
  createdAt: string;
}

export interface SystemHealth {
  status: 'healthy' | 'degraded';
  uptimeSeconds: number;
  version: string;
  goVersion: string;
  memoryBytes: number;
}

export interface PlatformSite {
  id: string;
  title: string;
  url: string;
  icon: string;
  description: string;
  categoryId: string;
  categoryName: string;
  enabled: boolean;
  sortOrder: number;
}

export interface PlatformCategory {
  id: string;
  name: string;
  icon: string;
  sortOrder: number;
  enabled: boolean;
  siteCount: number;
}

// Admin: global link management — site record with owner info
export interface AdminLink {
  id: string;
  title: string;
  url: string;
  icon: string;
  description: string;
  categoryName: string;
  ownerId: string;
  ownerName: string;
  ownerAvatar: string;
  createdAt: string;
  updatedAt: string;
}

export interface SystemSettings {
  instanceName: string;
  publicBaseUrl: string;
  registrationMode: 'invite' | 'closed' | 'open';
  limits: {
    maxCategoriesPerPage: number;
    maxSitesPerPage: number;
    maxUploadBytes: number;
  };
  analytics: {
    enabled: boolean;
    retentionDays: number;
  };
  domain: {
    rootDomain: string | null;
    subdomainsEnabled: boolean;
  };
}

export type ProviderKind = 'smtp' | 'storage' | 'dns' | 'oauth_google' | 'oauth_github';

export interface ProviderSummary {
  kind: ProviderKind;
  enabled: boolean;
  configured: boolean;
  hasSecret: boolean;
  updatedAt: string | null;
}

export interface ProviderConfig extends ProviderSummary {
  settings: Record<string, unknown>;
}

export interface SmtpSettings {
  host: string;
  port: number;
  tlsMode: 'none' | 'starttls' | 'tls';
  username: string;
  fromName: string;
  fromAddress: string;
}

export interface StorageSettings {
  driver: 'local' | 's3';
  endpoint?: string;
  region?: string;
  bucket?: string;
  prefix?: string;
  pathStyle?: boolean;
  accessKey?: string;
  publicBaseUrl?: string;
}

export interface DnsSettings {
  provider: string;
  zoneId: string;
  apiEndpoint?: string;
  ttl: number;
}

export interface OAuthAppSettings {
  clientId: string;
}

export type ProviderSettings = SmtpSettings | StorageSettings | DnsSettings | OAuthAppSettings;

export interface ProviderConfigUpdate {
  enabled: boolean;
  settings: ProviderSettings;
  secrets?: Record<string, string>;
}

export interface ProviderTestResult {
  success: boolean;
  durationMs: number;
  message: string;
}

export type UpdateStatus = 'idle' | 'checking' | 'available' | 'downloading' | 'applying' | 'restart-required' | 'failed';

export interface UpdateState {
  currentVersion: string;
  latestVersion: string | null;
  deployment: 'binary' | 'container' | 'development';
  channel: 'stable';
  autoCheck: boolean;
  autoApply: boolean;
  maintenanceWindow: string | null;
  status: UpdateStatus;
  releaseNotes: string;
  checkedAt: string | null;
  error: string;
}

export interface UpdateSettingsPatch {
  autoCheck?: boolean;
  autoApply?: boolean;
  maintenanceWindow?: string | null;
}

export interface Backup {
  id: string;
  reason: 'manual' | 'pre-update' | 'scheduled';
  size: number;
  sha256: string;
  createdAt: string;
}

export interface RestoreToken {
  restoreToken: string;
  expiresAt: string;
}

export interface AdminSubdomainRequest {
  id: string;
  userId: string;
  username?: string;
  label: string;
  fullDomain: string;
  status: ContractSubdomainStatus;
  appliedAt: string;
  reviewedAt: string | null;
  reason: string;
}

export interface SubdomainReviewRequest {
  decision: 'approve' | 'reject' | 'revoke';
  reason?: string;
}

export interface ApiResponse<T> {
  code: string;
  data: T;
  meta: {
    requestId?: string;
    message?: string;
    detail?: string;
    page?: number;
    pageSize?: number;
    total?: number;
    totalPages?: number;
  };
}

export interface PaginatedResponse<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
}

/** `/auth/session` 的契约模型；未登录时 user 必须为 null。 */
export interface AuthSession {
  authenticated: boolean;
  user: User | null;
  expiresAt?: string | null;
}

export interface LoginRequest {
  /** Preferred: email or username */
  account?: string;
  /** Legacy alias for account (email or username) */
  email?: string;
  username?: string;
  password: string;
}

export interface BootstrapStatus {
  initialized: boolean;
  setupRequired: boolean;
  version: string;
  instanceName: string;
  publicBaseUrl: string | null;
}

export interface BootstrapRequest {
  adminUsername: string;
  adminEmail: string;
  adminPassword: string;
  instanceName: string;
  publicBaseUrl: string;
}

export interface InviteRegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface UpdateProfileRequest {
  username?: string;
  bio?: string;
  avatarUrl?: string;
}

export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
  revokeOtherSessions?: boolean;
}

export interface SessionInfo {
  id: string;
  current: boolean;
  device: string;
  approximateLocation?: string;
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
}

export interface CreateCategoryRequest {
  name: string;
  icon: string;
  enabled?: boolean;
}

export interface CreateSiteRequest {
  categoryId: string;
  title: string;
  url: string;
  icon?: string;
  description?: string;
  enabled?: boolean;
}

export interface UpdateSiteRequest {
  title?: string;
  url?: string;
  icon?: string;
  description?: string;
  categoryId?: string;
  sortOrder?: number;
  enabled?: boolean;
}

export interface BatchSitesEnabledRequest {
  siteIds: string[];
  enabled: boolean;
  expectedRevision: number;
}

export interface ReorderRequest {
  items: { id: string; sortOrder: number }[];
}

export interface CreateInvitationRequest {
  email?: string;
  maxUses: number;
  expiresInDays: number;
  sendEmail?: boolean;
}

export interface PasswordResetLink {
  resetUrl: string;
  expiresAt: string;
  emailSent: boolean;
}

export interface CreatePlatformSiteRequest {
  title: string;
  url: string;
  icon: string;
  description: string;
  categoryId: string;
  enabled: boolean;
}

export type UpdatePlatformSiteRequest = Partial<CreatePlatformSiteRequest>;

export interface DirectoryCategoryInput {
  name: string;
  icon: string;
  enabled: boolean;
}

export type ImportFormat = 'bookmarks-html' | 'navax-json';
export type ExportFormat = 'navax-json' | 'bookmarks-html';

export interface ImportPreviewSite {
  sourceId: string;
  title: string;
  url: string;
  duplicate: boolean;
  valid: boolean;
  enabled: boolean;
  error?: string;
}

export interface ImportPreviewCategory {
  sourceId: string;
  name: string;
  enabled: boolean;
  sites: ImportPreviewSite[];
}

export interface ImportPreview {
  importToken: string;
  expiresAt: string;
  categories: ImportPreviewCategory[];
  totals: {
    categories: number;
    sites: number;
    duplicates: number;
    invalid: number;
  };
}

export interface ImportCommitRequest {
  importToken: string;
  mode: 'merge' | 'replace';
  selectedSiteIds: string[];
  expectedRevision: number;
  /** When set, overrides every selected site's enabled on commit (bookmark import UI). */
  sitesEnabled?: boolean;
}

export interface ImportResult {
  categoriesCreated: number;
  sitesCreated: number;
  duplicatesSkipped: number;
  invalidSkipped: number;
  draftRevision: number;
}

export interface LinkCheckResult {
  siteId: string;
  status: 'reachable' | 'unreachable' | 'blocked' | 'timeout';
  httpStatus?: number | null;
  latencyMs?: number | null;
  checkedAt: string;
  message?: string;
}

export interface SubdomainRequest {
  label?: string;
  /** @deprecated 使用 label。 */
  subdomain?: string;
}

export interface SubdomainInfo {
  id: string;
  userId: string;
  label?: string;
  status: ContractSubdomainStatus | LegacySubdomainStatus;
  fullDomain: string;
  customDomain?: string | null;
  appliedAt: string;
  reviewedAt?: string | null;
  reason?: string;
  /** @deprecated 使用 label。 */
  subdomain?: string;
  /** @deprecated 使用 reason。 */
  rejectionReason?: string;
}

export interface PublicEventRequest {
  type: 'page_view' | 'site_click';
  pageId: string;
  snapshotId?: string;
  siteId?: string;
  clientEventId?: string;
}

// ---- Analytics ----
export interface AnalyticsOverview {
  totalPV: number;
  totalUV: number;
  todayPV: number;
  todayUV: number;
  pvChange: number;
  uvChange: number;
  avgSessionPages: number;
  bounceRate: number;
  todayVisitors: number;
  visitorsChange: number;
}

export interface MetricBucket {
  key: string;
  label: string;
  value: number;
  /** 仅 topSites/categories 分布返回：站点或分类图标。 */
  icon?: string;
  /** 仅 topSites 返回：站点所属分类名。 */
  categoryName?: string;
}

export interface AnalyticsBreakdown {
  topSites: MetricBucket[];
  categories: MetricBucket[];
  devices: MetricBucket[];
  referrers: MetricBucket[];
  recentVisits: AnalyticsRecentVisit[];
}

export interface DailyStat {
  date: string;
  pv: number;
  uv: number;
}

export interface TopSiteClick {
  siteId: string;
  siteTitle: string;
  siteIcon: string;
  categoryName: string;
  clicks: number;
  ctr: number;
}

export interface CategoryClickStat {
  categoryName: string;
  categoryIcon: string;
  clicks: number;
  percentage: number;
}

export interface AnalyticsRecentVisit {
  anonymousId: string;
  referrerDomain: string;
  device: string;
  /** 由 User-Agent 解析的浏览器族，未知为空。 */
  browser?: string;
  /** 入库时解析的 2 位国家码，未知为空。 */
  country?: string;
  visitedAt: string;
}

/** @deprecated 待统计页面切换到 AnalyticsRecentVisit 后删除，后端不会返回完整 IP。 */
export interface VisitRecord {
  id: string;
  visitorIp: string;
  country: string;
  referrer: string;
  device: 'desktop' | 'tablet' | 'mobile';
  browser: string;
  pageTitle: string;
  visitedAt: string;
}

export interface AnalyticsResponse {
  overview: AnalyticsOverview;
  dailyStats: DailyStat[];
  topSites: TopSiteClick[];
  categoryStats: CategoryClickStat[];
  recentVisits: VisitRecord[];
}

// ---- Discover ----
export interface DiscoveredPage {
  slug: string;
  title: string;
  description: string;
  ownerName: string;
  themeId: string;
  /** Wallpaper / OG still for the card hero. */
  coverImage?: string;
  tags: string[];
  featured?: boolean;
  viewCount: number;
  publishedAt: string;
}
