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
    return (
      <EmptyState
        iconClass="ri-inbox-line"
        title={emptyTitle || (query ? '没有匹配的站点' : '该分类下暂无站点')}
        description={emptyDescription || (query ? '换个关键词试试' : '去站点管理添加一些收藏吧')}
        action={
          !query && showAddLink ? (
            <Link
              to={addLinkTo}
              className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
            >
              <i className="ri-add-line text-base" />
              添加站点
            </Link>
          ) : undefined
        }
      />
    );
  }

  if (density === 'list') {
    return (
      <div className={cn('material-card p-2 divide-y divide-secondary-100/25', className)}>
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
