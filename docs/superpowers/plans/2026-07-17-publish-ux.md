# Publish UX (方案 A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make draft vs published state obvious across the workspace, and give `/app/publish` a single clear primary CTA for first publish and update publish.

**Architecture:** Keep the existing backend draft/snapshot model. Add a pure `derivePublishUiState` helper from `Publication` + draft timestamps, shared UI (`PublishStatusControl`, `PublishDraftBanner`), and rewire AppShell / publish / overview / preview / edit-page toasts to that state machine. No OpenAPI or Go changes.

**Tech Stack:** React 19, TypeScript, TanStack Query, react-router v7, Vitest (`web/tests/**/*.test.ts`), existing `usePublish` / `useMyPage` hooks.

**Spec:** `docs/superpowers/specs/2026-07-17-publish-ux-design.md`

---

## File map

| File | Responsibility |
|------|----------------|
| Create `web/src/lib/publish-state.ts` | Pure three-state derivation + draft-save toast helper |
| Create `web/tests/publish-state.test.ts` | Table-driven unit tests |
| Create `web/src/hooks/usePublishUiState.ts` | Wrap `useMyPage` + derive state |
| Create `web/src/components/feature/PublishStatusControl.tsx` | AppShell primary publish control |
| Create `web/src/components/feature/PublishDraftBanner.tsx` | Dismissible draft banner for edit pages |
| Modify `web/src/components/feature/AppShell.tsx` | Swap static “发布 & 域名” link for control |
| Modify `web/src/components/base/SharedUI.tsx` | Align `PublishStatusBadge` labels with three states |
| Modify `web/src/pages/app/publish/page.tsx` | Primary card + remove toggle; keep domain section |
| Modify `web/src/pages/app/overview/page.tsx` | Draft vs live links; three-state stats; publish CTA |
| Modify `web/src/pages/app/preview/page.tsx` | Top bar with publish action |
| Modify `web/src/pages/app/themes/page.tsx` | Draft toasts + banner |
| Modify `web/src/pages/app/widgets/page.tsx` | Draft toasts + banner |
| Modify `web/src/pages/app/links/page.tsx` | Draft-aware save feedback + banner |
| Modify `web/src/pages/app/import-export/page.tsx` | Draft toast after import if applicable |

---

### Task 1: `derivePublishUiState` pure helper (TDD)

