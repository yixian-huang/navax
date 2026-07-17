// ============================================================
// Publish status + primary CTA for AppShell header toolbar
// ============================================================

import { Link, useNavigate } from 'react-router-dom';
import { Globe, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToast } from '@/components/base/Toast';
import { usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import {
  handlePublishError,
  navigateToVisibilityFix,
  publishSettingsPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';

function shortLabelClass(stateId: string): string {
  if (stateId === 'published_with_draft') return 'text-accent-600';
  if (stateId === 'published_current') return 'text-green-600';
  return 'text-foreground-400';
}

export default function PublishStatusControl() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const { state, scope, isLoading, refetch } = usePublishUiState('toolbar');
  const { mutate: publishMutation, isPending: publishing } = usePublish();

  if (isLoading) {
    return (
      <div className="hidden sm:inline-flex items-center gap-2">
        <Loader2 className="w-3.5 h-3.5 animate-spin text-foreground-400" />
      </div>
    );
  }

  const handlePrimary = () => {
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
    <div className="hidden sm:inline-flex items-center gap-2">
      <span
        className={cn(
          'text-xs font-medium whitespace-nowrap',
          shortLabelClass(state.id),
        )}
      >
        {state.shortLabel}
      </span>

      {state.primaryAction !== 'none' && (
        <button
          type="button"
          onClick={handlePrimary}
          disabled={publishing || isLoading || state.primaryDisabled}
          className="inline-flex items-center gap-1.5 h-7 px-3 rounded-md bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap"
        >
          {publishing ? (
            <Loader2 className="w-3 h-3 animate-spin" />
          ) : (
            <Globe className="w-3 h-3" />
          )}
          {publishing ? '发布中…' : state.primaryLabel}
        </button>
      )}

      <Link
        to={publishSettingsPath(scope)}
        className="text-xs text-foreground-500 hover:text-foreground-700 transition-colors duration-150 whitespace-nowrap"
      >
        发布设置
      </Link>
    </div>
  );
}
