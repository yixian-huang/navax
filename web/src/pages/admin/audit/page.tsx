// ============================================================
// nav.ax Admin Audit Page — /admin/audit
// ============================================================

import { useState, useMemo } from 'react';
import { useAdminAudit } from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import type { AuditEntry } from '@/api/types';

const actionLabels: Record<string, string> = {
  'user.disable': '禁用用户',
  'user.enable': '启用用户',
  'user.session.revoke': '撤销会话',
  'page.publish': '发布页面',
  'page.create': '创建页面',
  'page.delete': '删除页面',
  'invitation.create': '创建邀请',
  'invitation.revoke': '撤销邀请',
  'site.add': '添加站点',
  'site.remove': '删除站点',
  'theme.change': '切换主题',
  'directory.add': '添加推荐',
  'directory.remove': '删除推荐',
  'system.update': '系统更新',
};

export default function AdminAuditPage() {
  const [page, setPage] = useState(1);
  const [actionFilter, setActionFilter] = useState('');
  const pageSize = 20;

  const { data: paginated, isLoading, error, refetch } = useAdminAudit({ page, pageSize, action: actionFilter || undefined });

  const logs = paginated?.items || [];

  const allActions = useMemo(() => {
    if (!paginated?.items) return [];
    return [...new Set(paginated.items.map(l => l.action))];
  }, [paginated?.items]);

  const columns: Column<AuditEntry>[] = [
    {
      key: 'createdAt', header: '时间', sortable: true,
      className: 'whitespace-nowrap',
      render: (e) => (
        <span className="text-xs text-foreground-400">
          {new Date(e.createdAt).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })}
        </span>
      ),
    },
    {
      key: 'actor', header: '操作人', sortable: true,
      render: (e) => <span className="text-sm font-medium text-foreground-700">{e.actor}</span>,
    },
    {
      key: 'action', header: '操作类型',
      render: (e) => (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-background-100 text-foreground-500 font-mono whitespace-nowrap">
          {actionLabels[e.action] || e.action}
        </span>
      ),
    },
    {
      key: 'detail', header: '详情',
      render: (e) => <span className="text-sm text-foreground-600">{e.detail}</span>,
    },
    {
      key: 'target', header: '操作对象',
      render: (e) => <span className="text-xs text-foreground-400 font-mono">{e.target}</span>,
    },
  ];

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-xl font-bold font-heading text-foreground-950">操作审计</h1>
        <p className="text-xs text-foreground-400 mt-0.5">关键操作记录和追溯 · 共 {paginated?.total ?? 0} 条记录</p>
      </div>

      <DataTable<AuditEntry>
        columns={columns}
        data={logs}
        keyField="id"
        isLoading={isLoading}
        error={error ? (error as Error).message : undefined}
        onRetry={() => refetch()}
        searchPlaceholder="搜索操作人或详情..."
        searchFields={['actor', 'detail']}
        currentPage={page}
        totalPages={paginated?.totalPages}
        totalItems={paginated?.total}
        onPageChange={setPage}
        pageSize={pageSize}
        emptyTitle="没有操作记录"
        toolbar={
          <select
            value={actionFilter}
            onChange={e => { setActionFilter(e.target.value); setPage(1); }}
            className="h-8 px-2.5 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-600 focus:outline-none focus:border-primary-300"
          >
            <option value="">全部操作</option>
            {allActions.map(a => (
              <option key={a} value={a}>{actionLabels[a] || a}</option>
            ))}
          </select>
        }
      />
    </div>
  );
}
