// ============================================================
// nav.ax SiteTable — compact table view for batch link management
// ============================================================

import { useState } from 'react';
import { Edit2, Trash2, Search } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Site, Category } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';

export interface FlatSite extends Site {
  categoryName: string;
  categoryIcon: string;
}

export interface SiteTableProps {
  sites: FlatSite[];
  selectedIds: Set<string>;
  onToggleSelect: (id: string) => void;
  onToggleSelectAll: () => void;
  onEdit: (site: Site) => void;
  onDelete: (site: Site) => void;
}

export default function SiteTable({
  sites,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  onEdit,
  onDelete,
}: SiteTableProps) {
  const [filter, setFilter] = useState('');

  const filtered = filter
    ? sites.filter(
        s =>
          s.title.toLowerCase().includes(filter.toLowerCase()) ||
          s.url.toLowerCase().includes(filter.toLowerCase()) ||
          s.categoryName.toLowerCase().includes(filter.toLowerCase()),
      )
    : sites;

  const allFilteredSelected = filtered.length > 0 && filtered.every(s => selectedIds.has(s.id));
  const someFilteredSelected = filtered.some(s => selectedIds.has(s.id)) && !allFilteredSelected;

  const handleSelectAllToggle = () => {
    // Toggle select/deselect for filtered items
    if (allFilteredSelected) {
      // Deselect all filtered
      filtered.forEach(s => {
        if (selectedIds.has(s.id)) onToggleSelect(s.id);
      });
    } else {
      // Select all filtered that aren't already selected
      filtered.forEach(s => {
        if (!selectedIds.has(s.id)) onToggleSelect(s.id);
      });
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Search in table */}
      <div className="px-4 py-3 border-b border-background-100">
        <div className="relative">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-foreground-300" />
          <input
            type="text"
            value={filter}
            onChange={e => setFilter(e.target.value)}
            placeholder="搜索标题、域名或分类..."
            className="w-full h-8 pl-8 pr-3 rounded-md bg-background-50 border border-background-200/70 text-xs text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
          />
        </div>
      </div>

      {/* Table body */}
      <div className="flex-1 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="py-10 text-center text-xs text-foreground-400">
            {filter ? `没有匹配「${filter}」的结果` : '暂无站点'}
          </div>
        ) : (
          <table className="w-full">
            <thead className="sticky top-0 z-10">
              <tr className="border-b border-background-200/70 bg-background-50">
                <th className="w-8 px-2 py-2">
                  <button
                    onClick={handleSelectAllToggle}
                    className="w-4 h-4 rounded border border-background-300 flex items-center justify-center hover:border-primary-400 transition-colors duration-150"
                  >
                    {allFilteredSelected ? (
                      <i className="ri-check-line text-[10px] text-primary-500" />
                    ) : someFilteredSelected ? (
                      <div className="w-2 h-0.5 bg-primary-400 rounded-full" />
                    ) : null}
                  </button>
                </th>
                <th className="text-left px-2 py-2 text-[10px] font-medium text-foreground-400 w-8" />
                <th className="text-left px-0 py-2 text-[10px] font-medium text-foreground-400">站点</th>
                <th className="text-left px-2 py-2 text-[10px] font-medium text-foreground-400 hidden xl:table-cell">分类</th>
                <th className="w-16 px-2 py-2 text-[10px] font-medium text-foreground-400 text-right" />
              </tr>
            </thead>
            <tbody>
              {filtered.map(site => {
                const isSelected = selectedIds.has(site.id);
                return (
                  <tr
                    key={site.id}
                    className={cn(
                      'border-b border-background-100 last:border-b-0 hover:bg-background-50/70 transition-colors duration-150',
                      isSelected && 'bg-primary-50/40',
                    )}
                  >
                    <td className="px-2 py-2">
                      <button
                        onClick={() => onToggleSelect(site.id)}
                        className={cn(
                          'w-4 h-4 rounded border flex items-center justify-center transition-all duration-150',
                          isSelected
                            ? 'bg-primary-500 border-primary-500'
                            : 'border-background-300 hover:border-primary-400',
                        )}
                      >
                        {isSelected && <i className="ri-check-line text-[10px] text-background-50" />}
                      </button>
                    </td>
                    <td className="px-0 py-2">
                      <div className="w-6 h-6 rounded-md bg-background-100 flex items-center justify-center">
                        <IconRenderer icon={site.icon} className="text-[10px] text-foreground-500" />
                      </div>
                    </td>
                    <td className="px-0 py-2 min-w-0">
                      <div className="text-xs font-medium text-foreground-800 truncate max-w-[140px]">
                        {site.title}
                      </div>
                      <div className="text-[10px] text-foreground-400 truncate max-w-[140px] font-mono">
                        {site.url.replace(/^https?:\/\//, '').replace(/^www\./, '').split('/')[0]}
                      </div>
                    </td>
                    <td className="px-2 py-2 hidden xl:table-cell">
                      <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] text-foreground-500 bg-background-100">
                        <i className={cn(site.categoryIcon, 'text-[9px]')} />
                        {site.categoryName}
                      </span>
                    </td>
                    <td className="px-2 py-2 text-right">
                      <div className="flex items-center justify-end gap-0.5">
                        <button
                          onClick={() => onEdit(site)}
                          className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
                          aria-label={`编辑 ${site.title}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() => onDelete(site)}
                          className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
                          aria-label={`删除 ${site.title}`}
                        >
                          <Trash2 className="w-3 h-3" />
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
      <div className="border-t border-background-100 px-4 py-2 flex items-center justify-between">
        <span className="text-[10px] text-foreground-400">
          共 {sites.length} 个站点
        </span>
        {filter && (
          <span className="text-[10px] text-foreground-400">
            显示 {filtered.length} 个
          </span>
        )}
      </div>
    </div>
  );
}
