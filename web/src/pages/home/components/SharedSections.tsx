import { Link } from 'react-router-dom';
import SearchBar from '@/components/base/SearchBar';
import SiteGrid, { SiteCountLabel } from '@/components/base/SiteGrid';
import CategoryTabs from '@/components/base/CategoryTabs';
import CategoryFolderWall from '@/components/base/CategoryFolderWall';
import DensitySwitcher from '@/components/base/DensitySwitcher';
import { useCurrentUser } from '@/hooks/useQueries';
import { cn } from '@/lib/utils';
import type { Density, Site } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';

export type CategoryStyle = 'tabs' | 'sidebar' | 'grid' | 'folders';

export function SearchSection({
  query, onQueryChange, engine, onEngineChange, onSearch,
  delay, size = 'lg', suggestions, showEngineSelector = true,
  wallpaperMode = false,
}: {
  query: string; onQueryChange: (v: string) => void;
  engine: SearchEngine; onEngineChange: (e: SearchEngine) => void;
  onSearch: (q: string, e: SearchEngine) => void;
  delay: number;
  size?: 'lg' | 'md';
  suggestions?: string[];
  showEngineSelector?: boolean;
  /** Wallpaper mode: no permanent hint; slightly tighter spacing. */
  wallpaperMode?: boolean;
}) {
  // relative z-20: rise-in uses transform (stacking context). Without z-index,
  // later sections (SitesSection) paint over search engine / history dropdowns.
  return (
    <div
      className={cn(
        'relative z-20 rise-in',
        wallpaperMode ? 'mb-8 md:mb-10' : 'mb-14 md:mb-16',
      )}
      style={{ animationDelay: `${delay}ms` }}
    >
      <SearchBar
        value={query}
        onChange={onQueryChange}
        onSearch={onSearch}
        engine={engine}
        onEngineChange={onEngineChange}
        showEngineSelector={showEngineSelector}
        showHint={!wallpaperMode}
        size={size}
        suggestions={suggestions}
      />
    </div>
  );
}

export function SitesSection({
  categories, activeCategory, onCategoryChange,
  activeSites, density, onDensityChange,
  totalSites, query, onSiteOpen, delay,
  wallpaperMode = false,
  categoryStyle = 'tabs',
}: {
  categories: any[]; activeCategory: string;
  onCategoryChange: (id: string) => void;
  activeSites: Site[]; density: Density;
  onDensityChange: (d: Density) => void;
  totalSites: number; query: string;
  onSiteOpen: (s: Site) => void;
  delay: number;
  wallpaperMode?: boolean;
  categoryStyle?: CategoryStyle;
}) {
  const { data: authSession } = useCurrentUser();
  const canManageLinks = Boolean(authSession?.authenticated && authSession.user);
  const useFolders = categoryStyle === 'folders' && !query.trim();

  // No section-level slab for categories / density / site grid — wallpaper shows
  // through; individual site cards use low-opacity frost via [data-wallpaper].
  return (
    <div className="rise-in" style={{ animationDelay: `${delay}ms` }}>
      <div className={cn(
        'flex items-baseline justify-between gap-4',
        wallpaperMode ? 'mb-3' : 'mb-5',
      )}>
        <div className="flex items-baseline gap-3 min-w-0">
          {!wallpaperMode && (
            <h2 className="font-heading text-lg text-foreground-900 tracking-tight">收藏站点</h2>
          )}
          {wallpaperMode ? (
            query ? (
              <span className="text-[11px] text-foreground-700 tracking-wide truncate wallpaper-type">
                {activeSites.length} 个结果
              </span>
            ) : useFolders ? (
              <span className="text-[11px] text-foreground-700 tracking-wide truncate wallpaper-type">
                {categories.length} 个文件夹 · {totalSites} 个站点
              </span>
            ) : null
          ) : useFolders ? (
            <SiteCountLabel count={totalSites} total={totalSites} query="" />
          ) : (
            <SiteCountLabel count={activeSites.length} total={totalSites} query={query} />
          )}
        </div>
        {!useFolders && (
          <DensitySwitcher density={density} onChange={onDensityChange} />
        )}
      </div>

      {useFolders ? (
        <div className={cn(wallpaperMode && 'wallpaper-sites-scope')}>
          <CategoryFolderWall
            categories={categories}
            onSiteOpen={onSiteOpen}
          />
        </div>
      ) : (
        <>
          {categories.length > 1 && (
            <div className={cn(wallpaperMode ? 'mb-4 wallpaper-type wallpaper-ink-scope' : 'mb-8')}>
              <CategoryTabs
                categories={categories}
                activeId={activeCategory}
                onChange={onCategoryChange}
              />
            </div>
          )}

          <div className={cn(wallpaperMode && 'wallpaper-sites-scope')}>
            <SiteGrid
              sites={activeSites}
              density={density}
              query={query}
              onSiteOpen={onSiteOpen}
              showAddLink={canManageLinks && !query}
            />
          </div>
        </>
      )}
    </div>
  );
}

export function FooterActions({ wallpaperMode = false }: { wallpaperMode?: boolean }) {
  if (wallpaperMode) {
    // Wallpaper: quiet text links — no glow, no frosted pill (matches shell footer).
    return (
      <div className="mt-10 md:mt-12 flex justify-center gap-1 rise-in">
        <Link
          to="/app/links"
          className="h-8 px-3 inline-flex items-center gap-1.5 text-[11px] text-foreground-600/90 hover:text-primary-500 transition-colors duration-200 rounded-full hover:bg-background-50/20"
          title="管理站点"
        >
          <i className="ri-settings-3-line text-sm" />
          管理
        </Link>
        <Link
          to="/app"
          className="h-8 px-3 inline-flex items-center gap-1.5 text-[11px] text-foreground-600/90 hover:text-primary-500 transition-colors duration-200 rounded-full hover:bg-background-50/20"
          title="编辑主页"
        >
          <i className="ri-layout-grid-line text-sm" />
          编辑
        </Link>
        <Link
          to="/discover"
          className="h-8 px-3 inline-flex items-center gap-1.5 text-[11px] text-foreground-600/80 hover:text-primary-500 transition-colors duration-200 rounded-full hover:bg-background-50/20"
          title="发现精选"
        >
          <i className="ri-compass-3-line text-sm" />
          发现
        </Link>
      </div>
    );
  }

  return (
    <div className="mt-20 md:mt-24">
      <div className="hairline-gradient mb-7" />
      <div className="flex items-center justify-center gap-8">
        <Link
          to="/app/links"
          className="group inline-flex items-center gap-2 text-xs text-foreground-400 hover:text-primary-500 transition-colors duration-200"
        >
          <i className="ri-settings-3-line text-sm text-foreground-300 group-hover:text-primary-500 transition-colors duration-200" />
          管理站点
        </Link>
        <Link
          to="/app"
          className="group inline-flex items-center gap-2 text-xs text-foreground-400 hover:text-primary-500 transition-colors duration-200"
        >
          <i className="ri-layout-grid-line text-sm text-foreground-300 group-hover:text-primary-500 transition-colors duration-200" />
          编辑主页
        </Link>
        <Link
          to="/discover"
          className="group inline-flex items-center gap-2 text-xs text-foreground-300 hover:text-primary-500 transition-colors duration-200"
        >
          <i className="ri-compass-3-line text-sm text-foreground-300 group-hover:text-primary-500 transition-colors duration-200" />
          发现精选
        </Link>
      </div>
    </div>
  );
}
