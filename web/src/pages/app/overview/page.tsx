// ============================================================
// nav.ax App Overview Page — /app
// ============================================================

import { Link, useNavigate } from 'react-router-dom';
import { Link2, Puzzle, Palette, Globe, Upload, Eye, Clock, ArrowUpRight, ShieldCheck, Loader2, ExternalLink } from 'lucide-react';
import { usePublish, useSubdomain } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import {
  handlePublishError,
  navigateToVisibilityFix,
  previewPath,
  publishSettingsPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';
import { LoadingSkeleton, ErrorState } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';

function step0Desc(stateId: string): string {
  if (stateId === 'published_with_draft') return '有草稿未上线 · 访客仍看线上版';
  if (stateId === 'published_current') return '已发布 · 对外可见';
  return '未发布 · 设置域名和可见性';
}

export default function AppOverview() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const {
    state,
    scope,
    slug,
    isLoading,
    isError,
    publication,
    page,
    refetch,
  } = usePublishUiState('overview');
  const { data: subdomainData } = useSubdomain();
  const { mutate: publishMutation, isPending: publishing } = usePublish();

  if (isLoading) {
    return (
      <div>
        <div className="mb-8">
          <div className="skeleton h-8 w-48 mb-2" />
          <div className="skeleton h-4 w-32" />
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-8">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
        <LoadingSkeleton count={3} />
      </div>
    );
  }

  if (isError || !page) {
    return (
      <ErrorState
        message="加载导航数据失败"
        onRetry={() => refetch()}
      />
    );
  }

  const totalSites = page.categories.reduce((sum, c) => sum + c.sites.length, 0);
  const enabledDisplayItems = page.settings
    ? [page.settings.display.showClock, page.settings.display.showDate, page.settings.display.showGreeting].filter(Boolean).length
    : 0;
  const domainStatus = subdomainData?.status ?? 'none';
  const domainSubdomain = subdomainData?.label ?? subdomainData?.subdomain ?? '';
  const isPublished = publication?.published ?? false;
  const withScope = (path: string) => `${path}?scope=${scope}`;

  const handlePrimaryPublish = () => {
    const intent = resolvePrimaryPublishIntent(state);
    if (intent === 'noop') return;
    if (intent === 'redirect_visibility') {
      navigateToVisibilityFix(navigate, scope);
      return;
    }

    const stateBefore = state;
    publishMutation(undefined, {
      onSuccess: () => {
        toast('success', toastForPublishSuccess(stateBefore));
      },
      onError: (error: Error) => {
        handlePublishError(error, {
          toast,
          refetch: () => { void refetch(); },
          navigateToVisibilityFix: () => navigateToVisibilityFix(navigate, scope),
        });
      },
    });
  };

  const quickActions = [
    {
      to: publishSettingsPath(scope),
      icon: Globe,
      label: '发布与访问',
      desc: step0Desc(state.id),
      step: 0,
    },
    { to: withScope('/app/links'), icon: Link2, label: '导航编辑', desc: '管理分类站点 + 拖拽编排布局', step: 1 },
    { to: withScope('/app/widgets'), icon: Puzzle, label: '首页信息', desc: '设置时钟、日期与欢迎词', step: 2 },
    { to: withScope('/app/themes'), icon: Palette, label: '主题设置', desc: '切换外观和密度', step: 3 },
  ];

  return (
    <div>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-8">
        <div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">导航管理总览</h1>
          <p className="text-sm text-foreground-400 mt-1">{page.title}</p>
        </div>
        <div className="flex items-center gap-2 sm:gap-3 flex-wrap">
          <Link
            to={previewPath(scope)}
            className="h-9 px-4 rounded-lg border border-background-200/70 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 flex items-center gap-2 whitespace-nowrap"
          >
            <Eye className="w-4 h-4" />
            草稿预览
          </Link>
          {isPublished && slug && (
            <Link
              to={`/u/${slug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="h-9 px-4 rounded-lg border border-background-200/70 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 flex items-center gap-2 whitespace-nowrap"
            >
              <ExternalLink className="w-4 h-4" />
              线上版
            </Link>
          )}
          {state.primaryAction !== 'none' && (
            <button
              type="button"
              onClick={handlePrimaryPublish}
              disabled={publishing || state.primaryDisabled}
              className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors duration-150 flex items-center gap-2 whitespace-nowrap"
            >
              {publishing ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Globe className="w-4 h-4" />
              )}
              {publishing ? '发布中…' : state.primaryLabel}
            </button>
          )}
          <Link
            to={publishSettingsPath(scope)}
            className={cn(
              'h-9 px-4 rounded-lg text-sm font-medium transition-colors duration-150 flex items-center gap-2 whitespace-nowrap',
              state.primaryAction !== 'none'
                ? 'border border-background-200/70 text-foreground-600 hover:bg-background-100'
                : 'bg-primary-500 text-background-50 dark:text-foreground-950 hover:bg-primary-600',
            )}
          >
            <Globe className="w-4 h-4" />
            发布设置
          </Link>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-8">
        {[
          { label: '分类数', value: page.categories.length },
          { label: '站点数', value: totalSites },
          { label: '首页信息项', value: enabledDisplayItems },
          { label: '发布状态', value: state.shortLabel },
        ].map(stat => (
          <div key={stat.label} className="bg-white rounded-xl border border-background-200/70 p-4">
            <div className="text-2xl font-bold font-heading text-foreground-950">{stat.value}</div>
            <div className="text-xs text-foreground-400 mt-1">{stat.label}</div>
          </div>
        ))}
      </div>

      {/* Quick Actions — numbered flow */}
      <h2 className="text-sm font-semibold text-foreground-700 mb-3">设置流程</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-8">
        {quickActions.map(action => {
          const highlightDraft = action.step === 0 && state.id === 'published_with_draft';
          const highlightPublished = action.step === 0 && state.id === 'published_current';
          return (
            <Link
              key={action.to}
              to={action.to}
              className={cn(
                'flex items-center gap-4 p-4 rounded-xl border transition-all duration-150 group',
                highlightDraft
                  ? 'bg-accent-50/50 border-accent-200 hover:border-accent-300'
                  : highlightPublished
                    ? 'bg-green-50/50 border-green-200 hover:border-green-300'
                    : 'bg-white border-background-200/70 hover:border-primary-200 hover:bg-background-50',
              )}
            >
              <div className={cn(
                'w-10 h-10 rounded-lg flex items-center justify-center transition-colors duration-150 relative',
                highlightDraft
                  ? 'bg-accent-100 group-hover:bg-accent-200'
                  : highlightPublished
                    ? 'bg-green-100 group-hover:bg-green-200'
                    : 'bg-primary-50 group-hover:bg-primary-100',
              )}>
                <action.icon className={cn(
                  'w-5 h-5',
                  highlightDraft ? 'text-accent-600' : highlightPublished ? 'text-green-600' : 'text-primary-600',
                )} />
                <span className={cn(
                  'absolute -top-1.5 -right-1.5 w-5 h-5 rounded-full text-background-50 dark:text-foreground-950 text-[10px] font-bold flex items-center justify-center',
                  highlightDraft ? 'bg-accent-500' : highlightPublished ? 'bg-green-500' : 'bg-primary-500',
                )}>
                  {action.step}
                </span>
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <div className="text-sm font-medium text-foreground-900">{action.label}</div>
                  {action.step === 0 && state.id === 'published_with_draft' && (
                    <span className="px-1.5 py-0.5 rounded-full bg-accent-100 text-accent-700 text-[10px] font-medium flex-shrink-0">
                      有草稿
                    </span>
                  )}
                  {action.step === 0 && state.id === 'published_current' && (
                    <span className="px-1.5 py-0.5 rounded-full bg-green-100 text-green-700 text-[10px] font-medium flex-shrink-0">
                      已发布
                    </span>
                  )}
                  {action.step === 0 && domainStatus === 'approved' && (
                    <span className="px-1.5 py-0.5 rounded-full bg-green-100 text-green-700 text-[10px] font-medium flex-shrink-0">
                      <ShieldCheck className="w-3 h-3 inline mr-0.5" />
                      {domainSubdomain}
                    </span>
                  )}
                </div>
                <div className="text-xs text-foreground-400">{action.desc}</div>
              </div>
              <ArrowUpRight className="w-4 h-4 text-foreground-300 group-hover:text-primary-500 transition-colors duration-150 flex-shrink-0" />
            </Link>
          );
        })}
      </div>

      {/* Secondary actions */}
      <h2 className="text-sm font-semibold text-foreground-700 mb-3">更多</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-8">
        <Link
          to={withScope('/app/import-export')}
          className="flex items-center gap-4 p-4 rounded-xl border border-background-200/70 bg-white hover:border-primary-200 hover:bg-background-50 transition-all duration-150 group"
        >
          <div className="w-10 h-10 rounded-lg bg-secondary-50 flex items-center justify-center group-hover:bg-secondary-100 transition-colors duration-150">
            <Upload className="w-5 h-5 text-secondary-600" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium text-foreground-900">导入导出</div>
            <div className="text-xs text-foreground-400">书签导入与数据备份</div>
          </div>
          <ArrowUpRight className="w-4 h-4 text-foreground-300 group-hover:text-primary-500 transition-colors duration-150 flex-shrink-0" />
        </Link>
        <Link
          to={withScope('/app/settings')}
          className="flex items-center gap-4 p-4 rounded-xl border border-background-200/70 bg-white hover:border-primary-200 hover:bg-background-50 transition-all duration-150 group"
        >
          <div className="w-10 h-10 rounded-lg bg-secondary-50 flex items-center justify-center group-hover:bg-secondary-100 transition-colors duration-150">
            <i className="ri-settings-3-line text-lg text-secondary-600" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium text-foreground-900">个人设置</div>
            <div className="text-xs text-foreground-400">账号信息与偏好</div>
          </div>
          <ArrowUpRight className="w-4 h-4 text-foreground-300 group-hover:text-primary-500 transition-colors duration-150 flex-shrink-0" />
        </Link>
      </div>

      {/* Status Summary */}
      <h2 className="text-sm font-semibold text-foreground-700 mb-3">页面状态</h2>
      <div className="bg-white rounded-xl border border-background-200/70 p-5 space-y-3">
        <div className="flex items-center gap-3 text-sm">
          <Clock className="w-4 h-4 text-foreground-300 flex-shrink-0" />
          <span className="text-foreground-600">
            {publication?.hasUnpublishedChanges ? '草稿更新于 ' : '最后更新于 '}
            {new Date(page.draftUpdatedAt).toLocaleString('zh-CN')}
          </span>
        </div>
        {publication?.publishedAt && (
          <div className="flex items-center gap-3 text-sm">
            <Globe className="w-4 h-4 text-foreground-300 flex-shrink-0" />
            <span className="text-foreground-600">上次发布于 {new Date(publication.publishedAt).toLocaleString('zh-CN')}</span>
          </div>
        )}
        <div className="flex items-center gap-3 text-sm">
          <ShieldCheck className={cn(
            'w-4 h-4 flex-shrink-0',
            isPublished ? 'text-green-500' : 'text-foreground-300',
          )} />
          <span className={isPublished ? 'text-green-700 font-medium' : 'text-foreground-500'}>
            {state.id === 'published_with_draft'
              ? '已发布，但有草稿未上线'
              : isPublished
                ? '页面已发布，对外可见'
                : '页面未发布，对外不可见'}
          </span>
        </div>

        {publication?.hasUnpublishedChanges && publication.publishedAt && (
          <div className="mt-4 pt-4 border-t border-background-200/70 flex flex-col sm:flex-row items-start sm:items-center gap-3">
            <span className="px-2.5 py-1 rounded-full text-xs font-medium bg-accent-100 text-accent-700 whitespace-nowrap">
              存在未发布更改
            </span>
            <span className="text-xs text-foreground-500">
              草稿更新于 {new Date(page.draftUpdatedAt).toLocaleString('zh-CN')} · 发布版为 {new Date(publication.publishedAt).toLocaleString('zh-CN')}
            </span>
            <div className="sm:ml-auto flex items-center gap-2">
              <button
                type="button"
                onClick={handlePrimaryPublish}
                disabled={publishing || state.primaryDisabled || state.primaryAction === 'none'}
                className="inline-flex items-center gap-1.5 h-8 px-3 rounded-lg bg-accent-600 text-background-50 text-xs font-medium hover:bg-accent-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap"
              >
                {publishing && <Loader2 className="w-3 h-3 animate-spin" />}
                发布更新
              </button>
              <Link
                to={publishSettingsPath(scope)}
                className="inline-flex items-center gap-1.5 h-8 px-3 rounded-lg border border-accent-200 text-accent-700 text-xs font-medium hover:bg-accent-50 transition-colors duration-150 whitespace-nowrap"
              >
                发布设置
              </Link>
            </div>
          </div>
        )}

        {!publication?.publishedAt && (
          <div className="mt-4 pt-4 border-t border-background-200/70 flex flex-wrap items-center gap-2">
            {state.primaryAction !== 'none' && (
              <button
                type="button"
                onClick={handlePrimaryPublish}
                disabled={publishing || state.primaryDisabled}
                className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap"
              >
                {publishing ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Globe className="w-4 h-4" />
                )}
                {publishing ? '发布中…' : state.primaryLabel}
              </button>
            )}
            <Link
              to={publishSettingsPath(scope)}
              className="inline-flex items-center gap-2 h-9 px-4 rounded-lg border border-background-200/70 text-sm font-medium text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
            >
              前往发布设置
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
