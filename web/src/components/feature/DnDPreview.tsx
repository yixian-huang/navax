import { memo, useEffect, useMemo, useState } from 'react';
import {
  GripVertical, Clock, CalendarDays, ChevronRight, ExternalLink, Edit2, Trash2, Eye, EyeOff,
} from 'lucide-react';
import {
  SortableContext,
  useSortable,
  rectSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { cn } from '@/lib/utils';
import IconRenderer from '@/components/base/IconRenderer';
import type { Category, Site } from '@/api/types';

// ---- Density config — matches API enum: list | compact | comfortable ----
const densityConfig = {
  list: {
    icon: { container: 'w-10 h-10', font: 'text-base', px: 22 },
    text: 'text-[11px]',
    padding: 'p-1.5 gap-1',
  },
  compact: {
    icon: { container: 'w-9 h-9', font: 'text-sm', px: 20 },
    text: 'text-[10px]',
    padding: 'p-1.5 gap-1',
  },
  comfortable: {
    icon: { container: 'w-12 h-12', font: 'text-lg', px: 28 },
    text: 'text-xs',
    padding: 'p-2 gap-1.5',
  },
} as const;

export type SitePreviewActions = {
  onEdit?: (site: Site) => void;
  onDelete?: (site: Site) => void;
  onToggleEnabled?: (site: Site) => void;
};

// ---- SortableSiteCard ----
export const SortableSiteCard = memo(function SortableSiteCard({
  site, density, columns, actions,
}: {
  site: Site; density: string; columns?: number; actions?: SitePreviewActions;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: site.id,
    data: { type: 'site', categoryId: site.categoryId },
    animateLayoutChanges: () => false,
  });

  const style = {
    // Translate-only is cheaper than full Transform matrix during drag.
    transform: CSS.Translate.toString(transform),
    transition: isDragging ? undefined : transition,
    opacity: isDragging ? 0.45 : undefined,
    willChange: isDragging ? 'transform' : undefined,
  };

  const cfg = densityConfig[density as keyof typeof densityConfig] || densityConfig.comfortable;
  const colClass = columns !== undefined && columns <= 6 ? 'col-span-1' : '';
  const isHidden = site.enabled === false;

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className={cn(
        'relative flex flex-col items-center rounded-lg border group select-none touch-none',
        'cursor-grab active:cursor-grabbing',
        cfg.padding,
        colClass,
        isDragging && 'z-50 shadow-md ring-2 ring-primary-200/50 bg-background-50',
        isHidden
          ? 'bg-background-50/60 border-background-200/50'
          : 'bg-background-50 border-background-200/40',
        !isDragging && 'transition-shadow duration-100 hover:shadow-sm',
      )}
    >
      <div className={cn(
        'rounded-lg flex items-center justify-center relative',
        cfg.icon.container,
        'bg-background-100',
        isHidden && 'opacity-55',
      )}>
        <IconRenderer
          icon={site.icon}
          url={site.url}
          className={cn('text-foreground-500', cfg.icon.font)}
          size={cfg.icon.px}
          alt={site.title}
        />
        {isHidden && (
          <span
            className="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-background-50 border border-background-200 flex items-center justify-center text-foreground-400"
            title="已隐藏（发布后访客不可见）"
            aria-label="已隐藏"
          >
            <EyeOff className="w-2 h-2" />
          </span>
        )}
      </div>
      <span
        className={cn(
          'truncate w-full text-center pointer-events-none',
          cfg.text,
          isHidden ? 'text-foreground-400' : 'text-foreground-600',
        )}
        title={site.title}
      >
        {site.title}
      </span>
      {/* Action strip: stop drag so clicks work; whole card still drags elsewhere */}
      <div
        className="flex items-center justify-center gap-0.5 w-full min-h-[1rem] opacity-0 group-hover:opacity-100 transition-opacity duration-100"
        onPointerDown={e => e.stopPropagation()}
      >
        <span className="text-foreground-200 p-0.5" aria-hidden>
          <GripVertical className="w-3 h-3" />
        </span>
        <a
          href={site.url}
          target="_blank"
          rel="noopener noreferrer"
          className="p-0.5 rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50"
          title="打开链接"
          aria-label={`打开 ${site.title}`}
          onClick={e => e.stopPropagation()}
        >
          <ExternalLink className="w-3 h-3" />
        </a>
        {actions?.onToggleEnabled && (
          <button
            type="button"
            onClick={e => {
              e.stopPropagation();
              actions.onToggleEnabled?.(site);
            }}
            className="p-0.5 rounded text-foreground-300 hover:text-foreground-600 hover:bg-background-100"
            title={isHidden ? '上架' : '隐藏'}
            aria-label={isHidden ? `上架 ${site.title}` : `隐藏 ${site.title}`}
          >
            {isHidden ? <Eye className="w-3 h-3" /> : <EyeOff className="w-3 h-3" />}
          </button>
        )}
        {actions?.onEdit && (
          <button
            type="button"
            onClick={e => {
              e.stopPropagation();
              actions.onEdit?.(site);
            }}
            className="p-0.5 rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50"
            title="编辑"
            aria-label={`编辑 ${site.title}`}
          >
            <Edit2 className="w-3 h-3" />
          </button>
        )}
        {actions?.onDelete && (
          <button
            type="button"
            onClick={e => {
              e.stopPropagation();
              actions.onDelete?.(site);
            }}
            className="p-0.5 rounded text-foreground-300 hover:text-red-500 hover:bg-red-50"
            title="删除"
            aria-label={`删除 ${site.title}`}
          >
            <Trash2 className="w-3 h-3" />
          </button>
        )}
      </div>
    </div>
  );
});

