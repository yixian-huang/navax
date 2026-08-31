# 导航编辑页分组列表 UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/app/links` 以分组列表为默认工作面：布局旋钮进对话框，右键管动作，一套拖拽管顺序（含预览拖进左侧分组）。

**Architecture:** 投放与 ID 分面抽成纯函数并单测。布局对话框和分组列表从 `page.tsx` 拆出，页面继续编排保存。`DndContext` 提到左右外层；预览沿用裸 `site.id`，左侧用 `manage-*` 前缀。不改 API。

**Tech Stack:** React 19 + Vite + `@dnd-kit/core`/`sortable` + TanStack Query + Vitest (jsdom) + Playwright。仓库没有 `@testing-library/react`，组件行为用纯函数单测 + 现有 e2e 覆盖，不新增测试依赖。

**设计依据:** `docs/superpowers/specs/2026-08-31-links-editor-ux-design.md`

## Global Constraints

- 分支 `feat/links-editor-ux`（已存在，spec 已提交），最终单 PR。
- 每个任务提交前该任务测试必须绿。推送前：`make check`、`go test -race ./...`、`make build`；本计划改了前端则另跑 `cd web && npm run test:mock`，改 e2e 则 `make e2e`。
- Conventional Commit 主题行英文；用户可见文案与注释中文。
- 不新增依赖、不改 `api/openapi.yaml`、不改 `savePageComposition`。
- 前端二空格缩进，`@/` → `web/src/`。React / react-router / `useTranslation` 自动导入，不要手写那些 import。
- Vitest 只收录 `web/tests/**/*.test.ts`（不是 `*.tsx`，也不在 `src/` 旁）。
- 不把 `page.tsx` 整页重写；只抽对话框、分组列表、dnd 纯函数。
- 悬停按钮保留。表格不加右键、不当投放区。文件夹预览保持只读。

---

## 文件结构总览

| 文件 | 动作 | 任务 |
|---|---|---|
| `web/src/pages/app/links/dndIds.ts` | 新建：ID 编解码、`applySiteDrop`、`applyCategoryReorder` | 1 |
| `web/tests/links-editor-dnd.test.ts` | 新建：纯函数表驱动测试 | 1 |
| `web/src/pages/app/links/components/LayoutSettingsDialog.tsx` | 新建：布局对话框 + `layoutSummary` | 2 |
| `web/src/pages/app/links/page.tsx` | 默认 `card`、拆底栏、接对话框；随后抽列表、提升 DndContext | 2, 3, 4 |
| `tests/e2e/specs/user.spec.ts` | 「文件夹」改走布局对话框；补布局摘要/右键 | 2, 5 |
| `web/src/components/base/ContextMenu.tsx` | 站点补隐藏/上架；新增分类动作工厂 | 3 |
| `web/tests/context-menu-actions.test.ts` | 新建：动作工厂单测 | 3 |
| `web/src/pages/app/links/components/GroupedCategoryList.tsx` | 新建：左侧分组列表 | 3, 4 |
| `web/src/pages/app/links/dndSensor.ts` | 新建：PointerSensor 过滤交互元素 | 4 |
| `web/src/components/feature/DnDPreview.tsx` | `data.current` 补 `surface: 'preview'` | 4 |

---

### Task 1: ID 分面与投放纯函数

**Files:**
- Create: `web/src/pages/app/links/dndIds.ts`
- Test: `web/tests/links-editor-dnd.test.ts`

**Interfaces:**
- Consumes: `Category` from `@/api/types`；`arrayMove` from `@dnd-kit/sortable`
- Produces:
  - `manageSiteId(siteId: string): string` → `manage-site-{id}`
  - `manageCatId(categoryId: string): string` → `manage-cat-{id}`
  - `manageDropId(categoryId: string): string` → `manage-drop-{id}`
  - `parseDndId(id: string | number): ParsedDndId`
  - `applySiteDrop(input: ApplySiteDropInput): ApplySiteDropResult`
  - `applyCategoryReorder(categories: Category[], activeCategoryId: string, overCategoryId: string): Category[]`
  - types `DndItemData`, `ParsedDndId`, `ApplySiteDropOver`, `ApplySiteDropInput`, `ApplySiteDropResult`

