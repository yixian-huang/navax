import { useMemo, useState } from 'react';
import { Edit2, Eye, EyeOff, Loader2, Plus, Trash2, X } from 'lucide-react';
import {
  useAdminDirectoryCategories,
  useAdminDirectorySites,
  useCreateDirectorySite,
  useDeleteDirectorySite,
  useToggleDirectorySite,
  useUpdateDirectorySite,
} from '@/hooks/useQueries';
import { DataTable, type Column } from '@/components/base/DataTable';
import { Badge, ConfirmDialog } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import IconRenderer from '@/components/base/IconRenderer';
import type { CreatePlatformSiteRequest, PlatformSite } from '@/api/types';

const emptySite: CreatePlatformSiteRequest = {
  categoryId: '',
  title: '',
  url: 'https://',
  icon: 'ri-link',
  description: '',
  enabled: true,
};
const iconButtonClass = 'w-8 h-8 rounded-md inline-flex items-center justify-center text-foreground-400 hover:text-primary-600 hover:bg-primary-50 disabled:opacity-50';
const inputClass = 'mt-1 w-full h-10 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300';

export default function AdminDirectoryPage() {
  const [page, setPage] = useState(1);
  const [categoryFilter, setCategoryFilter] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<PlatformSite | null>(null);
  const [draft, setDraft] = useState<CreatePlatformSiteRequest>(emptySite);
  const [deleteTarget, setDeleteTarget] = useState<PlatformSite | null>(null);
  const pageSize = 12;

  const sitesQuery = useAdminDirectorySites({ page, pageSize, categoryId: categoryFilter || undefined });
  const categoriesQuery = useAdminDirectoryCategories();
  const createSite = useCreateDirectorySite();
  const updateSite = useUpdateDirectorySite();
  const toggleSite = useToggleDirectorySite();
  const deleteSite = useDeleteDirectorySite();
  const { toast } = useToast();

  const categories = categoriesQuery.data ?? [];
  const sites = useMemo(() => sitesQuery.data?.items ?? [], [sitesQuery.data?.items]);

  const openCreate = () => {
    setEditing(null);
    setDraft({ ...emptySite, categoryId: categories.find(category => category.enabled)?.id ?? categories[0]?.id ?? '' });
    setEditorOpen(true);
  };

  const openEdit = (site: PlatformSite) => {
    setEditing(site);
    setDraft({
      categoryId: site.categoryId,
      title: site.title,
      url: site.url,
      icon: site.icon,
      description: site.description,
      enabled: site.enabled,
    });
    setEditorOpen(true);
  };

  const saveSite = () => {
    const options = {
      onSuccess: () => {
        setEditorOpen(false);
        toast('success', editing ? '推荐站点已更新' : '推荐站点已创建');
      },
      onError: error => toast('error', error.message || '保存推荐站点失败'),
    };
    if (editing) updateSite.mutate({ id: editing.id, data: draft }, options);
    else createSite.mutate(draft, options);
  };

  const handleToggle = (site: PlatformSite) => {
    toggleSite.mutate({ id: site.id, enabled: !site.enabled }, {
      onSuccess: () => toast('success', site.enabled ? '站点已停用' : '站点已启用'),
      onError: error => toast('error', error.message || '更新站点状态失败'),
    });
  };

  const handleDelete = () => {
    if (!deleteTarget) return;
    deleteSite.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast('success', `已删除推荐站点「${deleteTarget.title}」`);
        setDeleteTarget(null);
      },
      onError: error => toast('error', error.message || '删除推荐站点失败'),
    });
  };

  const columns: Column<PlatformSite>[] = [
    {
      key: 'title', header: '站点', sortable: true,
      render: site => (
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-8 h-8 rounded-md bg-background-100 flex items-center justify-center"><IconRenderer icon={site.icon} /></div>
          <div className="min-w-0"><div className="text-sm font-medium truncate">{site.title}</div><div className="text-xs text-foreground-400 truncate">{site.url}</div></div>
        </div>
      ),
    },
    { key: 'categoryName', header: '分类', sortable: true, render: site => <span className="text-sm text-foreground-500">{site.categoryName}</span> },
    { key: 'enabled', header: '状态', render: site => <Badge variant={site.enabled ? 'success' : 'default'}>{site.enabled ? '已启用' : '已停用'}</Badge> },
    {
      key: 'actions', header: '操作', className: 'text-right', headerClassName: 'text-right',
      render: site => (
        <div className="flex justify-end gap-1">
          <button onClick={() => handleToggle(site)} disabled={toggleSite.isPending} className={iconButtonClass} aria-label={site.enabled ? '停用站点' : '启用站点'}>
            {site.enabled ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
          </button>
          <button onClick={() => openEdit(site)} className={iconButtonClass} aria-label="编辑站点"><Edit2 className="w-4 h-4" /></button>
          <button onClick={() => setDeleteTarget(site)} className={`${iconButtonClass} hover:text-red-500`} aria-label="删除站点"><Trash2 className="w-4 h-4" /></button>
        </div>
      ),
    },
  ];

  const saving = createSite.isPending || updateSite.isPending;

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-bold font-heading text-foreground-950">推荐站点库</h1>
          <p className="text-xs text-foreground-400 mt-0.5">管理平台公共推荐站点 · 共 {sitesQuery.data?.total ?? 0} 个站点</p>
        </div>
        <button onClick={openCreate} disabled={categories.length === 0} className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center gap-2">
          <Plus className="w-4 h-4" />添加站点
        </button>
      </div>

      <DataTable<PlatformSite>
        columns={columns}
        data={sites}
        keyField="id"
        isLoading={sitesQuery.isLoading}
        error={sitesQuery.error?.message}
        onRetry={() => sitesQuery.refetch()}
        searchPlaceholder="搜索当前页站点..."
        searchFields={['title', 'url']}
        currentPage={page}
        totalPages={sitesQuery.data?.totalPages}
        totalItems={sitesQuery.data?.total}
        onPageChange={setPage}
        pageSize={pageSize}
        emptyTitle="还没有推荐站点"
        emptyDescription={categories.length === 0 ? '请先创建公共分类' : '添加站点到推荐库供用户浏览'}
        toolbar={
          <select value={categoryFilter} onChange={event => { setCategoryFilter(event.target.value); setPage(1); }} className="h-8 px-2.5 rounded-md bg-background-50 border border-background-200/70 text-xs">
            <option value="">全部分类</option>
            {categories.map(category => <option key={category.id} value={category.id}>{category.name}</option>)}
          </select>
        }
      />

      {editorOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center">
          <button className="absolute inset-0 bg-black/30" onClick={() => setEditorOpen(false)} aria-label="关闭" />
          <div className="relative w-full max-w-lg mx-4 rounded-xl bg-white p-5 shadow-overlay space-y-3">
            <div className="flex items-center justify-between"><h2 className="font-semibold">{editing ? '编辑推荐站点' : '添加推荐站点'}</h2><button onClick={() => setEditorOpen(false)} className={iconButtonClass}><X className="w-4 h-4" /></button></div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label="标题"><input value={draft.title} onChange={event => setDraft({ ...draft, title: event.target.value })} maxLength={100} className={inputClass} /></Field>
              <Field label="分类"><select value={draft.categoryId} onChange={event => setDraft({ ...draft, categoryId: event.target.value })} className={inputClass}>{categories.map(category => <option key={category.id} value={category.id}>{category.name}</option>)}</select></Field>
            </div>
            <Field label="网址"><input type="url" value={draft.url} onChange={event => setDraft({ ...draft, url: event.target.value })} className={inputClass} /></Field>
            <Field label="图标"><input value={draft.icon} onChange={event => setDraft({ ...draft, icon: event.target.value })} maxLength={2048} className={inputClass} /></Field>
            <Field label="描述"><textarea value={draft.description} onChange={event => setDraft({ ...draft, description: event.target.value })} maxLength={300} rows={3} className={`${inputClass} h-auto py-2 resize-none`} /></Field>
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={draft.enabled} onChange={event => setDraft({ ...draft, enabled: event.target.checked })} />启用站点</label>
            <button onClick={saveSite} disabled={saving || !draft.categoryId || !draft.title.trim() || !draft.url.startsWith('http')} className="w-full h-10 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center justify-center gap-2">
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}保存
            </button>
          </div>
        </div>
      )}

      <ConfirmDialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)} onConfirm={handleDelete} title="删除推荐站点" description={`确定删除推荐站点「${deleteTarget?.title ?? ''}」吗？已添加到用户主页的站点不受影响。`} confirmLabel="确认删除" danger />
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block text-xs text-foreground-500">{label}{children}</label>;
}
