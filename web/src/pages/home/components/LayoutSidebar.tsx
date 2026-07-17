import { Link } from 'react-router-dom';
import SearchBar from '@/components/base/SearchBar';
import SiteCard from '@/components/base/SiteCard';
import SiteGrid from '@/components/base/SiteGrid';
import DensitySwitcher from '@/components/base/DensitySwitcher';
import { cn } from '@/lib/utils';
import IconRenderer from '@/components/base/IconRenderer';
import type { Density, Site, Category } from '@/api/types';
import type { SearchEngine } from '@/components/base/SearchBar';

interface LayoutProps {
  query: string;
  onQueryChange: (v: string) => void;
  engine: SearchEngine;
  onEngineChange: (e: SearchEngine) => void;
  onSearch: (q: string, e: SearchEngine) => void;
  categories: Category[];
  activeCategory: string;
  onCategoryChange: (id: string) => void;
  activeSites: Site[];
  density: Density;
  onDensityChange: (d: Density) => void;
  totalSites: number;
  onSiteOpen: (s: Site) => void;
  searchSuggestions?: string[];
  showEngineSelector?: boolean;
}

export default function LayoutSidebar({
  query, onQueryChange, engine, onEngineChange, onSearch, showEngineSelector,
  categories, activeCategory, onCategoryChange,
  activeSites, density, onDensityChange, totalSites, onSiteOpen,
  searchSuggestions,
}: LayoutProps) {
  return (
    <div className="mx-auto max-w-6xl px-6 md:px-8 pt-12 md:pt-16 pb-24">
      <div className="flex gap-8">
        {/* Left sidebar — categories */}
        <aside className="hidden md:block w-[200px] flex-shrink-0 rise-in">
          <div className="sticky top-24">
            <h3 className="font-heading text-xs font-semibold text-foreground-400 uppercase tracking-[0.15em] mb-4 px-2">
              分类导航
            </h3>
            <nav className="space-y-0.5">
              <button
                onClick={() => onCategoryChange('')}
                className={cn(
                  'w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap text-left cursor-pointer',
                  activeCategory === ''
                    ? 'bg-primary-100 text-primary-600'
                    : 'text-foreground-500 hover:text-foreground-700 hover:bg-background-100',
                )}
              >
                <i className="ri-apps-line text-base" />
                全部
                <span className="ml-auto text-[11px] text-foreground-300">{totalSites}</span>
              </button>
              {categories.map(cat => (
                <button
                  key={cat.id}
                  onClick={() => onCategoryChange(cat.id)}
                  className={cn(
                    'w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap text-left cursor-pointer',
                    activeCategory === cat.id
                      ? 'bg-primary-100 text-primary-600'
                      : 'text-foreground-500 hover:text-foreground-700 hover:bg-background-100',
                  )}
                >
                  <IconRenderer icon={cat.icon} className="text-base" />
                  <span className="truncate">{cat.name}</span>
                  <span className="ml-auto text-[11px] text-foreground-300">{cat.sites.length}</span>
                </button>
              ))}
            </nav>

            <div className="mt-8 pt-6 border-t border-background-200/60">
              <Link
                to="/app/links"
                className="flex items-center gap-2 px-3 py-2 text-xs text-foreground-400 hover:text-primary-500 transition-colors duration-200 rounded-lg hover:bg-background-100 whitespace-nowrap"
              >
                <i className="ri-settings-3-line text-sm" />
                管理站点
              </Link>
            </div>
          </div>
        </aside>

        {/* Right main area */}
        <main className="flex-1 min-w-0">
          {/* Mobile category tabs */}
          <div className="md:hidden mb-6 rise-in">
            <div className="flex items-center gap-2 overflow-x-auto pb-2 scrollbar-none">
              <button
                onClick={() => onCategoryChange('')}
                className={cn(
                  'flex-shrink-0 px-3 py-1.5 rounded-full text-xs font-medium transition-colors duration-200 whitespace-nowrap cursor-pointer',
                  activeCategory === '' ? 'bg-primary-500 text-background-50' : 'bg-background-100 text-foreground-500',
                )}
              >
                全部
              </button>
              {categories.map(cat => (
                <button
                  key={cat.id}
                  onClick={() => onCategoryChange(cat.id)}
                  className={cn(
                    'flex-shrink-0 flex items-center gap-1 px-3 py-1.5 rounded-full text-xs font-medium transition-colors duration-200 whitespace-nowrap cursor-pointer',
                    activeCategory === cat.id ? 'bg-primary-500 text-background-50' : 'bg-background-100 text-foreground-500',
                  )}
                >
                  <IconRenderer icon={cat.icon} className="text-xs" />
                  {cat.name}
                </button>
              ))}
            </div>
          </div>

          {/* Search + density row — z-20 so dropdowns paint above the sites grid */}
          <div className="relative z-20 flex items-center gap-4 mb-6 rise-in" style={{ animationDelay: '40ms' }}>
            <div className="flex-1 min-w-0">
              <SearchBar
                value={query}
                onChange={onQueryChange}
                onSearch={onSearch}
                engine={engine}
                onEngineChange={onEngineChange}
                showEngineSelector={showEngineSelector}
                size="md"
                suggestions={searchSuggestions}
              />
            </div>
            <DensitySwitcher density={density} onChange={onDensityChange} />
          </div>

          {/* Sites grid */}
          <div className="rise-in" style={{ animationDelay: '80ms' }}>
            <div className="flex items-baseline gap-2 mb-4">
              <span className="text-[11px] text-foreground-300 tracking-wide">
                {activeSites.length} 个站点
                {query && <span className="ml-1.5">· {query}</span>}
              </span>
            </div>

            <SiteGrid
              sites={activeSites}
              density={density}
              query={query}
              onSiteOpen={onSiteOpen}
              showAddLink={!query}
              comfortableCols="grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4"
              compactCols="grid-cols-3 sm:grid-cols-4 lg:grid-cols-5 gap-3"
            />
          </div>
        </main>
      </div>
    </div>
  );
}