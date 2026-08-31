import { describe, expect, it } from 'vitest';
import type { Category, Site } from '@/api/types';
import {
  applyCategoryReorder,
  applySiteDrop,
  manageCatId,
  manageDropId,
  manageSiteId,
  overFromEvent,
  parseDndId,
} from '@/pages/app/links/dndIds';
import { shouldHandlePointerDown } from '@/pages/app/links/dndSensor';
import { layoutSummary } from '@/pages/app/links/components/LayoutSettingsDialog';

function site(partial: Partial<Site> & Pick<Site, 'id' | 'categoryId'>): Site {
  return {
    title: partial.id,
    url: `https://example.com/${partial.id}`,
    icon: 'ri-link',
    description: '',
    sortOrder: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...partial,
  };
}

function cat(id: string, siteIds: string[]): Category {
  return {
    id,
    pageId: 'page_1',
    name: id,
    icon: 'ri-folder-line',
    sortOrder: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    sites: siteIds.map((sid, index) => site({ id: sid, categoryId: id, sortOrder: index })),
  };
}

describe('dndIds', () => {
  it('encodes and decodes manage prefixes', () => {
    expect(manageSiteId('abc')).toBe('manage-site-abc');
    expect(manageCatId('abc')).toBe('manage-cat-abc');
    expect(manageDropId('abc')).toBe('manage-drop-abc');
    expect(parseDndId(manageSiteId('abc'))).toEqual({ kind: 'manage-site', siteId: 'abc' });
    expect(parseDndId(manageCatId('abc'))).toEqual({ kind: 'manage-cat', categoryId: 'abc' });
    expect(parseDndId(manageDropId('abc'))).toEqual({ kind: 'manage-drop', categoryId: 'abc' });
  });

  it('treats unprefixed ids as preview', () => {
    expect(parseDndId('site_1')).toEqual({ kind: 'preview', id: 'site_1' });
  });

  it('returns unknown for empty or incomplete prefixes', () => {
    expect(parseDndId('')).toEqual({ kind: 'unknown', raw: '' });
    expect(parseDndId('manage-site-')).toEqual({ kind: 'unknown', raw: 'manage-site-' });
    expect(parseDndId('manage-cat-')).toEqual({ kind: 'unknown', raw: 'manage-cat-' });
    expect(parseDndId('manage-drop-')).toEqual({ kind: 'unknown', raw: 'manage-drop-' });
  });
});

describe('applySiteDrop', () => {
  const categories = () => [cat('c1', ['s1', 's2']), cat('c2', ['s3'])];

  it('appends to a collapsed drop target and asks to expand it', () => {
    const result = applySiteDrop({
      categories: categories(),
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: { type: 'drop', categoryId: 'c2', collapsed: true },
    });
    expect(result.changed).toBe(true);
    expect(result.expandCategoryId).toBe('c2');
    expect(result.categories[0].sites.map(s => s.id)).toEqual(['s2']);
    expect(result.categories[1].sites.map(s => s.id)).toEqual(['s3', 's1']);
    expect(result.categories[1].sites[1].categoryId).toBe('c2');
  });

  it('inserts before an anchor site in another group', () => {
    const result = applySiteDrop({
      categories: categories(),
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: { type: 'site', categoryId: 'c2', siteId: 's3' },
    });
    expect(result.categories[1].sites.map(s => s.id)).toEqual(['s1', 's3']);
    expect(result.expandCategoryId).toBeNull();
  });

  it('reorders within the same group', () => {
    const result = applySiteDrop({
      categories: categories(),
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: { type: 'site', categoryId: 'c1', siteId: 's2' },
    });
    expect(result.categories[0].sites.map(s => s.id)).toEqual(['s2', 's1']);
    expect(result.changed).toBe(true);
  });

  it('does not change categoryId when dropping on the current group header', () => {
    const input = categories();
    const result = applySiteDrop({
      categories: input,
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: { type: 'drop', categoryId: 'c1', collapsed: false },
    });
    expect(result.changed).toBe(false);
    expect(result.categories).toBe(input);
    expect(result.categories[0].sites[0].categoryId).toBe('c1');
  });

  it('returns the same array for null over or invisible targets', () => {
    const input = categories();
    const missing = applySiteDrop({
      categories: input,
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: null,
    });
    expect(missing.changed).toBe(false);
    expect(missing.categories).toBe(input);

    const hidden = applySiteDrop({
      categories: input,
      activeSiteId: 's1',
      activeCategoryId: 'c1',
      over: { type: 'drop', categoryId: 'c2', collapsed: true },
      visibleCategoryIds: new Set(['c1']),
    });
    expect(hidden.changed).toBe(false);
    expect(hidden.categories).toBe(input);
  });
});

describe('applyCategoryReorder', () => {
  it('moves categories with arrayMove semantics', () => {
    const input = [cat('c1', []), cat('c2', []), cat('c3', [])];
    const next = applyCategoryReorder(input, 'c1', 'c3');
    expect(next.map(c => c.id)).toEqual(['c2', 'c3', 'c1']);
  });

  it('returns the same array when ids match or are missing', () => {
    const input = [cat('c1', []), cat('c2', [])];
    expect(applyCategoryReorder(input, 'c1', 'c1')).toBe(input);
    expect(applyCategoryReorder(input, 'c1', 'missing')).toBe(input);
  });
});

describe('overFromEvent', () => {
  it('reads drop data and treats a different expanded group as collapsed', () => {
    expect(overFromEvent(
      manageDropId('c2'),
      { type: 'drop', surface: 'manage', categoryId: 'c2' },
      'c1',
    )).toEqual({ type: 'drop', categoryId: 'c2', collapsed: true });
  });

  it('returns null when over has no data and is not a manage id', () => {
    expect(overFromEvent('x', undefined, null)).toBeNull();
  });
});

describe('shouldHandlePointerDown', () => {
  it('ignores buttons, links, and data-no-dnd', () => {
    document.body.innerHTML = `
      <div id="row">
        <button id="btn">x</button>
        <a id="link" href="/">y</a>
        <span id="blocked" data-no-dnd="true">z</span>
        <span id="ok">drag</span>
      </div>`;
    expect(shouldHandlePointerDown({ target: document.getElementById('btn') } as unknown as PointerEvent)).toBe(false);
    expect(shouldHandlePointerDown({ target: document.getElementById('link') } as unknown as PointerEvent)).toBe(false);
    expect(shouldHandlePointerDown({ target: document.getElementById('blocked') } as unknown as PointerEvent)).toBe(false);
    expect(shouldHandlePointerDown({ target: document.getElementById('ok') } as unknown as PointerEvent)).toBe(true);
  });
});

describe('layoutSummary', () => {
  it('joins density label and columns', () => {
    expect(layoutSummary({
      template: 'full',
      density: 'comfortable',
      columns: 4,
      categoryStyle: 'tabs',
    })).toBe('舒适 · 4列');
    expect(layoutSummary({
      template: 'full',
      density: 'list',
      columns: 2,
      categoryStyle: 'folders',
    })).toBe('列表 · 2列');
  });
});
