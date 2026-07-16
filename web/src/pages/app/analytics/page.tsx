// ============================================================
// nav.ax Analytics Page — /app/analytics
// ============================================================

import { useState, useMemo, useCallback } from 'react';
import { TrendingUp, TrendingDown, Eye, Users, MousePointerClick, Clock, Globe, Monitor, Smartphone, Tablet } from 'lucide-react';
import { useAnalytics } from '@/hooks/useQueries';
import { LoadingSkeleton, ErrorState } from '@/components/base/SharedUI';
import IconRenderer from '@/components/base/IconRenderer';
import { cn } from '@/lib/utils';
import type { DailyStat, TopSiteClick, CategoryClickStat, VisitRecord } from '@/api/types';

type TimeRange = 7 | 30 | 90;

const timeRangeLabels: { value: TimeRange; label: string }[] = [
  { value: 7, label: '近 7 天' },
  { value: 30, label: '近 30 天' },
  { value: 90, label: '近 90 天' },
];

function StatCard({ label, value, change, suffix, icon: Icon, colorClass }: {
  label: string;
  value: string;
  change?: number;
  suffix?: string;
  icon: React.ComponentType<{ className?: string }>;
  colorClass: string;
}) {
  return (
    <div className="bg-white rounded-xl border border-background-200/70 p-5">
      <div className="flex items-start justify-between mb-3">
        <div className={cn('w-10 h-10 rounded-lg flex items-center justify-center', colorClass)}>
          <Icon className="w-5 h-5" />
        </div>
        {change !== undefined && (
          <div className={cn(
            'flex items-center gap-0.5 text-xs font-medium px-2 py-1 rounded-full',
            change >= 0 ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
          )}>
            {change >= 0 ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
            <span className="whitespace-nowrap">{change >= 0 ? '+' : ''}{change}%</span>
          </div>
        )}
      </div>
      <div className="text-2xl font-bold font-heading text-foreground-950">
        {value}{suffix && <span className="text-sm font-normal text-foreground-400 ml-1">{suffix}</span>}
      </div>
      <div className="text-xs text-foreground-400 mt-1">{label}</div>
    </div>
  );
}

function DailyChart({ data }: { data: DailyStat[] }) {
  const maxPV = useMemo(() => Math.max(...data.map(d => d.pv), 1), [data]);
  const maxUV = useMemo(() => Math.max(...data.map(d => d.uv), 1), [data]);
  const chartMax = Math.max(maxPV, maxUV);

  return (
    <div className="bg-white rounded-xl border border-background-200/70 p-5">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-foreground-900">每日 PV / UV 趋势</h3>
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-sm bg-primary-500" />
            <span className="text-foreground-500">PV</span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-sm bg-accent-500" />
            <span className="text-foreground-500">UV</span>
          </div>
        </div>
      </div>

      {/* Chart */}
      <div className="relative h-[220px] flex items-end gap-[2px]">
        {/* Y-axis grid lines */}
        <div className="absolute inset-0 flex flex-col justify-between pointer-events-none">
          {[0, 1, 2, 3].map(i => (
            <div
              key={i}
              className="border-b border-background-200/50"
              style={{ height: '25%' }}
            >
              <span className="absolute -left-1 -top-2 text-[10px] text-foreground-300">
                {Math.round(chartMax * (1 - i * 0.25))}
              </span>
            </div>
          ))}
        </div>

        {/* Bars */}
        <div className="flex-1 flex items-end gap-[2px] h-full ml-7">
          {data.map((day, idx) => {
            const pvHeight = (day.pv / maxPV) * 100;
            const uvHeight = (day.uv / maxUV) * 100;
            const showLabel = data.length <= 14 ? idx % 2 === 0 : idx % 5 === 0;
            return (
              <div
                key={day.date}
                className="flex-1 flex flex-col items-center justify-end min-w-0 group relative"
                style={{ height: '100%' }}
              >
                {/* Tooltip */}
                <div className="absolute -top-14 left-1/2 -translate-x-1/2 bg-foreground-900 text-background-50 text-[10px] rounded-md px-2 py-1.5 opacity-0 group-hover:opacity-100 transition-opacity duration-150 pointer-events-none whitespace-nowrap z-10">
                  <div>{day.date.slice(5)}</div>
                  <div>PV: {day.pv.toLocaleString()} &middot; UV: {day.uv.toLocaleString()}</div>
                  <div className="absolute top-full left-1/2 -translate-x-1/2 w-0 h-0 border-l-4 border-r-4 border-t-4 border-l-transparent border-r-transparent border-t-foreground-900" />
                </div>

                {/* PV bar */}
                <div
                  className="w-full max-w-[14px] bg-primary-500/80 hover:bg-primary-500 rounded-t-sm transition-all duration-200 origin-bottom"
                  style={{ height: `${Math.max(pvHeight, 0.5)}%` }}
                />
                {/* UV bar */}
                <div
                  className="w-full max-w-[14px] bg-accent-500/60 hover:bg-accent-500 rounded-t-sm transition-all duration-200 origin-bottom mt-[1px]"
                  style={{ height: `${Math.max(uvHeight, 0.5)}%` }}
                />
                {/* Label */}
                {showLabel && (
                  <span className="text-[10px] text-foreground-300 mt-1.5 truncate max-w-full">
                    {day.date.slice(5)}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function TopSitesRanking({ sites }: { sites: TopSiteClick[] }) {
  const maxClicks = sites[0]?.clicks || 1;
  return (
    <div className="bg-white rounded-xl border border-background-200/70 p-5">
      <h3 className="text-sm font-semibold text-foreground-900 mb-4">站点点击排行</h3>
      <div className="space-y-1">
        {sites.map((site, idx) => (
          <div
            key={site.siteId}
            className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-background-50 transition-colors duration-150 group"
          >
            <span className={cn(
              'w-5 h-5 rounded-md flex items-center justify-center text-[11px] font-bold flex-shrink-0',
              idx < 3 ? 'bg-accent-100 text-accent-700' : 'bg-background-100 text-foreground-400'
            )}>
              {idx + 1}
            </span>
            <div className="w-7 h-7 rounded-lg bg-background-100 flex items-center justify-center flex-shrink-0">
              <IconRenderer icon={site.siteIcon} className="text-foreground-600 text-sm" size={16} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <span className="text-sm text-foreground-800 truncate">{site.siteTitle}</span>
                <span className="text-xs font-medium text-foreground-600 ml-2 whitespace-nowrap">{site.clicks.toLocaleString()} 次</span>
              </div>
              {/* Progress bar */}
              <div className="mt-1.5 h-1.5 rounded-full bg-background-100 overflow-hidden">
                <div
                  className="h-full rounded-full bg-primary-400/60 transition-all duration-500"
                  style={{ width: `${(site.clicks / maxClicks) * 100}%` }}
                />
              </div>
              <div className="flex items-center justify-between mt-0.5">
                <span className="text-[10px] text-foreground-300">{site.categoryName}</span>
                <span className="text-[10px] text-foreground-300">{site.ctr}% CTR</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function CategoryDistribution({ stats }: { stats: CategoryClickStat[] }) {
  return (
    <div className="bg-white rounded-xl border border-background-200/70 p-5">
      <h3 className="text-sm font-semibold text-foreground-900 mb-4">分类点击分布</h3>
      <div className="space-y-3">
        {stats.map(stat => (
          <div key={stat.categoryName} className="group">
            <div className="flex items-center justify-between mb-1">
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0">
                  <IconRenderer icon={stat.categoryIcon} className="text-foreground-500" size={14} />
                </div>
                <span className="text-sm text-foreground-700">{stat.categoryName}</span>
              </div>
              <span className="text-xs font-medium text-foreground-600">{stat.percentage}%</span>
            </div>
            <div className="h-1.5 rounded-full bg-background-100 overflow-hidden">
              <div
                className="h-full rounded-full bg-secondary-400/70 transition-all duration-500"
                style={{ width: `${stat.percentage}%` }}
              />
            </div>
            <div className="text-[10px] text-foreground-300 mt-0.5 text-right">{stat.clicks.toLocaleString()} 次点击</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DeviceIcon({ device }: { device: VisitRecord['device'] }) {
  if (device === 'mobile') return <Smartphone className="w-3.5 h-3.5" />;
  if (device === 'tablet') return <Tablet className="w-3.5 h-3.5" />;
  return <Monitor className="w-3.5 h-3.5" />;
}

function RecentVisits({ visits }: { visits: VisitRecord[] }) {
  return (
    <div className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
      <div className="px-5 py-4 border-b border-background-100">
        <h3 className="text-sm font-semibold text-foreground-900">最近访问记录</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-background-100">
              <th className="text-left px-5 py-2.5 text-xs font-medium text-foreground-400">访问者</th>
              <th className="text-left px-4 py-2.5 text-xs font-medium text-foreground-400">来源</th>
              <th className="text-left px-4 py-2.5 text-xs font-medium text-foreground-400">设备</th>
              <th className="text-left px-4 py-2.5 text-xs font-medium text-foreground-400">浏览器</th>
              <th className="text-right px-5 py-2.5 text-xs font-medium text-foreground-400">时间</th>
            </tr>
          </thead>
          <tbody>
            {visits.map(visit => (
              <tr key={visit.id} className="border-b border-background-50 hover:bg-background-50/50 transition-colors duration-150">
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    <Globe className="w-3.5 h-3.5 text-foreground-300 flex-shrink-0" />
                    <div>
                      <div className="text-foreground-700 text-sm">{visit.country}</div>
                      <div className="text-[10px] text-foreground-300">{visit.visitorIp}</div>
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3 text-foreground-500 text-sm max-w-[140px] truncate">{visit.referrer}</td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-1.5 text-foreground-400">
                    <DeviceIcon device={visit.device} />
                    <span className="text-xs">{visit.device === 'desktop' ? '桌面端' : visit.device === 'mobile' ? '手机端' : '平板端'}</span>
                  </div>
                </td>
                <td className="px-4 py-3 text-foreground-500 text-xs">{visit.browser}</td>
                <td className="px-5 py-3 text-right text-xs text-foreground-400 whitespace-nowrap">
                  {new Date(visit.visitedAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function AnalyticsPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>(30);
  const { data, isLoading, isError, error, refetch } = useAnalytics({ days: timeRange, siteLimit: 15 });

  const handleRangeChange = useCallback((range: TimeRange) => {
    setTimeRange(range);
  }, []);

  if (isLoading) {
    return (
      <div>
        <div className="mb-8">
          <div className="skeleton h-8 w-48 mb-2" />
          <div className="skeleton h-4 w-32" />
        </div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="skeleton h-28 rounded-xl" />
          ))}
        </div>
        <div className="skeleton h-[320px] rounded-xl mb-6" />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div className="skeleton h-[400px] rounded-xl" />
          <div className="skeleton h-[400px] rounded-xl" />
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <ErrorState
        message={error instanceof Error ? error.message : '加载数据失败'}
        onRetry={() => refetch()}
      />
    );
  }

  const { overview, dailyStats, topSites, categoryStats, recentVisits } = data;

  return (
    <div>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">访问统计</h1>
          <p className="text-sm text-foreground-400 mt-1">导航页 PV / UV 流量数据监控</p>
        </div>

        {/* Time range selector */}
        <div className="flex items-center gap-1 bg-background-100 rounded-lg p-1">
          {timeRangeLabels.map(item => (
            <button
              key={item.value}
              onClick={() => handleRangeChange(item.value)}
              className={cn(
                'h-8 px-4 rounded-md text-xs font-medium transition-all duration-150 whitespace-nowrap',
                timeRange === item.value
                  ? 'bg-white text-foreground-900 shadow-sm'
                  : 'text-foreground-400 hover:text-foreground-600'
              )}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
        <StatCard
          label="总 PV（浏览量）"
          value={overview.totalPV.toLocaleString()}
          change={overview.pvChange}
          icon={Eye}
          colorClass="bg-primary-50 text-primary-600"
        />
        <StatCard
          label="总 UV（独立访客）"
          value={overview.totalUV.toLocaleString()}
          change={overview.uvChange}
          icon={Users}
          colorClass="bg-accent-50 text-accent-600"
        />
        <StatCard
          label="今日访问"
          value={overview.todayVisitors.toLocaleString()}
          change={overview.visitorsChange}
          suffix="人"
          icon={MousePointerClick}
          colorClass="bg-secondary-50 text-secondary-600"
        />
        <StatCard
          label="跳出率"
          value={`${overview.bounceRate}`}
          suffix="%"
          icon={Clock}
          colorClass="bg-foreground-100 text-foreground-600"
        />
      </div>

      {/* Daily Chart */}
      <div className="mb-6">
        <DailyChart data={dailyStats} />
      </div>

      {/* Top Sites + Category Distribution */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <TopSitesRanking sites={topSites} />
        <CategoryDistribution stats={categoryStats} />
      </div>

      {/* Recent Visits */}
      <RecentVisits visits={recentVisits} />
    </div>
  );
}
