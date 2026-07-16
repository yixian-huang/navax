// ============================================================
// nav.ax Analytics Mock Data
// ============================================================

import type {
  AnalyticsOverview,
  DailyStat,
  TopSiteClick,
  CategoryClickStat,
  VisitRecord,
  AnalyticsResponse,
} from '@/api/types';

// ---- Daily Stats (last 30 days) ----
function generateDailyStats(): DailyStat[] {
  const stats: DailyStat[] = [];
  const now = new Date();
  for (let i = 29; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const dateStr = d.toISOString().slice(0, 10);
    const base = 40 + Math.floor(Math.random() * 80);
    const pv = base + Math.floor(Math.random() * 60);
    const uv = Math.floor(pv * (0.45 + Math.random() * 0.35));
    stats.push({ date: dateStr, pv, uv });
  }
  return stats;
}

export const mockDailyStats: DailyStat[] = generateDailyStats();

// ---- Analytics Overview ----
const todayPv = mockDailyStats[mockDailyStats.length - 1].pv;
const todayUv = mockDailyStats[mockDailyStats.length - 1].uv;
const yesterdayPv = mockDailyStats[mockDailyStats.length - 2].pv;
const yesterdayUv = mockDailyStats[mockDailyStats.length - 2].uv;

export const mockAnalyticsOverview: AnalyticsOverview = {
  totalPV: mockDailyStats.reduce((s, d) => s + d.pv, 0),
  totalUV: mockDailyStats.reduce((s, d) => s + d.uv, 0),
  todayPV: todayPv,
  todayUV: todayUv,
  pvChange: Math.round(((todayPv - yesterdayPv) / yesterdayPv) * 100),
  uvChange: Math.round(((todayUv - yesterdayUv) / yesterdayUv) * 100),
  avgSessionPages: parseFloat((2.4 + Math.random() * 1.2).toFixed(1)),
  bounceRate: parseFloat((28 + Math.random() * 22).toFixed(1)),
  todayVisitors: todayUv,
  visitorsChange: Math.round(((todayUv - yesterdayUv) / yesterdayUv) * 100),
};

// ---- Top Sites ----
export const mockTopSites: TopSiteClick[] = [
  { siteId: 'site_001', siteTitle: 'GitHub', siteIcon: 'ri-github-fill', categoryName: '开发工具', clicks: 1847, ctr: parseFloat((12.4).toFixed(1)) },
  { siteId: 'site_011', siteTitle: 'Notion', siteIcon: 'ri-notion-fill', categoryName: '效率工具', clicks: 1523, ctr: parseFloat((10.2).toFixed(1)) },
  { siteId: 'site_007', siteTitle: 'Figma', siteIcon: 'ri-pen-nib-fill', categoryName: '设计资源', clicks: 1356, ctr: parseFloat((9.1).toFixed(1)) },
  { siteId: 'site_015', siteTitle: 'MDN', siteIcon: 'ri-book-2-fill', categoryName: '学习资源', clicks: 1189, ctr: parseFloat((8.0).toFixed(1)) },
  { siteId: 'site_005', siteTitle: 'Vercel', siteIcon: 'ri-vercel-fill', categoryName: '开发工具', clicks: 1024, ctr: parseFloat((6.9).toFixed(1)) },
  { siteId: 'site_018', siteTitle: 'Twitter / X', siteIcon: 'ri-twitter-x-fill', categoryName: '社交媒体', clicks: 967, ctr: parseFloat((6.5).toFixed(1)) },
  { siteId: 'site_012', siteTitle: 'Linear', siteIcon: 'ri-layout-4-fill', categoryName: '效率工具', clicks: 892, ctr: parseFloat((6.0).toFixed(1)) },
  { siteId: 'site_002', siteTitle: 'Stack Overflow', siteIcon: 'ri-stack-overflow-fill', categoryName: '开发工具', clicks: 845, ctr: parseFloat((5.7).toFixed(1)) },
  { siteId: 'site_014', siteTitle: 'Raycast', siteIcon: 'ri-flashlight-fill', categoryName: '效率工具', clicks: 723, ctr: parseFloat((4.8).toFixed(1)) },
  { siteId: 'site_021', siteTitle: 'Hacker News', siteIcon: 'ri-news-fill', categoryName: '资讯阅读', clicks: 689, ctr: parseFloat((4.6).toFixed(1)) },
  { siteId: 'site_008', siteTitle: 'Dribbble', siteIcon: 'ri-dribbble-fill', categoryName: '设计资源', clicks: 612, ctr: parseFloat((4.1).toFixed(1)) },
  { siteId: 'site_017', siteTitle: 'LeetCode', siteIcon: 'ri-brain-fill', categoryName: '学习资源', clicks: 578, ctr: parseFloat((3.9).toFixed(1)) },
  { siteId: 'site_019', siteTitle: 'Reddit', siteIcon: 'ri-reddit-fill', categoryName: '社交媒体', clicks: 534, ctr: parseFloat((3.6).toFixed(1)) },
  { siteId: 'site_003', siteTitle: 'CodePen', siteIcon: 'ri-codepen-fill', categoryName: '开发工具', clicks: 498, ctr: parseFloat((3.3).toFixed(1)) },
  { siteId: 'site_013', siteTitle: 'Todoist', siteIcon: 'ri-check-double-line', categoryName: '效率工具', clicks: 445, ctr: parseFloat((3.0).toFixed(1)) },
];

