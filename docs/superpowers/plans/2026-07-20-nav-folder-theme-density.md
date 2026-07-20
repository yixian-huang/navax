# Folder View · Theme Cull · Card Density Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add homepage `categoryStyle: folders` (folder tiles + hover/tap popover), reduce built-in themes to six with legacy id mapping, and tighten site cards ~30% across densities.

**Architecture:** Extend OpenAPI/`PageSettings` enum and Go validation; render a new `CategoryFolderWall` when `folders` and no search query; resolve unknown `themeId` via a shared map before `themeRegistry.activate`; tighten `SiteCard`/`SiteGrid` at the component layer (not per-theme CSS). Disable removed themes with an append-only migration.

**Tech Stack:** Go 1.25, OpenAPI, React 19 + TypeScript + Tailwind, Vitest, Playwright, SQLite migrations.

**Spec:** `docs/superpowers/specs/2026-07-20-nav-folder-theme-density-design.md`

---

## File map

| Path | Responsibility |
| --- | --- |
| `api/openapi.yaml` | `categoryStyle` enum `+ folders` |
| `internal/navigation/service.go` | Validate `folders` in settings |
| `web/src/api/types.ts` | TS union for `categoryStyle` |
| `web/src/api/mock-handlers.ts` | Pass-through already; no logic change beyond types |
| `web/src/lib/themeResolve.ts` | Create: legacy theme id → retained id |
| `web/src/components/feature/PublicShell.tsx` | Activate via `resolveThemeId` |
| `web/src/pages/app/themes/page.tsx` | Activate/select via `resolveThemeId` |
| `web/src/themes/packages/index.ts` | Register only 6 packages |
| `web/src/themes/packages/{kyoto,terracotta,mochi,pastelsky,mono,cyber}.ts` | Delete |
| `migrations/0013_disable_culled_themes.sql` | Create: `enabled=0` for culled theme rows; remapped pages optional |
| `web/src/components/base/CategoryFolderWall.tsx` | Create: folder grid + popover |
| `web/src/pages/home/components/SharedSections.tsx` | Branch folders vs tabs+grid; hide DensitySwitcher |
| `web/src/components/feature/PublicNavigationView.tsx` | Pass `categoryStyle` into layout props |
| `web/src/pages/home/components/Layout*.tsx` | Thread `categoryStyle` if needed |
| `web/src/pages/app/links/page.tsx` | UI control for categoryStyle |
| `web/src/components/base/SiteCard.tsx` | Medium density tighten |
| `web/src/components/base/SiteGrid.tsx` | Tighter gaps / more columns for compact |
| `web/tests/theme-resolve.test.ts` | Create: mapping unit tests |
| `web/tests/category-folder-wall.test.tsx` | Optional light DOM test if Vitest+RTL already set; else skip and cover via e2e |
| `tests/e2e/specs/user.spec.ts` | Folder style + theme still switchable |
| `internal/catalog/service_test.go` | Stop relying on `kyoto` if still enabled-count sensitive |

---

### Task 1: Contract — `folders` in OpenAPI + Go validation + TS types

**Files:**
- Modify: `api/openapi.yaml` (~line 2117)
- Modify: `internal/navigation/service.go` (~line 335)
- Modify: `web/src/api/types.ts` (LayoutConfig + PageSettings.layout)
- Test: `go test ./internal/navigation/ -count=1` (existing + any validation path)

- [ ] **Step 1: Extend OpenAPI enum**

In `api/openapi.yaml`, change:

```yaml
categoryStyle: { type: string, enum: [tabs, sidebar, grid] }
```

to:

```yaml
categoryStyle: { type: string, enum: [tabs, sidebar, grid, folders] }
```

- [ ] **Step 2: Extend Go validation**

In `internal/navigation/service.go`, find:

```go
!oneOf(settings.Layout.CategoryStyle, "tabs", "sidebar", "grid")
```

change to:

```go
!oneOf(settings.Layout.CategoryStyle, "tabs", "sidebar", "grid", "folders")
```

