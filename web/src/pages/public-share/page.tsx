// ============================================================
// nav.ax Public Share Page — /u/:slug
// Same navigation layout as home / subdomain; share is a FAB only.
// ============================================================

import { useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import PublicNavigationView from '@/components/feature/PublicNavigationView';
import { EmptyState } from '@/components/base/SharedUI';
import { usePublicPage } from '@/hooks/useQueries';

export default function PublicSharePage() {
  const { slug } = useParams<{ slug: string }>();
  const { data: page, isLoading, error, refetch } = usePublicPage(slug || '');

  const shareUrl = useMemo(() => {
    if (typeof window === 'undefined') return '';
    if (page?.subdomainStatus === 'approved' && page.subdomain) {
      return `https://${page.subdomain}`;
    }
    return window.location.href;
  }, [page?.subdomain, page?.subdomainStatus]);

  const displayName = page?.ownerName || page?.title || '朋友';

  return (
    <PublicNavigationView
      page={page}
      isLoading={isLoading}
      error={error}
      onRetry={() => refetch()}
      displayName={displayName}
      showBrowserGuide={false}
      share={page ? {
        title: page.title,
        url: shareUrl,
        ownerName: page.ownerName || page.title,
        subdomain: page.subdomain || undefined,
      } : null}
      empty404={(error as { status?: number } | null)?.status === 404 ? (
        <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
          <EmptyState
            title="导航页不存在"
            description="该分享链接无效，或作者已取消发布。"
            action={
              <Link
                to="/"
                className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 transition-colors duration-150"
              >
                返回首页
              </Link>
            }
          />
        </div>
      ) : undefined}
    />
  );
}
