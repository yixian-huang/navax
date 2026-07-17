import { SearchSection, SitesSection, FooterActions } from './SharedSections';
import { cn } from '@/lib/utils';
import type { Density, Site } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';

interface LayoutProps {
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

export default function LayoutBrowseFirst({
  query, onQueryChange, engine, onEngineChange, onSearch, showEngineSelector,
  categories, activeCategory, onCategoryChange,
  activeSites, density, onDensityChange, totalSites, onSiteOpen,
  searchSuggestions,
  wallpaperMode = false,
}: LayoutProps) {
  return (
    <div className={cn(
      'mx-auto max-w-4xl px-6 md:px-8 pb-24',
      wallpaperMode ? 'pt-10 md:pt-12' : 'pt-12 md:pt-16',
    )}>
      {/* Inline compact search at top — z-20 so menus clear the sites section */}
      <div className="relative z-20 mb-8 md:mb-10 rise-in">
        <SearchSection
          query={query} onQueryChange={onQueryChange} engine={engine}
          onEngineChange={onEngineChange} onSearch={onSearch} delay={0}
          size="md"
          suggestions={searchSuggestions} showEngineSelector={showEngineSelector}
          wallpaperMode={wallpaperMode}
        />
      </div>

      {/* Wallpaper: sites section already labels itself via tabs; skip extra prose */}
      {!wallpaperMode && (
        <div className="mb-8 rise-in" style={{ animationDelay: '40ms' }}>
          <h2 className="font-heading text-lg text-foreground-900 tracking-tight mb-1">
            我的收藏
          </h2>
          <p className="text-xs text-foreground-400 mb-4">
            快速访问你最常用的站点
          </p>
        </div>
      )}

      <SitesSection
        categories={categories} activeCategory={activeCategory}
        onCategoryChange={onCategoryChange} activeSites={activeSites}
        density={density} onDensityChange={onDensityChange}
        totalSites={totalSites} query={query} onSiteOpen={onSiteOpen} delay={80}
        wallpaperMode={wallpaperMode}
      />

      <FooterActions wallpaperMode={wallpaperMode} />
    </div>
  );
}