- [ ] **Step 1: Write the failing test**

Create `web/tests/links-editor-dnd.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import type { Category, Site } from '@/api/types';
import {
  applyCategoryReorder,
  applySiteDrop,
  manageCatId,
  manageDropId,
  manageSiteId,
  parseDndId,
} from '@/pages/app/links/dndIds';

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts`

Expected: FAIL, cannot resolve `@/pages/app/links/dndIds`

- [ ] **Step 3: Write minimal implementation**

Create `web/src/pages/app/links/dndIds.ts`:

```ts
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts`

Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/app/links/dndIds.ts web/tests/links-editor-dnd.test.ts
git commit -m "feat: add prefixed dnd ids and site drop helpers"
```

---

### Task 2: 布局对话框 + 分组列表为默认

**Files:**
- Create: `web/src/pages/app/links/components/LayoutSettingsDialog.tsx`
- Modify: `web/src/pages/app/links/page.tsx` (`viewMode` 初始值；删除底栏布局块；工具栏加「布局」按钮；接入对话框)
- Modify: `tests/e2e/specs/user.spec.ts`（「文件夹」按钮现在在对话框里）

**Interfaces:**
- Consumes: `PageSettings['layout']`, `ThemeLayout`, `LayoutTemplate`, `Density` from `@/api/types`; `HOME_LAYOUTS`, `HOME_LAYOUT_META` from `@/types/layout`
- Produces:
  - `layoutSummary(layout: PageSettings['layout']): string` e.g. `舒适 · 4列`
  - `LayoutSettingsDialog` props: `{ open, onClose, layout, themeLayout?, onTemplateChange, onDensityChange, onCategoryStyleChange, onColumnsChange }`

- [ ] **Step 1: Write the failing test**

Add to `web/tests/links-editor-dnd.test.ts`:

```ts
import { layoutSummary } from '@/pages/app/links/components/LayoutSettingsDialog';

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts`

Expected: FAIL, cannot resolve `LayoutSettingsDialog`

- [ ] **Step 3: Implement dialog and wire the page**

Create `web/src/pages/app/links/components/LayoutSettingsDialog.tsx`. Export `DENSITY_LABELS`, `CATEGORY_STYLE_OPTIONS`, `layoutSummary`, and the dialog. Copy the four control groups from the current bottom bar in `page.tsx` (导航页布局 / 密度 / 分类样式 / 列数), including `themeLayout` allowed/locked. Shell matches `AddCategoryDialog`: `fixed inset-0 z-[100]`, overlay `bg-black/30` clicking overlay calls `onClose`, card `bg-background-50 rounded-xl p-6 w-full max-w-md`. Title `布局设置`. Close button `aria-label="关闭"`. Escape: `useEffect` on `open` listening `keydown` for `Escape` → `onClose`. `if (!open) return null`.

```ts
export const DENSITY_LABELS: Record<Density, string> = {
  list: '列表',
  compact: '紧凑',
  comfortable: '舒适',
};

export const CATEGORY_STYLE_OPTIONS = [
  { id: 'tabs' as const, label: '标签' },
  { id: 'sidebar' as const, label: '侧栏' },
  { id: 'grid' as const, label: '网格' },
  { id: 'folders' as const, label: '文件夹' },
];

export function layoutSummary(layout: PageSettings['layout']): string {
  return `${DENSITY_LABELS[layout.density]} · ${layout.columns}列`;
}

