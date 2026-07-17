// ============================================================
// nav.ax Admin Discover Curation — /admin/discover
// ============================================================

import { useState } from 'react';
import { Loader2, Star } from 'lucide-react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { request } from '@/api/client';
import type { ApiResponse, PaginatedResponse } from '@/api/types';
import { DataTable, type Column } from '@/components/base/DataTable';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';

interface AdminDiscoverItem {
  pageId: string;
  slug: string;
  title: string;
  ownerName: string;
  ownerId: string;
  featured: boolean;
  tags: string[];
  publishedAt: string;
}

export default function AdminDiscoverPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [tagDrafts, setTagDrafts] = useState<Record<string, string>>({});
  const pageSize = 15;
  const qc = useQueryClient();
  const { toast } = useToast();

  const listQuery = useQuery({
    queryKey: ['admin', 'discover', page, search],
    queryFn: async () => {
      const response = await request<ApiResponse<AdminDiscoverItem[] | PaginatedResponse<AdminDiscoverItem>>>('/admin/discover', {
        params: { page, pageSize, search: search || undefined },
      });
      if (Array.isArray(response.data)) {
        const total = response.meta.total ?? response.data.length;
        const resolvedPageSize = response.meta.pageSize ?? pageSize;
        return {
          items: response.data,
          page: response.meta.page ?? page,
          pageSize: resolvedPageSize,
          total,
          totalPages: Math.max(1, Math.ceil(total / resolvedPageSize)),
        } satisfies PaginatedResponse<AdminDiscoverItem>;
      }
      return response.data;
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ pageId, featured, tags }: { pageId: string; featured?: boolean; tags?: string[] }) =>
      request<ApiResponse<AdminDiscoverItem>>(`/admin/discover/${pageId}`, {
        method: 'PATCH',
        body: { featured, tags },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'discover'] });
      toast('success', '发现页设置已更新');
    },
    onError: (cause: Error) => toast('error', cause.message || '更新失败'),
  });

  if (listQuery.isLoading) return <LoadingSkeleton count={5} />;
  if (listQuery.isError) {
    return <ErrorState message={listQuery.error.message || '加载失败'} onRetry={() => listQuery.refetch()} />;
  }

  const items = listQuery.data?.items ?? [];
  const total = listQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const columns: Column<AdminDiscoverItem>[] = [
    {
      key: 'title',
      header: '导航页',
      render: item => (
        <div className="min-w-0">
          <div className="text-sm font-medium text-foreground-900 truncate">{item.title}</div>
          <div className="text-xs text-foreground-400 truncate">/{item.slug} · {item.ownerName}</div>
        </div>
      ),
    },
    {
      key: 'featured',
      header: '精选',
      render: item => (
        <button
          type="button"
          onClick={() => updateMutation.mutate({ pageId: item.pageId, featured: !item.featured })}
          className={`inline-flex items-center gap-1 h-8 px-2.5 rounded-md text-xs font-medium ${
            item.featured ? 'bg-amber-50 text-amber-700' : 'bg-background-100 text-foreground-500'
          }`}
        >
          <Star className={`w-3.5 h-3.5 ${item.featured ? 'fill-amber-500 text-amber-500' : ''}`} />
          {item.featured ? '精选中' : '设为精选'}
        </button>
      ),
    },
    {
      key: 'tags',
      header: '标签',
      render: item => {
        const draft = tagDrafts[item.pageId] ?? item.tags.join(', ');
        return (
          <div className="flex items-center gap-2 min-w-[220px]">
            <input
              value={draft}
              onChange={e => setTagDrafts(prev => ({ ...prev, [item.pageId]: e.target.value }))}
              placeholder="逗号分隔标签"
              className="flex-1 h-8 px-2 rounded-md border border-background-200/70 bg-background-50 text-xs"
            />
            <button
              type="button"
              disabled={updateMutation.isPending}
              onClick={() => {
                const tags = draft.split(/[,，]/).map(tag => tag.trim()).filter(Boolean);
                updateMutation.mutate({ pageId: item.pageId, tags });
              }}
              className="h-8 px-2 rounded-md bg-primary-500 text-background-50 text-xs disabled:opacity-50"
            >
              保存
            </button>
          </div>
        );
      },
    },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold font-heading text-foreground-950">发现页运营</h1>
        <p className="text-xs text-foreground-400 mt-1">设置公开导航的精选状态与标签</p>
      </div>
      {updateMutation.isPending && (
        <div className="mb-3 text-xs text-foreground-400 inline-flex items-center gap-1.5">
          <Loader2 className="w-3.5 h-3.5 animate-spin" />
          保存中…
        </div>
      )}
      <DataTable
        columns={columns}
        data={items}
        keyField="pageId"
        searchPlaceholder="筛选当前页…"
        searchFields={['title', 'slug', 'ownerName']}
        currentPage={page}
        totalPages={totalPages}
        totalItems={total}
        onPageChange={setPage}
        toolbar={(
          <input
            value={search}
            onChange={e => { setSearch(e.target.value); setPage(1); }}
            placeholder="服务端搜索标题/slug/用户"
            className="h-8 px-3 rounded-md border border-background-200/70 bg-background-50 text-xs w-56"
          />
        )}
      />
    </div>
  );
}
