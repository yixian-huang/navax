import { SearchSection, SitesSection, FooterActions } from './SharedSections';
import { cn } from '@/lib/utils';
import type { Density, Site } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';

interface LayoutProps {
  greeting: string;
  displayName: string;
  dateStr: string;
  weekDay: string;
  timeStr: string;
  secondsStr: string;
  showGreeting: boolean;
  showDate: boolean;
  showClock: boolean;
  query: string;
  onQueryChange: (v: string) => void;
  engine: SearchEngine;
  onEngineChange: (e: SearchEngine) => void;
  onSearch: (q: string, e: SearchEngine) => void;
  categories: any[];
  activeCategory: string;
  onCategoryChange: (id: string) => void;
  activeSites: Site[];
  density: Density;
  onDensityChange: (d: Density) => void;
  totalSites: number;
  onSiteOpen: (s: Site) => void;
  searchSuggestions?: string[];
  showEngineSelector?: boolean;
  wallpaperMode?: boolean;
}

export default function LayoutFull({
  greeting, displayName, dateStr, weekDay, timeStr, secondsStr,
  showGreeting, showDate, showClock,
  query, onQueryChange, engine, onEngineChange, onSearch, showEngineSelector,
  categories, activeCategory, onCategoryChange,
  activeSites, density, onDensityChange, totalSites, onSiteOpen,
  searchSuggestions,
  wallpaperMode = false,
}: LayoutProps) {
  const showHeader = showGreeting || showDate || showClock;

  return (
    <div className={cn(
      'mx-auto max-w-4xl px-6 md:px-8 pb-24',
      wallpaperMode ? 'pt-10 md:pt-14' : 'pt-16 md:pt-24',
    )}>
      {showHeader && (
        <header
          className={cn(
            'rise-in',
            wallpaperMode
              ? 'wallpaper-surface rounded-2xl px-5 py-4 md:px-6 md:py-5 mb-6 md:mb-8'
              : 'mb-12 md:mb-16',
          )}
        >
          <div className="flex items-end justify-between gap-6 flex-wrap">
            <div className="min-w-0">
              {showDate && (
                <p className={cn(
                  'text-[11px] font-medium tracking-[0.22em] uppercase mb-2',
                  wallpaperMode ? 'text-accent-700' : 'text-accent-600 mb-3',
                )}>
                  {dateStr} · {weekDay}
                </p>
              )}
              {showGreeting && (
                <h1 className={cn(
                  'font-heading leading-[1.05] tracking-tight text-foreground-950',
                  wallpaperMode ? 'text-2xl md:text-3xl' : 'text-4xl md:text-5xl',
                )}>
                  {greeting}，
                  <span className="italic font-medium text-primary-500">{displayName}</span>
                </h1>
              )}
              {/* Wallpaper: drop the marketing tagline — less noise, clearer photo. */}
              {!wallpaperMode && (
                <p className="mt-4 text-sm text-foreground-400 max-w-md leading-relaxed">
                  愿你今天专注而从容 —— 这里是你的私人导航台。
                </p>
              )}
            </div>
            {showClock && (
              <div className="flex flex-col items-end flex-shrink-0">
                <span className={cn(
                  'font-heading tabular-nums text-foreground-800 tracking-tight',
                  wallpaperMode ? 'text-2xl md:text-3xl' : 'text-3xl md:text-4xl',
                )}>
                  {timeStr}
                </span>
                {/* Wallpaper: no seconds / site-count chrome */}
                {!wallpaperMode && (
                  <>
                    <span className="text-[11px] text-foreground-300 tracking-wide mt-1 font-mono tabular-nums">
                      {secondsStr} SEC
                    </span>
                    <span className="text-[10px] text-foreground-300 tracking-wide mt-3 font-mono tabular-nums">
                      {totalSites} 个站点 · {categories.length} 个分类
                    </span>
                  </>
                )}
              </div>
            )}
          </div>
        </header>
      )}

      <SearchSection
        query={query} onQueryChange={onQueryChange} engine={engine}
        onEngineChange={onEngineChange} onSearch={onSearch} delay={60}
        suggestions={searchSuggestions} showEngineSelector={showEngineSelector}
        wallpaperMode={wallpaperMode}
      />

      <SitesSection
        categories={categories} activeCategory={activeCategory}
        onCategoryChange={onCategoryChange} activeSites={activeSites}
        density={density} onDensityChange={onDensityChange}
        totalSites={totalSites} query={query} onSiteOpen={onSiteOpen} delay={120}
        wallpaperMode={wallpaperMode}
      />

      <FooterActions wallpaperMode={wallpaperMode} />
    </div>
  );
}
