// ============================================================
// nav.ax Public Share Page — /u/:slug
// ============================================================

import { useState, useMemo, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Globe, ShieldCheck, Calendar, Bookmark, Layers, Copy, Check } from 'lucide-react';
import PublicShell from '@/components/feature/PublicShell';
import BrowserPageMenu from '@/components/feature/BrowserPageMenu';
import SiteCard from '@/components/base/SiteCard';
import CategoryTabs from '@/components/base/CategoryTabs';
import DensitySwitcher from '@/components/base/DensitySwitcher';
import ShareButton from '@/components/base/ShareButton';
import { EmptyState, ErrorState } from '@/components/base/SharedUI';
import { usePublicPage } from '@/hooks/useQueries';
import { cn } from '@/lib/utils';
import type { Density } from '@/api/types';
import { usePublicEventTracker } from '@/hooks/usePublicEventTracker';

// ---- Skeleton ----
function SiteGridSkeleton({ density }: { density: Density }) {
  if (density === 'list') {
    return (
      <div className="space-y-1">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="skeleton h-12 w-full rounded-lg" />
        ))}
      </div>
    );
  }
  const cols = density === 'comfortable'
    ? 'grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8'
    : 'grid-cols-4 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10';
  return (
    <div className={cn('grid gap-2', cols)}>
      {Array.from({ length: 12 }).map((_, i) => (
        <div key={i} className={cn('skeleton rounded-xl', density === 'compact' ? 'h-16' : 'h-24')} />
      ))}
    </div>
  );
}

