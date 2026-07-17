// ============================================================
// nav.ax SiteGrid — shared site grid/list renderer
// Used by SharedSections, LayoutSidebar, and links page preview
// ============================================================
import { Link } from 'react-router-dom';
import SiteCard from '@/components/base/SiteCard';
import { EmptyState } from '@/components/base/SharedUI';
import { cn } from '@/lib/utils';
import type { Density, Site } from '@/api/types';

interface SiteGridProps {
  sites: Site[];
  density: Density;
  query?: string;
  onSiteOpen: (site: Site) => void;
  onSiteEdit?: (site: Site) => void;
  onSiteDelete?: (site: Site) => void;
  emptyTitle?: string;
  emptyDescription?: string;
  showAddLink?: boolean;
  addLinkTo?: string;
  /** Column override for comfortable/compact grid */
  comfortableCols?: string;
  compactCols?: string;
  className?: string;
}

export default function SiteGrid({
  sites,
  density,
  query = '',
  onSiteOpen,
  onSiteEdit,
  onSiteDelete,
  emptyTitle,
  emptyDescription,
  showAddLink,
  addLinkTo = '/app/links',
  comfortableCols,
  compactCols,
  className,
}: SiteGridProps) {
  if (sites.length === 0) {
    const isSearch = Boolean(query?.trim());
    return (
      <EmptyState
        variant="quiet"
        iconClass={isSearch ? 'ri-search-line' : 'ri-folder-open-line'}
        title={emptyTitle || (isSearch ? '没有找到相关站点' : '这个分类还是空的')}
        description={
          emptyDescription
          || (isSearch
            ? '换个关键词，或清空搜索看看全部分类'
            : showAddLink
              ? '切换上方分类浏览，或添加一些常用链接'
              : '切换上方分类看看，这里还没有收录站点')
        }
        action={
          !isSearch && showAddLink ? (
            <Link
              to={addLinkTo}
              className={cn(
                'empty-quiet-action inline-flex items-center gap-1.5 h-8 px-3.5 rounded-full text-xs font-medium whitespace-nowrap transition-colors duration-150',
                'border border-background-200/80 bg-background-50/80 text-foreground-600',
                'hover:border-primary-300 hover:text-primary-600 hover:bg-primary-50/60',
              )}
            >
              <i className="ri-add-line text-sm" />
              添加站点
            </Link>
          ) : undefined
        }
      />
    );
  }

  if (density === 'list') {
    // Not material-card: a frosted slab around the whole list fights wallpaper.
    // Plain panel + dividers; wallpaper mode is restyled in index.css.
    return (
      <div className={cn('site-card-list-panel p-1 sm:p-2 divide-y divide-background-200/40', className)}>
        {sites.map(site => (
          <SiteCard
            key={site.id}
            site={site}
            density="list"
            onOpen={onSiteOpen}
            onEdit={onSiteEdit}
            onDelete={onSiteDelete}
            searchQuery={query}
          />
        ))}
      </div>
    );
  }

  return (
    <div
      className={cn(
        'grid',
        density === 'comfortable'
          ? (comfortableCols || 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4')
          : (compactCols || 'grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-3'),
        className,
      )}
    >
      {sites.map(site => (
        <SiteCard
          key={site.id}
          site={site}
          density={density}
          onOpen={onSiteOpen}
          onEdit={onSiteEdit}
          onDelete={onSiteDelete}
          searchQuery={query}
        />
      ))}
    </div>
  );
}

// ---- Site count label ----
export function SiteCountLabel({ count, total, query }: { count: number; total: number; query?: string }) {
  return (
    <span className="text-[11px] text-foreground-300 tracking-wide">
      {count}/{total}
      {query && <span className="ml-1.5">· &ldquo;{query}&rdquo;</span>}
    </span>
  );
}
