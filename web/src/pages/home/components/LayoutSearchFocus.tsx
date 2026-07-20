import { SearchSection, SitesSection, FooterActions, type CategoryStyle } from './SharedSections';
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
  categoryStyle?: CategoryStyle;
}

export default function LayoutSearchFocus({
  query, onQueryChange, engine, onEngineChange, onSearch, showEngineSelector,
  categories, activeCategory, onCategoryChange,
  activeSites, density, onDensityChange, totalSites, onSiteOpen,
  searchSuggestions,
  wallpaperMode = false,
  categoryStyle = 'tabs',
}: LayoutProps) {
  return (
    <div className={cn(
      'mx-auto max-w-4xl px-6 md:px-8 pb-24',
      wallpaperMode ? 'pt-14 md:pt-20' : 'pt-20 md:pt-28',
    )}>
      {/* Wallpaper: search is self-explanatory — drop instructional headline */}
      {!wallpaperMode && (
        <div className="text-center mb-8 rise-in">
          <h2 className="font-heading text-lg text-foreground-500 mb-10 tracking-wide">
            搜索你的收藏，或直接输入网址
          </h2>
        </div>
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
        categoryStyle={categoryStyle}
      />

      <FooterActions wallpaperMode={wallpaperMode} />
    </div>
  );
}
