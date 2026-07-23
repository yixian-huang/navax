// ============================================================
// nav.ax Mock Data — realistic data for all entities
// ============================================================

import type {
  User,
  UserProfile,
  AuthSession,
  Invitation,
  LayoutConfig,
  PageSettings,
  PublishSettings,
  Category,
  Site,
  Widget,
  Theme,
  AdminOverview,
  AuditEntry,
  SystemHealth,
  PlatformSite,
  PlatformCategory,
  SystemSettings,
  PaginatedResponse,
  SubdomainInfo,
  SubdomainStatus,
  LegacySubdomainStatus,
  AdminLink,
} from '@/api/types';

// ---- Mock-internal page state ----
// Mock 用扁平可变结构作为内部“数据库”，响应时再投影为契约形状。
// 该结构仅存在于 dev mock 层，不属于生产契约。
export interface MockPageState {
  id: string;
  ownerId: string;
  ownerName: string;
  ownerAvatar: string;
  title: string;
  slug: string;
  description: string;
  isPublished: boolean;
  themeId: string;
  layout: LayoutConfig;
  background?: PageSettings['appearance']['background'];
  categories: Category[];
  widgets: Widget[];
  publishSettings: PublishSettings;
  draftUpdatedAt: string;
  publishedAt: string;
  updatedAt?: string;
  hasUnpublishedChanges: boolean;
}

export interface MockPublishedPage {
  id: string;
  ownerName: string;
  ownerAvatar: string;
  title: string;
  slug: string;
  description: string;
  themeId: string;
  layout: LayoutConfig;
  categories: Category[];
  widgets: Widget[];
  subdomain: string;
  subdomainStatus: SubdomainStatus | LegacySubdomainStatus;
  publishedAt: string;
  updatedAt: string;
}

// ---- Current mock user ----
type MockAuthenticatedSession = AuthSession & { authenticated: true; user: User };

export const mockAuthUser: MockAuthenticatedSession = {
  authenticated: true,
  user: {
    id: 'usr_001',
    username: 'lucaspeng',
    email: 'lucas@example.com',
    avatarUrl: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20male%20in%2030s%2C%20clean%20white%20background%2C%20warm%20natural%20lighting%2C%20modern%20professional%20look&width=120&height=120&seq=avatar-lucas&orientation=squarish',
    role: 'user',
    status: 'active',
    createdAt: '2025-08-15T08:00:00Z',
    updatedAt: '2026-07-10T14:30:00Z',
  },
};

export const mockAdminUser: MockAuthenticatedSession = {
  authenticated: true,
  user: {
    id: 'usr_admin',
    username: 'admin',
    email: 'admin@nav.ax',
    avatarUrl: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20female%20tech%20professional%2C%20clean%20white%20background%2C%20confident%20expression&width=120&height=120&seq=avatar-admin&orientation=squarish',
    role: 'admin',
    status: 'active',
    createdAt: '2025-06-01T00:00:00Z',
    updatedAt: '2026-07-14T09:00:00Z',
  },
};

// ---- User Profile ----
export const mockUserProfile: UserProfile = {
  ...mockAuthUser.user,
  bio: '前端开发者，开源爱好者。喜欢收集好用的工具和资源。',
};