export default function LayoutSettingsDialog({
  open, onClose, layout, themeLayout,
  onTemplateChange, onDensityChange, onCategoryStyleChange, onColumnsChange,
}: {
  open: boolean;
  onClose: () => void;
  layout: PageSettings['layout'];
  themeLayout?: ThemeLayout | null;
  onTemplateChange: (template: LayoutTemplate) => void;
  onDensityChange: (density: Density) => void;
  onCategoryStyleChange: (style: PageSettings['layout']['categoryStyle']) => void;
  onColumnsChange: (columns: number) => void;
}) { /* ... */ }
```

In `page.tsx`:

1. `const [viewMode, setViewMode] = useState<'card' | 'table'>('card');`
2. `const [showLayoutDialog, setShowLayoutDialog] = useState(false);`
3. Import `LayoutSettingsDialog, { layoutSummary }`.
4. Delete the entire bottom `{/* Layout Settings */}` block (`border-t ...` through the columns range). The left panel list container must be `flex-1 min-h-0 overflow-hidden` so the list eats remaining height.
5. In the left header, before the view-mode toggle, add:

```tsx
<button
  type="button"
  aria-haspopup="dialog"
  aria-expanded={showLayoutDialog}
  onClick={() => setShowLayoutDialog(true)}
  className="h-7 px-2 rounded-md text-[11px] font-medium text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1 max-w-[9rem]"
  title="布局设置"
>
  <Layout className="w-3.5 h-3.5 flex-shrink-0" />
  <span className="truncate">{layoutSummary(page.settings.layout)}</span>
</button>
```

`Layout` is already imported from `lucide-react`.

6. Mount the dialog next to the other dialogs:

```tsx
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
```

Change `setDensity` signature to `(d: Density)` if it currently takes `string`, so it matches the dialog.

7. Remove now-unused locals from `page.tsx` if the dialog owns them: `densityLabels`, `CATEGORY_STYLES`. Keep `HOME_LAYOUTS` import only if still used on the page (it should not be).

In `tests/e2e/specs/user.spec.ts`, replace the bare 文件夹 click:

```ts
await page.getByRole('button', { name: /舒适 ·|列表 ·|紧凑 ·/ }).click();
await page.getByRole('button', { name: '文件夹', exact: true }).click();
await page.keyboard.press('Escape');
```

The layout button accessible name is the summary text plus the icon; prefer:

```ts
await page.getByRole('button', { name: '布局设置' }).click();
```

because `title="布局设置"` is not the accessible name. Set `aria-label="布局设置"` on the toolbar button (keep the visible summary). E2E then uses `getByRole('button', { name: '布局设置' })`.

- [ ] **Step 4: Run tests**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts`

Expected: PASS, including `layoutSummary`

