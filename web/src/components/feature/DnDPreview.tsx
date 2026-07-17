import { GripVertical, Clock, CalendarDays } from 'lucide-react';
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

// ---- SortableSiteCard ----
export function SortableSiteCard({
  site, density, columns,
}: {
  site: Site; density: string; columns?: number;
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

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'flex flex-col items-center rounded-lg bg-background-50 border border-background-200/40 group transition-all duration-150 select-none',
        cfg.padding,
        colClass,
        isDragging && 'z-50 shadow-overlay bg-background-50 ring-2 ring-primary-200/60 scale-105',
      )}
    >
      <div className={cn('rounded-lg bg-background-100 flex items-center justify-center', cfg.icon.container)}>
        <IconRenderer icon={site.icon} className={cn('text-foreground-500', cfg.icon.font)} />
      </div>
      <span className={cn('text-foreground-600 truncate w-full text-center', cfg.text)}>
        {site.title}
      </span>
      <button
        {...attributes}
        {...listeners}
        className={cn(
          'text-foreground-200 opacity-0 group-hover:opacity-100 transition-opacity cursor-grab active:cursor-grabbing touch-none rounded hover:bg-background-100',
          cfg.grip,
        )}
        aria-label={`拖拽 ${site.title}`}
      >
        <GripVertical className="w-3 h-3" />
      </button>
    </div>
  );
}

// ---- SortableCategoryBlock ----
export function SortableCategoryBlock({
  category, density, columns, isOver,
}: {
  category: Category; density: string; columns: number; isOver: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
    data: { type: 'category' },
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'mb-4 rounded-lg transition-all duration-150',
        isDragging && 'z-40',
        isOver && 'ring-2 ring-primary-300 bg-primary-50/30 rounded-lg',
      )}
    >
      <div className="flex items-center gap-2 mb-2 px-1">
        <button
          {...attributes}
          {...listeners}
          className="cursor-grab active:cursor-grabbing text-foreground-300 hover:text-foreground-500 transition-colors duration-150 touch-none flex-shrink-0"
          aria-label={`拖拽 ${category.name}`}
        >
          <GripVertical className="w-4 h-4" />
        </button>
        <IconRenderer icon={category.icon} className="text-primary-500 text-sm" />
        <span className="text-sm font-medium text-foreground-800">{category.name}</span>
        <span className="text-xs text-foreground-400">({category.sites.length})</span>
        {isOver && (
          <span className="text-[10px] text-primary-500 font-medium ml-auto animate-pulse">松开移入此分类</span>
        )}
      </div>

      <SortableContext items={category.sites.map(s => s.id)} strategy={rectSortingStrategy}>
        <div
          className={cn(
            'grid gap-2 rounded-lg p-2 min-h-[40px] transition-colors duration-200',
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
            />
          ))}
          {category.sites.length === 0 && (
            <div className="col-span-full py-4 text-center text-xs text-foreground-300">
              拖拽站点到这里
            </div>
          )}
        </div>
      </SortableContext>
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
