// ============================================================
// nav.ax Admin Users Page — /admin/users
// ============================================================

import { useState, useCallback } from 'react';
import { Ban, RefreshCw, Shield, UserX, KeyRound, Copy, Check, MailCheck } from 'lucide-react';
import { useAdminUsers, useDisableUser, useEnableUser, useRevokeUserSessions, useResetUserPassword } from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import { ConfirmDialog, Badge } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { User, PasswordResetLink } from '@/api/types';

export default function AdminUsersPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const pageSize = 12;

  const { data: paginated, isLoading, error, refetch } = useAdminUsers({ page, pageSize });

  const disableMutation = useDisableUser();
  const enableMutation = useEnableUser();
  const revokeMutation = useRevokeUserSessions();
  const resetMutation = useResetUserPassword();
  const { toast } = useToast();

  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [actionType, setActionType] = useState<'disable' | 'enable' | 'revoke' | 'reset' | null>(null);
  const [resetResult, setResetResult] = useState<PasswordResetLink | null>(null);
  const [copied, setCopied] = useState(false);

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
    } else if (actionType === 'reset') {
      const username = selectedUser.username;
      resetMutation.mutate(selectedUser.id, {
        onSuccess: (res) => {
          setResetResult(res.data);
          setCopied(false);
          toast('success', res.data.emailSent ? `已向 ${username} 发送重置邮件` : `已生成 ${username} 的重置链接`);
        },
        onError: () => toast('error', '生成重置链接失败'),
      });
    }
    setSelectedUser(null);
    setActionType(null);
  }, [selectedUser, actionType, disableMutation, enableMutation, revokeMutation, resetMutation, toast]);

  const copyResetLink = useCallback(async () => {
    if (!resetResult) return;
    try {
      await navigator.clipboard.writeText(resetResult.resetUrl);
      setCopied(true);
    } catch {
      toast('error', '复制失败，请手动选择链接');
    }
  }, [resetResult, toast]);

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
          {u.role !== 'admin' && (
            <button
              onClick={() => { setSelectedUser(u); setActionType('reset'); }}
              className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
              aria-label="生成重置链接"
              title="生成密码重置链接"
            >
              <KeyRound className="w-3.5 h-3.5" />
            </button>
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
  const resetMsg = selectedUser
    ? `确定要为「${selectedUser.username}」生成密码重置链接吗？\n\n影响范围：\n• 该用户此前的重置链接将立即失效\n• 若已配置邮件服务，链接会发送到其邮箱\n• 未配置邮件时，链接将显示给你，请通过安全渠道转交`
    : '';

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
        title={
          actionType === 'disable' ? '禁用用户'
          : actionType === 'enable' ? '启用用户'
          : actionType === 'reset' ? '生成密码重置链接'
          : '撤销全部会话'
        }
        description={
          actionType === 'disable' ? disableMsg
          : actionType === 'enable' ? enableMsg
          : actionType === 'reset' ? resetMsg
          : revokeMsg
        }
        confirmLabel={
          actionType === 'disable' ? '确认禁用'
          : actionType === 'enable' ? '确认启用'
          : actionType === 'reset' ? '生成链接'
          : '确认撤销'
        }
        danger={actionType === 'disable' || actionType === 'revoke'}
      />

      {resetResult && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center">
          <div className="absolute inset-0 bg-black/30" onClick={() => setResetResult(null)} />
          <div className="relative bg-background-50 rounded-xl shadow-overlay p-6 w-full max-w-md mx-4">
            <h3 className="text-lg font-semibold text-foreground-900 mb-2">密码重置链接已生成</h3>
            <p className={cn('text-sm mb-4 flex items-center gap-1.5', resetResult.emailSent ? 'text-green-600' : 'text-foreground-500')}>
              {resetResult.emailSent
                ? (<><MailCheck className="w-4 h-4" />已通过邮件发送给用户</>)
                : '未配置邮件服务，请复制以下链接并通过安全渠道转交用户：'}
            </p>
            <div className="flex items-center gap-2 mb-2">
              <input
                readOnly
                value={resetResult.resetUrl}
                onFocus={e => e.currentTarget.select()}
                className="flex-1 h-9 px-3 rounded-lg bg-background-100 border border-background-200/70 text-xs text-foreground-700 font-mono focus:outline-none focus:border-primary-300"
              />
              <button
                onClick={copyResetLink}
                className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center gap-1.5 whitespace-nowrap"
              >
                {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                {copied ? '已复制' : '复制'}
              </button>
            </div>
            <p className="text-xs text-foreground-300 mb-6">链接将在 1 小时后失效，且只能使用一次。</p>
            <div className="flex items-center justify-end">
              <button
                onClick={() => setResetResult(null)}
                className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