- [ ] **Step 3: Extend TypeScript unions**

In `web/src/api/types.ts`:

```ts
categoryStyle: 'tabs' | 'sidebar' | 'grid' | 'folders';
```

(both deprecated `LayoutConfig` and `PageSettings.layout`)

- [ ] **Step 4: Verify Go package still tests clean**

Run:

```bash
go test ./internal/navigation/ -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/openapi.yaml internal/navigation/service.go web/src/api/types.ts
git commit -m "feat: allow categoryStyle folders in layout contract"
```

---

### Task 2: Theme resolve helper + unit tests

**Files:**
- Create: `web/src/lib/themeResolve.ts`
- Create: `web/tests/theme-resolve.test.ts`
- Modify: `web/src/components/feature/PublicShell.tsx`
- Modify: `web/src/pages/app/themes/page.tsx` (activate path)

- [ ] **Step 1: Write failing tests**

Create `web/tests/theme-resolve.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { resolveThemeId, THEME_ID_ALIASES, RETAINED_THEME_IDS } from '@/lib/themeResolve';

describe('resolveThemeId', () => {
  it('keeps retained ids', () => {
    for (const id of RETAINED_THEME_IDS) {
      expect(resolveThemeId(id)).toBe(id);
    }
  });

  it('maps culled ids', () => {
    expect(resolveThemeId('kyoto')).toBe('slate');
    expect(resolveThemeId('terracotta')).toBe('slate');
    expect(resolveThemeId('mono')).toBe('slate');
    expect(resolveThemeId('mochi')).toBe('sakura');
    expect(resolveThemeId('pastelsky')).toBe('sakura');
    expect(resolveThemeId('cyber')).toBe('orbit');
  });

  it('falls back to slate for unknown', () => {
    expect(resolveThemeId('nope')).toBe('slate');
    expect(resolveThemeId('')).toBe('slate');
  });

  it('alias table matches design', () => {
    expect(THEME_ID_ALIASES).toEqual({
      kyoto: 'slate',
      terracotta: 'slate',
      mono: 'slate',
      mochi: 'sakura',
      pastelsky: 'sakura',
      cyber: 'orbit',
    });
  });
});
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd web && npx vitest run tests/theme-resolve.test.ts
```

Expected: FAIL (module not found)

- [ ] **Step 3: Implement `themeResolve.ts`**

```ts
/** Themes still shipped as CSS packages. */
export const RETAINED_THEME_IDS = [
  'slate',
  'slate-dark',
  'sakura',
  'noir',
  'orbit',
  'terminal',
] as const;

export type RetainedThemeId = (typeof RETAINED_THEME_IDS)[number];

/** Culled package ids → closest retained theme. */
export const THEME_ID_ALIASES: Record<string, RetainedThemeId> = {
  kyoto: 'slate',
  terracotta: 'slate',
  mono: 'slate',
  mochi: 'sakura',
  pastelsky: 'sakura',
  cyber: 'orbit',
};

const DEFAULT_THEME: RetainedThemeId = 'slate';

export function resolveThemeId(themeId: string | null | undefined): RetainedThemeId {
  const raw = (themeId || '').trim();
  if ((RETAINED_THEME_IDS as readonly string[]).includes(raw)) {
    return raw as RetainedThemeId;
  }
  if (raw in THEME_ID_ALIASES) {
    return THEME_ID_ALIASES[raw];
  }
  return DEFAULT_THEME;
}
```

- [ ] **Step 4: Wire PublicShell**

Replace activate effect body with:

```ts
import { resolveThemeId } from '@/lib/themeResolve';
// ...
useEffect(() => {
  themeRegistry.activate(resolveThemeId(themeId));
}, [themeId]);
```

In themes page, when calling `themeRegistry.activate(id)` for preview of user selection, keep activating the chosen retained id; when loading `page.settings.appearance.themeId`, set active UI state via `resolveThemeId(...)` so culled ids show the mapped package.

