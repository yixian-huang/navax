import { useEffect } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Density, LayoutTemplate, PageSettings, ThemeLayout } from '@/api/types';
import { HOME_LAYOUTS, HOME_LAYOUT_META } from '@/types/layout';

export const DENSITY_LABELS: Record<Density, string> = {
  list: '列表',
  compact: '紧凑',
  comfortable: '舒适',
};

export const CATEGORY_STYLE_OPTIONS = [
  { id: 'tabs' as const, label: '标签' },
  { id: 'sidebar' as const, label: '侧栏' },
  { id: 'grid' as const, label: '网格' },
  { id: 'folders' as const, label: '文件夹' },
];

export function layoutSummary(layout: PageSettings['layout']): string {
  return `${DENSITY_LABELS[layout.density]} · ${layout.columns}列`;
}

export default function LayoutSettingsDialog({
  open, onClose, layout, themeLayout,
  onTemplateChange, onDensityChange, onCategoryStyleChange, onColumnsChange,
}: {
  open: boolean;
  onClose: () => void;
  layout: PageSettings['layout'];
  themeLayout?: ThemeLayout | null;
  onTemplateChange: (template: LayoutTemplate) => void;
  onDensityChange: (density: Density) => void;
  onCategoryStyleChange: (style: PageSettings['layout']['categoryStyle']) => void;
  onColumnsChange: (columns: number) => void;
}) {
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative bg-background-50 rounded-xl p-6 w-full max-w-md mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-5">
          <h3 className="text-lg font-semibold text-foreground-900">布局设置</h3>
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭"
            className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-3">
          <div className="space-y-1.5">
            <span className="text-[10px] text-foreground-400">导航页布局</span>
            <div className="grid grid-cols-2 gap-1">
              {HOME_LAYOUTS.filter(l => !themeLayout || themeLayout.template.allowed.includes(l)).map(l => {
                const meta = HOME_LAYOUT_META[l];
                const isActive = layout.template === l;
                const locked = Boolean(themeLayout?.template.locked);
                return (
                  <button
                    key={l}
                    type="button"
                    disabled={locked}
                    onClick={() => { if (!locked) onTemplateChange(l); }}
                    title={meta.description}
                    className={cn(
                      'flex items-center gap-1.5 px-2 py-1.5 rounded-md text-[10px] font-medium transition-all duration-150 whitespace-nowrap cursor-pointer',
                      isActive
                        ? 'bg-primary-100 text-primary-700'
                        : 'text-foreground-400 hover:bg-background-100 hover:text-foreground-600'
                    )}
                  >
                    <i className={cn(meta.icon, isActive ? 'text-primary-500' : 'text-foreground-400', 'text-xs')} />
                    {meta.name}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="space-y-1.5">
            <span className="text-[10px] text-foreground-400">密度</span>
            <div className="flex items-center bg-background-100 rounded-md p-0.5">
              {(['list', 'compact', 'comfortable'] as const).filter(d => !themeLayout || themeLayout.density.allowed.includes(d)).map(d => (
                <button
                  key={d}
                  type="button"
                  disabled={Boolean(themeLayout?.density.locked)}
                  onClick={() => { if (!themeLayout?.density.locked) onDensityChange(d); }}
                  className={cn(
                    'flex-1 py-1 rounded text-[10px] font-medium transition-colors duration-150 whitespace-nowrap',
                    layout.density === d
                      ? 'bg-white text-foreground-900 shadow-sm'
                      : 'text-foreground-400 hover:text-foreground-600',
                  )}
                >
                  {DENSITY_LABELS[d]}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-1.5">
            <span className="text-[10px] text-foreground-400">分类样式</span>
            <div className="grid grid-cols-2 gap-1">
              {CATEGORY_STYLE_OPTIONS.filter(s => !themeLayout || themeLayout.categoryStyle.allowed.includes(s.id)).map(s => {
                const isActive = (layout.categoryStyle ?? 'tabs') === s.id;
                const locked = Boolean(themeLayout?.categoryStyle.locked);
                return (
                  <button
                    key={s.id}
                    type="button"
                    disabled={locked}
                    onClick={() => { if (!locked) onCategoryStyleChange(s.id); }}
                    className={cn(
                      'px-2 py-1.5 rounded-md text-[10px] font-medium transition-all duration-150 whitespace-nowrap cursor-pointer',
                      isActive
                        ? 'bg-primary-100 text-primary-700'
                        : 'text-foreground-400 hover:bg-background-100 hover:text-foreground-600',
                    )}
                  >
                    {s.label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <span className="text-[10px] text-foreground-400">列数</span>
              <span className="text-[10px] font-mono text-foreground-600">
                {Math.min(8, Math.max(1, layout.columns))}
              </span>
            </div>
            <input
              type="range"
              min={themeLayout?.columns.min ?? 1}
              max={themeLayout?.columns.max ?? 8}
              disabled={Boolean(themeLayout?.columns.locked)}
              value={Math.min(themeLayout?.columns.max ?? 8, Math.max(themeLayout?.columns.min ?? 1, layout.columns || 4))}
              onChange={e => onColumnsChange(Number(e.target.value))}
              className="w-full accent-primary-500 h-1"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
