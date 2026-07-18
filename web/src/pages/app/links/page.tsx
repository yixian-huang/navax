// ============================================================
// nav.ax Merged Editor — /app/links
// Left: CRUD data management · Right: live DnD preview
// ============================================================

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import {
  Plus, Edit2, Trash2, ChevronRight, Search, Save,
  Monitor, Tablet, Smartphone,
  PanelLeftClose, PanelLeft, Layout, List, Grid3X3, X, Link2, Loader2, Check,
  Eye, EyeOff,
} from 'lucide-react';
import { navigationApi } from '@/api/navigation';
import {
  DndContext,
  closestCenter,
  pointerWithin,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  arrayMove,
} from '@dnd-kit/sortable';
import { SortableSiteCard, SortableCategoryBlock, WidgetPreview } from '@/components/feature/DnDPreview';
import {
  useMyPage,
  useCreateCategory,
  useUpdateCategory,
  useDeleteCategory,
  useCreateSite,
  useUpdateSite,
  useDeleteSite,
  useSavePageComposition,
} from '@/hooks/useQueries';
import { HOME_LAYOUTS, HOME_LAYOUT_META } from '@/types/layout';
import {
  ConfirmDialog,
  EmptyState,
  ErrorState,
  Badge,
  LoadingSkeleton,
} from '@/components/base/SharedUI';
import { AddCategoryDialog, AddSiteDialog } from '@/components/base/AddDialogs';
import PropertiesPanel, {
  type SiteEditData,
  type CategoryEditData,
} from '@/components/base/PropertiesPanel';
import { useSaveStatus } from '@/hooks/useSaveStatus';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import { draftSaveToastMessage } from '@/lib/publish-state';
import SiteTable, { type FlatSite } from '@/pages/app/links/components/SiteTable';
import BatchLinkChecker from '@/pages/app/links/components/BatchLinkChecker';
import IconRenderer from '@/components/base/IconRenderer';
import type { NavigationPage, Category, Site, Density } from '@/api/types';

/** Resolve which category an over/active id belongs to (category id or site id). */
function findCategoryId(page: NavigationPage, itemId: string | number): string | null {
  const id = String(itemId);
  if (page.categories.some(c => c.id === id)) return id;
  const cat = page.categories.find(c => c.sites.some(s => s.id === id));
  return cat?.id ?? null;
}

type Viewport = 'desktop' | 'tablet' | 'mobile';

const viewportWidths: Record<Viewport, string> = {
  desktop: 'w-full',
  tablet: 'max-w-[768px]',
  mobile: 'max-w-[375px]',
};

// Must match API Density enum: list | compact | comfortable (not "spacious").
const densityLabels: Record<Density, string> = {
  list: '列表',
  compact: '紧凑',
  comfortable: '舒适',
};

// ============================================================
// Main Page — SortablePreview components imported from DnDPreview
// ============================================================