Run: `cd web && npm run type-check`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/app/links/components/LayoutSettingsDialog.tsx web/src/pages/app/links/page.tsx web/tests/links-editor-dnd.test.ts tests/e2e/specs/user.spec.ts
git commit -m "feat: move editor layout settings into a dialog"
```

---

### Task 3: 右键动作工厂 + 分组列表抽出

**Files:**
- Modify: `web/src/components/base/ContextMenu.tsx`
- Create: `web/tests/context-menu-actions.test.ts`
- Create: `web/src/pages/app/links/components/GroupedCategoryList.tsx`
- Modify: `web/src/pages/app/links/page.tsx` (用组件替换内联分组 JSX)

**Interfaces:**
- Consumes: existing `useContextMenu`, `ContextMenuAction`
- Produces:
  - `createSiteContextActions` 增加可选 `onToggleEnabled?: () => void`；传入时插入「隐藏站点」或「上架站点」（看 `site.enabled === false`），位于编辑与删除之间
  - `createCategoryContextActions(category, { onEdit, onAddSite, onToggleEnabled, onDelete })` 顺序：编辑分类、添加站点、隐藏分类/显示分类、删除分类

- [ ] **Step 1: Write the failing test**

Create `web/tests/context-menu-actions.test.ts`:

```ts
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run tests/context-menu-actions.test.ts`

Expected: FAIL (`onToggleEnabled` unused / `createCategoryContextActions` is not exported)

- [ ] **Step 3: Implement factories and extract the list**

In `ContextMenu.tsx`:

- Import `Eye`, `EyeOff`, `Plus` from lucide (keep existing `ExternalLink`, `Copy`, `Pencil`, `Trash2`).
- Extend `createSiteContextActions` site arg with optional `enabled?: boolean`; callbacks with optional `onToggleEnabled`. After the edit action, if `onToggleEnabled` is set, push `{ id: 'toggle-enabled', label: site.enabled === false ? '上架站点' : '隐藏站点', icon: site.enabled === false ? Eye : EyeOff, onClick: onToggleEnabled }`.
- Add:

```ts
export function createCategoryContextActions(
  category: { id: string; name: string; enabled?: boolean },
  callbacks: {
    onEdit?: () => void;
    onAddSite?: () => void;
    onToggleEnabled?: () => void;
    onDelete?: () => void;
  },
): ContextMenuAction[] {
  const actions: ContextMenuAction[] = [];
  if (callbacks.onEdit) {
    actions.push({ id: 'edit', label: '编辑分类', icon: Pencil, onClick: callbacks.onEdit });
  }
  if (callbacks.onAddSite) {
    actions.push({ id: 'add-site', label: '添加站点', icon: Plus, onClick: callbacks.onAddSite });
  }
  if (callbacks.onToggleEnabled) {
    const hidden = category.enabled === false;
    actions.push({
      id: 'toggle-enabled',
      label: hidden ? '显示分类' : '隐藏分类',
      icon: hidden ? Eye : EyeOff,
      onClick: callbacks.onToggleEnabled,
    });
  }
  if (callbacks.onDelete) {
    actions.push({ id: 'delete', label: '删除分类', icon: Trash2, onClick: callbacks.onDelete, destructive: true });
  }
  return actions;
}
```

Create `GroupedCategoryList.tsx` by moving the `viewMode !== 'table'` branch from `page.tsx` (the `filtered.map(cat => ...)` block). Props:

```ts
export interface GroupedCategoryListProps {
  categories: Category[];
  expandedCat: string | null;
  selectedSiteIds: Set<string>;
  overCategoryId: string | null;
  onToggleCategory: (id: string) => void;
  onToggleSelectAllInCategory: (id: string) => void;
  onToggleSelect: (id: string) => void;
  onToggleCategoryEnabled: (cat: Category) => void;
  onEditCategory: (cat: Category) => void;
  onDeleteCategory: (cat: Category) => void;
  onAddSite: (categoryId: string) => void;
  onToggleSiteEnabled: (site: Site) => void;
  onEditSite: (site: Site) => void;
  onDeleteSite: (site: Site) => void;
}
```

Inside the component call `useContextMenu()` once and render `portal`. Category header `onContextMenu` → `createCategoryContextActions`. Site row `onContextMenu` → `createSiteContextActions` with `onToggleEnabled`. Put `data-no-dnd` on checkboxes, hover action buttons, and the URL `<a>`. This task does **not** attach `useSortable` yet.

Keep hover buttons (eye/edit/trash) exactly as today.

In `page.tsx`, replace the inline grouped list with:

```tsx
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
```

- [ ] **Step 4: Run tests**

Run: `cd web && npx vitest run tests/context-menu-actions.test.ts tests/links-editor-dnd.test.ts`

Expected: PASS

Run: `cd web && npm run type-check`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/base/ContextMenu.tsx web/tests/context-menu-actions.test.ts web/src/pages/app/links/components/GroupedCategoryList.tsx web/src/pages/app/links/page.tsx
git commit -m "feat: add editor context menus and extract grouped list"
```

---

### Task 4: 一套拖拽（提升 DndContext、左侧 sortable、DragOverlay）

**Files:**
- Create: `web/src/pages/app/links/dndSensor.ts`
- Modify: `web/src/pages/app/links/components/GroupedCategoryList.tsx`
- Modify: `web/src/pages/app/links/page.tsx`
- Modify: `web/src/components/feature/DnDPreview.tsx`

**Interfaces:**
- Consumes: Task 1 `parseDndId`, `applySiteDrop`, `applyCategoryReorder`, `manageSiteId`, `manageCatId`, `manageDropId`, `DndItemData`; Task 3 list props
- Produces: `shouldHandlePointerDown(event: PointerEvent): boolean`; `EditorPointerSensor` extends `PointerSensor`

