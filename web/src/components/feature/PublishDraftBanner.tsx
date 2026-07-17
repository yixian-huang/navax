// ============================================================
// Dismissible banner when published page has unpublished draft
// ============================================================

import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { X, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToast } from '@/components/base/Toast';
import { usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import {
  handlePublishError,
  navigateToVisibilityFix,
  previewPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';

function dismissKey(scope: string, pageId: string) {
  return `navax:publish-banner-dismissed:${scope}:${pageId}`;
}

export default function PublishDraftBanner({ className }: { className?: string }) {
  const navigate = useNavigate();
  const { toast } = useToast();
  const { state, scope, pageId, refetch } = usePublishUiState('banner');
  const { mutate: publishMutation, isPending: publishing } = usePublish();
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
    if (intent === 'noop') return;
    if (intent === 'redirect_visibility') {
      navigateToVisibilityFix(navigate, scope);
      return;
    }

    const stateBefore = state;
    publishMutation(undefined, {
      onSuccess: () => {
        toast('success', toastForPublishSuccess(stateBefore));
      },
      onError: (error: Error) => {
        handlePublishError(error, {
          toast,
          refetch: () => { void refetch(); },
          navigateToVisibilityFix: () => navigateToVisibilityFix(navigate, scope),
        });
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
          disabled={publishing}
          className="h-8 px-3 rounded-md bg-accent-600 text-background-50 text-xs font-medium hover:bg-accent-700 disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center gap-1.5 transition-colors duration-150"
        >
          {publishing && <Loader2 className="w-3 h-3 animate-spin" />}
          发布更新
        </button>
        <Link
          to={previewPath(scope)}
          className="h-8 px-3 rounded-md border border-accent-200 text-xs font-medium text-accent-800 hover:bg-accent-100 inline-flex items-center transition-colors duration-150"
        >
          草稿预览
        </Link>
        <button
          type="button"
          onClick={onDismiss}
          className="w-8 h-8 inline-flex items-center justify-center rounded-md text-accent-600 hover:bg-accent-100 transition-colors duration-150"
          aria-label="关闭提示"
        >
          <X className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}