**Files:**
- Create: `web/src/lib/publish-state.ts`
- Test: `web/tests/publish-state.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/tests/publish-state.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import {
  derivePublishUiState,
  draftSaveToastMessage,
  type PublicationLike,
} from '../src/lib/publish-state';

function pub(partial: Partial<PublicationLike>): PublicationLike {
  return {
    published: false,
    hasUnpublishedChanges: false,
    publishedAt: null,
    visibility: 'unlisted',
    ...partial,
  };
}

describe('derivePublishUiState', () => {
  it('maps never published', () => {
    const s = derivePublishUiState(pub({ published: false }), '2026-07-17T10:00:00Z');
    expect(s.id).toBe('never_published');
    expect(s.shortLabel).toBe('未发布');
    expect(s.primaryAction).toBe('publish');
    expect(s.primaryLabel).toBe('发布');
    expect(s.primaryDisabled).toBe(false);
    expect(s.showUnpublish).toBe(false);
  });

  it('maps published with draft changes', () => {
    const s = derivePublishUiState(
      pub({
        published: true,
        hasUnpublishedChanges: true,
        publishedAt: '2026-07-16T08:00:00Z',
      }),
      '2026-07-17T10:00:00Z',
    );
    expect(s.id).toBe('published_with_draft');
    expect(s.shortLabel).toBe('有草稿未上线');
    expect(s.primaryAction).toBe('publish_update');
    expect(s.primaryLabel).toBe('发布更新');
    expect(s.primaryDisabled).toBe(false);
    expect(s.showUnpublish).toBe(true);
  });

  it('maps published current', () => {
    const s = derivePublishUiState(
      pub({
        published: true,
        hasUnpublishedChanges: false,
        publishedAt: '2026-07-17T09:00:00Z',
      }),
      '2026-07-17T09:00:00Z',
    );
    expect(s.id).toBe('published_current');
    expect(s.shortLabel).toBe('已是最新');
    expect(s.primaryAction).toBe('none');
    expect(s.primaryLabel).toBe('已是最新');
    expect(s.primaryDisabled).toBe(true);
    expect(s.showUnpublish).toBe(true);
  });

  it('disables publish on publish page when visibility is private', () => {
    const s = derivePublishUiState(
      pub({ published: false, visibility: 'private' }),
      null,
      { surface: 'publish_page' },
    );
    expect(s.primaryDisabled).toBe(true);
    expect(s.blockReason).toMatch(/可见性/);
  });

  it('does not disable top-bar primary when private (navigates instead)', () => {
    const s = derivePublishUiState(
      pub({ published: false, visibility: 'private' }),
      null,
      { surface: 'toolbar' },
    );
    expect(s.primaryAction).toBe('publish');
    expect(s.primaryDisabled).toBe(false);
    expect(s.requiresVisibilityFix).toBe(true);
  });
});

describe('draftSaveToastMessage', () => {
  it('uses plain draft message when never published', () => {
    expect(draftSaveToastMessage(pub({ published: false }))).toBe('已保存到草稿');
  });

  it('warns visitors still see live when published', () => {
    expect(
      draftSaveToastMessage(pub({ published: true, hasUnpublishedChanges: true })),
    ).toBe('已保存到草稿 · 访客仍看线上版');
  });

  it('allows themed prefix', () => {
    expect(
      draftSaveToastMessage(pub({ published: false }), '主题已写入草稿：「石板」'),
    ).toBe('主题已写入草稿：「石板」');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run tests/publish-state.test.ts`

Expected: FAIL — cannot resolve `../src/lib/publish-state`

- [ ] **Step 3: Implement `web/src/lib/publish-state.ts`**

```ts
import type { Visibility } from '@/api/types';

export type PublishUiStateId =
  | 'never_published'
  | 'published_with_draft'
  | 'published_current';

export type PublishPrimaryAction = 'publish' | 'publish_update' | 'none';

export type PublishSurface = 'publish_page' | 'toolbar' | 'banner' | 'overview' | 'preview';

export interface PublicationLike {
  published: boolean;
  hasUnpublishedChanges: boolean;
  publishedAt: string | null;
  visibility: Visibility;
}

export interface PublishUiState {
  id: PublishUiStateId;
  shortLabel: string;
  primaryAction: PublishPrimaryAction;
  primaryLabel: string;
  primaryDisabled: boolean;
  showUnpublish: boolean;
  /** When true, clicking primary should navigate to publish page visibility section instead of calling API */
  requiresVisibilityFix: boolean;
  blockReason: string | null;
  publishedAt: string | null;
  draftUpdatedAt: string | null;
}

export function derivePublishUiState(
  publication: PublicationLike | null | undefined,
  draftUpdatedAt: string | null = null,
  options: { surface?: PublishSurface } = {},
): PublishUiState {
  const surface = options.surface ?? 'toolbar';
  const published = publication?.published ?? false;
  const hasDraft = publication?.hasUnpublishedChanges ?? false;
  const visibility = publication?.visibility ?? 'unlisted';
  const publishedAt = publication?.publishedAt ?? null;

  let id: PublishUiStateId;
  if (!published) id = 'never_published';
  else if (hasDraft) id = 'published_with_draft';
  else id = 'published_current';

  const base: Omit<PublishUiState, 'primaryDisabled' | 'requiresVisibilityFix' | 'blockReason'> = {
    id,
    shortLabel:
      id === 'never_published' ? '未发布' : id === 'published_with_draft' ? '有草稿未上线' : '已是最新',
    primaryAction:
      id === 'never_published' ? 'publish' : id === 'published_with_draft' ? 'publish_update' : 'none',
    primaryLabel:
      id === 'never_published' ? '发布' : id === 'published_with_draft' ? '发布更新' : '已是最新',
    showUnpublish: published,
    publishedAt,
    draftUpdatedAt,
  };

  const isPrivate = visibility === 'private';
  const tryingToPublish = base.primaryAction === 'publish' || base.primaryAction === 'publish_update';

  if (base.primaryAction === 'none') {
    return {
      ...base,
      primaryDisabled: true,
      requiresVisibilityFix: false,
      blockReason: null,
    };
  }

  if (isPrivate && tryingToPublish && surface === 'publish_page') {
    return {
      ...base,
      primaryDisabled: true,
      requiresVisibilityFix: true,
      blockReason: '请先将可见性改为「知道链接即可访问」或「公开展示」并保存设置',
    };
  }

  return {
    ...base,
    primaryDisabled: false,
    requiresVisibilityFix: isPrivate && tryingToPublish,
    blockReason: null,
  };
}

/** Toast after a draft-mutating save. Optional `override` replaces the whole message (e.g. themed prefix that already includes draft language). */
export function draftSaveToastMessage(
  publication: PublicationLike | null | undefined,
  override?: string,
): string {
  if (override) return override;
  if (publication?.published) return '已保存到草稿 · 访客仍看线上版';
  return '已保存到草稿';
}

export function publishSuccessToastMessage(stateId: PublishUiStateId): string {
  return stateId === 'published_with_draft' || stateId === 'published_current'
    ? '更新已发布'
    : '发布成功';
}
```

