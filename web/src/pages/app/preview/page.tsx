// ============================================================
// nav.ax Draft Preview — /app/preview
// Renders the current draft via GET /pages/{id}/preview.
// ============================================================

import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowLeft, ChevronRight, ExternalLink, Globe, Loader2 } from 'lucide-react';
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
import { request } from '@/api/client';
import type { ApiResponse, PublishedPage } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';
import { cn } from '@/lib/utils';

type PreviewCategory = NonNullable<PublishedPage['categories']>[number];
type PreviewSite = NonNullable<PreviewCategory['sites']>[number];

function PreviewCategorySection({ category }: { category: PreviewCategory }) {
  const sites = category.sites ?? [];
  const [collapsed, setCollapsed] = useState(sites.length > 24);

  return (
    <section>
      <button
        type="button"
        onClick={() => setCollapsed(c => !c)}
        className="w-full text-left text-sm font-semibold text-foreground-800 mb-2 flex items-center gap-2 rounded-md hover:bg-background-100/80 px-1 py-1 -mx-1"
        aria-expanded={!collapsed}
      >
        <ChevronRight
          className={cn(
            'w-3.5 h-3.5 text-foreground-300 flex-shrink-0 transition-transform',
            !collapsed && 'rotate-90',
          )}
        />
        {category.icon && <IconRenderer icon={category.icon} className="text-base" />}
        <span className="truncate">{category.name}</span>
        <span className="text-xs font-normal text-foreground-400">({sites.length})</span>
      </button>
      {!collapsed && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {sites.map(site => (
            <PreviewSiteCard key={site.id} site={site} />
          ))}
        </div>
      )}
      {collapsed && sites.length > 0 && (
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="w-full py-2 text-[11px] text-foreground-400 hover:text-primary-600 border border-dashed border-background-200/70 rounded-md"
        >
          已折叠 {sites.length} 个站点 · 点击展开
        </button>
      )}
    </section>
  );
}

function PreviewSiteCard({ site }: { site: PreviewSite }) {
  return (
    <a
      href={site.url}
      target="_blank"
      rel="noopener noreferrer"
      className="flex items-start gap-2.5 p-3 rounded-xl border border-background-200/70 hover:border-primary-200 hover:bg-primary-50/40 transition-colors"
    >
      <div className="w-8 h-8 rounded-lg bg-background-100 flex items-center justify-center flex-shrink-0">
        <IconRenderer icon={site.icon} url={site.url} size={16} alt={site.title} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-foreground-900 break-words leading-snug">{site.title}</div>
        {site.description ? (
          <div className="text-xs text-foreground-400 mt-0.5 line-clamp-2 break-words">{site.description}</div>
        ) : null}
        <div className="text-xs text-primary-600 mt-0.5 truncate font-mono flex items-center gap-1">
          <span className="truncate">{site.url.replace(/^https?:\/\//, '')}</span>
          <ExternalLink className="w-3 h-3 flex-shrink-0 opacity-70" />
        </div>
      </div>
    </a>
  );
}

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
  const [preview, setPreview] = useState<PublishedPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!pageQuery.data?.id) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    request<ApiResponse<PublishedPage>>(`/pages/${pageQuery.data.id}/preview`)
      .then(response => {
        if (!cancelled) setPreview(response.data);
      })
      .catch(cause => {
        if (!cancelled) setError(cause instanceof Error ? cause.message : '加载预览失败');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [pageQuery.data?.id]);

  if (pageQuery.isLoading || loading) return <LoadingSkeleton count={4} />;
  if (pageQuery.isError || error || !preview) {
    return (
      <ErrorState
        message={error || pageQuery.error?.message || '无法加载草稿预览'}
        onRetry={() => pageQuery.refetch()}
      />
    );
  }

  const isPublished = state.showUnpublish || publication?.published === true;
  const liveSlug = slug || publication?.slug || preview.slug;

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
    <div className="max-w-4xl">
      <div className="mb-5 flex items-center justify-between gap-3 flex-wrap">
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
            这是当前草稿的只读投影，未发布内容不会影响公开页
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

      <div className="rounded-2xl border border-background-200/70 bg-background-50 p-5 md:p-6 space-y-5">
        <div>
          <h2 className="text-xl font-semibold text-foreground-900">{preview.title}</h2>
          {preview.description && (
            <p className="text-sm text-foreground-500 mt-1">{preview.description}</p>
          )}
          <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-foreground-400">
            <span className="px-2 py-0.5 rounded-full bg-background-100">slug: {preview.slug}</span>
            <span className="px-2 py-0.5 rounded-full bg-background-100">可见性: {preview.visibility}</span>
            {preview.seoTitle && (
              <span className="px-2 py-0.5 rounded-full bg-background-100">SEO: {preview.seoTitle}</span>
            )}
          </div>
        </div>

        <div className="space-y-4">
          {(preview.categories ?? []).map(category => (
            <PreviewCategorySection key={category.id} category={category} />
          ))}
          {(preview.categories ?? []).length === 0 && (
            <p className="text-sm text-foreground-400">草稿中还没有分类与站点。</p>
          )}
        </div>
      </div>
    </div>
  );
}
