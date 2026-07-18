// ============================================================
// nav.ax PropertiesPanel — right-side slide-in property editor
// Uses shared FormField components
// ============================================================

import { useState, useEffect, useCallback } from 'react';
import { X, Save, Trash2 } from 'lucide-react';
import { useSaveStatus } from '@/hooks/useSaveStatus';
import { FormField, FormInput, FormTextarea } from '@/components/base/FormField';
import IconPicker from '@/components/base/IconPicker';

export interface SiteEditData {
  title: string;
  url: string;
  icon: string;
  description: string;
}

export interface CategoryEditData {
  name: string;
  icon: string;
}

interface PropertiesPanelProps {
  open: boolean;
  onClose: () => void;
  mode: 'site' | 'category';
  editData?: SiteEditData | CategoryEditData;
  onSave: (data: SiteEditData | CategoryEditData) => void;
  onDelete?: () => void;
  deleteLabel?: string;
  title: string;
}

export default function PropertiesPanel({
  open,
  onClose,
  mode,
  editData,
  onSave,
  onDelete,
  deleteLabel,
  title,
}: PropertiesPanelProps) {
  const [data, setData] = useState<SiteEditData | CategoryEditData>(editData || { title: '', url: '', icon: 'ri-link', description: '' });
  const [dirty, setDirty] = useState(false);
  const { markSaving, markSaved, markError } = useSaveStatus();

  useEffect(() => {
    if (open && editData) {
      setData(editData);
      setDirty(false);
    }
  }, [open, editData]);

  const update = useCallback((field: string, value: string | boolean) => {
    setData(prev => ({ ...prev, [field]: value }));
    setDirty(true);
  }, []);

  const handleSave = useCallback(() => {
    markSaving();
    try {
      onSave(data);
      markSaved();
      setDirty(false);
    } catch {
      markError('保存属性失败，请重试');
    }
  }, [data, onSave, markSaving, markSaved, markError]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      if (dirty) handleSave();
    }
  }, [onClose, dirty, handleSave]);

  if (!open) return null;

  const isSite = mode === 'site';
  const isCategory = mode === 'category';

  return (
    <>
      {/* Backdrop: always fixed so drawer is not trapped in page scroll */}
      <div
        className="fixed inset-0 bg-black/25 z-[60]"
        onClick={onClose}
        aria-hidden
      />

      <div
        className="fixed top-0 right-0 z-[70] flex flex-col w-[min(100vw,20rem)] h-dvh max-h-dvh bg-background-50 border-l border-background-200/70 shadow-xl"
        onKeyDown={handleKeyDown}
        role="dialog"
        aria-modal="true"
        aria-label="属性面板"
      >
        {/* Header — save always reachable without scrolling */}
        <div className="flex items-center gap-2 px-3 h-12 border-b border-background-200/70 flex-shrink-0 bg-background-50">
          <h3 className="text-sm font-semibold text-foreground-900 truncate flex-1 min-w-0">{title}</h3>
          <button
            type="button"
            onClick={handleSave}
            disabled={!dirty}
            className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed inline-flex items-center gap-1 flex-shrink-0"
          >
            <Save className="w-3.5 h-3.5" />
            保存
          </button>
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded-md text-foreground-400 hover:bg-background-100 flex-shrink-0"
            aria-label="关闭面板"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Scrollable fields only */}
        <div className="flex-1 min-h-0 overflow-y-auto overscroll-contain p-4 space-y-4">
          {isSite && (
            <>
              <FormField label="标题">
                <FormInput
                  type="text"
                  value={(data as SiteEditData).title}
                  onChange={e => update('title', e.target.value)}
                />
              </FormField>
              <FormField label="网址">
                <FormInput
                  type="url"
                  value={(data as SiteEditData).url}
                  onChange={e => update('url', e.target.value)}
                />
              </FormField>
              <FormField label="图标">
                <IconPicker
                  value={(data as SiteEditData).icon}
                  onChange={v => update('icon', v)}
                  compact
                />
              </FormField>
              <FormField label="简介">
                <FormTextarea
                  value={(data as SiteEditData).description}
                  onChange={e => update('description', e.target.value)}
                  maxLength={200}
                  rows={3}
                />
              </FormField>
            </>
          )}

          {isCategory && (
            <>
              <FormField label="分类名称">
                <FormInput
                  type="text"
                  value={(data as CategoryEditData).name}
                  onChange={e => update('name', e.target.value)}
                />
              </FormField>
              <FormField label="图标">
                <IconPicker
                  value={(data as CategoryEditData).icon}
                  onChange={v => update('icon', v)}
                  compact
                />
              </FormField>
            </>
          )}
        </div>

        {/* Footer — delete / cancel always visible */}
        <div className="border-t border-background-200/70 p-3 flex items-center gap-2 flex-shrink-0 bg-background-50">
          {onDelete && (
            <button
              type="button"
              onClick={onDelete}
              className="h-9 px-3 rounded-lg text-sm text-red-600 hover:bg-red-50 inline-flex items-center gap-1.5 whitespace-nowrap"
            >
              <Trash2 className="w-4 h-4" />
              {deleteLabel || '删除'}
            </button>
          )}
          <div className="flex-1" />
          <button
            type="button"
            onClick={onClose}
            className="h-9 px-3 rounded-lg text-sm text-foreground-500 hover:bg-background-100 whitespace-nowrap"
          >
            取消
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={!dirty}
            className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed inline-flex items-center gap-1.5 whitespace-nowrap"
          >
            <Save className="w-3.5 h-3.5" />
            保存
          </button>
        </div>
      </div>
    </>
  );
}
