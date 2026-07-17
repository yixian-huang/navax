// ============================================================
// Shared public navigation surface for:
//   - system / subdomain home  (/)
//   - personal share path      (/u/:slug)
// Layout is identical; callers only supply data + optional chrome.
// ============================================================

import { useState, useMemo, useEffect, useCallback, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import { EmptyState, ErrorState } from '@/components/base/SharedUI';
import LayoutFull from '@/pages/home/components/LayoutFull';
import LayoutSearchFocus from '@/pages/home/components/LayoutSearchFocus';
import LayoutBrowseFirst from '@/pages/home/components/LayoutBrowseFirst';
import LayoutSidebar from '@/pages/home/components/LayoutSidebar';
import BrowserGuide from '@/pages/home/components/BrowserGuide';
import BrowserPageMenu from '@/components/feature/BrowserPageMenu';
import QuickAddSiteFab from '@/components/feature/QuickAddSiteFab';
import SharePageFab from '@/components/feature/SharePageFab';
import { semanticFilterSites, buildSearchSuggestions } from '@/lib/searchIntel';
import type { HomeLayout } from '@/types/layout';
import type { Density, PublishedNavigationPage, Site } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';
import { engines } from '@/components/base/SearchBar';
import { usePublicEventTracker } from '@/hooks/usePublicEventTracker';

function getTimeGreeting(): string {
  const h = new Date().getHours();
  if (h < 6) return '夜深了';
  if (h < 9) return '早上好';
  if (h < 12) return '上午好';
  if (h < 14) return '中午好';
  if (h < 18) return '下午好';
  return '晚上好';
}

function useClock() {
  const [now, setNow] = useState(new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(t);
  }, []);
  return now;
}

const weekDays = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];

