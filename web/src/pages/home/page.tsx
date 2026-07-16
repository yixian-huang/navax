import { useState, useMemo, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import { EmptyState, ErrorState } from '@/components/base/SharedUI';
import { useCurrentUser, useSystemPage } from '@/hooks/useQueries';
import LayoutFull from '@/pages/home/components/LayoutFull';
import LayoutSearchFocus from '@/pages/home/components/LayoutSearchFocus';
import LayoutBrowseFirst from '@/pages/home/components/LayoutBrowseFirst';
import LayoutSidebar from '@/pages/home/components/LayoutSidebar';
import BrowserGuide from '@/pages/home/components/BrowserGuide';
import { semanticFilterSites, buildSearchSuggestions } from '@/lib/searchIntel';
import type { HomeLayout } from '@/types/layout';
import type { Density, Site } from '@/api/types';
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

// 若输入看起来是网址/域名则返回可直接访问的 URL，否则返回 null（走搜索引擎）。
function resolveDirectUrl(input: string): string | null {
  const value = input.trim();
  if (!value || /\s/.test(value)) return null;
  if (/^https?:\/\//i.test(value)) return value;
  if (/^localhost(?::\d+)?(?:[/?#]\S*)?$/i.test(value)) return `https://${value}`;
  // 形如 example.com、sub.example.com/path?q=1，要求至少含一个点与合法后缀。
  if (/^([a-z0-9-]+\.)+[a-z]{2,}(?::\d+)?(?:[/?#]\S*)?$/i.test(value)) return `https://${value}`;
  return null;
}

export default function HomePage() {
  const { data: page, isLoading, error, refetch } = useSystemPage();
  const { data: authSession } = useCurrentUser();
  const now = useClock();
  const recordSiteClick = usePublicEventTracker(page?.id, page?.snapshotId);

  const [query, setQuery] = useState('');
  const [engine, setEngine] = useState<SearchEngine>('google');
  const [activeCategory, setActiveCategory] = useState('');
  const settings = page?.settings;

  // 采用站点配置的默认搜索引擎（用户随后仍可手动切换）。
  useEffect(() => {
    if (settings?.search?.defaultEngine) setEngine(settings.search.defaultEngine);
  }, [settings?.search?.defaultEngine]);
  const layout: HomeLayout = settings?.layout.template ?? 'full';
  const [density, setDensity] = useState<Density>('comfortable');

  useEffect(() => {
    if (settings?.layout.density) setDensity(settings.layout.density);
  }, [settings?.layout.density]);

  // System homepage always greets with "你好" — or show user's name if logged in
  const displayName = authSession?.user?.username || '朋友';
  const categories = useMemo(() => page?.categories ?? [], [page?.categories]);

  useEffect(() => {
    if (categories.length > 0 && !activeCategory) {
      setActiveCategory(categories[0].id);
    }
  }, [categories, activeCategory]);

  // 语义搜索：先尝试口语/意图匹配，命中则用语义结果，否则回退到字面匹配
  const activeSites = useMemo(() => {
    const source = !activeCategory
      ? categories.flatMap((c: any) => c.sites)
      : (categories.find((c: any) => c.id === activeCategory)?.sites || []);
    if (!query.trim()) return source;
    const semantic = semanticFilterSites(source, query);
    if (semantic.length > 0) return semantic;
    // 兵底：纯字面匹配
    const q = query.toLowerCase();
    return source.filter((s: Site) =>
      s.title.toLowerCase().includes(q) ||
      s.description?.toLowerCase().includes(q) ||
      s.url.toLowerCase().includes(q)
    );
  }, [categories, activeCategory, query]);

  const totalSites = useMemo(() => categories.reduce((sum: number, c: any) => sum + c.sites.length, 0), [categories]);

  // 基于用户真实收藏 + 当前时段生成动态搜索建议
  const searchSuggestions = useMemo(() => {
    const titles = categories.flatMap((c: any) => c.sites.map((s: Site) => s.title));
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

  const layoutProps = {
    query, onQueryChange: setQuery, engine, onEngineChange: setEngine, onSearch: handleSearch,
    categories, activeCategory, onCategoryChange: setActiveCategory,
    activeSites, density, onDensityChange: setDensity,
    totalSites, onSiteOpen: handleSiteOpen,
    searchSuggestions,
    showEngineSelector: settings?.search?.showEngineSelector ?? true,
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
    // 契约规定主站未发布时返回 404：对访客是空状态而非故障，管理员额外给发布入口。
    if ((error as { status?: number }).status === 404) {
      return (
        <PublicShell showSearch={false}>
          <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
            <EmptyState
              title="站点尚未发布"
              description="管理员还没有发布导航内容，发布后这里会展示站点导航。"
              action={
                authSession?.user?.role === 'admin' ? (
                  <Link
                    to="/app?scope=system"
                    className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-background-100 border border-background-200 text-sm text-foreground-600 hover:bg-background-200 transition-colors duration-150"
                  >
                    去发布主站内容
                  </Link>
                ) : undefined
              }
            />
          </div>
        </PublicShell>
      );
    }
    return (
      <PublicShell showSearch={false}>
        <div className="mx-auto max-w-4xl px-6 md:px-8 pt-20 pb-20">
          <ErrorState
            message={!(error as { status?: number }).status ? '网络连接失败，请检查网络后重试' : '加载导航数据失败'}
            onRetry={() => refetch()}
          />
        </div>
      </PublicShell>
    );
  }

  return (
    <PublicShell showSearch={false} themeId={settings?.appearance.themeId}>
      {settings?.appearance.background.type === 'image' && settings.appearance.background.value && (
        <div className="fixed inset-0 z-0 pointer-events-none">
          <img
            src={settings.appearance.background.value}
            alt=""
            className="w-full h-full object-cover"
          />
          <div
            className="absolute inset-0"
            style={{ backgroundColor: `rgba(255,255,255,${1 - settings.appearance.background.opacity})` }}
          />
        </div>
      )}
      <div className="relative z-10">
        {renderLayout()}
      </div>
      <BrowserGuide />
    </PublicShell>
  );
}