Note: for success toast after publish, the **pre-click** state is `never_published` → “发布成功”, or `published_with_draft` → “更新已发布”. Callers should pass the state **before** mutation.

- [ ] **Step 4: Run tests**

Run: `cd web && npx vitest run tests/publish-state.test.ts`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/publish-state.ts web/tests/publish-state.test.ts
git commit -m "feat: add publish UI state derivation helper"
```

---

### Task 2: `usePublishUiState` hook

**Files:**
- Create: `web/src/hooks/usePublishUiState.ts`
- Modify (optional export re-export only if needed): none required

- [ ] **Step 1: Implement hook**

```ts
import { useMemo } from 'react';
import { useMyPage, usePageScope } from '@/hooks/useQueries';
import {
  derivePublishUiState,
  type PublishSurface,
  type PublishUiState,
} from '@/lib/publish-state';

export function usePublishUiState(surface: PublishSurface = 'toolbar'): {
  state: PublishUiState;
  pageId: string | undefined;
  slug: string;
  scope: 'personal' | 'system';
  isLoading: boolean;
  isError: boolean;
  publication: ReturnType<typeof useMyPage>['data'] extends infer P
    ? P extends { publication: infer Pub }
      ? Pub
      : undefined
    : undefined;
  page: ReturnType<typeof useMyPage>['data'];
  refetch: ReturnType<typeof useMyPage>['refetch'];
} {
  const scope = usePageScope();
  const query = useMyPage(scope);
  const publication = query.data?.publication;

  const state = useMemo(
    () =>
      derivePublishUiState(publication, query.data?.draftUpdatedAt ?? null, { surface }),
    [publication, query.data?.draftUpdatedAt, surface],
  );

  return {
    state,
    pageId: query.data?.id,
    slug: publication?.slug ?? '',
    scope,
    isLoading: query.isLoading,
    isError: query.isError,
    publication: publication as never,
    page: query.data,
    refetch: query.refetch,
  };
}
```

If `usePageScope` typing is awkward, simplify return type to explicit interfaces without complex conditionals — keep `page` as `NavigationPage | undefined` using existing types from `@/api/types` or the page type already used by `useMyPage`.

Prefer:

```ts
import type { NavigationPageContract, Publication } from '@/api/types';
// use the actual return type of useMyPage — if it's a richer NavigationPage, import that.
```

Check `useMyPage` return: it returns `res.data` from `getCurrentPage`. Use whatever type that is (likely includes `publication` and `draftUpdatedAt`).

- [ ] **Step 2: Typecheck**

Run: `cd web && npx tsc -p tsconfig.app.json --noEmit`  
(or rely on next `make check`)

Fix any type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/usePublishUiState.ts
git commit -m "feat: add usePublishUiState hook"
```

