// ============================================================
// Floating quick-add for logged-in users on public navigation pages.
// ============================================================

import { useMemo, useState } from 'react';
import { Plus } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { AddSiteDialog } from '@/components/base/AddDialogs';
import { useToast } from '@/components/base/Toast';
import { useCurrentUser, useMyPage } from '@/hooks/useQueries';
import { navigationApi } from '@/api/navigation';
import { draftSaveToastMessage } from '@/lib/publish-state';
import { cn } from '@/lib/utils';
import type { PageKind } from '@/api/types';

export default function QuickAddSiteFab({ className }: { className?: string }) {
  const { data: auth, isLoading: authLoading } = useCurrentUser();
  const isAdmin = auth?.user?.role === 'admin';
  // Public home is the system page for admins; personal for members.
  const scope: PageKind = isAdmin ? 'system' : 'personal';
  const { data: page } = useMyPage(scope);
  const qc = useQueryClient();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const authenticated = Boolean(auth?.authenticated && auth.user);
  const categories = useMemo(() => page?.categories ?? [], [page?.categories]);

  if (authLoading || !authenticated || !page || categories.length === 0) {
    return null;
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        disabled={busy}
        className={cn(
          'fixed bottom-6 right-6 z-40 h-12 w-12 rounded-full bg-primary-500 text-background-50',
          'shadow-overlay hover:bg-primary-600 transition-colors duration-150',
          'flex items-center justify-center focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50',
          'disabled:opacity-50',
          className,
        )}
        aria-label="快速添加站点"
        title="快速添加站点"
      >
        <Plus className="w-5 h-5" />
      </button>
      <AddSiteDialog
        open={open}
        onClose={() => setOpen(false)}
        categories={categories}
        onConfirm={async (data) => {
          setBusy(true);
          try {
            await navigationApi.forPage(page.id).createSite({
              categoryId: data.categoryId,
              title: data.title,
              url: data.url,
              icon: data.icon,
              description: data.description,
            });
            void qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] });
            toast(
              'success',
              draftSaveToastMessage(
                page.publication,
                `已添加「${data.title}」· 请到工作台发布后访客可见`,
              ),
            );
          } catch (cause) {
            toast('error', cause instanceof Error ? cause.message : '添加失败');
          } finally {
            setBusy(false);
          }
        }}
      />
    </>
  );
}