Example load:

```ts
setActiveId(resolveThemeId(page.settings.appearance.themeId));
themeRegistry.activate(resolveThemeId(page.settings.appearance.themeId));
```

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd web && npx vitest run tests/theme-resolve.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/themeResolve.ts web/tests/theme-resolve.test.ts \
  web/src/components/feature/PublicShell.tsx web/src/pages/app/themes/page.tsx
git commit -m "feat: map culled theme ids to retained packages"
```

---

### Task 3: Cull theme packages + disable DB rows

**Files:**
- Modify: `web/src/themes/packages/index.ts`
- Delete: `web/src/themes/packages/kyoto.ts`, `terracotta.ts`, `mochi.ts`, `pastelsky.ts`, `mono.ts`, `cyber.ts`
- Create: `migrations/0013_disable_culled_themes.sql`
- Modify: `internal/catalog/service_test.go` if it disables `kyoto` assuming it exists/enabled

- [ ] **Step 1: Shrink registry**

`web/src/themes/packages/index.ts`:

```ts
import { themeRegistry } from '@/themes/registry';
import { slateTheme } from '@/themes/packages/slate';
import { slateDarkTheme } from '@/themes/packages/slate-dark';
import { noirTheme } from '@/themes/packages/noir';
import { sakuraTheme } from '@/themes/packages/sakura';
import { orbitTheme } from '@/themes/packages/orbit';
import { terminalTheme } from '@/themes/packages/terminal';

themeRegistry.registerAll([
  slateTheme,
  slateDarkTheme,
  noirTheme,
  sakuraTheme,
  orbitTheme,
  terminalTheme,
]);
```

Delete the six culled package files.

- [ ] **Step 2: Migration to disable culled catalog rows**

Create `migrations/0013_disable_culled_themes.sql`:

```sql
-- Disable culled first-party themes so admin catalog matches SPA packages.
-- Existing page settings may still reference old themeIds; SPA resolveThemeId maps them.

