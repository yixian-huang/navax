// ============================================================
// nav.ax SiteTable — compact table view for batch link management
// ============================================================

import { useMemo, useState } from 'react';
import { Edit2, Trash2, Search, Eye, EyeOff, ExternalLink } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Site } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';

export interface FlatSite extends Site {
  categoryName: string;
  categoryIcon: string;
}

export interface SiteTableProps {
  sites: FlatSite[];
  /** Category options for the filter dropdown (id + name). */
  categories?: Array<{ id: string; name: string }>;
  selectedIds: Set<string>;
  onToggleSelect: (id: string) => void;
  onToggleSelectAll: () => void;
  onEdit: (site: Site) => void;
  onDelete: (site: Site) => void;
  onToggleEnabled?: (site: Site) => void;
}

export default function SiteTable({
  sites,
  categories = [],
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  onEdit,
  onDelete,
  onToggleEnabled,
}: SiteTableProps) {
  const [filter, setFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<string>('all');
  const [visibilityFilter, setVisibilityFilter] = useState<'all' | 'enabled' | 'hidden'>('all');

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return sites.filter(s => {
      if (categoryFilter !== 'all' && s.categoryId !== categoryFilter) return false;
      if (visibilityFilter === 'enabled' && s.enabled === false) return false;
      if (visibilityFilter === 'hidden' && s.enabled !== false) return false;
      if (!q) return true;
      return (
        s.title.toLowerCase().includes(q) ||
        s.url.toLowerCase().includes(q) ||
        s.categoryName.toLowerCase().includes(q) ||
        (s.description ? s.description.toLowerCase().includes(q) : false)
      );
    });
  }, [sites, filter, categoryFilter, visibilityFilter]);

  const allFilteredSelected = filtered.length > 0 && filtered.every(s => selectedIds.has(s.id));
  const someFilteredSelected = filtered.some(s => selectedIds.has(s.id)) && !allFilteredSelected;

  const handleSelectAllToggle = () => {
    if (allFilteredSelected) {
      filtered.forEach(s => {
        if (selectedIds.has(s.id)) onToggleSelect(s.id);
      });
    } else {
      filtered.forEach(s => {
        if (!selectedIds.has(s.id)) onToggleSelect(s.id);
      });
    }
  };

  const enabledCount = sites.filter(s => s.enabled !== false).length;
  const siteCountByCategory = useMemo(() => {
    const map = new Map<string, number>();
    for (const site of sites) {
      map.set(site.categoryId, (map.get(site.categoryId) ?? 0) + 1);
    }
    return map;
  }, [sites]);

  return (
    <div className="flex flex-col h-full min-w-0">
      {/* Search + filters */}
      <div className="px-3 py-2.5 border-b border-background-100 space-y-2">
        <div className="relative">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-foreground-300" />
          <input
            type="text"
            value={filter}
            onChange={e => setFilter(e.target.value)}
            placeholder="搜索标题、描述、域名或分类..."
            className="w-full h-8 pl-8 pr-3 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
          />
        </div>
        {categories.length > 0 && (
          <div className="flex flex-wrap gap-1" role="group" aria-label="分类快捷筛选">
            <button
              type="button"
              onClick={() => setCategoryFilter('all')}
              className={cn(
                'h-6 px-2 rounded-full text-[10px] font-medium border transition-colors',
                categoryFilter === 'all'
                  ? 'bg-primary-500 text-background-50 border-primary-500'
                  : 'bg-background-50 text-foreground-600 border-background-200 hover:border-primary-300',
              )}
            >
              全部 {sites.length}
            </button>
            {categories.map(cat => {
              const count = siteCountByCategory.get(cat.id) ?? 0;
              const active = categoryFilter === cat.id;
              return (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => setCategoryFilter(cat.id)}
                  className={cn(
                    'h-6 max-w-full px-2 rounded-full text-[10px] font-medium border transition-colors truncate',
                    active
                      ? 'bg-primary-500 text-background-50 border-primary-500'
                      : 'bg-background-50 text-foreground-600 border-background-200 hover:border-primary-300',
                  )}
                  title={count === 0 ? `${cat.name}（空分类）` : `${cat.name}（${count}）`}
                >
                  {cat.name}
                  <span className={cn('ml-1 tabular-nums', active ? 'opacity-80' : 'text-foreground-400')}>
                    {count}
                  </span>
                </button>
              );
            })}
          </div>
        )}
        <div className="flex items-center gap-2 flex-wrap">
          <select
            value={categoryFilter}
            onChange={e => setCategoryFilter(e.target.value)}
            className="h-7 min-w-0 flex-1 max-w-[11rem] px-2 rounded-md bg-background-50 border border-background-200/70 text-[11px] text-foreground-700 focus:outline-none focus:border-primary-300"
            aria-label="按分类筛选"
          >
            <option value="all">全部分类</option>
            {categories.map(cat => (
              <option key={cat.id} value={cat.id}>{cat.name}</option>
            ))}
          </select>
          <select
            value={visibilityFilter}
            onChange={e => setVisibilityFilter(e.target.value as typeof visibilityFilter)}
            className="h-7 px-2 rounded-md bg-background-50 border border-background-200/70 text-[11px] text-foreground-700 focus:outline-none focus:border-primary-300"
            aria-label="按可见性筛选"
          >
            <option value="all">全部状态</option>
            <option value="enabled">仅上架</option>
            <option value="hidden">仅隐藏</option>
          </select>
        </div>
      </div>

      {/* Table body */}
      <div className="flex-1 overflow-y-auto overflow-x-auto min-w-0">
        {filtered.length === 0 ? (
          <div className="py-10 text-center text-xs text-foreground-400">
            {filter || categoryFilter !== 'all' || visibilityFilter !== 'all'
              ? '没有匹配当前筛选的结果'
              : '暂无站点'}
          </div>
        ) : (
          <table className="w-full table-fixed min-w-[28rem]">
            <thead className="sticky top-0 z-10">
              <tr className="border-b border-background-200/70 bg-background-50">
                <th className="w-8 px-2 py-2">
                  <button
                    type="button"
                    onClick={handleSelectAllToggle}
                    className="w-4 h-4 rounded border border-background-300 flex items-center justify-center hover:border-primary-400 transition-colors duration-150"
                    aria-label={allFilteredSelected ? '取消全选' : '全选当前筛选'}
                  >
                    {allFilteredSelected ? (
                      <i className="ri-check-line text-[10px] text-primary-500" />
                    ) : someFilteredSelected ? (
                      <div className="w-2 h-0.5 bg-primary-400 rounded-full" />
                    ) : null}
                  </button>
                </th>
                <th className="w-9 px-1 py-2 text-[10px] font-medium text-foreground-400 text-left" />
                <th className="text-left px-2 py-2 text-[10px] font-medium text-foreground-400">站点</th>
                <th className="text-left px-2 py-2 text-[10px] font-medium text-foreground-400 w-[5.5rem] hidden sm:table-cell">状态</th>
                <th className="text-left px-2 py-2 text-[10px] font-medium text-foreground-400 w-24 hidden lg:table-cell">分类</th>
                <th className="w-[5.5rem] px-2 py-2 text-[10px] font-medium text-foreground-400 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(site => {
                const isSelected = selectedIds.has(site.id);
                const isHidden = site.enabled === false;
                return (
                  <tr
                    key={site.id}
                    className={cn(
                      'border-b border-background-100 last:border-b-0 hover:bg-background-50/70 transition-colors duration-150',
                      isSelected && 'bg-primary-50/40',
                      isHidden && 'bg-background-50/80',
                    )}
                  >
                    <td className="px-2 py-2.5 align-top">
                      <button
                        type="button"
                        onClick={() => onToggleSelect(site.id)}
                        className={cn(
                          'w-4 h-4 rounded border flex items-center justify-center transition-all duration-150 mt-0.5',
                          isSelected
                            ? 'bg-primary-500 border-primary-500'
                            : 'border-background-300 hover:border-primary-400',
                        )}
                      >
                        {isSelected && <i className="ri-check-line text-[10px] text-background-50" />}
                      </button>
                    </td>
                    <td className="px-1 py-2.5 align-top">
                      <div className={cn(
                        'w-7 h-7 rounded-md flex items-center justify-center',
                        isHidden ? 'bg-background-100 opacity-70' : 'bg-background-100',
                      )}>
                        <IconRenderer icon={site.icon} url={site.url} size={16} alt={site.title} />
                      </div>
                    </td>
                    <td className="px-2 py-2.5 min-w-0 align-top">
                      <div className={cn(
                        'text-xs font-medium break-words leading-snug',
                        isHidden ? 'text-foreground-500' : 'text-foreground-800',
                      )}>
                        {site.title}
                      </div>
                      {site.description ? (
                        <div className="text-[10px] text-foreground-400 mt-0.5 line-clamp-2 break-words leading-snug">
                          {site.description}
                        </div>
                      ) : null}
                      <a
                        href={site.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="mt-0.5 inline-flex items-center gap-0.5 max-w-full text-[10px] font-mono text-primary-600 hover:text-primary-700 hover:underline break-all"
                        title={site.url}
                        onClick={e => e.stopPropagation()}
                      >
                        <span className="truncate">
                          {site.url.replace(/^https?:\/\//, '').replace(/^www\./, '')}
                        </span>
                        <ExternalLink className="w-2.5 h-2.5 flex-shrink-0 opacity-70" />
                      </a>
                    </td>
                    <td className="px-2 py-2.5 align-top hidden sm:table-cell">
                      {isHidden ? (
                        <span
                          className="inline-flex items-center gap-1 text-[10px] text-foreground-400"
                          title="隐藏：发布后访客不可见"
                        >
                          <EyeOff className="w-3.5 h-3.5" />
                          隐藏
                        </span>
                      ) : (
                        <span className="sr-only">上架</span>
                      )}
                    </td>
                    <td className="px-2 py-2.5 align-top hidden lg:table-cell">
                      <span className="inline-flex items-center gap-1 max-w-full px-1.5 py-0.5 rounded text-[10px] text-foreground-500 bg-background-100">
                        <IconRenderer icon={site.categoryIcon} className="text-[9px]" size={10} />
                        <span className="truncate">{site.categoryName}</span>
                      </span>
                    </td>
                    <td className="px-2 py-2.5 text-right align-top">
                      <div className="flex items-center justify-end gap-0.5">
                        {onToggleEnabled && (
                          <button
                            type="button"
                            onClick={() => onToggleEnabled(site)}
                            className="w-7 h-7 flex items-center justify-center rounded transition-colors duration-150 text-foreground-300 hover:text-foreground-600 hover:bg-background-100"
                            aria-label={isHidden ? `上架 ${site.title}` : `隐藏 ${site.title}`}
                            title={isHidden ? '上架（需发布后生效）' : '隐藏（需发布后生效）'}
                          >
                            {isHidden ? <Eye className="w-3.5 h-3.5" /> : <EyeOff className="w-3.5 h-3.5" />}
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => onEdit(site)}
                          className="w-7 h-7 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
                          aria-label={`编辑 ${site.title}`}
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => onDelete(site)}
                          className="w-7 h-7 flex items-center justify-center rounded text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
                          aria-label={`删除 ${site.title}`}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer */}
      <div className="border-t border-background-100 px-3 py-2 flex items-center justify-between gap-2">
        <span className="text-[10px] text-foreground-400">
          上架 {enabledCount}/{sites.length}
          {sites.length - enabledCount > 0 ? ` · 隐藏 ${sites.length - enabledCount}` : ''}
        </span>
        {(filter || categoryFilter !== 'all' || visibilityFilter !== 'all') && (
          <span className="text-[10px] text-foreground-400">
            显示 {filtered.length} 个
          </span>
        )}
      </div>
    </div>
  );
}
