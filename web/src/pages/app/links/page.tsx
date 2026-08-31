// ============================================================
// nav.ax Merged Editor — /app/links
// Left: CRUD data management · Right: live DnD preview
// ============================================================

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import {
  Plus, Trash2, Search, Save,
  Monitor, Tablet, Smartphone,
  PanelLeftClose, PanelLeft, Layout, List, Grid3X3, Link2, Loader2, Check,
  Eye, EyeOff,
} from 'lucide-react';
import { navigationApi } from '@/api/navigation';
import {
  DndContext,
  DragOverlay,
  closestCenter,
  pointerWithin,
  KeyboardSensor,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { SortableCategoryBlock, WidgetPreview } from '@/components/feature/DnDPreview';
import CategoryFolderWall from '@/components/base/CategoryFolderWall';
import {
  useMyPage,
  useThemes,
  useCreateCategory,
  useUpdateCategory,
  useDeleteCategory,
  useCreateSite,
  useUpdateSite,
  useDeleteSite,
  useSavePageComposition,
} from '@/hooks/useQueries';
import {
  ConfirmDialog,
  EmptyState,
  ErrorState,
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
import LayoutSettingsDialog, { layoutSummary } from '@/pages/app/links/components/LayoutSettingsDialog';
import GroupedCategoryList from '@/pages/app/links/components/GroupedCategoryList';
import { EditorPointerSensor } from '@/pages/app/links/dndSensor';
import {
  applyCategoryReorder,
  applySiteDrop,
  overFromEvent,
  parseDndId,
  type DndItemData,
} from '@/pages/app/links/dndIds';
import type { NavigationPage, Category, Site, Density, LayoutTemplate, PageSettings } from '@/api/types';

function siteIdFromActive(id: string | number, data: DndItemData | undefined): string | null {
  if (data?.siteId) return data.siteId;
  const parsed = parseDndId(id);
  if (parsed.kind === 'preview') return parsed.id;
  if (parsed.kind === 'manage-site') return parsed.siteId;
  return null;
}

function categoryIdFromDnd(id: string | number, data: DndItemData | undefined): string | null {
  if (data?.categoryId) return data.categoryId;
  const parsed = parseDndId(id);
  if (parsed.kind === 'manage-cat' || parsed.kind === 'manage-drop') return parsed.categoryId;
  if (parsed.kind === 'preview') return parsed.id;
  return null;
}

type Viewport = 'desktop' | 'tablet' | 'mobile';

const viewportWidths: Record<Viewport, string> = {
  desktop: 'w-full',
  tablet: 'max-w-[768px]',
  mobile: 'max-w-[375px]',
};

// ============================================================
// Main Page — SortablePreview components imported from DnDPreview
// ============================================================

export default function LinksPage() {
  const { data: pageData, isLoading, isError, error, refetch } = useMyPage();
  const themesQuery = useThemes();
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
  const [viewMode, setViewMode] = useState<'card' | 'table'>('card');
  const [showLayoutDialog, setShowLayoutDialog] = useState(false);
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
  const [overlaySite, setOverlaySite] = useState<Site | null>(null);
  const [overlayCategory, setOverlayCategory] = useState<Category | null>(null);
  /** Category focused from the left panel — preview expands + scrolls to it. */
  const [previewFocusCatId, setPreviewFocusCatId] = useState<string | null>(null);
  const previewScrollRef = useRef<HTMLDivElement>(null);

  const page = useMemo(() => localPage || pageData, [localPage, pageData]);
  const themeLayout = themesQuery.data?.find(theme => theme.id === page?.settings?.appearance.themeId)?.layout;
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

  const setHomeLayout = useCallback((template: LayoutTemplate) => {
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
    useSensor(EditorPointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const clearDragChrome = useCallback(() => {
    setOverCategoryId(null);
    setOverlaySite(null);
    setOverlayCategory(null);
  }, []);

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

  const sitePreviewActions = useMemo(() => ({
    onEdit: (site: Site) => {
      setPanelMode('site');
      setPanelTitle('编辑站点');
      setEditingItem({ id: site.id, type: 'site' });
      setPanelOpen(true);
    },
    onDelete: (site: Site) => setDeleteTarget({ type: 'site', id: site.id, name: site.title }),
    onToggleEnabled: (site: Site) => { void handleToggleSiteEnabled(site); },
  }), [handleToggleSiteEnabled]);

  const scrollPreviewToCategory = useCallback((categoryId: string) => {
    setPreviewFocusCatId(categoryId);
    setExpandedCat(categoryId);
    // Expand first, then scroll after layout paints.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const el = document.getElementById(`preview-cat-${categoryId}`);
        if (!el) return;
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    });
  }, []);

  const focusCategoryFromManage = useCallback((categoryId: string) => {
    setExpandedCat(categoryId);
    // Scroll live preview when it is visible (preview-only or split).
    if (editorFocus === 'preview' || editorFocus === 'both') {
      scrollPreviewToCategory(categoryId);
    }
  }, [editorFocus, scrollPreviewToCategory]);

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

  // ---- DnD (one context: manage list + preview) ----
  const visibleCategoryIds = useMemo(
    () => (filter ? new Set(filtered.map(c => c.id)) : undefined),
    [filter, filtered],
  );

  const commitCategories = useCallback((snapshot: NavigationPage, categories: Category[]) => {
    const next = { ...snapshot, categories };
    pageRef.current = next;
    setLocalPage(next);
  }, []);

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const data = event.active.data.current as DndItemData | undefined;
    const current = pageRef.current;
    if (!current || !data) return;
    if (data.type === 'site') {
      const id = siteIdFromActive(event.active.id, data);
      const site = id
        ? current.categories.flatMap(cat => cat.sites).find(item => item.id === id)
        : undefined;
      setOverlaySite(site ?? null);
      setOverlayCategory(null);
      return;
    }
    if (data.type === 'category') {
      setOverlayCategory(current.categories.find(cat => cat.id === data.categoryId) ?? null);
      setOverlaySite(null);
    }
  }, []);

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event;
      if (!over) {
        setOverCategoryId(null);
        return;
      }

      const activeData = active.data.current as DndItemData | undefined;
      if (!activeData || activeData.type !== 'site') {
        setOverCategoryId(null);
        return;
      }

      const snapshot = pageRef.current;
      if (!snapshot) {
        setOverCategoryId(null);
        return;
      }

      const activeSiteId = siteIdFromActive(active.id, activeData);
      const liveCategoryId = activeSiteId
        ? snapshot.categories.find(cat => cat.sites.some(site => site.id === activeSiteId))?.id
        : undefined;
      const activeCategoryId = liveCategoryId ?? activeData.categoryId;
      const dropOver = overFromEvent(over.id, over.data.current as DndItemData | undefined, expandedCat);
      if (!activeSiteId || !activeCategoryId || !dropOver) {
        setOverCategoryId(null);
        return;
      }

      setOverCategoryId(dropOver.categoryId);
      // Same group: sortable order settles on drag end.
      if (activeCategoryId === dropOver.categoryId) return;

      const result = applySiteDrop({
        categories: snapshot.categories,
        activeSiteId,
        activeCategoryId,
        over: dropOver,
        visibleCategoryIds,
      });
      if (!result.changed) return;
      commitCategories(snapshot, result.categories);
      markLayoutDirty();
      if (result.expandCategoryId) setExpandedCat(result.expandCategoryId);
    },
    [expandedCat, visibleCategoryIds, commitCategories, markLayoutDirty],
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      clearDragChrome();
      let changed = layoutDirtyRef.current;
      const snapshot = pageRef.current;

      if (over && snapshot) {
        const activeData = active.data.current as DndItemData | undefined;
        const overData = over.data.current as DndItemData | undefined;

        if (activeData?.type === 'category') {
          const activeCatId = categoryIdFromDnd(active.id, activeData);
          const overCatId = categoryIdFromDnd(over.id, overData);
          if (activeCatId && overCatId) {
            const next = applyCategoryReorder(snapshot.categories, activeCatId, overCatId);
            if (next !== snapshot.categories) {
              commitCategories(snapshot, next);
              changed = true;
            }
          }
        } else if (activeData?.type === 'site') {
          const activeSiteId = siteIdFromActive(active.id, activeData);
          const liveCategoryId = activeSiteId
            ? snapshot.categories.find(cat => cat.sites.some(site => site.id === activeSiteId))?.id
            : undefined;
          const activeCategoryId = liveCategoryId ?? activeData.categoryId;
          const dropOver = overFromEvent(over.id, overData, expandedCat);
          if (activeSiteId && activeCategoryId) {
            const result = applySiteDrop({
              categories: snapshot.categories,
              activeSiteId,
              activeCategoryId,
              over: dropOver,
              visibleCategoryIds,
            });
            if (result.changed) {
              commitCategories(snapshot, result.categories);
              changed = true;
              if (result.expandCategoryId) setExpandedCat(result.expandCategoryId);
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
    [clearDragChrome, commitCategories, expandedCat, visibleCategoryIds, markLayoutDirty, persistLayout],
  );

  // ---- Layout settings ----
  const setDensity = useCallback(
    (d: Density) => {
      if (!page?.settings) return;
      setLocalPage(prev => {
        const base = prev || page;
        return {
          ...base,
          settings: { ...base.settings, layout: { ...base.settings.layout, density: d } },
        };
      });
      scheduleLayoutSave(350);
    },
    [page, scheduleLayoutSave],
  );

  const setCategoryStyle = useCallback(
    (style: PageSettings['layout']['categoryStyle']) => {
      if (!page?.settings) return;
      setLocalPage(prev => {
        const base = prev || page;
        return {
          ...base,
          settings: {
            ...base.settings,
            layout: { ...base.settings.layout, categoryStyle: style },
          },
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
    <div className="-m-4 md:-m-6 flex flex-col h-[calc(100dvh-7.5rem)] min-h-0 overflow-hidden">
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
      <DndContext
        sensors={showLayoutDialog ? [] : sensors}
        collisionDetection={collisionDetection}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
        onDragCancel={clearDragChrome}
      >
      <div className="flex flex-1 min-h-0 overflow-hidden">
      {/* ---- Left Panel ---- */}
      <div
        className={cn(
          'flex-shrink-0 border-r border-background-200/70 bg-white flex flex-col min-h-0 transition-all duration-200 overflow-hidden',
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
            <button
              type="button"
              aria-label="布局设置"
              aria-haspopup="dialog"
              aria-expanded={showLayoutDialog}
              onClick={() => setShowLayoutDialog(true)}
              className="h-7 px-2 rounded-md text-[11px] font-medium text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1 max-w-[9rem]"
              title="布局设置"
            >
              <Layout className="w-3.5 h-3.5 flex-shrink-0" />
              <span className="truncate">{layoutSummary(page.settings.layout)}</span>
            </button>
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
                  placeholder="搜索标题、描述、域名或分类..."
                  className="w-full h-8 pl-8 pr-3 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
                />
              </div>
              <div className="flex items-center gap-1.5">
                <button
                  onClick={() => setShowAddCat(true)}
                  className="flex-1 min-w-0 h-8 rounded-lg bg-white border border-background-200/70 text-[11px] text-foreground-600 hover:bg-background-100 transition-colors duration-150 flex items-center justify-center gap-1 whitespace-nowrap px-1.5"
                >
                  <Plus className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="truncate">新建分类</span>
                </button>
                <button
                  onClick={() => {
                    setAddSiteCatId(expandedCat || categories[0]?.id || '');
                    setShowAddSite(true);
                  }}
                  className="flex-1 min-w-0 h-8 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-[11px] font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center justify-center gap-1 whitespace-nowrap px-1.5"
                >
                  <Plus className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="truncate">添加站点</span>
                </button>
                <button
                  onClick={() => setBatchCheckerOpen(true)}
                  className="flex-1 min-w-0 h-8 rounded-lg bg-background-50 border border-background-200/70 text-[11px] text-foreground-600 hover:bg-background-100 hover:text-foreground-700 transition-colors duration-150 flex items-center justify-center gap-1 whitespace-nowrap px-1.5"
                  title="批量链接检测"
                >
                  <Link2 className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="truncate">链接检测</span>
                </button>
              </div>
            </>
          )}
        </div>

        {/* Category List / Table */}
        <div className="flex-1 min-h-0 overflow-hidden">
        {viewMode === 'table' ? (
          <SiteTable
            sites={flatSites}
            categories={(page?.categories ?? []).map(c => ({ id: c.id, name: c.name }))}
            searchQuery={filter}
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
            onCategorySelect={id => {
              if (id !== 'all') focusCategoryFromManage(id);
            }}
          />
        ) : (
          <div className="h-full overflow-y-auto">
          <GroupedCategoryList
            categories={filtered}
            expandedCat={expandedCat}
            selectedSiteIds={selectedSiteIds}
            overCategoryId={overCategoryId}
            onToggleCategory={id => {
              const next = expandedCat === id ? null : id;
              setExpandedCat(next);
              if (next) focusCategoryFromManage(next);
              else setPreviewFocusCatId(null);
            }}
            onToggleSelectAllInCategory={handleToggleSelectAllInCategory}
            onToggleSelect={handleToggleSelect}
            onToggleCategoryEnabled={cat => void handleToggleCategoryEnabled(cat)}
            onEditCategory={cat => {
              setPanelMode('category');
              setPanelTitle('编辑分类');
              setEditingItem({ id: cat.id, type: 'category' });
              setPanelOpen(true);
            }}
            onDeleteCategory={cat => setDeleteTarget({ type: 'category', id: cat.id, name: cat.name })}
            onAddSite={id => { setAddSiteCatId(id); setShowAddSite(true); }}
            onToggleSiteEnabled={site => void handleToggleSiteEnabled(site)}
            onEditSite={site => {
              setPanelMode('site');
              setPanelTitle('编辑站点');
              setEditingItem({ id: site.id, type: 'site' });
              setPanelOpen(true);
            }}
            onDeleteSite={site => setDeleteTarget({ type: 'site', id: site.id, name: site.title })}
          />

          {filtered.length === 0 && filter && (
            <div className="py-8 text-center text-xs text-foreground-400">
              没有匹配「{filter}」的结果
            </div>
          )}
        </div>
        )}
        </div>
      </div>

      {/* ---- Right Panel: Live Preview ---- */}
      <div className={cn(
        'flex-1 flex flex-col bg-background-50 min-w-0 min-h-0 overflow-hidden',
        !showPreview && 'hidden',
      )}>
        {/* Preview toolbar — save status lives here so no extra banner grows the page */}
        <div className="flex-shrink-0 flex items-center justify-between gap-2 px-4 py-2.5 border-b border-background-200/70 bg-white">
          <div className="flex items-center gap-2 min-w-0">
            {!leftOpen && showManage && (
              <button
                onClick={() => setLeftOpen(true)}
                className="w-7 h-7 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150 flex-shrink-0"
                aria-label="打开面板"
              >
                <PanelLeft className="w-4 h-4" />
              </button>
            )}
            <h2 className="text-sm font-semibold text-foreground-700 flex-shrink-0">实时预览</h2>
            <span
              className={cn(
                'hidden sm:inline-flex items-center gap-1 h-6 px-2 rounded-full text-[10px] font-medium truncate max-w-[14rem]',
                layoutSaveState === 'dirty' && 'bg-accent-50 text-accent-700',
                layoutSaveState === 'saving' && 'bg-background-100 text-foreground-600',
                layoutSaveState === 'saved' && 'bg-primary-50 text-primary-700',
                layoutSaveState === 'error' && 'bg-red-50 text-red-600',
                layoutSaveState === 'idle' && 'bg-background-50 text-foreground-400',
              )}
              title="拖拽与布局改动会自动保存到草稿"
            >
              {layoutSaveState === 'saving' && <Loader2 className="w-3 h-3 animate-spin flex-shrink-0" />}
              {layoutSaveState === 'saved' && <Check className="w-3 h-3 flex-shrink-0" />}
              {layoutSaveState === 'dirty' && '布局已改 · 自动保存中'}
              {layoutSaveState === 'saving' && '正在保存…'}
              {layoutSaveState === 'saved' && '草稿已更新'}
              {layoutSaveState === 'error' && '保存失败'}
              {layoutSaveState === 'idle' && '拖拽可改布局'}
            </span>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
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

        {/* Preview content — only this region scrolls */}
        <div ref={previewScrollRef} className="flex-1 min-h-0 overflow-y-auto overscroll-contain p-4 md:p-6">
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

                {page.settings.layout.categoryStyle === 'folders' ? (
                  <div className="space-y-3">
                    <CategoryFolderWall
                      categories={page.categories.map(cat => ({
                        id: cat.id,
                        name: cat.name,
                        sites: cat.sites.filter(s => s.enabled !== false),
                      }))}
                      onSiteOpen={site => {
                        window.open(site.url, '_blank', 'noopener,noreferrer');
                      }}
                    />
                    <p className="text-[11px] text-foreground-400 text-center">
                      文件夹样式预览（只读）· 站点排序请在左侧列表操作
                    </p>
                  </div>
                ) : (
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
                          defaultCollapsed={cat.sites.length > 24}
                          siteActions={sitePreviewActions}
                          forceExpand={previewFocusCatId === cat.id}
                        />
                      ))}
                    </div>
                  </SortableContext>
                )}
              </div>
            </div>

          <p className="text-[11px] text-foreground-300 mt-3 text-center">
            {page.settings.layout.categoryStyle === 'folders'
              ? '文件夹样式下预览只读 · 左侧编辑分类与站点 · 布局改动会自动保存到草稿'
              : '拖拽分类手柄排序 · 拖站点到其他分类即可移动 · 布局改动会自动保存到草稿'}
          </p>
        </div>
      </div>
      </div>
      <DragOverlay dropAnimation={null}>
        {overlaySite ? (
          <div className="px-3 py-2 rounded-md bg-white shadow-overlay border border-background-200 text-xs font-medium text-foreground-800">
            {overlaySite.title}
          </div>
        ) : overlayCategory ? (
          <div className="px-3 py-2 rounded-md bg-white shadow-overlay border border-background-200 text-xs font-medium">
            {overlayCategory.name}
          </div>
        ) : null}
      </DragOverlay>
      </DndContext>

      {/* Dialogs */}
      <LayoutSettingsDialog
        open={showLayoutDialog}
        onClose={() => setShowLayoutDialog(false)}
        layout={page.settings.layout}
        themeLayout={themeLayout}
        onTemplateChange={setHomeLayout}
        onDensityChange={setDensity}
        onCategoryStyleChange={setCategoryStyle}
        onColumnsChange={setColumns}
      />
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
  );
}