UPDATE themes
SET enabled = 0,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id IN ('kyoto', 'terracotta', 'mochi', 'pastelsky', 'mono', 'cyber');
```

Do **not** rewrite historical migrations.

- [ ] **Step 3: Fix catalog test that toggles kyoto**

In `internal/catalog/service_test.go`, if test disables `kyoto` and expects count−1, either:

- use a retained non-default theme (`noir` or `terminal`), or
- skip if theme already disabled.

Prefer switching the test to:

```go
if _, err := db.Exec("UPDATE themes SET enabled = 0 WHERE id = 'noir'"); err != nil {
```

and assert `noir` absent from list.

- [ ] **Step 4: Run checks**

```bash
go test ./internal/catalog/ ./internal/database/ -count=1
cd web && npx vitest run tests/theme-resolve.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/themes/packages migrations/0013_disable_culled_themes.sql internal/catalog/service_test.go
git commit -m "feat: ship six retained themes and disable culled catalog rows"
```

---

### Task 4: CategoryFolderWall component

**Files:**
- Create: `web/src/components/base/CategoryFolderWall.tsx`
- Reuse: site icon resolution patterns from `SiteCard.tsx` (extract shared `SiteIcon` only if trivial; otherwise duplicate minimal img/favicon logic inside folder component to avoid large refactor)

- [ ] **Step 1: Implement folder wall**

Create `web/src/components/base/CategoryFolderWall.tsx` with approximately:

```tsx
import { useCallback, useEffect, useId, useRef, useState } from 'react';
import { cn } from '@/lib/utils';
import type { Category, Site } from '@/api/types';

export interface CategoryFolderWallProps {
  categories: Array<Pick<Category, 'id' | 'name' | 'sites'> & { sites: Site[] }>;
  onSiteOpen: (site: Site) => void;
  className?: string;
}

function siteFavicon(site: Site): string {
  try {
    const host = new URL(site.url).hostname.replace(/^www\./, '');
    const icon = (site.icon || '').trim();
    if (/^https?:\/\//i.test(icon)) return icon;
    return `https://www.google.com/s2/favicons?domain=${host}&sz=64`;
  } catch {
    return '';
  }
}

function FolderTile({
  category,
  open,
  onToggle,
  onSiteOpen,
}: {
  category: { id: string; name: string; sites: Site[] };
  open: boolean;
  onToggle: () => void;
  onSiteOpen: (site: Site) => void;
}) {
  const panelId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const preview = category.sites.slice(0, 4);

  // Desktop: open on pointer enter; close on leave of tile+panel (with short delay)
  // Touch / keyboard: button toggles open
  // Escape closes when open (listener on document when open)

  return (
    <div
      ref={rootRef}
      className="relative"
      onMouseEnter={() => { /* set open via parent hover handler for fine pointers */ }}
      onMouseLeave={() => { /* delayed close */ }}
    >
      <button
        type="button"
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-controls={panelId}
        onClick={onToggle}
        className={cn(
          'w-full aspect-square rounded-2xl p-3 flex flex-col items-center justify-center gap-2',
          'bg-background-50/70 border border-background-200/50 hover:border-primary-300/60',
          'transition-colors duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50',
        )}
      >
        <div className="grid grid-cols-2 gap-1 w-10 h-10">
          {Array.from({ length: 4 }).map((_, i) => {
            const site = preview[i];
            return (
              <div key={i} className="rounded-md bg-background-100/80 overflow-hidden flex items-center justify-center">
                {site ? (
                  <img src={siteFavicon(site)} alt="" className="w-full h-full object-contain" loading="lazy" />
                ) : null}
              </div>
            );
          })}
        </div>
        <span className="text-xs font-medium text-foreground-800 truncate max-w-full">{category.name}</span>
        <span className="text-[10px] text-foreground-400">{category.sites.length}</span>
      </button>

      {open ? (
        <div
          id={panelId}
          role="dialog"
          aria-label={category.name}
          className={cn(
            'absolute z-30 left-1/2 -translate-x-1/2 top-[calc(100%+8px)] w-56 max-h-64 overflow-auto',
            'rounded-xl border border-background-200/60 bg-background-0/95 backdrop-blur-md p-3 shadow-lg',
          )}
        >
          {category.sites.length === 0 ? (
            <p className="text-xs text-foreground-400 text-center py-4">暂无站点</p>
          ) : (
            <div className="grid grid-cols-4 gap-2">
              {category.sites.map(site => (
                <a
                  key={site.id}
                  href={site.url}
                  title={site.title}
                  className="flex flex-col items-center gap-1 min-h-[44px] rounded-lg p-1 hover:bg-background-100/80"
                  onClick={e => {
                    e.preventDefault();
                    onSiteOpen(site);
                  }}
                >
                  <img src={siteFavicon(site)} alt="" width={28} height={28} className="w-7 h-7 object-contain" />
                  <span className="text-[9px] text-foreground-700 line-clamp-1 w-full text-center">{site.title}</span>
                </a>
              ))}
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}

export default function CategoryFolderWall({ categories, onSiteOpen, className }: CategoryFolderWallProps) {
  const [openId, setOpenId] = useState<string | null>(null);
  const closeTimer = useRef<number | null>(null);

  const clearCloseTimer = () => {
    if (closeTimer.current != null) {
      window.clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  };

  const scheduleClose = () => {
    clearCloseTimer();
    closeTimer.current = window.setTimeout(() => setOpenId(null), 120);
  };

  useEffect(() => () => clearCloseTimer(), []);

  useEffect(() => {
    if (!openId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpenId(null);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [openId]);

  // Prefer hover-open only when matchMedia('(hover: hover) and (pointer: fine)')
  const hoverCapable =
    typeof window !== 'undefined' &&
    window.matchMedia('(hover: hover) and (pointer: fine)').matches;

  return (
    <div className={cn('grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 gap-3', className)}>
      {categories.map(cat => (
        <FolderTile
          key={cat.id}
          category={cat}
          open={openId === cat.id}
          onToggle={() => setOpenId(prev => (prev === cat.id ? null : cat.id))}
          onSiteOpen={onSiteOpen}
          // Implement hover handlers inside FolderTile using hoverCapable + scheduleClose/clearCloseTimer + setOpenId
        />
      ))}
    </div>
  );
}
```

**Implementation notes for the agent:** finish hover wiring cleanly (props for `onHoverOpen` / `onHoverIntentClose`); do not leave the placeholder comments. Click outside should close on touch (document pointerdown when open).

- [ ] **Step 2: Manual sanity in dev (optional)** — skip if no time; e2e covers later.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/base/CategoryFolderWall.tsx
git commit -m "feat: add CategoryFolderWall with hover and tap popover"
```

---

### Task 5: Wire folders into public navigation + layout settings UI

**Files:**
- Modify: `web/src/components/feature/PublicNavigationView.tsx`
- Modify: `web/src/pages/home/components/SharedSections.tsx`
- Modify: `web/src/pages/home/components/LayoutFull.tsx`, `LayoutSearchFocus.tsx`, `LayoutBrowseFirst.tsx`, `LayoutSidebar.tsx` (thread props)
- Modify: `web/src/pages/app/links/page.tsx` (category style control near density)

- [ ] **Step 1: Pass `categoryStyle` from PublicNavigationView**

```ts
const categoryStyle = settings?.layout.categoryStyle ?? 'tabs';
// layoutProps:
categoryStyle,
```

- [ ] **Step 2: SitesSection branching**

In `SharedSections.tsx`:

```tsx
import CategoryFolderWall from '@/components/base/CategoryFolderWall';

// props: add categoryStyle?: 'tabs' | 'sidebar' | 'grid' | 'folders'

const useFolders = categoryStyle === 'folders' && !query.trim();

// header: only show DensitySwitcher when !useFolders
// body:
{useFolders ? (
  <CategoryFolderWall categories={categories} onSiteOpen={onSiteOpen} />
) : (
  <>
    {categories.length > 1 && (
      <CategoryTabs ... />
    )}
    <SiteGrid ... />
  </>
)}
```

When `useFolders`, do not filter by `activeCategory` for the wall — pass all categories with their sites. Keep `activeSites` logic for non-folder modes.

- [ ] **Step 3: Layout components**

Thread `categoryStyle` through each layout into `SitesSection` the same way as `density`.

- [ ] **Step 4: Links page layout settings**

Near density controls (~line 1290), add:

```tsx
const CATEGORY_STYLES = [
  { id: 'tabs' as const, label: '标签' },
  { id: 'sidebar' as const, label: '侧栏' },
  { id: 'grid' as const, label: '网格' },
  { id: 'folders' as const, label: '文件夹' },
];

// buttons that patch settings.layout.categoryStyle via same mutate pattern as density
```

Chinese labels only (match product language). Persist with existing `useUpdatePageSettings` / page settings mutation used for density.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feature/PublicNavigationView.tsx \
  web/src/pages/home/components web/src/pages/app/links/page.tsx
git commit -m "feat: wire folders category style into home and layout settings"
```

---

### Task 6: Medium card density tighten

**Files:**
- Modify: `web/src/components/base/SiteCard.tsx`
- Modify: `web/src/components/base/SiteGrid.tsx`

- [ ] **Step 1: Comfortable — icon + title, secondary only in tooltip**

Replace comfortable branch body classes/content with:

```tsx
// Comfortable: tighter; secondary (desc|domain) only in CardWrapper title tooltip (already tip)
return (
  <CardWrapper
    {...shared}
    className="material-card site-card-comfortable flex items-center gap-2.5 px-2.5 py-2 min-h-[3.25rem]"
  >
    <span className="site-card-favicon flex h-7 w-7 flex-shrink-0 items-center justify-center">
      <SiteIcon site={site} size={28} />
    </span>
    <div className="min-w-0 flex-1 flex items-center gap-1">
      <h3 className="site-card-title min-w-0 flex-1 text-[13px] font-semibold leading-5 text-foreground-900 line-clamp-1 group-hover:text-accent-500 transition-colors duration-200">
        <HighlightText text={site.title} query={q} />
      </h3>
      <i className="ri-arrow-right-up-line text-sm text-foreground-300 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex-shrink-0" />
    </div>
  </CardWrapper>
);
```

- [ ] **Step 2: Compact — slightly smaller**

```tsx
className="material-card flex flex-col items-center gap-1.5 p-2 min-h-[3.75rem]"
// SiteIcon size={22}
// title text-[10px] line-clamp-2
```

- [ ] **Step 3: List — reduce padding, keep one secondary line**

```tsx
className="site-card-list flex items-center gap-2.5 px-2.5 py-2 rounded-lg ..."
// favicon h-8 w-8, SiteIcon size={18}
// keep secondary line (desc || domain)
```

- [ ] **Step 4: SiteGrid gaps/columns**

```tsx
density === 'comfortable'
  ? (comfortableCols || 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-2.5')
  : (compactCols || 'grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-2'),
// list panel: p-0.5 sm:p-1
```

- [ ] **Step 5: Commit**

```bash
git add web/src/components/base/SiteCard.tsx web/src/components/base/SiteGrid.tsx
git commit -m "style: tighten site cards for medium density presentation"
```

---

### Task 7: E2E + verification gates

**Files:**
- Modify: `tests/e2e/specs/user.spec.ts` (or new focused test)
- Possibly: `tests/e2e/specs/guest.spec.ts` density assertion still valid

- [ ] **Step 1: Add e2e coverage**

In user flow after login:

1. Open layout settings on `/app/links` (or wherever category style control lives).
2. Select 「文件夹」.
3. Publish if required for public snapshot — **if** categoryStyle only affects draft preview, assert on preview; if public needs publish, publish then open public/share URL.
4. Expect folder tiles (`[aria-haspopup="dialog"]` or role button with category name).
5. Hover or click first folder; expect dialog with site links.

Minimal sketch:

```ts
test('文件夹分类样式可展开站点', async ({ page }) => {
  await page.goto('/app/links');
  await page.getByRole('button', { name: '文件夹' }).click();
  // if publish needed:
  // await page.goto('/app/publish'); ... publish ...
  await page.goto('/'); // or user public slug from setup
  const folder = page.locator('[aria-haspopup="dialog"]').first();
  await expect(folder).toBeVisible();
  await folder.click();
  await expect(page.getByRole('dialog')).toBeVisible();
});
```

Adapt selectors to actual UI after Task 5.

- [ ] **Step 2: Run merge gates**

```bash
make check
go test -race ./...
# if UI path touched:
make e2e
# full ship path also needs make build when done
make build
```

Expected: all green. Fix failures before claiming done.

- [ ] **Step 3: Browser smoke checklist (manual note in PR)**

Loading, empty category folder, error, mobile tap open/close, keyboard Escape, dark theme (slate-dark).

- [ ] **Step 4: Final commit**

```bash
git add tests/e2e
git commit -m "test: cover folder category style on public navigation"
```

---

## Spec coverage checklist

| Spec item | Task |
| --- | --- |
| `categoryStyle: folders` contract | 1 |
| Folder tiles + hover popover + touch + Escape | 4–5 |
| Search flattens to SiteGrid | 5 (`!query`) |
| Hide DensitySwitcher in folders | 5 |
| Layout settings UI | 5 |
| Six themes + mapping | 2–3 |
| Medium card tighten | 6 |
| Culled themes disabled in catalog | 3 |
| Tests / check / e2e | 1–2, 7 |

## Out of scope (do not implement)

- Folder as density mode, in-place expand animation, drag-reorder inside folders
- Third-party theme upload
- Rewriting old migration seed rows (only append `0013_…`)
