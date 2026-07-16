// ============================================================
// nav.ax Admin Invitations Page — /admin/invitations
// ============================================================

import { useState, useCallback } from 'react';
import { Plus, Copy, XCircle, Clock, Users } from 'lucide-react';
import { useAdminInvitations, useCreateInvitation, useRevokeInvitation } from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import { ConfirmDialog, Badge } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import type { Invitation } from '@/api/types';

export default function AdminInvitationsPage() {
  const [page, setPage] = useState(1);
  const pageSize = 12;

  const { data: paginated, isLoading, error, refetch } = useAdminInvitations({ page, pageSize });
  const createMutation = useCreateInvitation();
  const revokeMutation = useRevokeInvitation();
  const { toast } = useToast();

  const [showCreate, setShowCreate] = useState(false);
  const [maxUses, setMaxUses] = useState(10);
  const [expiresDays, setExpiresDays] = useState(30);
  const [revokeTarget, setRevokeTarget] = useState<Invitation | null>(null);

  const handleCreate = useCallback(() => {
    createMutation.mutate({ maxUses, expiresInDays: expiresDays }, {
      onSuccess: () => {
        toast('success', '邀请链接已创建');
        setShowCreate(false);
      },
      onError: () => toast('error', '创建失败'),
    });
  }, [maxUses, expiresDays, createMutation, toast]);

  const handleRevoke = useCallback(() => {
    if (!revokeTarget) return;
    revokeMutation.mutate(revokeTarget.id, {
      onSuccess: () => toast('success', '邀请链接已撤销'),
      onError: () => toast('error', '撤销失败'),
    });
    setRevokeTarget(null);
  }, [revokeTarget, revokeMutation, toast]);

  const handleCopy = useCallback((code: string) => {
    const url = `${window.location.origin}/invite/${code}`;
    navigator.clipboard.writeText(url);
    toast('info', '邀请链接已复制到剪贴板');
  }, [toast]);

  const invitations = paginated?.items || [];

  const getStatus = (inv: Invitation) => {
    if (inv.isRevoked) return { label: '已撤销', variant: 'danger' as const };
    if (inv.usedCount >= inv.maxUses) return { label: '已用完', variant: 'warning' as const };
    if (new Date(inv.expiresAt) < new Date()) return { label: '已过期', variant: 'default' as const };
    return { label: '有效', variant: 'success' as const };
  };

  const columns: Column<Invitation>[] = [
    {
      key: 'code', header: '邀请码', sortable: true,
      render: (inv) => <span className="text-sm font-mono font-medium text-foreground-900">{inv.code}</span>,
    },
    {
      key: 'usage', header: '使用情况',
      render: (inv) => (
        <div className="flex items-center gap-1.5 text-sm">
          <Users className="w-3 h-3 text-foreground-300" />
          <span className="text-foreground-600">{inv.usedCount} / {inv.maxUses}</span>
          {inv.usedCount >= inv.maxUses && <span className="text-xs text-foreground-300">(已满)</span>}
        </div>
      ),
    },
    {
      key: 'expiresAt', header: '有效期', sortable: true,
      render: (inv) => (
        <div className="flex items-center gap-1.5 text-sm">
          <Clock className="w-3 h-3 text-foreground-300" />
          <span className="text-foreground-500">{new Date(inv.expiresAt).toLocaleDateString('zh-CN')}</span>
        </div>
      ),
    },
    {
      key: 'status', header: '状态',
      render: (inv) => {
        const s = getStatus(inv);
        return <Badge variant={s.variant} className="text-[11px]">{s.label}</Badge>;
      },
    },
    {
      key: 'createdAt', header: '创建时间', sortable: true,
      render: (inv) => <span className="text-xs text-foreground-400">{new Date(inv.createdAt).toLocaleDateString('zh-CN')}</span>,
    },
    {
      key: 'actions', header: '操作', headerClassName: 'text-right',
      className: 'text-right',
      render: (inv) => (
        <div className="flex items-center justify-end gap-0.5">
          <button
            onClick={() => handleCopy(inv.code)}
            className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
            aria-label="复制邀请链接"
            title="复制邀请链接"
          >
            <Copy className="w-3.5 h-3.5" />
          </button>
          {!inv.isRevoked && new Date(inv.expiresAt) > new Date() && (
            <button
              onClick={() => setRevokeTarget(inv)}
              className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
              aria-label="撤销邀请"
              title="撤销邀请"
            >
              <XCircle className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      ),
    },
  ];

  const revokeDesc = revokeTarget
    ? `确定要撤销邀请「${revokeTarget.code}」吗？\n\n影响范围：\n• 该邀请链接将立即失效\n• 已通过该邀请注册的用户不受影响\n• 未使用的剩余次数将被废弃\n• 此操作不可撤销`
    : '';

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-bold font-heading text-foreground-950">邀请管理</h1>
          <p className="text-xs text-foreground-400 mt-0.5">创建、复制和撤销注册邀请链接</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="h-8 px-3.5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center gap-1.5 whitespace-nowrap"
        >
          <Plus className="w-3.5 h-3.5" />
          创建邀请
        </button>
      </div>

      <DataTable<Invitation>
        columns={columns}
        data={invitations}
        keyField="id"
        isLoading={isLoading}
        error={error ? (error as Error).message : undefined}
        onRetry={() => refetch()}
        searchPlaceholder="搜索邀请码..."
        searchFields={['code']}
        currentPage={page}
        totalPages={paginated?.totalPages}
        totalItems={paginated?.total}
        onPageChange={setPage}
        pageSize={pageSize}
        emptyTitle="还没有邀请链接"
        emptyDescription="创建第一个邀请链接来邀请新用户"
      />

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center">
          <div className="absolute inset-0 bg-black/30" onClick={() => setShowCreate(false)} />
          <div className="relative bg-white rounded-xl shadow-overlay p-5 w-full max-w-sm mx-4">
            <h3 className="text-base font-semibold text-foreground-900 mb-4">创建邀请链接</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-xs text-foreground-500 mb-1.5">最大使用次数</label>
                <input
                  type="number"
                  value={maxUses}
                  onChange={e => setMaxUses(Number(e.target.value))}
                  min={1}
                  max={100}
                  className="w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300"
                />
                <p className="text-xs text-foreground-300 mt-1">达到上限后邀请链接自动失效</p>
              </div>
              <div>
                <label className="block text-xs text-foreground-500 mb-1.5">有效期（天）</label>
                <input
                  type="number"
                  value={expiresDays}
                  onChange={e => setExpiresDays(Number(e.target.value))}
                  min={1}
                  max={365}
                  className="w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300"
                />
                <p className="text-xs text-foreground-300 mt-1">超过有效期后自动失效</p>
              </div>
            </div>
            <div className="flex items-center justify-end gap-2.5 mt-5">
              <button
                onClick={() => setShowCreate(false)}
                className="h-8 px-3.5 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                disabled={createMutation.isPending}
                className="h-8 px-3.5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 transition-colors duration-150 flex items-center gap-1.5 whitespace-nowrap"
              >
                {createMutation.isPending ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleRevoke}
        title="撤销邀请链接"
        description={revokeDesc}
        confirmLabel="确认撤销"
        danger
      />
    </div>
  );
}