// ---- Category Stats ----
export const mockCategoryStats: CategoryClickStat[] = [
  { categoryName: '开发工具', categoryIcon: 'ri-code-s-slash-line', clicks: 5532, percentage: 33.2 },
  { categoryName: '效率工具', categoryIcon: 'ri-rocket-line', clicks: 3583, percentage: 21.5 },
  { categoryName: '设计资源', categoryIcon: 'ri-palette-line', clicks: 1968, percentage: 11.8 },
  { categoryName: '学习资源', categoryIcon: 'ri-book-open-line', clicks: 1767, percentage: 10.6 },
  { categoryName: '社交媒体', categoryIcon: 'ri-chat-3-line', clicks: 1584, percentage: 9.5 },
  { categoryName: '资讯阅读', categoryIcon: 'ri-newspaper-line', clicks: 1230, percentage: 7.4 },
];

// ---- Recent Visits ----
export const mockRecentVisits: VisitRecord[] = [
  { id: 'vis_001', visitorIp: '203.0.113.42', country: '中国', referrer: 'https://google.com', device: 'desktop', browser: 'Chrome 127', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 2 * 60000).toISOString() },
  { id: 'vis_002', visitorIp: '198.51.100.17', country: '美国', referrer: 'https://github.com', device: 'desktop', browser: 'Firefox 128', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 8 * 60000).toISOString() },
  { id: 'vis_003', visitorIp: '192.0.2.88', country: '日本', referrer: '直接访问', device: 'mobile', browser: 'Safari 17', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 15 * 60000).toISOString() },
  { id: 'vis_004', visitorIp: '203.0.113.105', country: '中国', referrer: 'https://x.com', device: 'desktop', browser: 'Edge 127', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 32 * 60000).toISOString() },
  { id: 'vis_005', visitorIp: '198.51.100.55', country: '德国', referrer: 'https://dev.to', device: 'tablet', browser: 'Chrome 126', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 47 * 60000).toISOString() },
  { id: 'vis_006', visitorIp: '203.0.113.33', country: '中国', referrer: '直接访问', device: 'mobile', browser: 'WeChat Browser', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 65 * 60000).toISOString() },
  { id: 'vis_007', visitorIp: '192.0.2.177', country: '韩国', referrer: 'https://naver.com', device: 'desktop', browser: 'Chrome 127', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 120 * 60000).toISOString() },
  { id: 'vis_008', visitorIp: '198.51.100.201', country: '新加坡', referrer: '直接访问', device: 'desktop', browser: 'Chrome 127', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 180 * 60000).toISOString() },
  { id: 'vis_009', visitorIp: '203.0.113.76', country: '中国', referrer: 'https://readdy.ai', device: 'desktop', browser: 'Firefox 128', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 240 * 60000).toISOString() },
  { id: 'vis_010', visitorIp: '192.0.2.44', country: '英国', referrer: 'https://news.ycombinator.com', device: 'desktop', browser: 'Arc 1.45', pageTitle: 'Lucas 的导航', visitedAt: new Date(Date.now() - 360 * 60000).toISOString() },
];

// ---- Full Analytics Response ----
export const mockAnalyticsResponse: AnalyticsResponse = {
  overview: mockAnalyticsOverview,
  dailyStats: mockDailyStats,
  topSites: mockTopSites,
  categoryStats: mockCategoryStats,
  recentVisits: mockRecentVisits,
};

// ---- Generate stats for a given number of days ----
export function getDailyStatsForDays(days: number): DailyStat[] {
  return mockDailyStats.slice(-days);
}

export function getTopSitesForLimit(limit: number): TopSiteClick[] {
  return mockTopSites.slice(0, limit);
}