// ---- Mock Categories (Personal) ----
export const mockCategories: Category[] = [
  {
    id: 'cat_001',
    pageId: 'page_001',
    name: '开发工具',
    icon: 'ri-code-s-slash-line',
    sortOrder: 0,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  {
    id: 'cat_002',
    pageId: 'page_001',
    name: '设计资源',
    icon: 'ri-palette-line',
    sortOrder: 1,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  {
    id: 'cat_003',
    pageId: 'page_001',
    name: '效率工具',
    icon: 'ri-rocket-line',
    sortOrder: 2,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  {
    id: 'cat_004',
    pageId: 'page_001',
    name: '学习资源',
    icon: 'ri-book-open-line',
    sortOrder: 3,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  {
    id: 'cat_005',
    pageId: 'page_001',
    name: '社交媒体',
    icon: 'ri-chat-3-line',
    sortOrder: 4,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  {
    id: 'cat_006',
    pageId: 'page_001',
    name: '资讯阅读',
    icon: 'ri-newspaper-line',
    sortOrder: 5,
    sites: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
];

// ---- Mock Sites (Personal) ----
export const mockSites: Site[] = [
  { id: 'site_001', categoryId: 'cat_001', title: 'GitHub', url: 'https://github.com', icon: 'ri-github-fill', description: '全球最大的代码托管平台', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_002', categoryId: 'cat_001', title: 'Stack Overflow', url: 'https://stackoverflow.com', icon: 'ri-stack-overflow-fill', description: '程序员问答社区', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_003', categoryId: 'cat_001', title: 'CodePen', url: 'https://codepen.io', icon: 'ri-codepen-fill', description: '在线代码编辑与分享', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_004', categoryId: 'cat_001', title: 'npm', url: 'https://npmjs.com', icon: 'ri-npmjs-fill', description: 'JavaScript 包管理器', sortOrder: 3, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_005', categoryId: 'cat_001', title: 'Vercel', url: 'https://vercel.com', icon: 'ri-vercel-fill', description: '前端部署平台', sortOrder: 4, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_006', categoryId: 'cat_001', title: 'VS Code', url: 'https://code.visualstudio.com', icon: 'ri-terminal-box-fill', description: '微软开源编辑器', sortOrder: 5, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_007', categoryId: 'cat_002', title: 'Figma', url: 'https://figma.com', icon: 'ri-pen-nib-fill', description: '协作设计工具', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_008', categoryId: 'cat_002', title: 'Dribbble', url: 'https://dribbble.com', icon: 'ri-dribbble-fill', description: '设计师作品展示', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_009', categoryId: 'cat_002', title: 'Unsplash', url: 'https://unsplash.com', icon: 'ri-image-line', description: '免费高质量图片', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_010', categoryId: 'cat_002', title: 'ColorHunt', url: 'https://colorhunt.co', icon: 'ri-paint-fill', description: '配色方案灵感', sortOrder: 3, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_011', categoryId: 'cat_003', title: 'Notion', url: 'https://notion.so', icon: 'ri-notion-fill', description: '全能笔记工具', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_012', categoryId: 'cat_003', title: 'Linear', url: 'https://linear.app', icon: 'ri-layout-4-fill', description: '现代项目管理', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_013', categoryId: 'cat_003', title: 'Todoist', url: 'https://todoist.com', icon: 'ri-check-double-line', description: '任务管理工具', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_014', categoryId: 'cat_003', title: 'Raycast', url: 'https://raycast.com', icon: 'ri-flashlight-fill', description: 'macOS 效率启动器', sortOrder: 3, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_015', categoryId: 'cat_004', title: 'MDN', url: 'https://developer.mozilla.org', icon: 'ri-book-2-fill', description: 'Web 技术权威文档', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_016', categoryId: 'cat_004', title: 'freeCodeCamp', url: 'https://freecodecamp.org', icon: 'ri-code-box-fill', description: '免费编程学习', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_017', categoryId: 'cat_004', title: 'LeetCode', url: 'https://leetcode.com', icon: 'ri-brain-fill', description: '算法练习平台', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_018', categoryId: 'cat_005', title: 'Twitter / X', url: 'https://x.com', icon: 'ri-twitter-x-fill', description: '社交网络', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_019', categoryId: 'cat_005', title: 'Reddit', url: 'https://reddit.com', icon: 'ri-reddit-fill', description: '社区论坛', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_020', categoryId: 'cat_005', title: 'Discord', url: 'https://discord.com', icon: 'ri-discord-fill', description: '即时通讯社区', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_021', categoryId: 'cat_006', title: 'Hacker News', url: 'https://news.ycombinator.com', icon: 'ri-news-fill', description: '科技新闻聚合', sortOrder: 0, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_022', categoryId: 'cat_006', title: 'Dev.to', url: 'https://dev.to', icon: 'ri-file-code-fill', description: '开发者社区', sortOrder: 1, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_023', categoryId: 'cat_006', title: 'Medium', url: 'https://medium.com', icon: 'ri-medium-fill', description: '写作阅读平台', sortOrder: 2, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'site_024', categoryId: 'cat_006', title: 'RSS 订阅', url: 'https://feedly.com', icon: 'ri-rss-fill', description: 'RSS 阅读器', sortOrder: 3, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
];

// Populate categories with their sites
mockCategories.forEach(cat => {
  cat.sites = mockSites.filter(s => s.categoryId === cat.id).sort((a, b) => a.sortOrder - b.sortOrder);
});

// ---- Mock Widgets (Personal) ----
export const mockWidgets: Widget[] = [
  { id: 'wdg_001', pageId: 'page_001', type: 'clock', config: { timezone: 'Asia/Shanghai', format: '24h' }, position: { x: 0, y: 0 }, enabled: true, createdAt: '2026-06-01T00:00:00Z' },
  { id: 'wdg_002', pageId: 'page_001', type: 'date', config: { format: 'long', showLunar: true }, position: { x: 0, y: 1 }, enabled: true, createdAt: '2026-06-01T00:00:00Z' },
  { id: 'wdg_003', pageId: 'page_001', type: 'notes', config: { content: '今日待办：\n- 完成项目文档\n- 代码 review\n- 更新依赖' }, position: { x: 0, y: 2 }, enabled: true, createdAt: '2026-06-01T00:00:00Z' },
];

// ---- Mock Navigation Page (Personal) ----
export const mockNavigationPage: MockPageState = {
  id: 'page_001',
  ownerId: 'usr_001',
  ownerName: 'lucaspeng',
  ownerAvatar: mockAuthUser.user.avatarUrl,
  title: 'Lucas 的导航',
  slug: 'lucas',
  description: '收集好用的开发工具、设计资源和效率利器',
  isPublished: true,
  themeId: 'slate',
  layout: { density: 'comfortable', showClock: true, showDate: true, columns: 6, categoryStyle: 'tabs' },
  categories: mockCategories,
  widgets: mockWidgets,
  publishSettings: { title: 'Lucas 的导航', slug: 'lucas', description: '收集好用的开发工具、设计资源和效率利器', isPublished: true, customDomain: '', showAuthor: true },
  draftUpdatedAt: '2026-07-15T10:00:00Z',
  publishedAt: '2026-07-14T08:00:00Z',
  hasUnpublishedChanges: true,
};

// ============================================================
// System Page Data — nav.ax homepage (managed by admin in App)
// ============================================================

// ---- System Categories ----
export const mockSystemCategories: Category[] = [
  {
    id: 'sys_cat_001',
    pageId: 'page_system',
    name: '热门工具',
    icon: 'ri-fire-line',
    sortOrder: 0,
    sites: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-07-15T00:00:00Z',
  },
  {
    id: 'sys_cat_002',
    pageId: 'page_system',
    name: '开发必备',
    icon: 'ri-code-s-slash-line',
    sortOrder: 1,
    sites: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-07-15T00:00:00Z',
  },
  {
    id: 'sys_cat_003',
    pageId: 'page_system',
    name: '设计灵感',
    icon: 'ri-palette-line',
    sortOrder: 2,
    sites: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-07-15T00:00:00Z',
  },
  {
    id: 'sys_cat_004',
    pageId: 'page_system',
    name: '效率提升',
    icon: 'ri-rocket-line',
    sortOrder: 3,
    sites: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-07-15T00:00:00Z',
  },
  {
    id: 'sys_cat_005',
    pageId: 'page_system',
    name: '知识学习',
    icon: 'ri-book-open-line',
    sortOrder: 4,
    sites: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-07-15T00:00:00Z',
  },
];

// ---- System Sites ----
export const mockSystemSites: Site[] = [
  { id: 'sys_site_001', categoryId: 'sys_cat_001', title: 'Google', url: 'https://google.com', icon: 'ri-google-fill', description: '全球最流行的搜索引擎', sortOrder: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_002', categoryId: 'sys_cat_001', title: 'YouTube', url: 'https://youtube.com', icon: 'ri-youtube-fill', description: '视频分享与创作平台', sortOrder: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_003', categoryId: 'sys_cat_001', title: 'Wikipedia', url: 'https://wikipedia.org', icon: 'ri-earth-line', description: '自由百科全书', sortOrder: 2, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_004', categoryId: 'sys_cat_001', title: 'Reddit', url: 'https://reddit.com', icon: 'ri-reddit-fill', description: '社区内容聚合', sortOrder: 3, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_005', categoryId: 'sys_cat_002', title: 'GitHub', url: 'https://github.com', icon: 'ri-github-fill', description: '全球最大的代码托管平台', sortOrder: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_006', categoryId: 'sys_cat_002', title: 'Stack Overflow', url: 'https://stackoverflow.com', icon: 'ri-stack-overflow-fill', description: '程序员问答社区', sortOrder: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_007', categoryId: 'sys_cat_002', title: 'MDN', url: 'https://developer.mozilla.org', icon: 'ri-book-2-fill', description: 'Web 技术权威文档', sortOrder: 2, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_008', categoryId: 'sys_cat_002', title: 'Docker Hub', url: 'https://hub.docker.com', icon: 'ri-server-fill', description: '容器镜像仓库', sortOrder: 3, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_009', categoryId: 'sys_cat_003', title: 'Figma', url: 'https://figma.com', icon: 'ri-pen-nib-fill', description: '协作设计工具', sortOrder: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_010', categoryId: 'sys_cat_003', title: 'Dribbble', url: 'https://dribbble.com', icon: 'ri-dribbble-fill', description: '设计师作品展示', sortOrder: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_011', categoryId: 'sys_cat_003', title: 'Behance', url: 'https://behance.net', icon: 'ri-behance-line', description: '创意作品展示', sortOrder: 2, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_012', categoryId: 'sys_cat_003', title: 'Unsplash', url: 'https://unsplash.com', icon: 'ri-image-line', description: '免费高质量图片', sortOrder: 3, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_013', categoryId: 'sys_cat_004', title: 'Notion', url: 'https://notion.so', icon: 'ri-notion-fill', description: '全能笔记工具', sortOrder: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_014', categoryId: 'sys_cat_004', title: 'ChatGPT', url: 'https://chat.openai.com', icon: 'ri-chat-3-line', description: 'AI 对话助手', sortOrder: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_015', categoryId: 'sys_cat_004', title: 'DeepL', url: 'https://deepl.com', icon: 'ri-translate', description: '精准翻译工具', sortOrder: 2, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_016', categoryId: 'sys_cat_004', title: 'Excalidraw', url: 'https://excalidraw.com', icon: 'ri-pencil-ruler-line', description: '手绘风格白板', sortOrder: 3, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_017', categoryId: 'sys_cat_005', title: 'Coursera', url: 'https://coursera.org', icon: 'ri-book-2-line', description: '在线课程平台', sortOrder: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_018', categoryId: 'sys_cat_005', title: 'freeCodeCamp', url: 'https://freecodecamp.org', icon: 'ri-code-box-fill', description: '免费编程学习', sortOrder: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_019', categoryId: 'sys_cat_005', title: 'Khan Academy', url: 'https://khanacademy.org', icon: 'ri-lightbulb-line', description: '可汗学院教育', sortOrder: 2, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'sys_site_020', categoryId: 'sys_cat_005', title: 'W3Schools', url: 'https://w3schools.com', icon: 'ri-code-box-line', description: 'Web 技术教程', sortOrder: 3, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
];

// Populate system categories
mockSystemCategories.forEach(cat => {
  cat.sites = mockSystemSites.filter(s => s.categoryId === cat.id).sort((a, b) => a.sortOrder - b.sortOrder);
});

// ---- System Widgets ----
export const mockSystemWidgets: Widget[] = [
  { id: 'sys_wdg_001', pageId: 'page_system', type: 'clock', config: { timezone: 'Asia/Shanghai', format: '24h' }, position: { x: 0, y: 0 }, enabled: true, createdAt: '2026-01-01T00:00:00Z' },
  { id: 'sys_wdg_002', pageId: 'page_system', type: 'date', config: { format: 'long', showLunar: false }, position: { x: 0, y: 1 }, enabled: true, createdAt: '2026-01-01T00:00:00Z' },
  { id: 'sys_wdg_003', pageId: 'page_system', type: 'notes', config: { content: 'nav.ax 导航站\n精选实用工具与资源\n欢迎探索' }, position: { x: 0, y: 2 }, enabled: true, createdAt: '2026-01-01T00:00:00Z' },
];

// ---- System Navigation Page ----
export const mockSystemPage: MockPageState = {
  id: 'page_system',
  ownerId: 'system',
  ownerName: 'nav.ax',
  ownerAvatar: '',
  title: 'nav.ax 导航',
  slug: 'nav',
  description: '精选实用工具与资源导航',
  isPublished: true,
  themeId: 'slate',
  layout: { density: 'comfortable', showClock: true, showDate: true, columns: 5, categoryStyle: 'tabs' },
  categories: mockSystemCategories,
  widgets: mockSystemWidgets,
  publishSettings: { title: 'nav.ax 导航', slug: 'nav', description: '精选实用工具与资源导航', isPublished: true, customDomain: 'nav.ax', showAuthor: false },
  draftUpdatedAt: '2026-07-15T10:00:00Z',
  publishedAt: '2026-07-15T08:00:00Z',
  hasUnpublishedChanges: false,
};

// ---- Mock Published Page ----
export const mockPublishedPage: MockPublishedPage = {
  id: 'page_001',
  ownerName: 'lucaspeng',
  ownerAvatar: mockAuthUser.user.avatarUrl,
  title: 'Lucas 的导航',
  slug: 'lucas',
  description: '收集好用的开发工具、设计资源和效率利器',
  themeId: 'slate',
  layout: { density: 'comfortable', showClock: true, showDate: true, columns: 6, categoryStyle: 'tabs' },
  categories: mockCategories,
  widgets: mockWidgets,
  subdomain: '',
  subdomainStatus: 'none',
  publishedAt: '2026-07-14T08:00:00Z',
  updatedAt: '2026-07-14T08:00:00Z',
};

// ---- Mock Themes ----
export const mockThemes: Theme[] = [
  {
    id: 'slate', name: 'Slate', subtitle: '克制·当代', version: '1.0.0', author: 'nav.ax',
    description: '中性冷灰基底，克制的当代排版。', mode: 'light', preview: '',
    enabled: true, default: true, isDefault: true, isActive: true,
    currentVersionId: 'v00000000000000000000000000000001',
    cssHref: '/api/v1/public/themes/v00000000000000000000000000000001.css',
    tier: 1, scope: 'catalog', vibe: 'serious',
    swatches: ['#fafafa', '#8a8f98', '#1c1f24'],
  },
  {
    id: 'slate-dark', name: 'Slate Dark', subtitle: '克制·夜间', version: '1.0.0', author: 'nav.ax',
    description: '深色版本的中性冷灰。', mode: 'dark', preview: '',
    enabled: true, default: false, isDefault: false, isActive: false,
    currentVersionId: 'v00000000000000000000000000000002',
    cssHref: '/api/v1/public/themes/v00000000000000000000000000000002.css',
    tier: 1, scope: 'catalog', vibe: 'serious',
    swatches: ['#16181d', '#6f757e', '#e8eaed'],
  },
  {
    id: 'sakura', name: 'Sakura', subtitle: '樱花·魔法', version: '1.0.0', author: 'nav.ax',
    description: '梦幻樱花粉 × 薄荷绿点缀。', mode: 'light', preview: '',
    enabled: true, default: false, isDefault: false, isActive: false,
    currentVersionId: 'v00000000000000000000000000000003',
    cssHref: '/api/v1/public/themes/v00000000000000000000000000000003.css',
    tier: 1, scope: 'catalog', vibe: 'cute',
    swatches: ['#fef5f7', '#e88da5', '#8ecfba'],
  },
];

// ---- Mock Invitations ----
export const mockInvitations: Invitation[] = [
  { id: 'inv_001', tokenPreview: 'navalpha…', creatorName: 'admin', maxUses: 10, usedCount: 6, expiresAt: '2026-12-31T23:59:59Z', createdAt: '2026-06-01T00:00:00Z' },
  { id: 'inv_002', tokenPreview: 'devteam0…', creatorName: 'admin', maxUses: 5, usedCount: 5, expiresAt: '2026-08-15T23:59:59Z', createdAt: '2026-07-01T00:00:00Z' },
  { id: 'inv_003', tokenPreview: 'friends2…', creatorName: 'admin', maxUses: 20, usedCount: 0, expiresAt: '2026-09-30T23:59:59Z', createdAt: '2026-07-14T00:00:00Z' },
];

// ---- Mock Users (admin view) ----
export const mockUsers: User[] = [
  { id: 'usr_admin', username: 'admin', email: 'admin@nav.ax', avatarUrl: mockAdminUser.user.avatarUrl, role: 'admin', status: 'active', createdAt: '2025-06-01T00:00:00Z', updatedAt: '2026-07-14T09:00:00Z' },
  { id: 'usr_001', username: 'lucaspeng', email: 'lucas@example.com', avatarUrl: mockAuthUser.user.avatarUrl, role: 'user', status: 'active', createdAt: '2025-08-15T08:00:00Z', updatedAt: '2026-07-10T14:30:00Z' },
  { id: 'usr_002', username: 'sarahchen', email: 'sarah@example.com', avatarUrl: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', role: 'user', status: 'active', createdAt: '2025-10-20T12:00:00Z', updatedAt: '2026-07-12T16:00:00Z' },
  { id: 'usr_003', username: 'mikez', email: 'mike@example.com', avatarUrl: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Caucasian%20male%20engineer%2C%20clean%20white%20background%2C%20natural%20light&width=120&height=120&seq=avatar-mike&orientation=squarish', role: 'user', status: 'active', createdAt: '2026-01-05T09:00:00Z', updatedAt: '2026-07-13T11:00:00Z' },
  { id: 'usr_004', username: 'emilyliu', email: 'emily@example.com', avatarUrl: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', role: 'user', status: 'active', createdAt: '2026-02-14T10:00:00Z', updatedAt: '2026-07-14T08:30:00Z' },
  { id: 'usr_005', username: 'disabled_user', email: 'old@example.com', avatarUrl: '', role: 'user', status: 'disabled', createdAt: '2025-11-01T00:00:00Z', updatedAt: '2026-06-01T00:00:00Z' },
];

// ---- Mock Platform Sites ----
export const mockPlatformSites: PlatformSite[] = [
  { id: 'pls_001', title: 'GitHub', url: 'https://github.com', icon: 'ri-github-fill', description: '全球最大的代码托管平台', categoryId: 'plcat_001', categoryName: '开发工具', enabled: true, sortOrder: 0 },
  { id: 'pls_002', title: 'Figma', url: 'https://figma.com', icon: 'ri-pen-nib-fill', description: '协作设计工具', categoryId: 'plcat_002', categoryName: '设计资源', enabled: true, sortOrder: 0 },
  { id: 'pls_003', title: 'Notion', url: 'https://notion.so', icon: 'ri-notion-fill', description: '全能笔记工具', categoryId: 'plcat_003', categoryName: '效率工具', enabled: true, sortOrder: 0 },
  { id: 'pls_004', title: 'MDN', url: 'https://developer.mozilla.org', icon: 'ri-book-2-fill', description: 'Web 技术文档', categoryId: 'plcat_004', categoryName: '学习资源', enabled: true, sortOrder: 0 },
  { id: 'pls_005', title: 'YouTube', url: 'https://youtube.com', icon: 'ri-youtube-fill', description: '视频分享平台', categoryId: 'plcat_005', categoryName: '媒体娱乐', enabled: false, sortOrder: 0 },
];

// ---- Mock Platform Categories ----
export const mockPlatformCategories: PlatformCategory[] = [
  { id: 'plcat_001', name: '开发工具', icon: 'ri-code-s-slash-line', sortOrder: 0, enabled: true, siteCount: 1 },
  { id: 'plcat_002', name: '设计资源', icon: 'ri-palette-line', sortOrder: 1, enabled: true, siteCount: 1 },
  { id: 'plcat_003', name: '效率工具', icon: 'ri-rocket-line', sortOrder: 2, enabled: true, siteCount: 1 },
  { id: 'plcat_004', name: '学习资源', icon: 'ri-book-open-line', sortOrder: 3, enabled: true, siteCount: 1 },
  { id: 'plcat_005', name: '媒体娱乐', icon: 'ri-film-line', sortOrder: 4, enabled: true, siteCount: 1 },
];

// ---- Mock System Health ----
export const mockSystemHealth: SystemHealth = {
  status: 'healthy',
  uptimeSeconds: 1_231_920,
  version: '1.2.0',
  goVersion: 'go1.22.4',
  memoryBytes: 128 * 1024 * 1024,
};

// ---- Mock Audit Entries ----
export const mockAuditEntries: AuditEntry[] = [
  { id: 'aud_001', actor: 'admin', action: 'user.disable', target: 'disabled_user', detail: '管理员禁用用户 disabled_user', createdAt: '2026-07-14T22:00:00Z' },
  { id: 'aud_002', actor: 'lucaspeng', action: 'page.publish', target: 'Lucas 的导航', detail: '发布导航页更新', createdAt: '2026-07-14T20:30:00Z' },
  { id: 'aud_003', actor: 'sarahchen', action: 'page.create', target: 'Sarah 的设计资源', detail: '创建新的导航页', createdAt: '2026-07-14T18:00:00Z' },
  { id: 'aud_004', actor: 'admin', action: 'invitation.create', target: 'FRIENDS-2026', detail: '创建邀请链接 FRIENDS-2026', createdAt: '2026-07-14T09:00:00Z' },
  { id: 'aud_005', actor: 'mikez', action: 'site.add', target: 'Linear', detail: '添加站点 Linear 到效率工具分类', createdAt: '2026-07-13T15:00:00Z' },
  { id: 'aud_006', actor: 'emilyliu', action: 'theme.change', target: 'default-dark', detail: '切换主题到深色默认', createdAt: '2026-07-13T11:00:00Z' },
  { id: 'aud_007', actor: 'admin', action: 'directory.add', target: 'GitHub', detail: '添加平台推荐站点 GitHub', createdAt: '2026-07-13T08:00:00Z' },
  { id: 'aud_008', actor: 'admin', action: 'system.update', target: 'instanceName', detail: '更新实例名称为 nav.ax', createdAt: '2026-07-12T16:30:00Z' },
];

// ---- Mock Admin Overview ----
export const mockAdminOverview: AdminOverview = {
  totalUsers: 6,
  activeUsers: 5,
  activeInvitations: 2,
  publicPages: 3,
  recentActions: mockAuditEntries.slice(0, 5),
  health: mockSystemHealth,
};

// ---- Mock System Settings ----
export const mockSystemSettings: SystemSettings = {
  instanceName: 'nav.ax',
  publicBaseUrl: 'https://nav.ax',
  registrationMode: 'invite',
  limits: { maxCategoriesPerPage: 20, maxSitesPerPage: 100, maxUploadBytes: 10 * 1024 * 1024 },
  analytics: { enabled: true, retentionDays: 90 },
  domain: { rootDomain: 'nav.ax', subdomainsEnabled: true },
};

// ---- Pagination helper ----
export function paginate<T>(items: T[], page: number, pageSize: number): PaginatedResponse<T> {
  const total = items.length;
  const totalPages = Math.ceil(total / pageSize);
  const start = (page - 1) * pageSize;
  const paged = items.slice(start, start + pageSize);
  return { items: paged, page, pageSize, total, totalPages };
}

// ---- Mock Subdomain info (runtime state for testing - starts as 'none') ----
let mockSubdomainState: SubdomainInfo | null = null;

export function getMockSubdomain(): SubdomainInfo | null {
  return mockSubdomainState;
}

export function setMockSubdomain(subdomain: string): SubdomainInfo {
  const automaticallyApproved = subdomain.length >= 4;
  mockSubdomainState = {
    id: 'sub_001',
    userId: 'usr_001',
    subdomain,
    status: automaticallyApproved ? 'approved' : 'pending',
    fullDomain: `${subdomain}.nav.ax`,
    appliedAt: new Date().toISOString(),
    ...(automaticallyApproved ? { reviewedAt: new Date().toISOString() } : {}),
  };
  return mockSubdomainState;
}

export function approveMockSubdomain(): SubdomainInfo | null {
  if (!mockSubdomainState || mockSubdomainState.status !== 'pending') return null;
  mockSubdomainState = {
    ...mockSubdomainState,
    status: 'approved',
    reviewedAt: new Date().toISOString(),
  };
  return mockSubdomainState;
}

export function rejectMockSubdomain(reason: string): SubdomainInfo | null {
  if (!mockSubdomainState || mockSubdomainState.status !== 'pending') return null;
  mockSubdomainState = {
    ...mockSubdomainState,
    status: 'rejected',
    rejectionReason: reason,
    reviewedAt: new Date().toISOString(),
  };
  return mockSubdomainState;
}

export function cancelMockSubdomain(): void {
  if (mockSubdomainState?.status === 'pending') {
    mockSubdomainState = null;
  }
}

// ---- Mock All Links (admin view — across all users) ----
export const mockAllLinks: AdminLink[] = [
  // lucaspeng's links
  { id: 'link_001', title: 'GitHub', url: 'https://github.com', icon: 'ri-github-fill', description: '全球最大的代码托管平台', categoryName: '开发工具', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_002', title: 'Stack Overflow', url: 'https://stackoverflow.com', icon: 'ri-stack-overflow-fill', description: '程序员问答社区', categoryName: '开发工具', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_003', title: 'VS Code', url: 'https://code.visualstudio.com', icon: 'ri-terminal-box-fill', description: '微软开源编辑器', categoryName: '开发工具', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_004', title: 'Figma', url: 'https://figma.com', icon: 'ri-pen-nib-fill', description: '协作设计工具', categoryName: '设计资源', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_005', title: 'Dribbble', url: 'https://dribbble.com', icon: 'ri-dribbble-fill', description: '设计师作品展示', categoryName: '设计资源', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_006', title: 'Notion', url: 'https://notion.so', icon: 'ri-notion-fill', description: '全能笔记工具', categoryName: '效率工具', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_007', title: 'Linear', url: 'https://linear.app', icon: 'ri-layout-4-fill', description: '现代项目管理', categoryName: '效率工具', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_008', title: 'MDN', url: 'https://developer.mozilla.org', icon: 'ri-book-2-fill', description: 'Web 技术权威文档', categoryName: '学习资源', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  { id: 'link_009', title: 'LeetCode', url: 'https://leetcode.com', icon: 'ri-brain-fill', description: '算法练习平台', categoryName: '学习资源', ownerId: 'usr_001', ownerName: 'lucaspeng', ownerAvatar: mockAuthUser.user.avatarUrl, createdAt: '2026-01-10T00:00:00Z', updatedAt: '2026-07-01T00:00:00Z' },
  // sarahchen's links
  { id: 'link_010', title: 'Behance', url: 'https://behance.net', icon: 'ri-behance-line', description: '创意作品展示平台', categoryName: '设计资源', ownerId: 'usr_002', ownerName: 'sarahchen', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', createdAt: '2026-06-15T10:30:00Z', updatedAt: '2026-07-12T09:00:00Z' },
  { id: 'link_011', title: 'Pinterest', url: 'https://pinterest.com', icon: 'ri-pinterest-fill', description: '图片灵感收集', categoryName: '设计资源', ownerId: 'usr_002', ownerName: 'sarahchen', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', createdAt: '2026-06-15T10:31:00Z', updatedAt: '2026-07-12T09:01:00Z' },
  { id: 'link_012', title: 'Coolors', url: 'https://coolors.co', icon: 'ri-palette-line', description: '配色方案生成器', categoryName: '设计资源', ownerId: 'usr_002', ownerName: 'sarahchen', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', createdAt: '2026-06-15T10:32:00Z', updatedAt: '2026-07-12T09:02:00Z' },
  { id: 'link_013', title: 'LottieFiles', url: 'https://lottiefiles.com', icon: 'ri-movie-line', description: '轻量动画资源库', categoryName: '设计资源', ownerId: 'usr_002', ownerName: 'sarahchen', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', createdAt: '2026-06-16T08:00:00Z', updatedAt: '2026-07-13T14:00:00Z' },
  { id: 'link_014', title: 'Tailwind CSS', url: 'https://tailwindcss.com', icon: 'ri-tailwind-css-line', description: '实用优先的CSS框架', categoryName: '开发工具', ownerId: 'usr_002', ownerName: 'sarahchen', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20portrait%20of%20Asian%20female%20designer%20in%20her%2020s%2C%20clean%20white%20background%2C%20warm%20lighting&width=120&height=120&seq=avatar-sarah&orientation=squarish', createdAt: '2026-06-16T08:30:00Z', updatedAt: '2026-07-13T14:30:00Z' },
  // mikez's links
  { id: 'link_015', title: 'Docker', url: 'https://docker.com', icon: 'ri-server-fill', description: '容器化平台', categoryName: '开发工具', ownerId: 'usr_003', ownerName: 'mikez', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Caucasian%20male%20engineer%2C%20clean%20white%20background%2C%20natural%20light&width=120&height=120&seq=avatar-mike&orientation=squarish', createdAt: '2026-07-01T11:00:00Z', updatedAt: '2026-07-14T10:00:00Z' },
  { id: 'link_016', title: 'Kubernetes', url: 'https://kubernetes.io', icon: 'ri-cloud-fill', description: '容器编排平台', categoryName: '开发工具', ownerId: 'usr_003', ownerName: 'mikez', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Caucasian%20male%20engineer%2C%20clean%20white%20background%2C%20natural%20light&width=120&height=120&seq=avatar-mike&orientation=squarish', createdAt: '2026-07-01T11:05:00Z', updatedAt: '2026-07-14T10:05:00Z' },
  { id: 'link_017', title: 'Terraform', url: 'https://terraform.io', icon: 'ri-code-box-line', description: '基础设施即代码', categoryName: '开发工具', ownerId: 'usr_003', ownerName: 'mikez', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Caucasian%20male%20engineer%2C%20clean%20white%20background%2C%20natural%20light&width=120&height=120&seq=avatar-mike&orientation=squarish', createdAt: '2026-07-01T11:10:00Z', updatedAt: '2026-07-14T10:10:00Z' },
  { id: 'link_018', title: 'Grafana', url: 'https://grafana.com', icon: 'ri-line-chart-fill', description: '监控可视化平台', categoryName: '开发工具', ownerId: 'usr_003', ownerName: 'mikez', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Caucasian%20male%20engineer%2C%20clean%20white%20background%2C%20natural%20light&width=120&height=120&seq=avatar-mike&orientation=squarish', createdAt: '2026-07-02T09:00:00Z', updatedAt: '2026-07-14T16:00:00Z' },
  // emilyliu's links
  { id: 'link_019', title: 'Slack', url: 'https://slack.com', icon: 'ri-slack-fill', description: '团队沟通协作', categoryName: '效率工具', ownerId: 'usr_004', ownerName: 'emilyliu', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', createdAt: '2026-07-05T08:00:00Z', updatedAt: '2026-07-14T08:30:00Z' },
  { id: 'link_020', title: 'Jira', url: 'https://atlassian.com/jira', icon: 'ri-trello-fill', description: '敏捷项目管理', categoryName: '效率工具', ownerId: 'usr_004', ownerName: 'emilyliu', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', createdAt: '2026-07-05T08:10:00Z', updatedAt: '2026-07-14T08:31:00Z' },
  { id: 'link_021', title: 'Miro', url: 'https://miro.com', icon: 'ri-grid-fill', description: '在线协作白板', categoryName: '效率工具', ownerId: 'usr_004', ownerName: 'emilyliu', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', createdAt: '2026-07-05T08:15:00Z', updatedAt: '2026-07-14T08:32:00Z' },
  { id: 'link_022', title: 'Product Hunt', url: 'https://producthunt.com', icon: 'ri-rocket-line', description: '新产品发现平台', categoryName: '效率工具', ownerId: 'usr_004', ownerName: 'emilyliu', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', createdAt: '2026-07-05T08:20:00Z', updatedAt: '2026-07-14T08:33:00Z' },
  { id: 'link_023', title: 'Calendar', url: 'https://calendar.google.com', icon: 'ri-calendar-line', description: '日程管理工具', categoryName: '效率工具', ownerId: 'usr_004', ownerName: 'emilyliu', ownerAvatar: 'https://readdy.ai/api/search-image?query=Professional%20headshot%20of%20Asian%20female%20product%20manager%2C%20clean%20white%20background%2C%20warm%20studio%20lighting&width=120&height=120&seq=avatar-emily&orientation=squarish', createdAt: '2026-07-06T09:00:00Z', updatedAt: '2026-07-14T09:00:00Z' },
  // System page links (admin-managed homepage)
  { id: 'link_024', title: 'Google', url: 'https://google.com', icon: 'ri-google-fill', description: '全球最流行的搜索引擎', categoryName: '热门工具', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_025', title: 'ChatGPT', url: 'https://chat.openai.com', icon: 'ri-chat-3-line', description: 'AI 对话助手', categoryName: '热门工具', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_026', title: 'YouTube', url: 'https://youtube.com', icon: 'ri-youtube-fill', description: '视频分享与创作平台', categoryName: '热门工具', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_027', title: 'Wikipedia', url: 'https://wikipedia.org', icon: 'ri-earth-line', description: '自由百科全书', categoryName: '热门工具', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_028', title: 'DeepL', url: 'https://deepl.com', icon: 'ri-translate', description: '精准翻译工具', categoryName: '效率提升', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_029', title: 'Excalidraw', url: 'https://excalidraw.com', icon: 'ri-pencil-ruler-line', description: '手绘风格白板', categoryName: '效率提升', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
  { id: 'link_030', title: 'Coursera', url: 'https://coursera.org', icon: 'ri-book-2-line', description: '在线课程平台', categoryName: '知识学习', ownerId: 'system', ownerName: '系统首页', ownerAvatar: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-07-15T00:00:00Z' },
];