export default function LinksPage() {
  const { data: pageData, isLoading, isError, error, refetch } = useMyPage();
  const createCategory = useCreateCategory();
  const updateCategory = useUpdateCategory();
  const deleteCategory = useDeleteCategory();
  const createSite = useCreateSite();
  const updateSite = useUpdateSite();
  const deleteSite = useDeleteSite();
  const saveComposition = useSavePageComposition();
  const { markSaving, markSaved, markError } = useSaveStatus();
  const { toast } = useToast();

  // UI — focus mode for large catalogs: manage | preview | both (side-by-side)
  const [editorFocus, setEditorFocus] = useState<'manage' | 'preview' | 'both'>(() => {
    if (typeof window !== 'undefined' && window.innerWidth < 1024) return 'manage';
    return 'both';
  });
  const [leftOpen, setLeftOpen] = useState(true);
  const [expandedCat, setExpandedCat] = useState<string | null>(null);
  const [filter, setFilter] = useState('');
  const [viewport, setViewport] = useState<Viewport>('desktop');
  const [viewMode, setViewMode] = useState<'card' | 'table'>('table');
  const [showAddCat, setShowAddCat] = useState(false);
  const [showAddSite, setShowAddSite] = useState(false);
  const [addSiteCatId, setAddSiteCatId] = useState<string>('');
  const [deleteTarget, setDeleteTarget] = useState<{
    type: 'category' | 'site';
    id: string;
    name: string;
  } | null>(null);

  // Batch selection
  const [selectedSiteIds, setSelectedSiteIds] = useState<Set<string>>(new Set());
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchCheckerOpen, setBatchCheckerOpen] = useState(false);

  // Properties panel
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelMode, setPanelMode] = useState<'site' | 'category'>('site');
  const [panelTitle, setPanelTitle] = useState('');
  const [editingItem, setEditingItem] = useState<{
    id: string;
    type: 'site' | 'category';
  } | null>(null);

  // Local layout changes — auto-saved to draft so left/right panels stay in sync.
  const [localPage, setLocalPage] = useState<NavigationPage | null>(null);
  const [hasLayoutChanges, setHasLayoutChanges] = useState(false);
  const [layoutSaveState, setLayoutSaveState] = useState<'idle' | 'dirty' | 'saving' | 'saved' | 'error'>('idle');
  const [overCategoryId, setOverCategoryId] = useState<string | null>(null);

  const page = useMemo(() => localPage || pageData, [localPage, pageData]);
  const homeLayout = page?.settings?.layout.template ?? 'full';
  const pageRef = useRef(page);
  pageRef.current = page;
  const layoutSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const layoutSavingRef = useRef(false);
  const layoutSavePendingRef = useRef(false);
  const layoutDirtyRef = useRef(false);

  const markLayoutDirty = useCallback(() => {
    layoutDirtyRef.current = true;
    setHasLayoutChanges(true);
    setLayoutSaveState('dirty');
  }, []);

  const persistLayout = useCallback(async () => {
    const snapshot = pageRef.current;
    if (!snapshot?.settings) return;
    if (layoutSavingRef.current) {
      layoutSavePendingRef.current = true;
      return;
    }
    layoutSavingRef.current = true;
    setLayoutSaveState('saving');
    markSaving();

    const density = (['list', 'compact', 'comfortable'] as const).includes(snapshot.settings.layout.density as Density)
      ? snapshot.settings.layout.density
      : 'comfortable';
    const columns = Math.min(8, Math.max(1, snapshot.settings.layout.columns || 4));
    const settings = {
      ...snapshot.settings,
      layout: { ...snapshot.settings.layout, density, columns },
    };

    try {
      await saveComposition.mutateAsync({
        categories: snapshot.categories.map(category => ({
          id: category.id,
          siteIds: (category.sites ?? []).map(site => site.id),
        })),
        settings,
      });
      markSaved();
      layoutDirtyRef.current = false;
      setHasLayoutChanges(false);
      setLocalPage(null);
      setLayoutSaveState('saved');
    } catch (cause) {
      markError('保存布局失败');
      setLayoutSaveState('error');
      toast('error', cause instanceof Error ? cause.message : '布局保存失败，请点「立即保存」重试');
    } finally {
      layoutSavingRef.current = false;
      if (layoutSavePendingRef.current) {
        layoutSavePendingRef.current = false;
        void persistLayout();
      }
    }
  }, [saveComposition, markSaving, markSaved, markError, toast]);

  /** Mark dirty and schedule auto-save (debounced). */
  const scheduleLayoutSave = useCallback((delayMs = 450) => {
    markLayoutDirty();
    if (layoutSaveTimerRef.current) clearTimeout(layoutSaveTimerRef.current);
    layoutSaveTimerRef.current = setTimeout(() => {
      layoutSaveTimerRef.current = null;
      void persistLayout();
    }, delayMs);
  }, [markLayoutDirty, persistLayout]);

  const flushLayoutSave = useCallback(() => {
    if (layoutSaveTimerRef.current) {
      clearTimeout(layoutSaveTimerRef.current);
      layoutSaveTimerRef.current = null;
    }
    void persistLayout();
  }, [persistLayout]);

  useEffect(() => () => {
    if (layoutSaveTimerRef.current) clearTimeout(layoutSaveTimerRef.current);
  }, []);

  const setHomeLayout = useCallback((template: (typeof HOME_LAYOUTS)[number]) => {
    if (!page?.settings) return;
    setLocalPage(previous => {
      const current = previous || page;
      return {
        ...current,
        settings: {
          ...page.settings!,
          ...current.settings,
          layout: { ...page.settings!.layout, ...current.settings?.layout, template },
        },
      };
    });
    scheduleLayoutSave(300);
  }, [page, scheduleLayoutSave]);

  // Flat site list for table view (all sites with category info)
  const flatSites = useMemo<FlatSite[]>(() => {
    if (!page?.categories) return [];
    return page.categories.flatMap(cat =>
      cat.sites.map(s => ({
        ...s,
        categoryName: cat.name,
        categoryIcon: cat.icon,
      })),
    );
  }, [page?.categories]);

  // First expand
  useEffect(() => {
    if (page?.categories && page.categories.length > 0 && !expandedCat) {
      setExpandedCat(page.categories[0].id);
    }
  }, [page?.categories, expandedCat]);

  // Sensors
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  // ---- Handlers ----
  const handleCreateCategory = (name: string, icon: string) => {
    markSaving();
    createCategory.mutate(
      { name, icon },
      { onSuccess: () => markSaved(), onError: () => markError('创建分类失败') },
    );
  };

  const handleCreateSite = (data: {
    title: string;
    url: string;
    icon: string;
    description: string;
    categoryId: string;
  }) => {
    markSaving();
    createSite.mutate(
      {
        categoryId: data.categoryId,
        title: data.title,
        url: data.url,
        icon: data.icon,
        description: data.description,
      },
      {
        onSuccess: () => {
          markSaved();
          toast('success', draftSaveToastMessage(page?.publication, `已添加「${data.title}」`));
        },
        onError: (cause) => {
          markError('添加站点失败');
          toast('error', cause instanceof Error ? cause.message : '添加站点失败');
        },
      },
    );
  };

  // Batch adds fire multiple handleCreateSite calls; coalesce toast noise is acceptable.

  const confirmDelete = () => {
    if (!deleteTarget) return;
    markSaving();
    if (deleteTarget.type === 'category') {
      deleteCategory.mutate(deleteTarget.id, {
        onSuccess: () => markSaved(),
        onError: () => markError('删除分类失败'),
      });
    } else {
      deleteSite.mutate(deleteTarget.id, {
        onSuccess: () => markSaved(),
        onError: () => markError('删除站点失败'),
      });
    }
    setDeleteTarget(null);
    if (deleteTarget.type === 'category') setPanelOpen(false);
  };

  const handleSavePanel = (data: SiteEditData | CategoryEditData) => {
    if (!editingItem) return;
    markSaving();
    if (editingItem.type === 'site') {
      const sd = data as SiteEditData;
      updateSite.mutate(
        {
          id: editingItem.id,
          data: { title: sd.title, url: sd.url, icon: sd.icon, description: sd.description },
        },
        {
          onSuccess: () => {
            markSaved();
            setPanelOpen(false);
            toast('success', draftSaveToastMessage(page?.publication));
          },
          onError: () => markError('保存站点失败'),
        },
      );
    } else {
      const cd = data as CategoryEditData;
      updateCategory.mutate(
        { id: editingItem.id, data: { name: cd.name, icon: cd.icon } },
        {
          onSuccess: () => {
            markSaved();
            setPanelOpen(false);
            toast('success', draftSaveToastMessage(page?.publication));
          },
          onError: () => markError('保存分类失败'),
        },
      );
    }
  };

  const handleDeletePanel = () => {
    if (!editingItem) return;
    if (editingItem.type === 'site') {
      const site = findSite(editingItem.id);
      if (site) setDeleteTarget({ type: 'site', id: site.id, name: site.title });
    } else {
      const cat = page?.categories?.find(c => c.id === editingItem.id);
      if (cat) setDeleteTarget({ type: 'category', id: cat.id, name: cat.name });
    }
  };

  const findSite = (id: string): Site | undefined => {
    for (const cat of page?.categories || []) {
      const s = cat.sites.find(s => s.id === id);
      if (s) return s;
    }
    return undefined;
  };

  // Derived: filtered categories (must be defined before callbacks that depend on it)
  const filtered = useMemo(() => {
    const categories = page?.categories || [];
    if (!filter) return categories;
    return categories
      .map(cat => ({
        ...cat,
        sites: cat.sites.filter(
          s =>
            s.title.toLowerCase().includes(filter.toLowerCase()) ||
            s.url.toLowerCase().includes(filter.toLowerCase()) ||
            (s.description && s.description.toLowerCase().includes(filter.toLowerCase())),
        ),
      }))
      .filter(
        cat => cat.name.toLowerCase().includes(filter.toLowerCase()) || cat.sites.length > 0,
      );
  }, [page?.categories, filter]);

  // ---- Batch selection handlers ----
  const handleToggleSelect = useCallback((id: string) => {
    setSelectedSiteIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const handleToggleSelectAllInCategory = useCallback((catId: string) => {
    setSelectedSiteIds(prev => {
      const category = page?.categories?.find(c => c.id === catId);
      if (!category) return prev;
      const siteIds = category.sites.map(s => s.id);
      const allSelected = siteIds.every(id => prev.has(id));
      const next = new Set(prev);
      siteIds.forEach(id => {
        if (allSelected) next.delete(id);
        else next.add(id);
      });
      return next;
    });
  }, [page?.categories]);

  const handleClearSelection = useCallback(() => {
    setSelectedSiteIds(new Set());
  }, []);

  const handleSelectAllVisible = useCallback(() => {
    // Select all sites currently visible (filtered)
    const visibleSites = viewMode === 'table'
      ? flatSites.filter(
          s => !filter ||
            s.title.toLowerCase().includes(filter.toLowerCase()) ||
            s.url.toLowerCase().includes(filter.toLowerCase()) ||
            s.categoryName.toLowerCase().includes(filter.toLowerCase()),
        )
      : filtered.flatMap(cat => cat.sites);
    const allSelected = visibleSites.length > 0 && visibleSites.every(s => selectedSiteIds.has(s.id));
    setSelectedSiteIds(prev => {
      const next = new Set(prev);
      visibleSites.forEach(s => {
        if (allSelected) next.delete(s.id);
        else next.add(s.id);
      });
      return next;
    });
  }, [viewMode, flatSites, filter, filtered, selectedSiteIds]);

  const handleBatchDelete = useCallback(async () => {
    const ids = Array.from(selectedSiteIds);
    if (ids.length === 0) return;
    markSaving();
    const results = await Promise.allSettled(
      ids.map(id => deleteSite.mutateAsync(id)),
    );
    const failed = results.filter(r => r.status === 'rejected').length;
    setSelectedSiteIds(new Set());
    setBatchDeleteOpen(false);
    if (failed > 0) {
      markError(`${failed} 个站点删除失败`);
    } else {
      markSaved();
      toast('success', draftSaveToastMessage(page?.publication));
    }
  }, [selectedSiteIds, deleteSite, markSaving, markSaved, markError, toast, page?.publication]);

  const handleBatchSetEnabled = useCallback(async (enabled: boolean) => {
    const ids = Array.from(selectedSiteIds);
    if (ids.length === 0 || !page) return;
    markSaving();
    try {
      await navigationApi.forPage(page.id).batchSetSitesEnabled({
        siteIds: ids,
        enabled,
        expectedRevision: page.draftRevision ?? 0,
      });
      setSelectedSiteIds(new Set());
      await refetch();
      markSaved();
      toast(
        'success',
        `${enabled ? '已上架' : '已隐藏'} ${ids.length} 个站点 · ${draftSaveToastMessage(page.publication)}`,
      );
    } catch (cause) {
      markError(cause instanceof Error ? cause.message : '批量更新失败');
    }
  }, [selectedSiteIds, page, markSaving, markSaved, markError, toast, refetch]);

  const handleToggleSiteEnabled = useCallback(async (site: Site) => {
    if (!page) return;
    const next = !(site.enabled ?? true);
    markSaving();
    updateSite.mutate(
      { id: site.id, data: { enabled: next } },
      {
        onSuccess: () => {
          markSaved();
          toast(
            'success',
            `${next ? '已上架' : '已隐藏'}「${site.title}」· ${draftSaveToastMessage(page.publication)}`,
          );
        },
        onError: (error: Error) => markError(error.message || '更新失败'),
      },
    );
  }, [page, updateSite, markSaving, markSaved, markError, toast]);

  const handleToggleCategoryEnabled = useCallback(async (cat: Category) => {
    if (!page) return;
    const next = !(cat.enabled ?? true);
    markSaving();
    updateCategory.mutate(
      { id: cat.id, data: { enabled: next } },
      {
        onSuccess: () => {
          markSaved();
          toast(
            'success',
            `${next ? '已显示分类' : '已隐藏分类'}「${cat.name}」· ${draftSaveToastMessage(page.publication)}`,
          );
        },
        onError: (error: Error) => markError(error.message || '更新失败'),
      },
    );
  }, [page, updateCategory, markSaving, markSaved, markError, toast]);

  const siteStats = useMemo(() => {
    const sites = page?.categories.flatMap(c => c.sites) ?? [];
    const total = sites.length;
    const enabled = sites.filter(s => s.enabled !== false).length;
    return { total, enabled, hidden: total - enabled };
  }, [page?.categories]);

  // Derive managed links for batch checker
  const managedLinks = useMemo(() => {
    if (!page?.categories) return [];
    return page.categories.flatMap(cat =>
      cat.sites.map(s => ({ id: s.id, title: s.title, url: s.url, enabled: s.enabled !== false }))
    );
  }, [page?.categories]);

  const getEditData = () => {
    if (!editingItem) return undefined;
    if (editingItem.type === 'site') {
      const site = findSite(editingItem.id);
      if (!site) return undefined;
      return { title: site.title, url: site.url, icon: site.icon, description: site.description } as SiteEditData;
    }
    const cat = page?.categories?.find(c => c.id === editingItem.id);
    if (!cat) return undefined;
    return { name: cat.name, icon: cat.icon } as CategoryEditData;
  };

  // Prefer pointer-within so dropping on another category's sites hits that container.
  const collisionDetection = useCallback<CollisionDetection>((args) => {
    const pointerHits = pointerWithin(args);
    if (pointerHits.length > 0) return pointerHits;
    return closestCenter(args);
  }, []);

  // ---- DnD (multi-container: categories + sites across categories) ----
  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event;
      if (!over || !page) {
        setOverCategoryId(null);
        return;
      }

      // Only sites cross containers; categories reorder on drag end.
      if (active.data.current?.type === 'category') {
        setOverCategoryId(null);
        return;
      }

      const activeCatId =
        (active.data.current?.categoryId as string | undefined)
        ?? findCategoryId(page, active.id);
      const overCatId = findCategoryId(page, over.id);

      if (!activeCatId || !overCatId) {
        setOverCategoryId(null);
        return;
      }

      setOverCategoryId(overCatId);

      // Same category: sortable handles order on drag end.
      if (activeCatId === overCatId) return;

      // Cross-category: move site into target list while dragging (dnd-kit multi-container).
      setLocalPage(prev => {
        const p = prev || page;
        const sourceCat = p.categories.find(c => c.sites.some(s => s.id === active.id));
        const targetCat = p.categories.find(c => c.id === overCatId);
        if (!sourceCat || !targetCat || sourceCat.id === targetCat.id) return p;

        const site = sourceCat.sites.find(s => s.id === active.id);
        if (!site) return p;

        const overIsSiteInTarget = targetCat.sites.some(s => s.id === over.id);
        const overIndex = overIsSiteInTarget
          ? targetCat.sites.findIndex(s => s.id === over.id)
          : targetCat.sites.length;

        const moved: Site = { ...site, categoryId: targetCat.id };
        return {
          ...p,
          categories: p.categories.map(c => {
            if (c.id === sourceCat.id) {
              return { ...c, sites: c.sites.filter(s => s.id !== active.id) };
            }
            if (c.id === targetCat.id) {
              const sites = c.sites.filter(s => s.id !== active.id);
              const next = [...sites];
              next.splice(Math.max(0, overIndex), 0, moved);
              return { ...c, sites: next };
            }
            return c;
          }),
        };
      });
      markLayoutDirty();
    },
    [page, markLayoutDirty],
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      setOverCategoryId(null);
      let changed = layoutDirtyRef.current;

      if (over && page) {
        const activeData = active.data.current;

        // Category reorder
        if (activeData?.type === 'category') {
          const overCatId = findCategoryId(page, over.id);
          if (overCatId && active.id !== overCatId) {
            const catIdx = page.categories.findIndex(c => c.id === active.id);
            const overIdx = page.categories.findIndex(c => c.id === overCatId);
            if (catIdx !== -1 && overIdx !== -1 && catIdx !== overIdx) {
              setLocalPage(prev => {
                const p = prev || page;
                return { ...p, categories: arrayMove(p.categories, catIdx, overIdx) };
              });
              changed = true;
            }
          }
        } else {
          // Site: cross-category already applied in dragOver; finalize same-category reorder.
          const sourceCat = page.categories.find(c => c.sites.some(s => s.id === active.id));
          if (sourceCat) {
            const overCatId = findCategoryId(page, over.id);
            if (overCatId && overCatId !== sourceCat.id) {
              setLocalPage(prev => {
                const p = prev || page;
                const from = p.categories.find(c => c.sites.some(s => s.id === active.id));
                if (!from || from.id === overCatId) return p;
                const site = from.sites.find(s => s.id === active.id);
                if (!site) return p;
                return {
                  ...p,
                  categories: p.categories.map(c => {
                    if (c.id === from.id) return { ...c, sites: c.sites.filter(s => s.id !== active.id) };
                    if (c.id === overCatId) {
                      if (c.sites.some(s => s.id === active.id)) return c;
                      return { ...c, sites: [...c.sites, { ...site, categoryId: c.id }] };
                    }
                    return c;
                  }),
                };
              });
              changed = true;
            } else if (overCatId === sourceCat.id && active.id !== over.id) {
              const oldIndex = sourceCat.sites.findIndex(s => s.id === active.id);
              const newIndex = sourceCat.sites.findIndex(s => s.id === over.id);
              if (oldIndex !== -1 && newIndex !== -1 && oldIndex !== newIndex) {
                setLocalPage(prev => {
                  const p = prev || page;
                  return {
                    ...p,
                    categories: p.categories.map(c => {
                      if (c.id !== sourceCat.id) return c;
                      return { ...c, sites: arrayMove(c.sites, oldIndex, newIndex) };
                    }),
                  };
                });
                changed = true;
              }
            }
          }
        }
      }

      if (changed) {
        markLayoutDirty();
        // Debounce slightly so setLocalPage commits and pageRef updates first.
        if (layoutSaveTimerRef.current) clearTimeout(layoutSaveTimerRef.current);
        layoutSaveTimerRef.current = setTimeout(() => {
          layoutSaveTimerRef.current = null;
          void persistLayout();
        }, 80);
      }
    },
    [page, markLayoutDirty, persistLayout],
  );

  // ---- Layout settings ----
  const setDensity = useCallback(
    (d: string) => {
      if (!page?.settings) return;
      setLocalPage(prev => {
        const base = prev || page;
        return {
          ...base,
          settings: { ...base.settings, layout: { ...base.settings.layout, density: d as Density } },
        };
      });
      scheduleLayoutSave(350);
    },
    [page, scheduleLayoutSave],
  );

  const setColumns = useCallback(
    (c: number) => {
      if (!page?.settings) return;
      setLocalPage(prev => {
        const base = prev || page;
        return {
          ...base,
          settings: { ...base.settings, layout: { ...base.settings.layout, columns: c } },
        };
      });
      scheduleLayoutSave(500);
    },
    [page, scheduleLayoutSave],
  );

  const handleSaveLayout = useCallback(() => {
    flushLayoutSave();
  }, [flushLayoutSave]);

  // ---- Loading ----
  if (isLoading) return <LoadingSkeleton count={4} />;

  if (isError || !page) {
    return (
      <ErrorState
        message={error instanceof Error ? error.message : '加载数据失败'}
        onRetry={() => refetch()}
      />
    );
  }

  const categories = page.categories;

  // Empty state
  if (categories.length === 0) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold font-heading text-foreground-950">导航编辑</h1>
        </div>
        <EmptyState
          iconClass="ri-link-m"
          title="开始构建你的导航"
          description="创建分类并添加你常用的站点"
          action={
            <button
              onClick={() => setShowAddCat(true)}
              className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 inline-flex items-center gap-2 whitespace-nowrap"
            >
              <Plus className="w-4 h-4" />
              创建第一个分类
            </button>
          }
        />
        <AddCategoryDialog
          open={showAddCat}
          onClose={() => setShowAddCat(false)}
          onConfirm={handleCreateCategory}
        />
      </div>
    );
  }

  const showManage = editorFocus === 'manage' || editorFocus === 'both';
  const showPreview = editorFocus === 'preview' || editorFocus === 'both';

  return (
    <div className="-m-4 md:-m-6 flex flex-col h-[calc(100vh-7.5rem)]">
      {/* Focus tabs — critical when managing thousands of links */}
      <div className="flex-shrink-0 border-b border-background-200/70 bg-background-50 px-3 py-2 flex items-center gap-2 flex-wrap">
        <span className="text-[11px] text-foreground-400 mr-1">工作区</span>
        {([
          { id: 'manage' as const, label: '链接管理', hint: '分类/表格，适合大批量' },
          { id: 'preview' as const, label: '实时预览', hint: '拖拽布局' },
          { id: 'both' as const, label: '分栏', hint: '宽屏对照' },
        ]).map(tab => (
          <button
            key={tab.id}
            type="button"
            title={tab.hint}
            onClick={() => {
              setEditorFocus(tab.id);
              if (tab.id === 'manage' || tab.id === 'both') setLeftOpen(true);
            }}
            className={cn(
              'h-8 px-3 rounded-md text-xs font-medium transition-colors',
              editorFocus === tab.id
                ? 'bg-primary-500 text-background-50'
                : 'bg-background-100 text-foreground-500 hover:text-foreground-700',
            )}
          >
            {tab.label}
          </button>
        ))}
        <span className="text-[11px] text-foreground-300 ml-auto hidden sm:inline">
          站点多时建议用「链接管理」+ 表格视图
        </span>
      </div>
      <div className="flex flex-1 min-h-0">
      {/* ---- Left Panel ---- */}
      <div
        className={cn(
          'flex-shrink-0 border-r border-background-200/70 bg-white flex flex-col transition-all duration-200 overflow-hidden',
          !showManage && 'w-0 border-0',
          showManage && editorFocus === 'manage' && 'w-full border-0',
          showManage && editorFocus === 'both' && (leftOpen ? 'w-80 xl:w-[360px]' : 'w-0'),
        )}
      >
        {/* Left Panel Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-background-100">
          <div className="flex items-center gap-2 min-w-0">
            <h2 className="text-sm font-semibold text-foreground-700">链接管理</h2>
            <span className="text-[10px] text-foreground-400 truncate" title="上架数 / 草稿总数（隐藏也占配额）">
              上架 {siteStats.enabled}/{siteStats.total}
              {siteStats.hidden > 0 ? ` · 隐藏 ${siteStats.hidden}` : ''}
            </span>
          </div>
          <div className="flex items-center gap-1">
            {/* View mode toggle */}
            <div className="flex items-center bg-background-100 rounded-md p-0.5">
              <button
                onClick={() => { setViewMode('card'); setSelectedSiteIds(new Set()); }}
                className={cn(
                  'w-6 h-6 flex items-center justify-center rounded transition-colors duration-150',
                  viewMode === 'card' ? 'bg-white text-foreground-700 shadow-sm' : 'text-foreground-400 hover:text-foreground-600',
                )}
                aria-label="卡片视图"
                title="卡片视图"
              >
                <Grid3X3 className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={() => { setViewMode('table'); setSelectedSiteIds(new Set()); }}
                className={cn(
                  'w-6 h-6 flex items-center justify-center rounded transition-colors duration-150',
                  viewMode === 'table' ? 'bg-white text-foreground-700 shadow-sm' : 'text-foreground-400 hover:text-foreground-600',
                )}
                aria-label="表格视图"
                title="表格视图"
              >
                <List className="w-3.5 h-3.5" />
              </button>
            </div>
            <button
              onClick={() => setLeftOpen(false)}
              className="w-7 h-7 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150"
              aria-label="关闭面板"
            >
              <PanelLeftClose className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Search + Actions */}
        <div className="px-4 py-3 border-b border-background-100 space-y-2">
          {selectedSiteIds.size > 0 ? (
            /* Batch Action Bar */
            <div className="flex items-center gap-2 px-2 py-1.5 rounded-lg bg-primary-50 border border-primary-200/60">
              <span className="text-[11px] font-medium text-primary-700 whitespace-nowrap">
                已选 {selectedSiteIds.size} 项
              </span>
              <div className="flex-1" />
              <button
                onClick={handleSelectAllVisible}
                className="text-[10px] text-primary-600 hover:text-primary-700 font-medium whitespace-nowrap"
              >
                全选
              </button>
              <button
                onClick={handleClearSelection}
                className="text-[10px] text-foreground-400 hover:text-foreground-600 whitespace-nowrap"
              >
                取消
              </button>
              <button
                onClick={() => void handleBatchSetEnabled(true)}
                className="h-6 px-2 rounded text-[10px] font-medium bg-white border border-primary-200 text-primary-700 hover:bg-primary-50 transition-colors duration-150 flex items-center gap-1 whitespace-nowrap"
              >
                <Eye className="w-3 h-3" />
                上架
              </button>
              <button
                onClick={() => void handleBatchSetEnabled(false)}
                className="h-6 px-2 rounded text-[10px] font-medium bg-white border border-background-200 text-foreground-600 hover:bg-background-100 transition-colors duration-150 flex items-center gap-1 whitespace-nowrap"
              >
                <EyeOff className="w-3 h-3" />
                隐藏
              </button>
              <button
                onClick={() => setBatchDeleteOpen(true)}
                className="h-6 px-2 rounded text-[10px] font-medium bg-red-500 text-background-50 hover:bg-red-600 transition-colors duration-150 flex items-center gap-1 whitespace-nowrap"
              >
                <Trash2 className="w-3 h-3" />
                删除
              </button>
            </div>
          ) : (
            <>
              <div className="relative">
                <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-foreground-300" />
                <input
                  type="text"
                  value={filter}
                  onChange={e => setFilter(e.target.value)}
                  placeholder="搜索站点..."
                  className="w-full h-8 pl-8 pr-3 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
                />
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setShowAddCat(true)}
                  className="flex-1 h-8 rounded-lg bg-white border border-background-200/70 text-xs text-foreground-600 hover:bg-background-100 transition-colors duration-150 flex items-center justify-center gap-1.5 whitespace-nowrap"
                >
                  <Plus className="w-3.5 h-3.5" />
                  新建分类
                </button>
                <button
                  onClick={() => {
                    setAddSiteCatId(expandedCat || categories[0]?.id || '');
                    setShowAddSite(true);
                  }}
                  className="flex-1 h-8 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center justify-center gap-1.5 whitespace-nowrap"
                >
                  <Plus className="w-3.5 h-3.5" />
                  添加站点
                </button>
              </div>
              <button
                onClick={() => setBatchCheckerOpen(true)}
                className="w-full h-7 rounded-lg bg-background-50 border border-background-200/70 text-[10px] text-foreground-500 hover:bg-background-100 hover:text-foreground-700 transition-colors duration-150 flex items-center justify-center gap-1.5 whitespace-nowrap"
              >
                <Link2 className="w-3 h-3" />
                批量链接检测
              </button>
            </>
          )}
        </div>

        {/* Category List / Table */}
        {viewMode === 'table' ? (
          <SiteTable
            sites={flatSites}
            selectedIds={selectedSiteIds}
            onToggleSelect={handleToggleSelect}
            onToggleSelectAll={handleSelectAllVisible}
            onEdit={(site) => {
              setPanelMode('site');
              setPanelTitle('编辑站点');
              setEditingItem({ id: site.id, type: 'site' });
              setPanelOpen(true);
            }}
            onDelete={(site) =>
              setDeleteTarget({ type: 'site', id: site.id, name: site.title })
            }
            onToggleEnabled={site => void handleToggleSiteEnabled(site)}
          />
        ) : (
          <div className="flex-1 overflow-y-auto">
          {filtered.map(cat => (
            <div key={cat.id} className="border-b border-background-100 last:border-b-0">
              <button
                onClick={() => setExpandedCat(expandedCat === cat.id ? null : cat.id)}
                className="w-full flex items-center gap-2 px-4 py-2.5 hover:bg-background-50 transition-colors duration-150 text-left"
              >
                {/* Category select all checkbox */}
                {cat.sites.length > 0 && (() => {
                  const allInCatSelected = cat.sites.every(s => selectedSiteIds.has(s.id));
                  const someInCatSelected = cat.sites.some(s => selectedSiteIds.has(s.id));
                  return (
                    <button
                      onClick={e => { e.stopPropagation(); handleToggleSelectAllInCategory(cat.id); }}
                      className={cn(
                        'w-4 h-4 rounded border flex items-center justify-center flex-shrink-0 transition-all duration-150',
                        allInCatSelected
                          ? 'bg-primary-500 border-primary-500'
                          : someInCatSelected
                            ? 'border-primary-400 bg-primary-50'
                            : 'border-background-300 hover:border-primary-400',
                      )}
                    >
                      {allInCatSelected ? (
                        <i className="ri-check-line text-[10px] text-background-50" />
                      ) : someInCatSelected ? (
                        <div className="w-2 h-0.5 bg-primary-400 rounded-full" />
                      ) : null}
                    </button>
                  );
                })()}
                <div className="w-6 h-6 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0">
                  <IconRenderer icon={cat.icon} className="text-xs text-primary-500" />
                </div>
                <span className={cn(
                  'flex-1 text-xs font-medium truncate',
                  cat.enabled === false ? 'text-foreground-400' : 'text-foreground-900',
                )}>
                  {cat.name}
                </span>
                {cat.enabled === false && (
                  <span className="text-[10px] text-foreground-400 px-1.5 py-0.5 rounded bg-background-100">已隐藏</span>
                )}
                <Badge>{cat.sites.length}</Badge>
                <ChevronRight
                  className={cn(
                    'w-3.5 h-3.5 text-foreground-300 transition-transform duration-150',
                    expandedCat === cat.id && 'rotate-90',
                  )}
                />
                <button
                  onClick={e => {
                    e.stopPropagation();
                    void handleToggleCategoryEnabled(cat);
                  }}
                  className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
                  aria-label={cat.enabled === false ? `显示分类 ${cat.name}` : `隐藏分类 ${cat.name}`}
                  title={cat.enabled === false ? '显示分类（需发布后生效）' : '隐藏分类（需发布后生效）'}
                >
                  {cat.enabled === false ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                </button>
                <button
                  onClick={e => {
                    e.stopPropagation();
                    setPanelMode('category');
                    setPanelTitle('编辑分类');
                    setEditingItem({ id: cat.id, type: 'category' });
                    setPanelOpen(true);
                  }}
                  className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
                  aria-label={`编辑 ${cat.name}`}
                >
                  <Edit2 className="w-3 h-3" />
                </button>
                <button
                  onClick={e => {
                    e.stopPropagation();
                    setDeleteTarget({ type: 'category', id: cat.id, name: cat.name });
                  }}
                  className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
                  aria-label={`删除 ${cat.name}`}
                >
                  <Trash2 className="w-3 h-3" />
                </button>
              </button>

              {expandedCat === cat.id && (
                <div className="bg-background-50/50">
                  {cat.sites.length === 0 ? (
                    <div className="px-4 py-3 text-center">
                      <p className="text-[11px] text-foreground-400">暂无站点</p>
                    </div>
                  ) : (
                    cat.sites.map(site => {
                      const isSiteSelected = selectedSiteIds.has(site.id);
                      return (
                      <div
                        key={site.id}
                        className={cn(
                          'flex items-center gap-2 px-4 py-2 hover:bg-background-100/50 transition-colors duration-150 group',
                          isSiteSelected && 'bg-primary-50/40',
                        )}
                      >
                        <button
                          onClick={() => handleToggleSelect(site.id)}
                          className={cn(
                            'w-4 h-4 rounded border flex items-center justify-center flex-shrink-0 transition-all duration-150',
                            isSiteSelected
                              ? 'bg-primary-500 border-primary-500'
                              : 'border-background-300 hover:border-primary-400',
                          )}
                        >
                          {isSiteSelected && <i className="ri-check-line text-[10px] text-background-50" />}
                        </button>
                        <div className="w-6 h-6 rounded bg-background-100 flex items-center justify-center flex-shrink-0">
                          <IconRenderer icon={site.icon} className="text-[10px] text-foreground-500" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className={cn(
                            'text-xs font-medium truncate',
                            site.enabled === false ? 'text-foreground-400' : 'text-foreground-800',
                          )}>
                            {site.title}
                            {site.enabled === false && (
                              <span className="ml-1 text-[10px] text-foreground-400">· 已隐藏</span>
                            )}
                          </div>
                          <div className="text-[10px] text-foreground-400 truncate">
                            {site.url.replace(/^https?:\/\//, '').replace(/^www\./, '').split('/')[0]}
                          </div>
                        </div>
                        <button
                          onClick={() => void handleToggleSiteEnabled(site)}
                          className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 opacity-0 group-hover:opacity-100 hover:text-primary-500 hover:bg-primary-50 transition-all duration-150 flex-shrink-0"
                          aria-label={site.enabled === false ? `上架 ${site.title}` : `隐藏 ${site.title}`}
                          title={site.enabled === false ? '上架（需发布后生效）' : '隐藏（需发布后生效）'}
                        >
                          {site.enabled === false ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                        </button>
                        <button
                          onClick={() => {
                            setPanelMode('site');
                            setPanelTitle('编辑站点');
                            setEditingItem({ id: site.id, type: 'site' });
                            setPanelOpen(true);
                          }}
                          className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 opacity-0 group-hover:opacity-100 hover:text-primary-500 hover:bg-primary-50 transition-all duration-150 flex-shrink-0"
                          aria-label={`编辑 ${site.title}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() =>
                            setDeleteTarget({ type: 'site', id: site.id, name: site.title })
                          }
                          className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 opacity-0 group-hover:opacity-100 hover:text-red-500 hover:bg-red-50 transition-all duration-150 flex-shrink-0"
                          aria-label={`删除 ${site.title}`}
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                      );
                    })
                  )}
                  <button
                    onClick={() => {
                      setAddSiteCatId(cat.id);
                      setShowAddSite(true);
                    }}
                    className="w-full px-4 py-2 text-[10px] text-primary-600 hover:text-primary-700 font-medium hover:bg-primary-50/30 transition-colors duration-150 text-left"
                  >
                    + 添加站点到此分类
                  </button>
                </div>
              )}
            </div>
          ))}

          {filtered.length === 0 && filter && (
            <div className="py-8 text-center text-xs text-foreground-400">
              没有匹配「{filter}」的结果
            </div>
          )}
        </div>
        )}

        {/* Layout Settings — changes auto-save to draft; status mirrors preview bar */}
        <div className="border-t border-background-200/70 px-4 py-3 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[11px] font-medium text-foreground-500">布局设置</span>
            <span
              className={cn(
                'inline-flex items-center gap-1 text-[10px] font-medium whitespace-nowrap',
                layoutSaveState === 'saving' && 'text-foreground-500',
                layoutSaveState === 'dirty' && 'text-accent-600',
                layoutSaveState === 'saved' && 'text-primary-600',
                layoutSaveState === 'error' && 'text-red-600',
                layoutSaveState === 'idle' && 'text-foreground-400',
              )}
            >
              {layoutSaveState === 'saving' && <Loader2 className="w-3 h-3 animate-spin" />}
              {layoutSaveState === 'saved' && <Check className="w-3 h-3" />}
              {layoutSaveState === 'dirty' && '待自动保存…'}
              {layoutSaveState === 'saving' && '保存中…'}
              {layoutSaveState === 'saved' && '已写入草稿'}
              {layoutSaveState === 'error' && '保存失败'}
              {layoutSaveState === 'idle' && '拖拽/调整后自动保存'}
            </span>
          </div>
          <p className="text-[10px] text-foreground-400 leading-relaxed -mt-1">
            右侧预览拖拽与下方选项会<strong className="font-medium text-foreground-500">自动保存到草稿</strong>
            ，发布后访客可见。
          </p>

          {/* Homepage Layout Mode */}
          <div className="space-y-1.5">
            <span className="text-[10px] text-foreground-400">导航页布局</span>
            <div className="grid grid-cols-2 gap-1">
              {HOME_LAYOUTS.map(l => {
                const meta = HOME_LAYOUT_META[l];
                const isActive = homeLayout === l;
                return (
                  <button
                    key={l}
                    onClick={() => setHomeLayout(l)}
                    title={meta.description}
                    className={cn(
                      'flex items-center gap-1.5 px-2 py-1.5 rounded-md text-[10px] font-medium transition-all duration-150 whitespace-nowrap cursor-pointer',
                      isActive
                        ? 'bg-primary-100 text-primary-700'
                        : 'text-foreground-400 hover:bg-background-100 hover:text-foreground-600'
                    )}
                  >
                    <i className={cn(meta.icon, isActive ? 'text-primary-500' : 'text-foreground-400', 'text-xs')} />
                    {meta.name}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="space-y-1.5">
            <span className="text-[10px] text-foreground-400">密度</span>
            <div className="flex items-center bg-background-100 rounded-md p-0.5">
              {(['list', 'compact', 'comfortable'] as const).map(d => (
                <button
                  key={d}
                  type="button"
                  onClick={() => setDensity(d)}
                  className={cn(
                    'flex-1 py-1 rounded text-[10px] font-medium transition-colors duration-150 whitespace-nowrap',
                    page.settings.layout.density === d
                      ? 'bg-white text-foreground-900 shadow-sm'
                      : 'text-foreground-400 hover:text-foreground-600',
                  )}
                >
                  {densityLabels[d]}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <span className="text-[10px] text-foreground-400">列数</span>
              <span className="text-[10px] font-mono text-foreground-600">
                {Math.min(8, Math.max(1, page.settings.layout.columns))}
              </span>
            </div>
            <input
              type="range"
              min={1}
              max={8}
              value={Math.min(8, Math.max(1, page.settings.layout.columns || 4))}
              onChange={e => setColumns(Number(e.target.value))}
              className="w-full accent-primary-500 h-1"
            />
          </div>
        </div>
      </div>

      {/* ---- Right Panel: Live Preview ---- */}
      <div className={cn(
        'flex-1 flex flex-col bg-background-50 min-w-0',
        !showPreview && 'hidden',
      )}>
        {/* Preview toolbar */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-background-200/70 bg-white">
          <div className="flex items-center gap-2">
            {!leftOpen && showManage && (
              <button
                onClick={() => setLeftOpen(true)}
                className="w-7 h-7 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150"
                aria-label="打开面板"
              >
                <PanelLeft className="w-4 h-4" />
              </button>
            )}
            <h2 className="text-sm font-semibold text-foreground-700">实时预览</h2>
            {(layoutSaveState === 'dirty' || layoutSaveState === 'saving' || layoutSaveState === 'error' || layoutSaveState === 'saved') && (
              <span
                className={cn(
                  'hidden sm:inline-flex items-center gap-1 h-6 px-2 rounded-full text-[10px] font-medium',
                  layoutSaveState === 'dirty' && 'bg-accent-50 text-accent-700',
                  layoutSaveState === 'saving' && 'bg-background-100 text-foreground-600',
                  layoutSaveState === 'saved' && 'bg-primary-50 text-primary-700',
                  layoutSaveState === 'error' && 'bg-red-50 text-red-600',
                )}
              >
                {layoutSaveState === 'saving' && <Loader2 className="w-3 h-3 animate-spin" />}
                {layoutSaveState === 'saved' && <Check className="w-3 h-3" />}
                {layoutSaveState === 'dirty' && '布局已改 · 即将保存'}
                {layoutSaveState === 'saving' && '正在保存草稿…'}
                {layoutSaveState === 'saved' && '草稿已更新'}
                {layoutSaveState === 'error' && '保存失败'}
              </span>
            )}
          </div>
          <div className="flex items-center gap-1">
            {(layoutSaveState === 'dirty' || layoutSaveState === 'error') && (
              <button
                type="button"
                onClick={handleSaveLayout}
                className="h-7 px-2.5 rounded-md text-[11px] font-medium bg-primary-500 text-background-50 hover:bg-primary-600 inline-flex items-center gap-1 mr-1 whitespace-nowrap"
              >
                <Save className="w-3 h-3" />
                立即保存
              </button>
            )}
            {([
              { key: 'desktop' as Viewport, icon: Monitor, label: '桌面' },
              { key: 'tablet' as Viewport, icon: Tablet, label: '平板' },
              { key: 'mobile' as Viewport, icon: Smartphone, label: '手机' },
            ]).map(v => (
              <button
                key={v.key}
                onClick={() => setViewport(v.key)}
                className={cn(
                  'flex items-center gap-1 h-7 px-2.5 rounded-md text-[11px] font-medium transition-colors duration-150 whitespace-nowrap',
                  viewport === v.key
                    ? 'bg-primary-100 text-primary-700'
                    : 'text-foreground-400 hover:bg-background-100',
                )}
              >
                <v.icon className="w-3 h-3" />
                {v.label}
              </button>
            ))}
          </div>
        </div>

        {/* Sticky save strip when dirty — sits where the user is dragging */}
        {(layoutSaveState === 'dirty' || layoutSaveState === 'saving' || layoutSaveState === 'error') && (
          <div
            className={cn(
              'flex items-center justify-between gap-3 px-4 py-2 border-b text-xs',
              layoutSaveState === 'error'
                ? 'bg-red-50 border-red-100 text-red-700'
                : 'bg-accent-50/80 border-accent-100/80 text-accent-800',
            )}
          >
            <span className="min-w-0">
              {layoutSaveState === 'saving' && '正在把布局写入草稿…'}
              {layoutSaveState === 'dirty' && '预览中的拖拽与布局调整会自动保存到草稿，无需回到左侧。'}
              {layoutSaveState === 'error' && '自动保存失败，可点右侧按钮重试。'}
            </span>
            <button
              type="button"
              onClick={handleSaveLayout}
              disabled={layoutSaveState === 'saving'}
              className={cn(
                'h-7 px-3 rounded-md text-[11px] font-medium inline-flex items-center gap-1 flex-shrink-0 whitespace-nowrap',
                layoutSaveState === 'saving'
                  ? 'bg-background-200 text-foreground-400 cursor-wait'
                  : 'bg-primary-500 text-background-50 hover:bg-primary-600',
              )}
            >
              {layoutSaveState === 'saving' ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <Save className="w-3 h-3" />
              )}
              {layoutSaveState === 'saving' ? '保存中' : '立即保存'}
            </button>
          </div>
        )}

        {/* Preview content */}
        <div className="flex-1 overflow-y-auto p-4 md:p-6">
          <DndContext
            sensors={sensors}
            collisionDetection={collisionDetection}
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
                  {page.title} — 实时预览
                </span>
              </div>

              <div className="p-3 md:p-5">
                {page.settings && (page.settings.display.showClock || page.settings.display.showDate) && (
                  <WidgetPreview
                    showClock={page.settings.display.showClock}
                    showDate={page.settings.display.showDate}
                  />
                )}
                <div className="h-10 bg-background-100 rounded-lg mb-5 flex items-center px-4">
                  <span className="text-xs text-foreground-300">搜索或输入网址...</span>
                </div>

                <SortableContext
                  items={page.categories.map(c => c.id)}
                  strategy={verticalListSortingStrategy}
                >
                  <div>
                    {page.categories.map(cat => (
                      <SortableCategoryBlock
                        key={cat.id}
                        category={cat}
                        density={page.settings.layout.density}
                        columns={page.settings.layout.columns}
                        isOver={overCategoryId === cat.id}
                      />
                    ))}
                  </div>
                </SortableContext>
              </div>
            </div>
          </DndContext>

          <p className="text-[11px] text-foreground-300 mt-3 text-center">
            拖拽分类手柄排序 · 拖站点到其他分类即可移动 · 布局改动会自动保存到草稿
          </p>
        </div>
      </div>

      {/* Dialogs */}
      <AddCategoryDialog
        open={showAddCat}
        onClose={() => setShowAddCat(false)}
        onConfirm={handleCreateCategory}
      />
      <AddSiteDialog
        open={showAddSite}
        onClose={() => setShowAddSite(false)}
        categories={categories}
        defaultCategoryId={addSiteCatId}
        onConfirm={handleCreateSite}
      />
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={confirmDelete}
        title={deleteTarget?.type === 'category' ? '删除分类' : '删除站点'}
        description={
          deleteTarget
            ? `确定要删除「${deleteTarget.name}」吗？${deleteTarget.type === 'category' ? '该分类下的所有站点也将被删除。' : ''}此操作不可撤销。`
            : ''
        }
        confirmLabel="删除"
        danger
      />
      <ConfirmDialog
        open={batchDeleteOpen}
        onClose={() => setBatchDeleteOpen(false)}
        onConfirm={handleBatchDelete}
        title="批量删除站点"
        description={`确定要删除已选择的 ${selectedSiteIds.size} 个站点吗？此操作不可撤销。`}
        confirmLabel="批量删除"
        danger
      />
      <BatchLinkChecker
        open={batchCheckerOpen}
        onClose={() => setBatchCheckerOpen(false)}
        pageId={page.id}
        managedLinks={managedLinks}
      />
      <PropertiesPanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        mode={panelMode}
        title={panelTitle}
        editData={getEditData()}
        onSave={handleSavePanel}
        onDelete={handleDeletePanel}
        deleteLabel={panelMode === 'category' ? '删除分类' : '删除站点'}
      />
      </div>
    </div>
  );
}
