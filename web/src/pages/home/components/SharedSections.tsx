import { Link } from 'react-router-dom';
import SearchBar from '@/components/base/SearchBar';
import SiteGrid, { SiteCountLabel } from '@/components/base/SiteGrid';
import CategoryTabs from '@/components/base/CategoryTabs';
import DensitySwitcher from '@/components/base/DensitySwitcher';
import type { Density, Site } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';

export function SearchSection({
  query, onQueryChange, engine, onEngineChange, onSearch,
  delay, size = 'lg', suggestions,
}: {
  query: string; onQueryChange: (v: string) => void;
  engine: SearchEngine; onEngineChange: (e: SearchEngine) => void;
  onSearch: (q: string, e: SearchEngine) => void;
  delay: number;
  size?: 'lg' | 'md';
  suggestions?: string[];
}) {
  return (
    <div className="mb-14 md:mb-16 rise-in" style={{ animationDelay: `${delay}ms` }}>
      <SearchBar
        value={query}
        onChange={onQueryChange}
        onSearch={onSearch}
        engine={engine}
        onEngineChange={onEngineChange}
        showEngineSelector
        showHint
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
}: {
  categories: any[]; activeCategory: string;
  onCategoryChange: (id: string) => void;
  activeSites: Site[]; density: Density;
  onDensityChange: (d: Density) => void;
  totalSites: number; query: string;
  onSiteOpen: (s: Site) => void;
  delay: number;
}) {
  return (
    <div className="rise-in" style={{ animationDelay: `${delay}ms` }}>
      <div className="flex items-baseline justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-3">
          <h2 className="font-heading text-lg text-foreground-900 tracking-tight">收藏站点</h2>
          <SiteCountLabel count={activeSites.length} total={totalSites} query={query} />
        </div>
        <DensitySwitcher density={density} onChange={onDensityChange} />
      </div>

      {categories.length > 1 && (
        <div className="mb-8">
          <CategoryTabs
            categories={categories}
            activeId={activeCategory}
            onChange={onCategoryChange}
          />
        </div>
      )}

      <SiteGrid
        sites={activeSites}
        density={density}
        query={query}
        onSiteOpen={onSiteOpen}
        showAddLink={!query}
      />
    </div>
  );
}

export function FooterActions() {
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