// ---- SortableCategoryBlock ----
export const SortableCategoryBlock = memo(function SortableCategoryBlock({
  category, density, columns, isOver, defaultCollapsed = false, siteActions, forceExpand = false,
}: {
  category: Category;
  density: string;
  columns: number;
  isOver: boolean;
  /** Start collapsed when a category has many sites (large imports). */
  defaultCollapsed?: boolean;
  siteActions?: SitePreviewActions;
  /** When true (e.g. left panel focused this category), expand for scroll-into-view. */
  forceExpand?: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
    data: { type: 'category', categoryId: category.id },
    animateLayoutChanges: () => false,
  });
  const [collapsed, setCollapsed] = useState(() => defaultCollapsed || category.sites.length > 24);

  useEffect(() => {
    if (forceExpand) setCollapsed(false);
  }, [forceExpand]);

  const style = {
    transform: CSS.Translate.toString(transform),
    transition: isDragging ? undefined : transition,
    opacity: isDragging ? 0.45 : undefined,
    willChange: isDragging ? 'transform' : undefined,
  };

  const siteIds = useMemo(() => category.sites.map(s => s.id), [category.sites]);
  const isHidden = category.enabled === false;
  const hiddenSites = useMemo(
    () => category.sites.reduce((n, s) => n + (s.enabled === false ? 1 : 0), 0),
    [category.sites],
  );
  const enabledSites = category.sites.length - hiddenSites;

  return (
    <div
      ref={setNodeRef}
      id={`preview-cat-${category.id}`}
      style={style}
      className={cn(
        'mb-3 rounded-lg scroll-mt-3',
        isDragging && 'z-40',
        isOver && 'ring-2 ring-primary-300/80 bg-primary-50/20',
        forceExpand && 'ring-1 ring-primary-200/70',
      )}
      data-category-id={category.id}
    >
      <div className="flex items-center gap-1 mb-1.5 px-0.5 min-w-0">
        <button
          type="button"
          {...attributes}
          {...listeners}
          className="cursor-grab active:cursor-grabbing text-foreground-300 hover:text-foreground-500 touch-none flex-shrink-0 p-0.5 rounded hover:bg-background-100"
          aria-label={`拖拽分类 ${category.name}`}
        >
          <GripVertical className="w-4 h-4" />
        </button>
        <button
          type="button"
          onClick={() => setCollapsed(c => !c)}
          className="flex items-center gap-1.5 min-w-0 flex-1 text-left rounded-md hover:bg-background-50 px-1 py-0.5"
          aria-expanded={!collapsed}
          aria-label={collapsed ? `展开分类 ${category.name}` : `折叠分类 ${category.name}`}
        >
          <ChevronRight
            className={cn(
              'w-3.5 h-3.5 text-foreground-300 flex-shrink-0 transition-transform duration-100',
              !collapsed && 'rotate-90',
            )}
          />
          <IconRenderer icon={category.icon} className="text-primary-500 text-sm flex-shrink-0" />
          <span className={cn(
            'text-sm font-medium truncate',
            isHidden ? 'text-foreground-500' : 'text-foreground-800',
          )}>
            {category.name}
          </span>
          <span className="text-[11px] text-foreground-400 flex-shrink-0 tabular-nums">
            {category.sites.length}
            {hiddenSites > 0 ? `/${enabledSites}上架` : ''}
          </span>
          {isHidden && (
            <EyeOff className="w-3 h-3 text-foreground-400 flex-shrink-0" aria-label="分类已隐藏" />
          )}
        </button>
        {isOver && (
          <span className="text-[10px] text-primary-500 font-medium ml-auto flex-shrink-0">移入此处</span>
        )}
      </div>

      {!collapsed && (
        <SortableContext items={siteIds} strategy={rectSortingStrategy} id={category.id}>
          <div
            className={cn(
              'grid gap-2 rounded-lg p-1.5 min-h-[44px]',
              isOver && 'bg-primary-50/40 border border-dashed border-primary-200/80',
            )}
            style={{ gridTemplateColumns: `repeat(${Math.min(columns ?? 4, 8)}, minmax(0, 1fr))` }}
          >
            {category.sites.map(site => (
              <SortableSiteCard
                key={site.id}
                site={site}
                density={density}
                columns={columns}
                actions={siteActions}
              />
            ))}
            {category.sites.length === 0 && (
              <div className="col-span-full py-3 text-center text-xs text-foreground-300 border border-dashed border-background-200/60 rounded-md">
                拖拽站点到这里
              </div>
            )}
          </div>
        </SortableContext>
      )}
      {collapsed && category.sites.length > 0 && (
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="w-full py-1.5 text-[11px] text-foreground-400 hover:text-primary-600 border border-dashed border-background-200/60 rounded-md"
        >
          已折叠 {category.sites.length} 个站点
          {hiddenSites > 0 ? `（${hiddenSites} 隐藏）` : ''}
          {' · 点击展开'}
        </button>
      )}
    </div>
  );
});

// ---- WidgetPreview ----
export function WidgetPreview({ showClock = true, showDate = true }: { showClock?: boolean; showDate?: boolean }) {
  const now = new Date();
  const timeStr = now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  const dateStr = now.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' });

  return (
    <div className="flex items-center gap-4 mb-4">
      {showClock && (
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-background-50 border border-background-200/40">
          <Clock className="w-3.5 h-3.5 text-primary-400" />
          <span className="text-sm font-mono font-medium text-foreground-700">{timeStr}</span>
        </div>
      )}
      {showDate && (
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-background-50 border border-background-200/40">
          <CalendarDays className="w-3.5 h-3.5 text-primary-400" />
          <span className="text-xs text-foreground-600">{dateStr}</span>
        </div>
      )}
    </div>
  );
}
