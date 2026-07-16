// ============================================================
// nav.ax Admin Overview — /admin
// ============================================================

import { Link } from 'react-router-dom';
import { Users, Link2, Globe, Shield, ArrowUpRight, CheckCircle, AlertTriangle, Activity, Server } from 'lucide-react';
import { useAdminOverview } from '@/hooks/useQueries';
import { LoadingSkeleton, ErrorState, Badge } from '@/components/base/SharedUI';
import { cn } from '@/lib/utils';

export default function AdminOverview() {
  const { data: overview, isLoading, error, refetch } = useAdminOverview();

  if (isLoading) {
    return (
      <div>
        <div className="mb-5"><div className="skeleton h-7 w-36 mb-1" /><div className="skeleton h-4 w-56" /></div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-5">
          {Array.from({ length: 4 }).map((_, i) => (<div key={i} className="skeleton h-24 rounded-xl" />))}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <div className="skeleton h-40 rounded-xl" />
          <div className="skeleton h-40 rounded-xl" />
          <div className="skeleton h-60 rounded-xl" />
        </div>
      </div>
    );
  }

  if (error || !overview) {
    return (
      <div>
        <div className="mb-5"><h1 className="text-xl font-bold font-heading text-foreground-950">运营概览</h1></div>
        <ErrorState message={(error as Error)?.message || '加载失败'} onRetry={() => refetch()} />
      </div>
    );
  }

  const h = overview.health;

  const statItems = [
    { label: '总用户数', value: overview.totalUsers, icon: Users, detail: `${overview.activeUsers} 活跃` },
    { label: '有效邀请', value: overview.activeInvitations, icon: Link2, detail: '可用邀请链接' },
    { label: '公开页面', value: overview.publicPages, icon: Globe, detail: '已发布导航' },
    { label: '系统状态', value: null, icon: Shield, detail: h.status === 'healthy' ? '运行正常' : '异常', custom: true },
  ];

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-xl font-bold font-heading text-foreground-950">运营概览</h1>
        <p className="text-xs text-foreground-400 mt-0.5">nav.ax 实例运行状态与关键指标</p>
      </div>

      {/* Stat Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-5">
        {statItems.map(stat => (
          <div key={stat.label} className="bg-white rounded-lg border border-background-200/70 p-3.5">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-foreground-400">{stat.label}</span>
              <div className="w-8 h-8 rounded-md bg-background-100 flex items-center justify-center">
                {stat.custom ? (
                  <Shield className="w-4 h-4 text-foreground-400" />
                ) : (
                  <stat.icon className="w-4 h-4 text-foreground-400" />
                )}
              </div>
            </div>
            {stat.custom ? (
              <div className="flex items-center gap-1.5">
                <span className={cn('w-2 h-2 rounded-full', h.status === 'healthy' ? 'bg-green-500' : 'bg-accent-500')} />
                <span className="text-lg font-bold font-heading text-foreground-950">{h.status === 'healthy' ? '正常' : '降级'}</span>
              </div>
            ) : (
              <div className="text-lg font-bold font-heading text-foreground-950">{stat.value}</div>
            )}
            <div className="text-xs text-foreground-400 mt-0.5">{stat.detail}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Health Panel */}
        <div className="bg-white rounded-lg border border-background-200/70 p-4">
          <div className="flex items-center gap-2 mb-3">
            <Server className="w-3.5 h-3.5 text-foreground-400" />
            <h3 className="text-sm font-semibold text-foreground-700">系统信息</h3>
          </div>
          <div className="space-y-2 text-sm">
            <InfoRow label="运行时间" value={formatDuration(h.uptimeSeconds)} />
            <InfoRow label="nav.ax 版本" value={`v${h.version}`} />
            <InfoRow label="Go 版本" value={h.goVersion} />
            <InfoRow label="内存使用" value={formatBytes(h.memoryBytes)} />
          </div>
        </div>

        {/* Quick Actions */}
        <div className="bg-white rounded-lg border border-background-200/70 p-4">
          <h3 className="text-sm font-semibold text-foreground-700 mb-3">快捷操作</h3>
          <div className="space-y-0.5">
            {[
              { to: '/admin/users', label: '用户管理', desc: '查看、禁用或启用账号' },
              { to: '/admin/invitations', label: '创建邀请', desc: '管理注册邀请链接' },
              { to: '/admin/directory', label: '推荐站点', desc: '维护平台推荐站点库' },
              { to: '/admin/settings', label: '系统配置', desc: '实例参数和注册策略' },
            ].map(link => (
              <Link
                key={link.to}
                to={link.to}
                className="flex items-center justify-between px-3 py-2 rounded-lg text-sm hover:bg-background-50 transition-colors duration-150 group whitespace-nowrap"
              >
                <div>
                  <div className="text-foreground-700 font-medium group-hover:text-foreground-900">{link.label}</div>
                  <div className="text-xs text-foreground-400">{link.desc}</div>
                </div>
                <ArrowUpRight className="w-3.5 h-3.5 text-foreground-300 group-hover:text-primary-500 flex-shrink-0 ml-2 transition-colors" />
              </Link>
            ))}
          </div>
        </div>

        {/* Recent Activity */}
        <div className="bg-white rounded-lg border border-background-200/70 p-4">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Activity className="w-3.5 h-3.5 text-foreground-400" />
              <h3 className="text-sm font-semibold text-foreground-700">最近操作</h3>
            </div>
            <Link to="/admin/audit" className="text-xs text-primary-600 hover:text-primary-700 font-medium whitespace-nowrap">查看全部</Link>
          </div>
          <div className="space-y-0.5 -mx-4">
            {overview.recentActions.map(entry => (
              <div key={entry.id} className="flex items-center gap-2.5 px-4 py-2 hover:bg-background-50 transition-colors duration-150 text-sm">
                <span className="text-foreground-500 font-medium w-16 flex-shrink-0 text-xs truncate">{entry.actor}</span>
                <Badge variant="default" className="text-[10px] px-1.5 py-0">{entry.action}</Badge>
                <span className="text-foreground-400 truncate flex-1 text-xs">{entry.detail}</span>
                <span className="text-xs text-foreground-300 flex-shrink-0 whitespace-nowrap">
                  {new Date(entry.createdAt).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-xs text-foreground-400">{label}</span>
      <span className="text-sm text-foreground-700 font-medium">{value}</span>
    </div>
  );
}

function formatDuration(totalSeconds: number): string {
  const days = Math.floor(totalSeconds / 86_400);
  const hours = Math.floor((totalSeconds % 86_400) / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  return days > 0 ? `${days} 天 ${hours} 小时` : `${hours} 小时 ${minutes} 分钟`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}