- [ ] **Step 1: Write the failing sensor test**

Add to `web/tests/links-editor-dnd.test.ts`:

```ts
import { shouldHandlePointerDown } from '@/pages/app/links/dndSensor';

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts`

Expected: FAIL, cannot resolve `dndSensor`

- [ ] **Step 3: Sensor, list sortable, lift context, rewrite handlers**

`web/src/pages/app/links/dndSensor.ts`:

```ts
import { PointerSensor } from '@dnd-kit/core';

export function shouldHandlePointerDown(event: PointerEvent): boolean {
  const target = event.target;
  if (!(target instanceof Element)) return true;
  return !target.closest('button, a, input, textarea, [data-no-dnd]');
}

export class EditorPointerSensor extends PointerSensor {
  static activators = [
    {
      eventName: 'onPointerDown' as const,
      handler: ({ nativeEvent }: { nativeEvent: PointerEvent }) => {
        if (!nativeEvent.isPrimary || nativeEvent.button !== 0) return false;
        return shouldHandlePointerDown(nativeEvent);
      },
    },
  ];
}
```

**GroupedCategoryList DnD (only when a parent `DndContext` exists):**

- Import `useDroppable` from `@dnd-kit/core`, `useSortable`, `SortableContext`, `verticalListSortingStrategy` from `@dnd-kit/sortable`, `CSS` from `@dnd-kit/utilities`, `GripVertical` from lucide, and the id helpers.
- Wrap the category list in `<SortableContext items={categories.map(c => manageCatId(c.id))} strategy={verticalListSortingStrategy}>`.
- Per category, a small inner `SortableCategoryHeader` using `useSortable({ id: manageCatId(cat.id), data: { type: 'category', surface: 'manage', categoryId: cat.id }, animateLayoutChanges: () => false })`. Attach `listeners`/`attributes` only on a `GripVertical` handle (`aria-label="拖动分类"`), not on the expand button.
- On the header container, `useDroppable({ id: manageDropId(cat.id), data: { type: 'drop', surface: 'manage', categoryId: cat.id } })`. Merge refs (sortable + droppable) with a callback ref. When `overCategoryId === cat.id`, add the same ring classes as preview: `ring-2 ring-primary-300/80 bg-primary-50/20`.
- Expanded sites: wrap in `SortableContext items={cat.sites.map(s => manageSiteId(s.id))}`. Each row is a memo `SortableManageSite` with `useSortable({ id: manageSiteId(site.id), data: { type: 'site', surface: 'manage', siteId: site.id, categoryId: site.categoryId }, animateLayoutChanges: () => false })`. Translate-only style like `SortableSiteCard`. `data-no-dnd` remains on interactive children.

**DnDPreview.tsx:** add `surface: 'preview'` to both `useSortable` `data` objects (site and category). Do not change ids.

**page.tsx handlers:**

Replace `findCategoryId(page, active.id)` usage in `handleDragOver` / `handleDragEnd` with bare ids from `data.current` (cast `DndItemData`) or `parseDndId`. Never pass a `manage-*` id into `findCategoryId`.

`handleDragOver`:

```ts
const activeData = active.data.current as DndItemData | undefined;
if (activeData?.type === 'category') {
  setOverCategoryId(null);
  return;
}
const activeSiteId = activeData?.siteId ?? (parseDndId(active.id).kind === 'preview' ? String(active.id) : parseDndId(active.id).kind === 'manage-site' ? parseDndId(active.id).siteId : null);
// Prefer a helper:
```

Handlers **require** `data.current` (`DndItemData`). If `!activeData || activeData.type !== 'site'`, `handleDragOver` returns. Add `overFromEvent` to `dndIds.ts` **in this task** (failing tests first):