function resolveDirectUrl(input: string): string | null {
  const value = input.trim();
  if (!value || /\s/.test(value)) return null;
  if (/^https?:\/\//i.test(value)) return value;
  if (/^localhost(?::\d+)?(?:[/?#]\S*)?$/i.test(value)) return `https://${value}`;
  if (/^([a-z0-9-]+\.)+[a-z]{2,}(?::\d+)?(?:[/?#]\S*)?$/i.test(value)) return `https://${value}`;
  return null;
}

export interface PublicNavigationViewProps {
  page: PublishedNavigationPage | undefined;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
  /** Greeting name (logged-in visitor name, or page owner, or 朋友). */
  displayName: string;
  /** First-visit browser guide (typically system home only). */
  showBrowserGuide?: boolean;
  /** Floating share panel for /u and personal public pages. */
  share?: {
    title: string;
    url: string;
    ownerName: string;
    subdomain?: string;
  } | null;
  /** Custom empty state when page is 404 / unpublished. */
  empty404?: ReactNode;
}

export default function PublicNavigationView({
  page,
  isLoading,
  error,
  onRetry,
  displayName,
  showBrowserGuide = false,
  share = null,
  empty404,
}: PublicNavigationViewProps) {
  const now = useClock();
  const recordSiteClick = usePublicEventTracker(page?.id, page?.snapshotId);

  const [query, setQuery] = useState('');
  const [engine, setEngine] = useState<SearchEngine>('google');
  const [activeCategory, setActiveCategory] = useState('');
  const settings = page?.settings;

  useEffect(() => {
    if (settings?.search?.defaultEngine) setEngine(settings.search.defaultEngine as SearchEngine);
  }, [settings?.search?.defaultEngine]);

  const layout: HomeLayout = (settings?.layout.template as HomeLayout) ?? 'full';
  const [density, setDensity] = useState<Density>('comfortable');

  useEffect(() => {
    if (settings?.layout.density) setDensity(settings.layout.density);
  }, [settings?.layout.density]);

  const categories = useMemo(() => page?.categories ?? [], [page?.categories]);

  useEffect(() => {
    if (categories.length > 0 && !activeCategory) {
      setActiveCategory(categories[0].id);
    }
  }, [categories, activeCategory]);

  const activeSites = useMemo(() => {
    const source = !activeCategory
      ? categories.flatMap((c) => c.sites)
      : (categories.find((c) => c.id === activeCategory)?.sites || []);
    if (!query.trim()) return source;
    const semantic = semanticFilterSites(source, query);
    if (semantic.length > 0) return semantic;
    const q = query.toLowerCase();
    return source.filter((s: Site) =>
      s.title.toLowerCase().includes(q)
      || s.description?.toLowerCase().includes(q)
      || s.url.toLowerCase().includes(q),
    );
  }, [categories, activeCategory, query]);

  const totalSites = useMemo(
    () => categories.reduce((sum, c) => sum + c.sites.length, 0),
    [categories],
  );

  const searchSuggestions = useMemo(() => {
    const titles = categories.flatMap((c) => c.sites.map((s: Site) => s.title));
    return buildSearchSuggestions(titles, now.getHours());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [categories]);

  const handleSearch = useCallback((queryStr: string, eng: SearchEngine) => {
    const direct = resolveDirectUrl(queryStr);
    if (direct) {
      window.open(direct, '_blank', 'noopener,noreferrer');
      return;
    }
    const engObj = engines.find(e => e.key === eng);
    if (engObj) window.open(engObj.url + encodeURIComponent(queryStr), '_blank', 'noopener,noreferrer');
  }, []);

  const handleSiteOpen = useCallback((site: Site) => {
    recordSiteClick(site.id);
    window.open(site.url, '_blank', 'noopener,noreferrer');
  }, [recordSiteClick]);

  const timeStr = now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
  const secondsStr = now.toLocaleTimeString('zh-CN', { second: '2-digit' }).padStart(2, '0');
  const dateStr = now.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' });
  const weekDay = weekDays[now.getDay()];
  const greeting = getTimeGreeting();

  const bg = settings?.appearance.background;
  const backgroundUrl =
    (bg?.type === 'image' || bg?.type === 'video') && bg.value ? bg.value : undefined;
  const backgroundMediaType = bg?.type === 'video' ? 'video' as const : 'image' as const;
  const backgroundPoster = bg?.poster ?? undefined;
  const wallpaperMode = Boolean(backgroundUrl);

  const layoutProps = {
    query, onQueryChange: setQuery, engine, onEngineChange: setEngine, onSearch: handleSearch,
    categories, activeCategory, onCategoryChange: setActiveCategory,
    activeSites, density, onDensityChange: setDensity,
    totalSites, onSiteOpen: handleSiteOpen,
    searchSuggestions,
    showEngineSelector: settings?.search?.showEngineSelector ?? true,
    wallpaperMode,
  };

  const renderLayout = () => {
    switch (layout) {
      case 'search-focus':
        return <LayoutSearchFocus {...layoutProps} />;
      case 'browse-first':
        return <LayoutBrowseFirst {...layoutProps} />;
      case 'sidebar':
        return <LayoutSidebar {...layoutProps} />;
      default:
        return (
          <LayoutFull
            {...layoutProps}
            greeting={greeting}
            displayName={displayName}
            dateStr={dateStr}
            weekDay={weekDay}
            timeStr={timeStr}
            secondsStr={secondsStr}
            showGreeting={settings?.display.showGreeting ?? true}
            showDate={settings?.display.showDate ?? true}
            showClock={settings?.display.showClock ?? true}
          />
        );
    }
  };

  if (isLoading) {
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-6 md:px-8 pt-16 md:pt-24 pb-24">
          <div className="skeleton h-4 w-32 rounded mb-5" />
          <div className="skeleton h-12 w-80 rounded-lg mb-14" />
          <div className="skeleton h-16 w-full rounded-2xl mb-16" />
          <div className="flex gap-6 mb-10">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="skeleton h-5 w-20 rounded" />
            ))}
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="skeleton h-24 rounded-xl" />
            ))}
          </div>
        </div>
      </PublicShell>
    );
  }

  if (error) {
    if ((error as { status?: number }).status === 404) {
      if (empty404) {
        return <PublicShell showSearch={false}>{empty404}</PublicShell>;
      }
      return (
        <PublicShell showSearch={false}>
          <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
            <EmptyState title="导航尚未发布" description="发布后这里会展示导航内容。" />
          </div>
        </PublicShell>
      );
    }
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
          <ErrorState
            message={!(error as { status?: number }).status ? '网络连接失败，请检查网络后重试' : '加载导航数据失败'}
            onRetry={onRetry}
          />
        </div>
      </PublicShell>
    );
  }

  if (!page) {
    return null;
  }

  return (
    <PublicShell
      showSearch={false}
      themeId={settings?.appearance.themeId}
      backgroundUrl={backgroundUrl}
      backgroundOpacity={bg?.opacity ?? 1}
      backgroundMediaType={backgroundMediaType}
      backgroundPoster={backgroundPoster}
    >
      {renderLayout()}
      {showBrowserGuide && !wallpaperMode && <BrowserGuide />}
      <BrowserPageMenu />
      <QuickAddSiteFab />
      {share && (
        <SharePageFab
          title={share.title}
          url={share.url}
          ownerName={share.ownerName}
          subdomain={share.subdomain}
        />
      )}
    </PublicShell>
  );
}

/** Helper empty state used by system home 404. */
export function HomeEmpty404({
  isSubdomainHost,
  isLoggedIn,
  isAdmin,
}: {
  isSubdomainHost: boolean;
  isLoggedIn: boolean;
  isAdmin: boolean;
}) {
  return (
    <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
      <EmptyState
        title={isSubdomainHost ? '该子域名导航尚未发布' : '站点尚未发布'}
        description={
          isSubdomainHost
            ? '子域名绑定的是你的「个人导航」已发布内容。请到工作台切换到「我的导航」，添加链接后点击「发布」。主站内容不会自动出现在子域名上。'
            : '管理员还没有发布导航内容，发布后这里会展示站点导航。'
        }
        action={
          isLoggedIn ? (
            <Link
              to={isSubdomainHost ? '/app?scope=personal' : (isAdmin ? '/app?scope=system' : '/app?scope=personal')}
              className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-background-100 border border-background-200 text-sm text-foreground-600 hover:bg-background-200 transition-colors duration-150"
            >
              {isSubdomainHost ? '去发布我的导航' : '去发布主站内容'}
            </Link>
          ) : undefined
        }
      />
    </div>
  );
}
