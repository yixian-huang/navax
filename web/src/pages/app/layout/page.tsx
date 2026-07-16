// ============================================================
// nav.ax Layout Editor — /app/layout
// Drag-and-drop: categories + cross-category site reordering
// ============================================================

import { useState, useCallback, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { Monitor, Tablet, Smartphone, Save, GripVertical } from 'lucide-react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { SortableSiteCard, SortableCategoryBlock, WidgetPreview } from '@/components/feature/DnDPreview';
import { useMyPage, usePageScope, useSavePageComposition } from '@/hooks/useQueries';
import { LoadingSkeleton, ErrorState, EmptyState } from '@/components/base/SharedUI';
import { useSaveStatus } from '@/hooks/useSaveStatus';
import { cn } from '@/lib/utils';
import type { NavigationPage, Category, Site } from '@/api/types';

type Viewport = 'desktop' | 'tablet' | 'mobile';

const viewportWidths: Record<Viewport, string> = {
  desktop: 'w-full',
  tablet: 'max-w-[768px]',
  mobile: 'max-w-[375px]',
};

const densityLabels: Record<string, string> = {
  compact: '紧凑',
  comfortable: '舒适',
  spacious: '舒展',
};

// Sortable preview components imported from @/components/feature/DnDPreview

// ---- Main Page ----
export default function LayoutPage() {
  const scope = usePageScope();
  const { data: pageData, isLoading, isError, error, refetch } = useMyPage();
  const saveComposition = useSavePageComposition();
  const { markSaving, markSaved, markError } = useSaveStatus();

  const [viewport, setViewport] = useState<Viewport>('desktop');
  const [localPage, setLocalPage] = useState<NavigationPage | null>(null);
  const [hasLocalChanges, setHasLocalChanges] = useState(false);
  const [overCategoryId, setOverCategoryId] = useState<string | null>(null);

  // Sync from query data
  const page = useMemo(() => localPage || pageData, [localPage, pageData]);

  // Sensors
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  // ---- Drag End Handler (supports cross-category) ----
  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      setOverCategoryId(null);
      if (!over || active.id === over.id || !page) return;

      const activeData = active.data.current;
      const isCategory = activeData?.type === 'category';

      if (isCategory) {
        // Category reorder
        const catIdx = page.categories.findIndex(c => c.id === active.id);
        const overIdx = page.categories.findIndex(c => c.id === over.id);
        if (catIdx === -1 || overIdx === -1 || catIdx === overIdx) return;

        setLocalPage(prev => {
          const p = prev || page;
          const cats = [...p.categories];
          const [moved] = cats.splice(catIdx, 1);
          cats.splice(overIdx, 0, moved);
          return { ...p, categories: cats };
        });
        setHasLocalChanges(true);
        return;
      }

      // Site drag — find source and target
      const sourceCat = page.categories.find(c => c.sites.some(s => s.id === active.id));
      if (!sourceCat) return;

      // Check if over is a category (move to end)
      const overCategory = page.categories.find(c => c.id === over.id && c.sites.every(s => s.id !== over.id));
      if (overCategory && overCategory.id !== sourceCat.id) {
        // Cross-category: move site to end of target category
        setLocalPage(prev => {
          const p = prev || page;
          const site = sourceCat.sites.find(s => s.id === active.id);
          if (!site) return p;
          return {
            ...p,
            categories: p.categories.map(c => {
              if (c.id === sourceCat.id) {
                return { ...c, sites: c.sites.filter(s => s.id !== active.id) };
              }
              if (c.id === overCategory.id) {
                return { ...c, sites: [...c.sites, { ...site, categoryId: c.id }] };
              }
              return c;
            }),
          };
        });
        setHasLocalChanges(true);
        return;
      }

      // Same-category site reorder
      const sourceIdx = sourceCat.sites.findIndex(s => s.id === active.id);
      const overIdx = sourceCat.sites.findIndex(s => s.id === over.id);
      if (sourceIdx === -1 || overIdx === -1 || sourceIdx === overIdx) return;

      setLocalPage(prev => {
        const p = prev || page;
        return {
          ...p,
          categories: p.categories.map(c => {
            if (c.id !== sourceCat.id) return c;
            const sites = [...c.sites];
            const [moved] = sites.splice(sourceIdx, 1);
            sites.splice(overIdx, 0, moved);
            return { ...c, sites };
          }),
        };
      });
      setHasLocalChanges(true);
    },
    [page],
  );

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event;
      if (!over || !active.data.current || active.data.current.type !== 'site') {
        setOverCategoryId(null);
        return;
      }
      // Check if over is a category block
      if (over.data.current?.type === 'category') {
        setOverCategoryId(over.id as string);
      } else {
        // Check if over is a site — find its category
        if (!page) return;
        const cat = page.categories.find(c => c.sites.some(s => s.id === over.id));
        setOverCategoryId(cat?.id || null);
      }
    },
    [page],
  );

  // ---- Save ----
  const handleSave = useCallback(() => {
    if (!page) return;
    markSaving();
    saveComposition.mutate({
      categories: page.categories.map(category => ({ id: category.id, siteIds: category.sites.map(site => site.id) })),
      layout: page.layout,
      template: page.settings?.layout.template ?? 'full',
    }, {
      onSuccess: () => {
        markSaved();
        setHasLocalChanges(false);
        setLocalPage(null);
      },
      onError: () => markError('保存布局失败'),
    });
  }, [page, saveComposition, markSaving, markSaved, markError]);

  // ---- Density ----
  const setDensity = useCallback(
    (d: string) => {
      if (!page) return;
      setLocalPage(prev => ({
        ...(prev || page),
        layout: { ...(prev || page).layout, density: d as NavigationPage['layout']['density'] },
      }));
      setHasLocalChanges(true);
    },
    [page],
  );

  // ---- Columns ----
  const setColumns = useCallback(
    (c: number) => {
      if (!page) return;
      setLocalPage(prev => ({
        ...(prev || page),
        layout: { ...(prev || page).layout, columns: c },
      }));
      setHasLocalChanges(true);
    },
    [page],
  );

  // ---- Loading ----
  if (isLoading) return <LoadingSkeleton count={4} />;

  // ---- Error ----
  if (isError || !page) {
    return (
      <ErrorState
        message={error instanceof Error ? error.message : '加载布局数据失败'}
        onRetry={() => refetch()}
      />
    );
  }

  // ---- Empty ----
  if (page.categories.length === 0) {
    return (
      <EmptyState
        title="还没有内容"
        description="先去链接管理页面添加一些分类和站点，再来调整布局"
        action={
          <Link
            to={`/app/links?scope=${scope}`}
            className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 inline-flex items-center gap-2 whitespace-nowrap"
          >
            去链接管理
          </Link>
        }
      />
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
        <div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">布局编排</h1>
          <p className="text-sm text-foreground-400 mt-1">
            拖拽分类和站点调整排版，支持跨分类移动站点
          </p>
        </div>
        <button
          onClick={handleSave}
          disabled={!hasLocalChanges}
          className={cn(
            'h-9 px-4 rounded-lg text-sm font-medium transition-all duration-150 flex items-center gap-2 whitespace-nowrap',
            hasLocalChanges
              ? 'bg-primary-500 text-background-50 dark:text-foreground-950 hover:bg-primary-600'
              : 'border border-background-200/70 text-foreground-400 cursor-not-allowed',
          )}
        >
          <Save className="w-4 h-4" />
          {hasLocalChanges ? '保存布局' : '已保存'}
        </button>
      </div>

      {/* Settings bar */}
      <div className="bg-white rounded-xl border border-background-200/70 p-4 mb-4 space-y-4">
        <div className="flex flex-wrap items-center gap-6">
          {/* Density */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-foreground-500">密度</span>
            <div className="flex items-center bg-background-100 rounded-lg p-0.5">
              {(['compact', 'comfortable', 'spacious'] as const).map(d => (
                <button
                  key={d}
                  onClick={() => setDensity(d)}
                  className={cn(
                    'px-3 py-1.5 rounded-md text-xs font-medium transition-colors duration-150 whitespace-nowrap',
                    page.layout.density === d
                      ? 'bg-white text-foreground-900 shadow-sm'
                      : 'text-foreground-400 hover:text-foreground-600',
                  )}
                >
                  {densityLabels[d]}
                </button>
              ))}
            </div>
          </div>

          {/* Columns */}
          <div className="flex items-center gap-3">
            <span className="text-xs text-foreground-500">
              列数 <span className="font-mono text-foreground-700">{page.layout.columns}</span>
            </span>
            <input
              type="range"
              min={4}
              max={12}
              value={page.layout.columns}
              onChange={e => setColumns(Number(e.target.value))}
              className="w-28 accent-primary-500"
            />
            <div className="hidden sm:flex items-center gap-0.5 text-[10px] text-foreground-300">
              <span>4</span>
              <span className="mx-1">—</span>
              <span>12</span>
            </div>
          </div>
        </div>

        {/* Cross-category hint */}
        <div className="flex items-center gap-2 text-[11px] text-foreground-400 bg-background-50 rounded-lg px-3 py-2">
          <GripVertical className="w-3 h-3 text-primary-400 flex-shrink-0" />
          拖拽站点到分类标题上即可跨分类移动
        </div>
      </div>

      {/* Viewport switcher */}
      <div className="flex items-center gap-1 mb-4">
        {([
          { key: 'desktop' as Viewport, icon: Monitor, label: '桌面' },
          { key: 'tablet' as Viewport, icon: Tablet, label: '平板' },
          { key: 'mobile' as Viewport, icon: Smartphone, label: '手机' },
        ]).map(v => (
          <button
            key={v.key}
            onClick={() => setViewport(v.key)}
            className={cn(
              'flex items-center gap-1.5 h-8 px-3 rounded-lg text-xs font-medium transition-colors duration-150 whitespace-nowrap',
              viewport === v.key
                ? 'bg-primary-100 text-primary-700'
                : 'text-foreground-400 hover:bg-background-100',
            )}
          >
            <v.icon className="w-3.5 h-3.5" />
            {v.label}
          </button>
        ))}
      </div>

      {/* Preview area */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
        onDragOver={handleDragOver}
      >
        <div
          className={cn(
            'mx-auto border border-background-200/70 rounded-xl bg-white overflow-hidden transition-all duration-300',
            viewportWidths[viewport],
          )}
        >
          {/* Browser chrome */}
          <div className="h-10 bg-background-100 border-b border-background-200/70 flex items-center px-3 gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full bg-red-300" />
            <div className="w-2.5 h-2.5 rounded-full bg-accent-300" />
            <div className="w-2.5 h-2.5 rounded-full bg-green-300" />
            <span className="ml-3 text-xs text-foreground-400 truncate">
              {page.title} — 拖拽排序预览
            </span>
          </div>

          <div className="p-3 md:p-5">
            {/* Widget preview */}
            {page.settings && (page.settings.display.showClock || page.settings.display.showDate) && (
              <WidgetPreview showClock={page.settings.display.showClock} showDate={page.settings.display.showDate} />
            )}

            {/* Simulated search */}
            <div className="h-10 bg-background-100 rounded-lg mb-5 flex items-center px-4">
              <span className="text-xs text-foreground-300">搜索或输入网址...</span>
            </div>

            {/* Sortable categories */}
            <SortableContext
              items={page.categories.map(c => c.id)}
              strategy={verticalListSortingStrategy}
            >
              <div>
                {page.categories.map(cat => (
                  <SortableCategoryBlock
                    key={cat.id}
                    category={cat}
                    density={page.layout.density}
                    columns={page.layout.columns}
                    isOver={overCategoryId === cat.id}
                  />
                ))}
              </div>
            </SortableContext>
          </div>
        </div>
      </DndContext>

      {/* Hint */}
      <p className="text-xs text-foreground-300 mt-3 text-center">
        拖拽分类手柄上下排序 · 拖拽站点手柄同分类内排序或拖到分类标题上跨分类移动
      </p>
    </div>
  );
}
