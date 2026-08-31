import { describe, expect, it, vi } from 'vitest';
import {
  createCategoryContextActions,
  createSiteContextActions,
} from '@/components/base/ContextMenu';

describe('createSiteContextActions', () => {
  it('inserts hide/show between edit and delete', () => {
    const onToggleEnabled = vi.fn();
    const enabled = createSiteContextActions(
      { id: 's1', title: 'A', url: 'https://a.example', enabled: true },
      { onEdit: vi.fn(), onToggleEnabled, onDelete: vi.fn() },
    );
    expect(enabled.map(a => a.id)).toEqual(['edit', 'toggle-enabled', 'delete']);
    expect(enabled[1].label).toBe('隐藏站点');

    const hidden = createSiteContextActions(
      { id: 's1', title: 'A', url: 'https://a.example', enabled: false },
      { onToggleEnabled },
    );
    expect(hidden[0].label).toBe('上架站点');
  });
});

describe('createCategoryContextActions', () => {
  it('lists edit, add, toggle, delete', () => {
    const actions = createCategoryContextActions(
      { id: 'c1', name: '工具', enabled: true },
      {
        onEdit: vi.fn(),
        onAddSite: vi.fn(),
        onToggleEnabled: vi.fn(),
        onDelete: vi.fn(),
      },
    );
    expect(actions.map(a => a.id)).toEqual(['edit', 'add-site', 'toggle-enabled', 'delete']);
    expect(actions[2].label).toBe('隐藏分类');
    expect(actions[3].destructive).toBe(true);
  });

  it('labels restore when the category is hidden', () => {
    const actions = createCategoryContextActions(
      { id: 'c1', name: '工具', enabled: false },
      { onToggleEnabled: vi.fn() },
    );
    expect(actions[0].id).toBe('toggle-enabled');
    expect(actions[0].label).toBe('显示分类');
  });
});
