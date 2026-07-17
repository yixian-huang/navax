import type { NavigateFunction } from 'react-router-dom';
import { ApiError } from '@/api/client';
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

export function handlePublishError(
  error: unknown,
  opts: {
    toast: (type: 'error', message: string) => void;
    refetch?: () => void;
    navigateToVisibilityFix?: () => void;
  },
): void {
  const message = error instanceof Error ? error.message : '发布失败';
  if (error instanceof ApiError && error.status === 409) {
    opts.toast('error', '内容已变更，请刷新后重试');
    opts.refetch?.();
    return;
  }
  opts.toast('error', message || '发布失败');
  if (/visibility|private|可见性|私密/i.test(message)) {
    opts.navigateToVisibilityFix?.();
  }
}
