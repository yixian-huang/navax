// ============================================================
// nav.ax Analytics API Service
// ============================================================

import { request } from './client';
import type {
  AnalyticsBreakdown,
  AnalyticsOverview,
  AnalyticsResponse,
  ApiResponse,
  CategoryClickStat,
  DailyStat,
  TopSiteClick,
  VisitRecord,
  PublicEventRequest,
} from './types';

function period(days?: number): 7 | 30 | 90 {
  if (days === 7 || days === 90) return days;
  return 30;
}

export const analyticsApi = {
  recordPublicEvent: (event: PublicEventRequest) =>
    request<ApiResponse<null>>('/public/events', { method: 'POST', body: event }),

  getOverview: (days?: number) =>
    request<ApiResponse<AnalyticsOverview>>('/me/analytics/overview', { params: { period: period(days) } }),

  getTrends: (days?: number) =>
    request<ApiResponse<DailyStat[]>>('/me/analytics/trends', { params: { period: period(days) } }),

  getBreakdown: (days?: number) =>
    request<ApiResponse<AnalyticsBreakdown>>('/me/analytics/breakdown', { params: { period: period(days) } }),

  /** @deprecated 待统计页面分别消费 overview/trends/breakdown 后删除。 */
  getAnalytics: async (params?: { days?: number; siteLimit?: number }): Promise<ApiResponse<AnalyticsResponse>> => {
    const [overviewResponse, trendsResponse, breakdownResponse] = await Promise.all([
      analyticsApi.getOverview(params?.days),
      analyticsApi.getTrends(params?.days),
      analyticsApi.getBreakdown(params?.days),
    ]);
    const breakdown = breakdownResponse.data;
    const siteLimit = params?.siteLimit ?? breakdown.topSites.length;
    // CTR 采用「点击 / 页面浏览」口径（无需曝光埋点），上限 100%。
    const pageViews = overviewResponse.data.totalPV;
    const clickRate = (clicks: number) =>
      pageViews > 0 ? Math.min(100, Math.round((clicks / pageViews) * 1000) / 10) : 0;
    const topSites: TopSiteClick[] = breakdown.topSites.slice(0, siteLimit).map(bucket => ({
      siteId: bucket.key,
      siteTitle: bucket.label,
      siteIcon: bucket.icon ?? '',
      categoryName: bucket.categoryName ?? '',
      clicks: bucket.value,
      ctr: clickRate(bucket.value),
    }));
    // 契约的 category bucket 只提供计数；占比可由点击总数在前端派生。
    const totalCategoryClicks = breakdown.categories.reduce((sum, bucket) => sum + bucket.value, 0);
    const categoryStats: CategoryClickStat[] = breakdown.categories.map(bucket => ({
      categoryName: bucket.label,
      categoryIcon: bucket.icon ?? '',
      clicks: bucket.value,
      percentage: totalCategoryClicks > 0 ? Math.round((bucket.value / totalCategoryClicks) * 1000) / 10 : 0,
    }));
    const recentVisits: VisitRecord[] = breakdown.recentVisits.map((visit, index) => ({
      id: `${visit.anonymousId}-${index}`,
      visitorIp: visit.anonymousId,
      country: visit.country ?? '',
      referrer: visit.referrerDomain,
      device: visit.device === 'mobile' || visit.device === 'tablet' ? visit.device : 'desktop',
      browser: visit.browser ?? '',
      pageTitle: '',
      visitedAt: visit.visitedAt,
    }));
    return {
      ...overviewResponse,
      data: {
        overview: overviewResponse.data,
        dailyStats: trendsResponse.data,
        topSites,
        categoryStats,
        recentVisits,
      },
    };
  },

  getDailyStats: (days?: number) => analyticsApi.getTrends(days),

  getTopSites: async (limit?: number) => {
    const response = await analyticsApi.getBreakdown();
    return {
      ...response,
      data: response.data.topSites.slice(0, limit).map(bucket => ({
        siteId: bucket.key,
        siteTitle: bucket.label,
        siteIcon: bucket.icon ?? '',
        categoryName: bucket.categoryName ?? '',
        clicks: bucket.value,
        ctr: 0,
      })),
    } as ApiResponse<TopSiteClick[]>;
  },
};