---

### Task 3: Shared publish action helper + `PublishStatusControl`

**Files:**
- Create: `web/src/components/feature/PublishStatusControl.tsx`
- Create: `web/src/lib/publish-actions.ts` (small shared click handler helpers used by control, banner, overview)

- [ ] **Step 1: Implement `web/src/lib/publish-actions.ts`**

```ts
import type { NavigateFunction } from 'react-router-dom';
import type { PublishUiState } from '@/lib/publish-state';
import { publishSuccessToastMessage } from '@/lib/publish-state';

export function publishSettingsPath(scope: string, highlight?: 'visibility'): string {
  const params = new URLSearchParams({ scope });
  if (highlight) params.set('highlight', highlight);
  return `/app/publish?${params.toString()}`;
}

export function previewPath(scope: string): string {
  return `/app/preview?scope=${scope}`;
}

/**
 * Decide whether to call publish API or redirect for visibility fix.
 * Returns 'publish' | 'redirect_visibility' | 'noop'.
 */
export function resolvePrimaryPublishIntent(state: PublishUiState): 'publish' | 'redirect_visibility' | 'noop' {
  if (state.primaryAction === 'none' || state.primaryDisabled) return 'noop';
  if (state.requiresVisibilityFix) return 'redirect_visibility';
  return 'publish';
}

export function toastForPublishSuccess(stateBefore: PublishUiState): string {
  return publishSuccessToastMessage(stateBefore.id === 'published_with_draft' ? 'published_with_draft' : 'never_published');
}

export function navigateToVisibilityFix(navigate: NavigateFunction, scope: string): void {
  navigate(publishSettingsPath(scope, 'visibility'));
}
```

- [ ] **Step 2: Implement `PublishStatusControl`**

```tsx
// web/src/components/feature/PublishStatusControl.tsx
import { Link, useNavigate } from 'react-router-dom';
import { Globe, Loader2 } from 'lucide-react';
import { usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import {
  navigateToVisibilityFix,
  publishSettingsPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';

export default function PublishStatusControl({ className }: { className?: string }) {
  const { state, scope, isLoading } = usePublishUiState('toolbar');
  const publish = usePublish();
  const { toast } = useToast();
  const navigate = useNavigate();

  const onPrimary = () => {
    const intent = resolvePrimaryPublishIntent(state);
    if (intent === 'noop') return;
    if (intent === 'redirect_visibility') {
      navigateToVisibilityFix(navigate, scope);
      return;
    }
    const before = state;
    publish.mutate(undefined, {
      onSuccess: () => toast('success', toastForPublishSuccess(before)),
      onError: (e: Error) => {
        toast('error', e.message || '发布失败');
        if (/visibility|私密|private/i.test(e.message || '')) {
          navigateToVisibilityFix(navigate, scope);
        }
      },
    });
  };

  if (isLoading) {
    return (
      <div className={cn('hidden sm:inline-flex items-center gap-2 h-7 px-2 text-xs text-foreground-400', className)}>
        <Loader2 className="w-3 h-3 animate-spin" />
      </div>
    );
  }

  return (
    <div className={cn('hidden sm:inline-flex items-center gap-2', className)}>
      <span
        className={cn(
          'text-xs',
          state.id === 'published_with_draft' && 'text-accent-600 font-medium',
          state.id === 'published_current' && 'text-green-600',
          state.id === 'never_published' && 'text-foreground-500',
        )}
      >
        {state.shortLabel}
      </span>
      {state.primaryAction !== 'none' && (
        <button
          type="button"
          onClick={onPrimary}
          disabled={publish.isPending || state.primaryDisabled}
          className="inline-flex items-center gap-1.5 h-7 px-3 rounded-md bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-50 transition-colors"
        >
          {publish.isPending ? <Loader2 className="w-3 h-3 animate-spin" /> : <Globe className="w-3 h-3" />}
          {state.primaryLabel}
        </button>
      )}
      <Link
        to={publishSettingsPath(scope)}
        className="inline-flex items-center h-7 px-2 rounded-md text-xs text-foreground-500 hover:text-foreground-700 hover:bg-background-100 transition-colors"
      >
        发布设置
      </Link>
    </div>
  );
}
```

