import { useState } from 'react';
import { Edit2, Eye, EyeOff, Loader2, Plus, Trash2, X } from 'lucide-react';
import {
  useAdminDirectoryCategories,
  useCreateDirectoryCategory,
  useDeleteDirectoryCategory,
  useUpdateDirectoryCategory,
} from '@/hooks/useQueries';
import { ConfirmDialog, ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import IconRenderer from '@/components/base/IconRenderer';
import type { DirectoryCategoryInput, PlatformCategory } from '@/api/types';

const emptyCategory: DirectoryCategoryInput = { name: '', icon: 'ri-folder-line', enabled: true };
const iconButtonClass = 'w-8 h-8 rounded-md inline-flex items-center justify-center text-foreground-400 hover:text-primary-600 hover:bg-primary-50 disabled:opacity-50';
const formInputClass = 'mt-1 w-full h-10 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300';

export default function AdminCategoriesPage() {
  const categoriesQuery = useAdminDirectoryCategories();
  const createCategory = useCreateDirectoryCategory();
  const updateCategory = useUpdateDirectoryCategory();
  const deleteCategory = useDeleteDirectoryCategory();
  const { toast } = useToast();
  const [editing, setEditing] = useState<PlatformCategory | null>(null);
  const [draft, setDraft] = useState<DirectoryCategoryInput>(emptyCategory);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PlatformCategory | null>(null);

  if (categoriesQuery.isLoading) return <LoadingSkeleton count={5} />;
  if (categoriesQuery.isError) {
    return <ErrorState message={categoriesQuery.error?.message || '加载分类失败'} onRetry={() => categoriesQuery.refetch()} />;
  }

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyCategory);
    setEditorOpen(true);
  };

  const openEdit = (category: PlatformCategory) => {
    setEditing(category);
    setDraft({ name: category.name, icon: category.icon, enabled: category.enabled });
    setEditorOpen(true);
  };

  const closeEditor = () => setEditorOpen(false);

  const saveCategory = () => {
    const options = {
      onSuccess: () => {
        closeEditor();
        toast('success', editing ? '分类已更新' : '分类已创建');
      },
      onError: error => toast('error', error.message || '保存分类失败'),
    };
    if (editing) updateCategory.mutate({ id: editing.id, data: draft }, options);
    else createCategory.mutate(draft, options);
  };

  const toggleCategory = (category: PlatformCategory) => {
    updateCategory.mutate({
      id: category.id,
      data: { name: category.name, icon: category.icon, enabled: !category.enabled },
    }, {
      onSuccess: () => toast('success', category.enabled ? '分类已停用' : '分类已启用'),
      onError: error => toast('error', error.message || '更新分类状态失败'),
    });
  };

  const confirmDelete = () => {
    if (!deleteTarget) return;
    deleteCategory.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast('success', `已删除分类「${deleteTarget.name}」`);
        setDeleteTarget(null);
      },
      onError: error => toast('error', error.message || '删除分类失败'),
    });
  };

  const items = categoriesQuery.data ?? [];
  const saving = createCategory.isPending || updateCategory.isPending;

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-bold font-heading text-foreground-950">公共分类</h1>
          <p className="text-xs text-foreground-400 mt-0.5">管理推荐站点库的分类结构与启用状态</p>
        </div>
        <button onClick={openCreate} className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium inline-flex items-center gap-2">
          <Plus className="w-4 h-4" />新建分类
        </button>
      </div>

      <div className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
        {items.length === 0 ? (
          <div className="py-16 text-center text-sm text-foreground-400">还没有公共分类</div>
        ) : items.map(category => (
          <div key={category.id} className="flex items-center gap-3 px-4 py-3 border-b last:border-b-0 border-background-100">
            <div className="w-8 h-8 rounded-md bg-primary-50 flex items-center justify-center">
              <IconRenderer icon={category.icon} className="text-primary-600" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-foreground-900">{category.name}</span>
                {!category.enabled && <span className="text-[10px] text-foreground-400">已停用</span>}
              </div>
              <div className="text-xs text-foreground-400">{category.siteCount} 个站点</div>
            </div>
            <button onClick={() => toggleCategory(category)} disabled={updateCategory.isPending} className={iconButtonClass} aria-label={category.enabled ? '停用分类' : '启用分类'}>
              {category.enabled ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
            <button onClick={() => openEdit(category)} className={iconButtonClass} aria-label="编辑分类"><Edit2 className="w-4 h-4" /></button>
            <button onClick={() => setDeleteTarget(category)} className={`${iconButtonClass} hover:text-red-500`} aria-label="删除分类"><Trash2 className="w-4 h-4" /></button>
          </div>
        ))}
      </div>

      {editorOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center">
          <button className="absolute inset-0 bg-black/30" onClick={closeEditor} aria-label="关闭" />
          <div className="relative w-full max-w-md mx-4 rounded-xl bg-white p-5 shadow-overlay space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-foreground-900">{editing ? '编辑分类' : '新建分类'}</h2>
              <button onClick={closeEditor} className={iconButtonClass}><X className="w-4 h-4" /></button>
            </div>
            <label className="block text-xs text-foreground-500">名称<input value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} maxLength={60} className={formInputClass} /></label>
            <label className="block text-xs text-foreground-500">图标<input value={draft.icon} onChange={event => setDraft({ ...draft, icon: event.target.value })} maxLength={256} className={formInputClass} /></label>
            <label className="flex items-center gap-2 text-sm text-foreground-600"><input type="checkbox" checked={draft.enabled} onChange={event => setDraft({ ...draft, enabled: event.target.checked })} />启用分类</label>
            <button onClick={saveCategory} disabled={saving || !draft.name.trim()} className="w-full h-10 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center justify-center gap-2">
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}保存
            </button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        onConfirm={confirmDelete}
        title="删除分类"
        description={deleteTarget?.siteCount ? `分类内仍有 ${deleteTarget.siteCount} 个站点，请先删除或移动这些站点。` : `确定删除分类「${deleteTarget?.name ?? ''}」吗？`}
        confirmLabel="确认删除"
        danger
      />
    </div>
  );
}