// ---- Main Component ----
export default function PublicSharePage() {
  const { slug } = useParams<{ slug: string }>();
  const { data: page, isLoading, error, refetch } = usePublicPage(slug || '');
  const recordSiteClick = usePublicEventTracker(page?.id, page?.snapshotId);
  const [query, setQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState('');
  const [density, setDensity] = useState<Density>('comfortable');
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    if (!page?.settings?.display.showClock) return;
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, [page?.settings?.display.showClock]);

  useEffect(() => {
    if (page?.settings?.layout.density) setDensity(page.settings.layout.density);
  }, [page?.settings?.layout.density]);

  // Set initial active category
  useEffect(() => {
    if (page?.categories && page.categories.length > 0 && !activeCategory) {
      setActiveCategory(page.categories[0].id);
    }
  }, [page?.categories, activeCategory]);

  const categories = useMemo(() => page?.categories ?? [], [page?.categories]);
  const totalSites = categories.reduce((sum, c) => sum + c.sites.length, 0);
  const domainApproved = page?.subdomainStatus === 'approved' && !!page?.subdomain;
  const customDomain = domainApproved ? `https://${page!.subdomain}` : null;
  const shareUrl = typeof window !== 'undefined'
    ? (customDomain || window.location.href)
    : '';
  const display = page?.settings?.display;
  const greeting = now.getHours() < 12 ? '上午好' : now.getHours() < 18 ? '下午好' : '晚上好';
  const formattedNowDate = now.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' });
  const formattedTime = now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });

  const activeSites = useMemo(() => {
    if (!activeCategory) {
      const all = categories.flatMap(c => c.sites);
      if (!query.trim()) return all;
      const q = query.toLowerCase();
      return all.filter(s =>
        s.title.toLowerCase().includes(q) ||
        s.description?.toLowerCase().includes(q) ||
        s.url.toLowerCase().includes(q)
      );
    }
    const cat = categories.find(c => c.id === activeCategory);
    const sites = cat?.sites || [];
    if (!query.trim()) return sites;
    const q = query.toLowerCase();
    return sites.filter(s =>
      s.title.toLowerCase().includes(q) ||
      s.description?.toLowerCase().includes(q) ||
      s.url.toLowerCase().includes(q)
    );
  }, [categories, activeCategory, query]);

  const isNetworkError = error && !(error as { status?: number }).status;
  const is404 = error && (error as { status?: number }).status === 404;

  const copyShareUrl = () => {
    navigator.clipboard.writeText(shareUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  // Format date
  const formattedDate = page?.updatedAt
    ? new Date(page.updatedAt).toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
    : '';

  // ---- Loading ----
  if (isLoading) {
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-4 md:px-6 pt-12 md:pt-16 pb-16">
          <div className="flex items-center gap-4 mb-8">
            <div className="skeleton w-16 h-16 rounded-full" />
            <div>
              <div className="skeleton h-5 w-32 rounded mb-2" />
              <div className="skeleton h-4 w-48 rounded" />
            </div>
          </div>
          <div className="skeleton h-8 w-48 rounded-lg mb-2" />
          <div className="skeleton h-5 w-64 rounded-lg mb-10" />
          <div className="skeleton h-12 w-full rounded-xl mb-6" />
          <SiteGridSkeleton density={density} />
        </div>
      </PublicShell>
    );
  }

  // ---- 404 ----
  if (is404 || (!isLoading && !page)) {
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-4 md:px-6 pt-20 pb-16">
          <ErrorState message="该导航页不存在或已被设为私密" />
          <div className="text-center mt-6">
            <Link
              to="/"
              className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
            >
              <i className="ri-arrow-left-line text-base" />
              返回 nav.ax 首页
            </Link>
          </div>
        </div>
      </PublicShell>
    );
  }

  // ---- Error ----
  if (error) {
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-4 md:px-6 pt-20 pb-16">
          <ErrorState
            message={isNetworkError ? '网络连接失败，请检查网络后重试' : '加载导航页失败'}
            onRetry={() => refetch()}
          />
        </div>
      </PublicShell>
    );
  }

  const bg = page.settings?.appearance.background;
  const backgroundUrl =
    (bg?.type === 'image' || bg?.type === 'video') && bg.value ? bg.value : undefined;
  const backgroundMediaType = bg?.type === 'video' ? 'video' as const : 'image' as const;
  const backgroundPoster = bg?.poster ?? undefined;
  const wallpaperMode = Boolean(backgroundUrl);

  return (
    <PublicShell
      showSearch={false}
      themeId={page.settings?.appearance.themeId}
      backgroundUrl={backgroundUrl}
      backgroundOpacity={bg?.opacity ?? 1}
      backgroundMediaType={backgroundMediaType}
      backgroundPoster={backgroundPoster}
    >
      <div className={cn(
        'mx-auto max-w-4xl px-4 md:px-6 pb-16',
        wallpaperMode ? 'pt-10 md:pt-14' : 'pt-10 md:pt-14',
      )}>
        {/* ---- Author Card ---- */}
        <div className="flex flex-col sm:flex-row items-center sm:items-start gap-5 mb-8">
          {/* Avatar */}
          <div className="w-16 h-16 md:w-18 md:h-18 rounded-full overflow-hidden bg-background-200 flex-shrink-0 ring-2 ring-background-200/50">
            {page.ownerAvatar ? (
              <img
                src={page.ownerAvatar}
                alt={page.ownerName}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center text-foreground-400">
                <i className="ri-user-line text-2xl" />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0 text-center sm:text-left">
            {/* Name + Domain badge */}
            <div className="flex flex-col sm:flex-row items-center sm:items-center gap-2 mb-1.5">
              <h1 className="text-xl md:text-2xl font-bold font-heading text-foreground-950">
                {page.title}
              </h1>
              {domainApproved && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-green-50 border border-green-200 text-green-700 text-[11px] font-medium whitespace-nowrap">
                  <ShieldCheck className="w-3 h-3" />
                  {page.subdomain}
                </span>
              )}
            </div>

            {/* Description */}
            {page.description && (
              <p className="text-sm text-foreground-500 leading-relaxed max-w-lg">
                {page.description}
              </p>
            )}

            {/* Stats row */}
            <div className="flex flex-wrap items-center justify-center sm:justify-start gap-4 mt-3">
              <div className="flex items-center gap-1.5 text-xs text-foreground-400">
                <Bookmark className="w-3.5 h-3.5" />
                <span>{categories.length} 个分类</span>
              </div>
              <div className="flex items-center gap-1.5 text-xs text-foreground-400">
                <Layers className="w-3.5 h-3.5" />
                <span>{totalSites} 个站点</span>
              </div>
              {formattedDate && (
                <div className="flex items-center gap-1.5 text-xs text-foreground-400">
                  <Calendar className="w-3.5 h-3.5" />
                  <span>更新于 {formattedDate}</span>
                </div>
              )}
              <span className="inline-flex items-center gap-1.5 text-xs text-foreground-500">
                <span className="w-1.5 h-1.5 rounded-full bg-green-400" />
                {page.ownerName}
              </span>
            </div>

            {/* Domain CTA — when no custom domain */}
            {page.subdomainStatus === 'none' && (
              <div className="mt-3 inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-background-100 border border-background-200/70 text-[11px] text-foreground-400">
                <Globe className="w-3 h-3" />
                {window.location.hostname}/u/{slug}
              </div>
            )}
          </div>

          {/* Share + Copy */}
          <div className="flex items-center gap-2 flex-shrink-0">
            <div className="flex items-center gap-1 bg-background-50/90 rounded-lg border border-background-200/70 p-1">
              <span className="hidden sm:block px-2 text-[11px] text-foreground-400 truncate max-w-[180px]">
                {shareUrl.replace(/^https?:\/\//, '')}
              </span>
              <button
                onClick={copyShareUrl}
                className="h-8 px-3 rounded-md bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center gap-1.5 whitespace-nowrap"
              >
                {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                {copied ? '已复制' : '复制链接'}
              </button>
            </div>
            <ShareButton url={shareUrl} title={page.title} />
          </div>
        </div>

        {(display?.showGreeting || display?.showDate || display?.showClock) && (
          <div className="mb-8 flex flex-wrap items-center gap-x-5 gap-y-2 rounded-xl border border-background-200/70 bg-background-50 px-4 py-3 text-sm text-foreground-500">
            {display.showGreeting && <span className="font-medium text-foreground-800">{greeting}</span>}
            {display.showDate && <span>{formattedNowDate}</span>}
            {display.showClock && <span className="font-mono tabular-nums text-foreground-800">{formattedTime}</span>}
          </div>
        )}

        {/* ---- Search ---- */}
        <div className="relative mb-6">
          <div className="flex items-center bg-background-50/90 rounded-xl border border-background-200/70 focus-within:border-primary-300 transition-all duration-200">
            <i className="ri-search-line absolute left-4 w-5 h-5 text-foreground-300" />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="在当前收藏中搜索..."
              className="flex-1 h-12 pl-12 pr-4 bg-transparent text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none rounded-xl"
              aria-label="搜索收藏"
            />
            {query && (
              <button
                onClick={() => setQuery('')}
                className="mr-3 w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-foreground-500 hover:bg-background-100 transition-colors duration-150"
                aria-label="清除搜索"
              >
                <i className="ri-close-line text-sm" />
              </button>
            )}
          </div>
        </div>

        {/* ---- Categories ---- */}
        {categories.length > 1 && (
          <CategoryTabs
            categories={categories}
            activeId={activeCategory}
            onChange={setActiveCategory}
            showAll
          />
        )}

        {/* ---- Toolbar ---- */}
        <div className="flex items-center justify-between mt-5 mb-4">
          <span className="text-xs text-foreground-400">
            {activeSites.length} 个站点
            {query && <span className="ml-1">匹配 &ldquo;{query}&rdquo;</span>}
          </span>
          <DensitySwitcher density={density} onChange={setDensity} />
        </div>

        {/* ---- Sites ---- */}
        {activeSites.length === 0 ? (
          <EmptyState
            iconClass="ri-bookmark-line"
            title={query ? '没有匹配的站点' : '暂无收藏站点'}
            description={query ? '换个关键词试试' : '该导航页暂未添加任何站点'}
          />
        ) : (
          <>
            {density === 'list' ? (
              <div className="space-y-1">
                {activeSites.map(site => (
                  <SiteCard
                    key={site.id}
                    site={site}
                    density="list"
                    onOpen={clickedSite => {
                      recordSiteClick(clickedSite.id);
                      window.open(clickedSite.url, '_blank', 'noopener,noreferrer');
                    }}
                  />
                ))}
              </div>
            ) : (
              <div className={cn(
                'grid gap-1.5',
                density === 'comfortable'
                  ? 'grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8'
                  : 'grid-cols-4 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10'
              )}>
                {activeSites.map(site => (
                  <SiteCard
                    key={site.id}
                    site={site}
                    density={density}
                    onOpen={clickedSite => {
                      recordSiteClick(clickedSite.id);
                      window.open(clickedSite.url, '_blank', 'noopener,noreferrer');
                    }}
                  />
                ))}
              </div>
            )}
          </>
        )}

        {/* ---- Footer ---- */}
        <div className="mt-14 pt-8 border-t border-background-200/50">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              {page.ownerAvatar && (
                <img
                  src={page.ownerAvatar}
                  alt=""
                  className="w-8 h-8 rounded-full object-cover"
                />
              )}
              <div className="text-xs text-foreground-400">
                由 <span className="font-medium text-foreground-600">{page.ownerName}</span> 创建
              </div>
            </div>
            <Link
              to="/"
              className="inline-flex items-center gap-1.5 text-xs text-foreground-300 hover:text-primary-500 transition-colors duration-150"
            >
              Powered by <span className="font-heading font-bold text-foreground-400">nav.ax</span>
            </Link>
          </div>
        </div>
      </div>
      <BrowserPageMenu />
    </PublicShell>
  );
}