```ts
export function overFromEvent(
  overId: string | number,
  overData: DndItemData | undefined,
  expandedCat: string | null,
): ApplySiteDropOver | null {
  if (overData?.type === 'drop') {
    return { type: 'drop', categoryId: overData.categoryId, collapsed: expandedCat !== overData.categoryId };
  }
  if (overData?.type === 'site' && overData.siteId) {
    return { type: 'site', categoryId: overData.categoryId, siteId: overData.siteId };
  }
  if (overData?.type === 'category') {
    return { type: 'category', categoryId: overData.categoryId };
  }
  const parsed = parseDndId(overId);
  if (parsed.kind === 'manage-drop') {
    return { type: 'drop', categoryId: parsed.categoryId, collapsed: expandedCat !== parsed.categoryId };
  }
  if (parsed.kind === 'manage-cat') {
    return { type: 'category', categoryId: parsed.categoryId };
  }
  return null;
}
```

Tests: `overFromEvent(manageDropId('c2'), { type: 'drop', surface: 'manage', categoryId: 'c2' }, 'c1')` → `{ type: 'drop', categoryId: 'c2', collapsed: true }`; `overFromEvent('x', undefined, null)` → `null`.

Visible ids during search: `const visibleCategoryIds = new Set(filtered.map(c => c.id))`. Pass to `applySiteDrop`. If `filter` is empty, omit `visibleCategoryIds`.

On `result.changed`, `setLocalPage` with new categories, `markLayoutDirty()`. If `result.expandCategoryId`, `setExpandedCat(result.expandCategoryId)`.

`handleDragEnd` for categories: resolve both ids via `data.current.categoryId` or `parseDndId` (`manage-cat` / `manage-drop` / preview). `applyCategoryReorder` then `setLocalPage`. Sites: if `applySiteDrop` already ran in over for cross-group, still run it on end for same-group reorder (function is idempotent enough; if unchanged, `changed: false` skips). Then existing persist debounce.

**Lift DndContext** so it wraps the `flex flex-1` row that contains left + right, not only the preview scroller. Remove the inner `DndContext` in the preview pane; keep `SortableContext` there.

Sensors:

```ts
const sensors = useSensors(
  useSensor(EditorPointerSensor, { activationConstraint: { distance: 6 } }),
  useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
);
```

When `showLayoutDialog`, pass `sensors={[]}` to `DndContext` so a new drag cannot start.

`DragOverlay` as sibling inside `DndContext`, `dropAnimation={null}`:

```tsx
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
```

Track overlay entities in `onDragStart` from `data.current` + `pageRef`. Clear in `onDragEnd` / `onDragCancel`.

Table mode: `GroupedCategoryList` unmounted → no manage sortables. Preview drag unchanged.

Folders: preview already skips `SortableContext`; do not change that. Left list still sortable.

Collision detection stays `pointerWithin` then `closestCenter`.

- [ ] **Step 4: Run tests**

Run: `cd web && npx vitest run tests/links-editor-dnd.test.ts tests/context-menu-actions.test.ts`

Expected: PASS

