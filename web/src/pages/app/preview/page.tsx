// ============================================================
// nav.ax Draft Preview — /app/preview
// 复用公开页渲染组件:预览端点返回的就是 PublishedPage 契约形状,
// 主题、壁纸、布局模板与发布后一致(所见即所得)。
// ============================================================

import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowLeft, ExternalLink, Globe, Loader2 } from 'lucide-react';
import { useMyPage, usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import {
  handlePublishError,
  navigateToVisibilityFix,
  publishSettingsPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { navigationApi } from '@/api/navigation';
import PublicNavigationView from '@/components/feature/PublicNavigationView';

export default function PreviewPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const pageQuery = useMyPage();
  const {
    state,
    scope,
    slug,
    publication,
    refetch,
  } = usePublishUiState('preview');
  const { mutate: publishMutation, isPending: publishing } = usePublish();

  const pageId = pageQuery.data?.id;
  const previewQuery = useQuery({
    queryKey: ['preview', pageId],
    queryFn: async () => {
      const response = await navigationApi.getPreview(pageId!);
      return response.data;
    },
    enabled: !!pageId,
    staleTime: 0,
  });

  if (pageQuery.isLoading) return <LoadingSkeleton count={4} />;
  if (pageQuery.isError) {
    return (
      <ErrorState
        message={pageQuery.error?.message || '无法加载草稿预览'}
        onRetry={() => pageQuery.refetch()}
      />
    );
  }

  const isPublished = state.showUnpublish || publication?.published === true;
  const liveSlug = slug || publication?.slug || previewQuery.data?.title;

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
      onError: (cause: Error) => {
        handlePublishError(cause, {
          toast,
          refetch: () => { void refetch(); },
          navigateToVisibilityFix: () => navigateToVisibilityFix(navigate, scope),
        });
      },
    });
  };

  return (
    <div>
      <div className="mb-4 flex items-center justify-between gap-3 flex-wrap">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Link
              to={publishSettingsPath(scope)}
              className="inline-flex items-center gap-1 text-xs text-foreground-400 hover:text-foreground-600"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              返回发布
            </Link>
          </div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">草稿预览 · 非公开</h1>
          <p className="text-sm text-foreground-400 mt-1">
            与发布后完全一致的渲染(主题、壁纸、布局),未发布内容不会影响公开页
          </p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {state.primaryAction !== 'none' && (
            <button
              type="button"
              onClick={handlePrimaryPublish}
              disabled={publishing || state.primaryDisabled}
              className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed inline-flex items-center gap-1.5 transition-colors duration-150"
            >
              {publishing ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <Globe className="w-3.5 h-3.5" />
              )}
              {publishing ? '发布中…' : state.primaryLabel}
            </button>
          )}
          {isPublished && liveSlug && (
            <Link
              to={`/u/${liveSlug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="h-9 px-3 rounded-lg border border-background-200 text-sm text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1.5"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              打开线上版
            </Link>
          )}
        </div>
      </div>

      {/* 全宽渲染公开页组件:负 margin 抵消 AppShell 内容区的 p-4/md:p-5。 */}
      <div className="-mx-4 md:-mx-5 -mb-4 md:-mb-5 border-t border-background-200/70">
        <PublicNavigationView
          page={previewQuery.data}
          isLoading={previewQuery.isLoading}
          error={previewQuery.error ?? undefined}
          onRetry={() => { void previewQuery.refetch(); }}
          displayName={previewQuery.data?.ownerName || '朋友'}
          showBrowserGuide={false}
          share={null}
          trackEvents={false}
        />
      </div>
    </div>
  );
}
