import { arrayMove } from '@dnd-kit/sortable';
import type { Category } from '@/api/types';

const SITE_PREFIX = 'manage-site-';
const CAT_PREFIX = 'manage-cat-';
const DROP_PREFIX = 'manage-drop-';

export type DndSurface = 'manage' | 'preview';
export type DndItemType = 'site' | 'category' | 'drop';

export interface DndItemData {
  type: DndItemType;
  surface: DndSurface;
  siteId?: string;
  categoryId: string;
}

export type ParsedDndId =
  | { kind: 'manage-site'; siteId: string }
  | { kind: 'manage-cat'; categoryId: string }
  | { kind: 'manage-drop'; categoryId: string }
  | { kind: 'preview'; id: string }
  | { kind: 'unknown'; raw: string };

export function manageSiteId(siteId: string): string {
  return SITE_PREFIX + siteId;
}

export function manageCatId(categoryId: string): string {
  return CAT_PREFIX + categoryId;
}

export function manageDropId(categoryId: string): string {
  return DROP_PREFIX + categoryId;
}

export function parseDndId(id: string | number): ParsedDndId {
  const raw = String(id);
  if (!raw) return { kind: 'unknown', raw };
  if (raw.startsWith(SITE_PREFIX)) {
    const siteId = raw.slice(SITE_PREFIX.length);
    return siteId ? { kind: 'manage-site', siteId } : { kind: 'unknown', raw };
  }
  if (raw.startsWith(CAT_PREFIX)) {
    const categoryId = raw.slice(CAT_PREFIX.length);
    return categoryId ? { kind: 'manage-cat', categoryId } : { kind: 'unknown', raw };
  }
  if (raw.startsWith(DROP_PREFIX)) {
    const categoryId = raw.slice(DROP_PREFIX.length);
    return categoryId ? { kind: 'manage-drop', categoryId } : { kind: 'unknown', raw };
  }
  return { kind: 'preview', id: raw };
}

export type ApplySiteDropOver = {
  type: 'drop' | 'site' | 'category';
  categoryId: string;
  siteId?: string;
  collapsed?: boolean;
};

export interface ApplySiteDropInput {
  categories: Category[];
  activeSiteId: string;
  activeCategoryId: string;
  over: ApplySiteDropOver | null;
  visibleCategoryIds?: ReadonlySet<string>;
}

export interface ApplySiteDropResult {
  categories: Category[];
  expandCategoryId: string | null;
  changed: boolean;
}

export function applySiteDrop(input: ApplySiteDropInput): ApplySiteDropResult {
  const { categories, activeSiteId, activeCategoryId, over, visibleCategoryIds } = input;
  const unchanged: ApplySiteDropResult = { categories, expandCategoryId: null, changed: false };
  if (!over) return unchanged;
  if (visibleCategoryIds && !visibleCategoryIds.has(over.categoryId)) return unchanged;

  const sourceCat = categories.find(c => c.id === activeCategoryId);
  const site = sourceCat?.sites.find(s => s.id === activeSiteId);
  if (!sourceCat || !site) return unchanged;

  const sameGroup = activeCategoryId === over.categoryId;
  // Own-group header / category block is not a reorder: spec §6.4.
  if ((over.type === 'drop' || over.type === 'category') && sameGroup) return unchanged;

  if (over.type === 'site' && sameGroup) {
    if (!over.siteId || over.siteId === activeSiteId) return unchanged;
    const oldIndex = sourceCat.sites.findIndex(s => s.id === activeSiteId);
    const newIndex = sourceCat.sites.findIndex(s => s.id === over.siteId);
    if (oldIndex < 0 || newIndex < 0 || oldIndex === newIndex) return unchanged;
    return {
      categories: categories.map(c =>
        c.id === sourceCat.id ? { ...c, sites: arrayMove(c.sites, oldIndex, newIndex) } : c,
      ),
      expandCategoryId: null,
      changed: true,
    };
  }

  const target = categories.find(c => c.id === over.categoryId);
  if (!target) return unchanged;

  let insertIndex = target.sites.length;
  if (over.type === 'site' && over.siteId) {
    const anchor = target.sites.findIndex(s => s.id === over.siteId);
    insertIndex = anchor < 0 ? target.sites.length : anchor;
  }

  const moved = { ...site, categoryId: over.categoryId };
  return {
    categories: categories.map(c => {
      if (c.id === sourceCat.id) return { ...c, sites: c.sites.filter(s => s.id !== activeSiteId) };
      if (c.id === over.categoryId) {
        const next = c.sites.filter(s => s.id !== activeSiteId);
        next.splice(insertIndex, 0, moved);
        return { ...c, sites: next };
      }
      return c;
    }),
    expandCategoryId: over.type === 'drop' && over.collapsed ? over.categoryId : null,
    changed: true,
  };
}

export function applyCategoryReorder(
  categories: Category[],
  activeCategoryId: string,
  overCategoryId: string,
): Category[] {
  if (activeCategoryId === overCategoryId) return categories;
  const from = categories.findIndex(c => c.id === activeCategoryId);
  const to = categories.findIndex(c => c.id === overCategoryId);
  if (from < 0 || to < 0 || from === to) return categories;
  return arrayMove(categories, from, to);
}
