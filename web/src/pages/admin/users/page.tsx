// ============================================================
// nav.ax Admin Users Page — /admin/users
// ============================================================

import { useState, useCallback } from 'react';
import { Ban, RefreshCw, Shield, UserX } from 'lucide-react';
import { useAdminUsers, useDisableUser, useEnableUser, useRevokeUserSessions } from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import { ConfirmDialog, Badge } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { User } from '@/api/types';

export default function AdminUsersPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const pageSize = 12;

  const { data: paginated, isLoading, error, refetch } = useAdminUsers({ page, pageSize });

  const disableMutation = useDisableUser();
  const enableMutation = useEnableUser();
  const revokeMutation = useRevokeUserSessions();
  const { toast } = useToast();

  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [actionType, setActionType] = useState<'disable' | 'enable' | 'revoke' | null>(null);

  const handleAction = useCallback(() => {
    if (!selectedUser || !actionType) return;
    if (actionType === 'disable') {
      disableMutation.mutate(selectedUser.id, {
        onSuccess: () => toast('success', `已禁用用户 ${selectedUser.username}`),
        onError: () => toast('error', '操作失败'),
      });
    } else if (actionType === 'enable') {
      enableMutation.mutate(selectedUser.id, {
        onSuccess: () => toast('success', `已启用用户 ${selectedUser.username}`),
        onError: () => toast('error', '操作失败'),
      });
    } else if (actionType === 'revoke') {
      revokeMutation.mutate(selectedUser.id, {
        onSuccess: () => toast('success', `已撤销 ${selectedUser.username} 的所有活动会话`),
        onError: () => toast('error', '操作失败'),
      });
    }
    setSelectedUser(null);
    setActionType(null);
  }, [selectedUser, actionType, disableMutation, enableMutation, revokeMutation, toast]);

  const users = paginated?.items || [];

  const columns: Column<User>[] = [
    {
      key: 'user', header: '用户', sortable: true,
      render: (u) => (
        <div className="flex items-center gap-2.5">
          {u.avatarUrl ? (
            <img src={u.avatarUrl} alt="" className="w-7 h-7 rounded-full object-cover" />
          ) : (
            <div className="w-7 h-7 rounded-full bg-background-200 flex items-center justify-center text-xs font-medium text-foreground-400">
              {u.username[0]?.toUpperCase()}
            </div>
          )}
          <span className="text-sm font-medium text-foreground-900">{u.username}</span>
        </div>
      ),
    },
    {
      key: 'email', header: '邮箱', sortable: true,
      render: (u) => <span className="text-sm text-foreground-500">{u.email}</span>,
    },
    {
      key: 'role', header: '角色',
      render: (u) => (
        u.role === 'admin'
          ? <div className="flex items-center gap-1 text-xs text-primary-600 font-medium"><Shield className="w-3 h-3" />管理员</div>
          : <span className="text-xs text-foreground-400">用户</span>
      ),
    },
    {
      key: 'status', header: '状态',
      render: (u) => (
        <Badge variant={u.status === 'active' ? 'success' : 'danger'} className="text-[11px]">
          {u.status === 'active' ? '正常' : '已禁用'}
        </Badge>
      ),
    },
    {
      key: 'createdAt', header: '注册时间', sortable: true,
      render: (u) => <span className="text-xs text-foreground-400">{new Date(u.createdAt).toLocaleDateString('zh-CN')}</span>,
    },
    {
      key: 'actions', header: '操作', headerClassName: 'text-right',
      className: 'text-right',
      render: (u) => (
        <div className="flex items-center justify-end gap-0.5">
          {u.role !== 'admin' && u.status === 'active' && (
            <>
              <button
                onClick={() => { setSelectedUser(u); setActionType('disable'); }}
                className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
                aria-label="禁用用户"
                title="禁用用户"
              >
                <Ban className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={() => { setSelectedUser(u); setActionType('revoke'); }}
                className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-accent-500 hover:bg-accent-50 transition-colors duration-150"
                aria-label="撤销会话"
                title="撤销全部会话"
              >
                <UserX className="w-3.5 h-3.5" />
              </button>
            </>
          )}
          {u.role !== 'admin' && u.status === 'disabled' && (
            <button
              onClick={() => { setSelectedUser(u); setActionType('enable'); }}
              className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-green-500 hover:bg-green-50 transition-colors duration-150"
              aria-label="启用用户"
              title="启用用户"
            >
              <RefreshCw className="w-3.5 h-3.5" />
            </button>
          )}
          {u.role === 'admin' && (
            <span className="text-xs text-foreground-300">—</span>
          )}
        </div>
      ),
    },
  ];

  const disableMsg = selectedUser
    ? `确定要禁用用户「${selectedUser.username}」吗？\n\n影响范围：\n• 该用户将无法登录\n• 已登录的会话将立即失效\n• 已发布的公开页面将暂时隐藏\n• 该操作不会删除用户数据`
    : '';
  const revokeMsg = selectedUser
    ? `确定要撤销「${selectedUser.username}」的所有活动会话吗？\n\n影响范围：\n• 该用户在全部设备上将被登出\n• 需重新登录才能访问\n• 未保存的草稿可能丢失`
    : '';
  const enableMsg = selectedUser ? `确定要启用用户「${selectedUser.username}」吗？该用户将恢复登录权限。` : '';

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-xl font-bold font-heading text-foreground-950">用户管理</h1>
        <p className="text-xs text-foreground-400 mt-0.5">查看、禁用或启用平台用户 · 共 {paginated?.total ?? 0} 个账号</p>
      </div>

      <DataTable<User>
        columns={columns}
        data={users}
        keyField="id"
        isLoading={isLoading}
        error={error ? (error as Error).message : undefined}
        onRetry={() => refetch()}
        searchPlaceholder="搜索用户名或邮箱..."
        searchFields={['username', 'email']}
        currentPage={page}
        totalPages={paginated?.totalPages}
        totalItems={paginated?.total}
        onPageChange={setPage}
        pageSize={pageSize}
        emptyTitle="没有找到匹配的用户"
      />

      <ConfirmDialog
        open={!!actionType && !!selectedUser}
        onClose={() => { setSelectedUser(null); setActionType(null); }}
        onConfirm={handleAction}
        title={actionType === 'disable' ? '禁用用户' : actionType === 'enable' ? '启用用户' : '撤销全部会话'}
        description={
          actionType === 'disable' ? disableMsg
          : actionType === 'enable' ? enableMsg
          : revokeMsg
        }
        confirmLabel={actionType === 'disable' ? '确认禁用' : actionType === 'enable' ? '确认启用' : '确认撤销'}
        danger={actionType === 'disable' || actionType === 'revoke'}
      />
    </div>
  );
}