Also add a compact mobile variant in the same file or show icon-only on `sm:hidden` if the plan’s design needs it — minimum: keep control `hidden sm:inline-flex` and leave the sidebar “发布 & 域名” nav item as secondary entry (sidebar already has the route).

- [ ] **Step 3: Wire into AppShell**

In `web/src/components/feature/AppShell.tsx`, replace the static header link:

```tsx
// remove:
// <Link to={`/app/publish?scope=${scope}`} ...>发布 & 域名</Link>

// add:
import PublishStatusControl from '@/components/feature/PublishStatusControl';
// in header actions:
<PublishStatusControl />
```

- [ ] **Step 4: Smoke typecheck / lint on touched files**

Run: `cd web && npx tsc -p tsconfig.app.json --noEmit`

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/publish-actions.ts web/src/components/feature/PublishStatusControl.tsx web/src/components/feature/AppShell.tsx
git commit -m "feat: show publish status and primary CTA in app shell"
```

---

### Task 4: `PublishDraftBanner` component

**Files:**
- Create: `web/src/components/feature/PublishDraftBanner.tsx`

- [ ] **Step 1: Implement banner**

```tsx
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { X, Loader2 } from 'lucide-react';
import { usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import { useToast } from '@/components/base/Toast';
import {
  navigateToVisibilityFix,
  previewPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';
import { cn } from '@/lib/utils';

function dismissKey(scope: string, pageId: string) {
  return `navax:publish-banner-dismissed:${scope}:${pageId}`;
}

export default function PublishDraftBanner({ className }: { className?: string }) {
  const { state, scope, pageId } = usePublishUiState('banner');
  const publish = usePublish();
  const { toast } = useToast();
  const navigate = useNavigate();
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    if (!pageId) return;
    setDismissed(sessionStorage.getItem(dismissKey(scope, pageId)) === '1');
  }, [scope, pageId]);

  if (state.id !== 'published_with_draft' || dismissed || !pageId) return null;

  const onDismiss = () => {
    sessionStorage.setItem(dismissKey(scope, pageId), '1');
    setDismissed(true);
  };

  const onPublish = () => {
    const intent = resolvePrimaryPublishIntent(state);
    if (intent === 'redirect_visibility') {
      navigateToVisibilityFix(navigate, scope);
      return;
    }
    if (intent !== 'publish') return;
    const before = state;
    publish.mutate(undefined, {
      onSuccess: () => toast('success', toastForPublishSuccess(before)),
      onError: (e: Error) => {
        toast('error', e.message || '发布失败');
        if (/visibility|私密|private/i.test(e.message || '')) {
          navigateToVisibilityFix(navigate, scope);
        }
      },
    });
  };

  return (
    <div
      className={cn(
        'mb-4 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-3 rounded-lg border border-accent-200 bg-accent-50 px-3 py-2.5 text-sm text-accent-800',
        className,
      )}
      role="status"
    >
      <span className="flex-1">你有未上线的草稿</span>
      <div className="flex items-center gap-2 flex-wrap">
        <button
          type="button"
          onClick={onPublish}
          disabled={publish.isPending}
          className="h-8 px-3 rounded-md bg-accent-600 text-white text-xs font-medium hover:bg-accent-700 disabled:opacity-50 inline-flex items-center gap-1.5"
        >
          {publish.isPending && <Loader2 className="w-3 h-3 animate-spin" />}
          发布更新
        </button>
        <Link
          to={previewPath(scope)}
          className="h-8 px-3 rounded-md border border-accent-200 text-xs font-medium hover:bg-accent-100 inline-flex items-center"
        >
          草稿预览
        </Link>
        <button
          type="button"
          onClick={onDismiss}
          className="w-8 h-8 inline-flex items-center justify-center rounded-md text-accent-600 hover:bg-accent-100"
          aria-label="关闭提示"
        >
          <X className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/feature/PublishDraftBanner.tsx
git commit -m "feat: add dismissible publish draft banner"
```

---

### Task 5: Redesign publish page primary card

**Files:**
- Modify: `web/src/pages/app/publish/page.tsx`

- [ ] **Step 1: Restructure page**

Key changes (implement fully in file, keep subdomain/CNAME blocks largely intact):

1. Import `usePublishUiState`, `derive` not needed if hook used with `surface: 'publish_page'`.
2. **Remove** `handleTogglePublish` switch UI (the `w-14 h-8 rounded-full` toggle).
3. Top card structure:

```tsx
const { state } = usePublishUiState('publish_page');
// ...
const handlePublish = () => {
  if (state.primaryDisabled || state.primaryAction === 'none') return;
  const before = state;
  publishMutation(undefined, {
    onSuccess: () => toast('success', toastForPublishSuccess(before)),
    onError: (e: Error) => toast('error', e.message || '发布失败'),
  });
};

const handleUnpublish = () => {
  if (!window.confirm('取消后公开链接将不可访问；草稿保留。确定取消发布？')) return;
  unpublishMutation(undefined, {
    onSuccess: () => toast('success', '已取消发布'),
    onError: (e: Error) => toast('error', e.message || '取消发布失败'),
  });
};
```

4. Primary card JSX (replace toggle block):

```tsx
<div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
  <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
    <div>
      <h3 className="text-base font-semibold text-foreground-900">{state.shortLabel}</h3>
      <p className="text-xs text-foreground-400 mt-1">
        {state.draftUpdatedAt && <>草稿更新于 {new Date(state.draftUpdatedAt).toLocaleString('zh-CN')}</>}
        {state.publishedAt && <> · 上次发布 {new Date(state.publishedAt).toLocaleString('zh-CN')}</>}
      </p>
      {state.id === 'published_with_draft' && (
        <p className="text-xs text-accent-600 mt-2">当前访客仍看到线上版</p>
      )}
      {state.blockReason && (
        <p className="text-xs text-red-500 mt-2" id="visibility-hint">{state.blockReason}</p>
      )}
    </div>
    <div className="flex flex-col items-stretch sm:items-end gap-2">
      {state.primaryAction !== 'none' ? (
        <button
          type="button"
          onClick={handlePublish}
          disabled={publishing || state.primaryDisabled}
          className="h-10 px-5 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50"
        >
          {publishing ? '发布中…' : state.primaryLabel}
        </button>
      ) : (
        <button type="button" disabled className="h-10 px-5 rounded-lg bg-background-200 text-foreground-500 text-sm font-medium cursor-default">
          已是最新
        </button>
      )}
      <div className="flex flex-wrap gap-2 justify-end text-xs">
        <Link to={`/app/preview?scope=...`} className="...">草稿预览</Link>
        {isPublished && (
          <Link to={`/u/${slug}`} target="_blank" className="...">打开线上版</Link>
        )}
        {state.showUnpublish && (
          <button type="button" onClick={handleUnpublish} disabled={unpublishing} className="text-red-500 hover:text-red-600">
            取消发布
          </button>
        )}
      </div>
    </div>
  </div>
</div>
```

5. Page subtitle: `先确认内容上线，再管理访问方式与域名`

6. Publication settings card: add note under save button:

```tsx
<p className="text-[11px] text-foreground-400">
  保存设置后，若页面已发布，需再点「发布更新」才会进入公开快照（含 slug / SEO 等）。
</p>
```

7. Support `?highlight=visibility` from URL: on mount, if `highlight=visibility`, scroll/focus the visibility select and optionally add `ring-2 ring-primary-300` for a few seconds.

```tsx
const [searchParams] = useSearchParams();
useEffect(() => {
  if (searchParams.get('highlight') !== 'visibility') return;
  const el = document.getElementById('publication-visibility');
  el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  el?.focus();
}, [searchParams]);
// put id="publication-visibility" on the visibility <select>
```

8. Remove obsolete tips bullet “更改导航内容后记得重新发布”.

9. Do **not** use publish toggle for unpublish anymore.

- [ ] **Step 2: Manual sanity** — page still loads under mock/dev if used.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/app/publish/page.tsx
git commit -m "feat: redesign publish page with explicit primary CTA"
```

---

### Task 6: Overview + preview pages

**Files:**
- Modify: `web/src/pages/app/overview/page.tsx`
- Modify: `web/src/pages/app/preview/page.tsx`

- [ ] **Step 1: Overview**

Using `usePublishUiState('overview')` and `usePublish`:

1. Replace header “预览” (`/u/{slug}`) with:
   - Link「草稿预览」→ `/app/preview?scope=`
   - If `publication.published`, Link「线上版」→ `/u/{slug}` target blank
2. Stats “发布状态” value → `state.shortLabel` (三态)
3. Header primary button can stay “发布设置” or become state-aware; prefer keep link to publish page + if `primaryAction !== 'none'` show「发布 / 发布更新」button next to it
4. Unpublished changes block: primary button calls `usePublish` with label「发布更新」, secondary link「发布设置」
5. quickActions step0:
   - label: `发布与访问`
   - desc based on state
   - highlight card when `state.id === 'published_with_draft'` (accent border)

- [ ] **Step 2: Preview page top bar**

In `preview/page.tsx` header area:

```tsx
import { usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
// ...
const { state, scope } = usePublishUiState('preview');
const publish = usePublish();
// Show bar:
// 草稿预览 · 非公开 | [发布/发布更新] | [打开线上版 if published]
```

Only show live link when `state.showUnpublish` / `publication.published` (currently always shows “打开已发布页” if slug exists — gate on `published`).

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/app/overview/page.tsx web/src/pages/app/preview/page.tsx
git commit -m "feat: fix overview and preview publish affordances"
```

---

### Task 7: Draft save toasts + banners on edit pages

**Files:**
- Modify: `web/src/pages/app/themes/page.tsx`
- Modify: `web/src/pages/app/widgets/page.tsx`
- Modify: `web/src/pages/app/links/page.tsx`
- Modify: `web/src/pages/app/import-export/page.tsx`
- Modify: `web/src/components/base/SharedUI.tsx` (`PublishStatusBadge`)

- [ ] **Step 1: Align `PublishStatusBadge`**

```tsx
export function PublishStatusBadge({
  hasUnpublishedChanges,
  publishedAt,
  published,
}: {
  hasUnpublishedChanges: boolean;
  publishedAt: string | null;
  published?: boolean;
}) {
  const isPublished = published ?? Boolean(publishedAt);
  if (!isPublished) return <Badge variant="default">未发布</Badge>;
  if (hasUnpublishedChanges) return <Badge variant="warning">有草稿未上线</Badge>;
  return <Badge variant="success">已是最新</Badge>;
}
```

(If existing call sites only pass two props, keep optional `published` with fallback.)

- [ ] **Step 2: Themes page**

- Import `PublishDraftBanner`, `draftSaveToastMessage`
- After successful theme/bg saves:

```ts
const publication = page?.publication;
toast('success', draftSaveToastMessage(publication, `主题已写入草稿：「${pkg?.meta.name || id}」`));
// background:
toast('success', draftSaveToastMessage(publication));
// clear bg can stay info but prefer:
toast('info', draftSaveToastMessage(publication, '背景图已从草稿清除'));
```

- Render `<PublishDraftBanner />` below page title.

- [ ] **Step 3: Widgets page**

```ts
onSuccess: () => toast('success', draftSaveToastMessage(pageQuery.data?.publication)),
```

Add `<PublishDraftBanner />` under title.

- [ ] **Step 4: Links page**

After mutations that already `markSaved()`, also toast when appropriate — links page currently relies on `useSaveStatus` indicator without toast for many ops. Spec requires draft-aware feedback:

- On `markSaved` paths that are user-visible saves (create/update/delete site/category, save composition): call `toast('success', draftSaveToastMessage(page?.publication))` **or** only toast on composition save / panel save to avoid spam. **Prefer:** toast on panel save, composition save, and batch delete; leave rapid DnD without per-move toast (composition save already batches).
- Add `<PublishDraftBanner />` near top of page content (below title / toolbar).

- [ ] **Step 5: Import-export**

If import mutates draft and shows success toast, switch message through `draftSaveToastMessage(page?.publication)`.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/app/themes/page.tsx web/src/pages/app/widgets/page.tsx \
  web/src/pages/app/links/page.tsx web/src/pages/app/import-export/page.tsx \
  web/src/components/base/SharedUI.tsx
git commit -m "feat: draft-aware save toasts and publish banners on edit pages"
```

---

### Task 8: Verification

**Files:** none new

- [ ] **Step 1: Unit tests**

Run: `cd web && npx vitest run tests/publish-state.test.ts`  
Expected: PASS

- [ ] **Step 2: Frontend check**

Run: `make check`  
Expected: TypeScript + ESLint + mock contract pass

- [ ] **Step 3: Go tests (no backend change, regression only)**

Run: `go test -race ./...`  
Expected: PASS (unchanged)

- [ ] **Step 4: Manual smoke checklist**

1. Never published → edit theme → toast “已保存到草稿” → top bar “未发布” +「发布」→ publish → “发布成功” → “已是最新”
2. Edit link/theme while published → toast mentions 访客仍看线上版 → banner + top bar “有草稿未上线” →「发布更新」→ banner gone
3. Publish page: no toggle; primary button matches state; unpublish confirms
4. Overview: 草稿预览 vs 线上版; stats three-state
5. Preview: publish CTA; live link only if published
6. Set visibility private → publish page primary disabled with hint; top bar publish navigates with `highlight=visibility`

- [ ] **Step 5: Final commit if any fixups**

```bash
git add -A
git status
# commit only intentional fixups
git commit -m "fix: polish publish UX after verification"
```

---

## Spec coverage checklist

| Spec section | Task |
|--------------|------|
| §3 three-state model | Task 1 |
| §4 AppShell control | Task 3 |
| §5 publish page | Task 5 |
| §6 draft toasts + banner P0 | Task 4, 7 |
| §7 overview + preview | Task 6 |
| §9 private visibility / unpublish confirm / highlight | Task 1, 3, 5 |
| §10 tests | Task 1, 8 |
| Non-goals (no auto-publish, no API change) | All tasks respect |

## Type consistency

- `PublishUiStateId`: `never_published` | `published_with_draft` | `published_current`
- `primaryAction`: `publish` | `publish_update` | `none`
- Surfaces: `publish_page` | `toolbar` | `banner` | `overview` | `preview`
- Paths: `publishSettingsPath(scope, highlight?)`, `previewPath(scope)`
- Success toasts: pre-mutation state → `toastForPublishSuccess`
- Draft save: `draftSaveToastMessage(publication, optionalOverride)`
