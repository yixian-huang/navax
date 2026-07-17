// ============================================================
// nav.ax Draft Preview — /app/preview
// Renders the current draft via GET /pages/{id}/preview.
// ============================================================

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, ExternalLink } from 'lucide-react';
import { useMyPage } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { request } from '@/api/client';
import type { ApiResponse, PublishedPage } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';

export default function PreviewPage() {
  const pageQuery = useMyPage();
  const [preview, setPreview] = useState<PublishedPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const scope = new URLSearchParams(window.location.search).get('scope') === 'system' ? 'system' : 'personal';

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

  return (
    <div className="max-w-4xl">
      <div className="mb-5 flex items-center justify-between gap-3 flex-wrap">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Link
              to={`/app/publish?scope=${scope}`}
              className="inline-flex items-center gap-1 text-xs text-foreground-400 hover:text-foreground-600"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              返回发布
            </Link>
          </div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">草稿预览</h1>
          <p className="text-sm text-foreground-400 mt-1">
            这是当前草稿的只读投影，未发布内容不会影响公开页
          </p>
        </div>
        {preview.slug && (
          <Link
            to={`/u/${preview.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="h-9 px-3 rounded-lg border border-background-200 text-sm text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1.5"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            打开已发布页
          </Link>
        )}
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
            <section key={category.id}>
              <h3 className="text-sm font-semibold text-foreground-800 mb-2 flex items-center gap-2">
                {category.icon && <IconRenderer icon={category.icon} className="text-base" />}
                {category.name}
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {(category.sites ?? []).map(site => (
                  <a
                    key={site.id}
                    href={site.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-start gap-2.5 p-3 rounded-xl border border-background-200/70 hover:border-primary-200 hover:bg-primary-50/40 transition-colors"
                  >
                    <div className="w-8 h-8 rounded-lg bg-background-100 flex items-center justify-center flex-shrink-0">
                      <IconRenderer icon={site.icon} className="text-sm text-foreground-500" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-foreground-900 truncate">{site.title}</div>
                      <div className="text-xs text-foreground-400 truncate">{site.url}</div>
                    </div>
                  </a>
                ))}
              </div>
            </section>
          ))}
          {(preview.categories ?? []).length === 0 && (
            <p className="text-sm text-foreground-400">草稿中还没有分类与站点。</p>
          )}
        </div>
      </div>
    </div>
  );
}