Run: `cd web && npm run type-check && npm run lint`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/app/links/dndSensor.ts web/src/pages/app/links/dndIds.ts web/src/pages/app/links/components/GroupedCategoryList.tsx web/src/pages/app/links/page.tsx web/src/components/feature/DnDPreview.tsx web/tests/links-editor-dnd.test.ts
git commit -m "feat: share editor dnd across manage list and preview"
```

---

### Task 5: E2E 与回归

**Files:**
- Modify: `tests/e2e/specs/user.spec.ts`

**Interfaces:**
- Consumes: Task 2 layout dialog (`aria-label="布局设置"`), Task 3 context menus (labels 编辑站点 / 删除站点 / 编辑分类)
- Produces: no new API

仓库没有 RTL，本任务用 Playwright 覆盖 spec §8.2/§8.3。不测 dnd-kit 指针拖拽（不稳定，spec 允许改测对话框 + 右键）。

- [ ] **Step 1: Update “创建分类与站点” if needed**

After creating `我的书签`, click that category row so it expands (card view only shows sites in the expanded group). Keep the IETF assertions.

```ts
await page.getByRole('button', { name: /我的书签/ }).first().click();
```

If the row click toggles collapse of an already-expanded first category, click only when the URL link is missing:

```ts
const bookmarkRow = page.getByRole('button', { name: /我的书签/ }).first();
await bookmarkRow.click();
```

- [ ] **Step 2: Add layout-dialog + context-menu test**

Append inside `test.describe('用户工作台')`:

```ts
test('布局对话框改密度并支持站点右键', async ({ page }) => {
  await page.goto('/app/links');
  await page.getByRole('button', { name: '布局设置' }).click();
  const dialog = page.getByRole('heading', { name: '布局设置' }).locator('..');
  await expect(dialog).toBeVisible();
  await page.getByRole('button', { name: '紧凑', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('heading', { name: '布局设置' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '布局设置' })).toContainText('紧凑');

  const siteRow = page.getByText('IETF', { exact: true }).first();
  await siteRow.click({ button: 'right' });
  await expect(page.getByRole('button', { name: '编辑站点' })).toBeVisible();
  await page.getByRole('button', { name: '编辑站点' }).click();
  await expect(page.getByText('编辑站点').first()).toBeVisible();
});
```

This test depends on the earlier test having created IETF **or** must seed itself. E2E files in this repo share storage state across tests in the same worker, but tests must not import each other. Make this test self-contained: if IETF is missing, create a category+site like the first test (use a unique name `右键用例` / `ContextSite`).

- [ ] **Step 3: Run frontend unit tests**

Run: `cd web && npm run test:mock`

Expected: PASS (existing mock contract + new tests)

- [ ] **Step 4: Run e2e user spec**

Run: `cd tests/e2e && npx playwright test specs/user.spec.ts --grep "布局对话框|文件夹分类|创建分类"`

Expected: PASS. If the context menu portal does not use `role=button`, switch assertions to `getByText('编辑站点')` inside the portal (`div.fixed.z-\\[100\\]`).

- [ ] **Step 5: Browser smoke (required by workspace UI rule)**

`go run ./cmd/navax` (or existing dev server) + open `/app/links` logged in. Confirm:

1. Default is grouped list; no layout strip at the bottom; list is taller.
2. 布局 dialog changes density/columns; preview updates; draft saves.
3. Site and category context menus work; hover buttons still work.
4. Drag site within a left group; drag category by handle; drag a preview card onto another left group; refresh keeps order.
5. Table mode still batch-selects; folder preview stays read-only; locked theme knobs stay disabled in the dialog.

- [ ] **Step 6: Commit**

```bash
git add tests/e2e/specs/user.spec.ts
git commit -m "test: cover layout dialog and editor context menu"
```

---

## Spec coverage

| Spec | Task |
|---|---|
| §4.1 默认 card、布局按钮与摘要、去掉底栏 | 2 |
| §4.2 布局对话框、主题锁定、自动保存、Escape | 2 |
| §4.3 站点/分类右键、悬停按钮保留 | 3, 5 |
| §4.4 手柄、data-no-dnd、6px、DragOverlay | 4 |
| §5 文件拆分 | 1–4 |
| §6 ID 前缀、applySiteDrop、搬家而非复制 | 1, 4 |
| §6.4 收起追加并展开、过滤可见组、表格非投放区、文件夹预览只读 | 1, 4 |
| §6.5 pointerWithin / closestCenter / ring | 4 |
| §6.6 传感器过滤、对话框打开时不起拖 | 4 |
| §7 保存失败不回滚、删除确认、Escape 层级 | 现有 persist + 2/3 |
| §8 单测 | 1, 3 |
| §8.2/8.3 组件/e2e（无 RTL → Playwright） | 5 |
| §10 验收 | 5 浏览器 |

## Placeholder / type check

- 无 TBD。`overFromEvent` 在 Task 4 先测后写，名称不得改成别的。
- `createSiteContextActions` 的 `enabled` 字段与 `Site.enabled` 一致（`false` 为隐藏）。
- `layoutSummary` 密度文案只来自 `DENSITY_LABELS`，与对话框按钮同一份常量。
