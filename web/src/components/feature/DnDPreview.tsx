import { useState } from 'react';
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
    icon: { container: 'w-7 h-7', font: 'text-xs' },
    text: 'text-[11px]',
    padding: 'p-1.5 gap-1',
    grip: 'h-3',
  },
  compact: {
    icon: { container: 'w-7 h-7', font: 'text-xs' },
    text: 'text-[10px]',
    padding: 'p-1 gap-0.5',
    grip: 'h-3',
  },
  comfortable: {
    icon: { container: 'w-9 h-9', font: 'text-sm' },
    text: 'text-xs',
    padding: 'p-2 gap-1',
    grip: 'h-4',
  },
} as const;

export type SitePreviewActions = {
  onEdit?: (site: Site) => void;
  onDelete?: (site: Site) => void;
  onToggleEnabled?: (site: Site) => void;
};

// ---- SortableSiteCard ----
export function SortableSiteCard({
  site, density, columns, actions,
}: {
  site: Site; density: string; columns?: number; actions?: SitePreviewActions;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: site.id,
    data: { type: 'site', categoryId: site.categoryId },
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  const cfg = densityConfig[density as keyof typeof densityConfig] || densityConfig.comfortable;
  const colClass = columns !== undefined && columns <= 6 ? 'col-span-1' : '';
  const isHidden = site.enabled === false;

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'relative flex flex-col items-center rounded-lg border group transition-all duration-150 select-none',
        cfg.padding,
        colClass,
        isDragging && 'z-50 shadow-overlay ring-2 ring-primary-200/60 scale-105',
        isHidden
          ? 'bg-background-50/80 border-dashed border-background-300 opacity-75'
          : 'bg-background-50 border-background-200/40',
      )}
    >
      {isHidden && (
        <span className="absolute -top-1.5 -right-1.5 z-10 inline-flex items-center gap-0.5 px-1 py-0.5 rounded-full bg-background-200 text-foreground-500 text-[9px] font-medium border border-background-300">
          <EyeOff className="w-2.5 h-2.5" />
          隐藏
        </span>
      )}
      <div className={cn(
        'rounded-lg flex items-center justify-center',
        cfg.icon.container,
        isHidden ? 'bg-background-100/80' : 'bg-background-100',
      )}>
        <IconRenderer
          icon={site.icon}
          url={site.url}
          className={cn('text-foreground-500', cfg.icon.font)}
          size={density === 'comfortable' ? 18 : 14}
          alt={site.title}
        />
      </div>
      <a
        href={site.url}
        target="_blank"
        rel="noopener noreferrer"
        className={cn(
          'truncate w-full text-center hover:text-primary-600 hover:underline',
          cfg.text,
          isHidden ? 'text-foreground-400' : 'text-foreground-600',
        )}
        title={`${site.title}\n${site.url}`}
        onClick={e => e.stopPropagation()}
      >
        {site.title}
      </a>
      <div className="flex items-center justify-center gap-0.5 w-full min-h-[1rem]">
        <button
          type="button"
          {...attributes}
          {...listeners}
          className={cn(
            'text-foreground-200 opacity-0 group-hover:opacity-100 transition-opacity cursor-grab active:cursor-grabbing touch-none rounded hover:bg-background-100 p-0.5',
            cfg.grip,
          )}
          aria-label={`拖拽 ${site.title}`}
        >
          <GripVertical className="w-3 h-3" />
        </button>
        <a
          href={site.url}
          target="_blank"
          rel="noopener noreferrer"
          className="opacity-0 group-hover:opacity-100 p-0.5 rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-all"
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
            className={cn(
              'opacity-0 group-hover:opacity-100 p-0.5 rounded transition-all',
              isHidden
                ? 'text-foreground-400 hover:text-emerald-600 hover:bg-emerald-50'
                : 'text-emerald-600/80 hover:text-foreground-500 hover:bg-background-100',
            )}
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
            className="opacity-0 group-hover:opacity-100 p-0.5 rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-all"
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
            className="opacity-0 group-hover:opacity-100 p-0.5 rounded text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-all"
            title="删除"
            aria-label={`删除 ${site.title}`}
          >
            <Trash2 className="w-3 h-3" />
          </button>
        )}
      </div>
    </div>
  );
}

