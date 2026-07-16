// ============================================================
// nav.ax Admin Links Page — /admin/links
// ============================================================

import { useState, useCallback, useMemo } from 'react';
import { Trash2, ExternalLink, Search, Filter } from 'lucide-react';
import { useAdminLinks, useDeleteLink } from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import { ConfirmDialog, Badge } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import IconRenderer from '@/components/base/IconRenderer';
import type { AdminLink } from '@/api/types';

export default function AdminLinksPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [ownerFilter, setOwnerFilter] = useState('');
  const pageSize = 15;

  const { data: paginated, isLoading, error, refetch } = useAdminLinks({
    page, pageSize, search: search || undefined, ownerId: ownerFilter || undefined,
  });

  const deleteMutation = useDeleteLink();
  const { toast } = useToast();

  const [deleteTarget, setDeleteTarget] = useState<AdminLink | null>(null);

  const handleDelete = useCallback(() => {
    if (!deleteTarget) return;
    deleteMutation.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast('success', `已删除链接「${deleteTarget.title}」`);
        setDeleteTarget(null);
      },
      onError: () => toast('error', '删除失败，请稍后重试'),
    });
  }, [deleteTarget, deleteMutation, toast]);

  const links = useMemo(() => paginated?.items ?? [], [paginated?.items]);

  // Unique owners for filter dropdown
  const ownerOptions = useMemo(() => {
    const seen = new Set<string>();
    const result: { id: string; name: string }[] = [];
    for (const l of links) {
      if (!seen.has(l.ownerId)) {
        seen.add(l.ownerId);
        result.push({ id: l.ownerId, name: l.ownerName });
      }
    }
    return result;
  }, [links]);

  const columns: Column<AdminLink>[] = [
    {
      key: 'title', header: '链接', sortable: true,
      render: (l) => (
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-7 h-7 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0">
            <IconRenderer icon={l.icon} className="text-sm text-foreground-500" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5">
              <span className="text-sm font-medium text-foreground-900 truncate">{l.title}</span>
              <a
                href={l.url}
                target="_blank"
                rel="noopener noreferrer"
                className="w-4 h-4 flex items-center justify-center flex-shrink-0 text-foreground-300 hover:text-primary-500 transition-colors duration-150"
                aria-label={`打开 ${l.title}`}
                title="打开链接"
              >
                <ExternalLink className="w-3 h-3" />
              </a>
            </div>
            <div className="text-xs text-foreground-400 truncate">{l.url}</div>
          </div>
        </div>
      ),
    },
    {
      key: 'categoryName', header: '分类', sortable: true,
      render: (l) => (
        <Badge variant="default" className="text-[11px]">{l.categoryName}</Badge>
      ),
    },
    {
      key: 'ownerName', header: '所属用户', sortable: true,
      render: (l) => (
        <div className="flex items-center gap-2">
          {l.ownerAvatar ? (
            <img src={l.ownerAvatar} alt="" className="w-5 h-5 rounded-full object-cover flex-shrink-0" />
          ) : (
            <div className="w-5 h-5 rounded-full bg-background-200 flex items-center justify-center flex-shrink-0">
              <i className="ri-building-2-line text-[10px] text-foreground-400" />
            </div>
          )}
          <span className={`text-sm ${l.ownerId === 'system' ? 'text-accent-600 font-medium' : 'text-foreground-600'}`}>
            {l.ownerName}
          </span>
        </div>
      ),
    },
    {
      key: 'createdAt', header: '创建时间', sortable: true,
      render: (l) => (
        <span className="text-xs text-foreground-400 whitespace-nowrap">
          {new Date(l.createdAt).toLocaleDateString('zh-CN', { year: 'numeric', month: 'short', day: 'numeric' })}
        </span>
      ),
    },
    {
      key: 'actions', header: '操作', headerClassName: 'text-right',
      className: 'text-right',
      render: (l) => (
        <div className="flex items-center justify-end gap-0.5">
          <button
            onClick={() => setDeleteTarget(l)}
            className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
            aria-label="删除链接"
            title="删除链接"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      ),
    },
  ];

  const deleteDesc = deleteTarget
    ? `确定要删除链接「${deleteTarget.title}」吗？\n\n链接地址：${deleteTarget.url}\n所有者：${deleteTarget.ownerName}\n\n此操作不可撤销，将从用户的导航页中移除该链接。`
    : '';

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-bold font-heading text-foreground-950">链接管理</h1>
          <p className="text-xs text-foreground-400 mt-0.5">
            管理所有用户创建的链接 · 共 {paginated?.total ?? 0} 条
          </p>
        </div>
      </div>

      <DataTable<AdminLink>
        columns={columns}
        data={links}
        keyField="id"
        isLoading={isLoading}
        error={error ? (error as Error).message : undefined}
        onRetry={() => refetch()}
        searchPlaceholder="搜索链接名称、地址或用户名..."
        searchFields={['title', 'url', 'ownerName']}
        currentPage={page}
        totalPages={paginated?.totalPages}
        totalItems={paginated?.total}
        onPageChange={setPage}
        pageSize={pageSize}
        emptyTitle="暂无链接"
        emptyDescription="用户创建链接后将在此处展示"
        toolbar={
          <select
            value={ownerFilter}
            onChange={e => { setOwnerFilter(e.target.value); setPage(1); }}
            className="h-8 px-2.5 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-600 focus:outline-none focus:border-primary-300"
          >
            <option value="">全部用户</option>
            {ownerOptions.map(o => (
              <option key={o.id} value={o.id}>{o.name}</option>
            ))}
          </select>
        }
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="删除链接"
        description={deleteDesc}
        confirmLabel="确认删除"
        danger
      />
    </div>
  );
}