// ---- SortableCategoryBlock ----
export function SortableCategoryBlock({
  category, density, columns, isOver, defaultCollapsed = false, siteActions,
}: {
  category: Category;
  density: string;
  columns: number;
  isOver: boolean;
  /** Start collapsed when a category has many sites (large imports). */
  defaultCollapsed?: boolean;
  siteActions?: SitePreviewActions;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
    data: { type: 'category', categoryId: category.id },
  });
  const [collapsed, setCollapsed] = useState(defaultCollapsed || category.sites.length > 24);

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  const siteIds = category.sites.map(s => s.id);
  const isHidden = category.enabled === false;
  const enabledSites = category.sites.filter(s => s.enabled !== false).length;
  const hiddenSites = category.sites.length - enabledSites;

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'mb-4 rounded-lg transition-all duration-150',
        isDragging && 'z-40',
        isOver && 'ring-2 ring-primary-300 bg-primary-50/30 rounded-lg',
        isHidden && 'opacity-80',
      )}
      data-category-id={category.id}
    >
      <div className="flex items-center gap-1.5 mb-2 px-1 min-w-0">
        <button
          type="button"
          {...attributes}
          {...listeners}
          className="cursor-grab active:cursor-grabbing text-foreground-300 hover:text-foreground-500 transition-colors duration-150 touch-none flex-shrink-0"
          aria-label={`拖拽分类 ${category.name}`}
        >
          <GripVertical className="w-4 h-4" />
        </button>
        <button
          type="button"
          onClick={() => setCollapsed(c => !c)}
          className="flex items-center gap-1.5 min-w-0 flex-1 text-left rounded-md hover:bg-background-50 px-1 py-0.5 -mx-1"
          aria-expanded={!collapsed}
          aria-label={collapsed ? `展开分类 ${category.name}` : `折叠分类 ${category.name}`}
        >
          <ChevronRight
            className={cn(
              'w-3.5 h-3.5 text-foreground-300 flex-shrink-0 transition-transform duration-150',
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
          <span className="text-xs text-foreground-400 flex-shrink-0">
            ({category.sites.length}
            {hiddenSites > 0 ? ` · 隐${hiddenSites}` : ''})
          </span>
          {isHidden && (
            <span className="flex-shrink-0 inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded-full bg-background-100 text-foreground-500 text-[10px] font-medium border border-background-200">
              <EyeOff className="w-3 h-3" />
              分类隐藏
            </span>
          )}
        </button>
        {isOver && (
          <span className="text-[10px] text-primary-500 font-medium ml-auto animate-pulse flex-shrink-0">松开移入此分类</span>
        )}
      </div>

      {!collapsed && (
        <SortableContext items={siteIds} strategy={rectSortingStrategy} id={category.id}>
          <div
            className={cn(
              'grid gap-2 rounded-lg p-2 min-h-[48px] transition-colors duration-200',
              isOver ? 'bg-primary-50/60 border border-dashed border-primary-200' : 'border border-transparent',
              density === 'comfortable' ? 'gap-3' : '',
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
              <div className="col-span-full py-4 text-center text-xs text-foreground-300 border border-dashed border-background-200/60 rounded-md">
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
          className="w-full py-2 text-[11px] text-foreground-400 hover:text-primary-600 border border-dashed border-background-200/70 rounded-md"
        >
          已折叠 {category.sites.length} 个站点（上架 {enabledSites}
          {hiddenSites > 0 ? ` · 隐藏 ${hiddenSites}` : ''}）· 点击展开
        </button>
      )}
    </div>
  );
}

// ---- WidgetPreview ----
export function WidgetPreview({ showClock = true, showDate = true }: { showClock?: boolean; showDate?: boolean }) {
  const now = new Date();
  const timeStr = now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  const dateStr = now.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' });

  return (
    <div className="flex items-center gap-4 mb-5">
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
