// ============================================================
// nav.ax Admin DataTable — compact density, sort, pagination, all states
// ============================================================

import { useState, useMemo } from 'react';
import { ChevronLeft, ChevronRight, ChevronUp, ChevronDown, ArrowUpDown, Search, RotateCw, AlertTriangle, Inbox } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface Column<T> {
  key: string;
  header: string;
  sortable?: boolean;
  className?: string;
  headerClassName?: string;
  render: (item: T, index: number) => React.ReactNode;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyField: string;
  isLoading?: boolean;
  error?: string;
  onRetry?: () => void;
  pageSize?: number;
  showPageSize?: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
  emptyAction?: React.ReactNode;
  // Client-side search
  searchPlaceholder?: string;
  searchFields?: string[];
  // External pagination (for server-side)
  currentPage?: number;
  totalPages?: number;
  totalItems?: number;
  onPageChange?: (page: number) => void;
  // Actions
  toolbar?: React.ReactNode;
}

export function DataTable<T extends object>({
  columns,
  data,
  keyField,
  isLoading,
  error,
  onRetry,
  pageSize = 15,
  emptyTitle = '没有数据',
  emptyDescription,
  emptyAction,
  searchPlaceholder,
  searchFields = [],
  currentPage: externalPage,
  totalPages: externalTotalPages,
  totalItems: externalTotal,
  onPageChange,
  toolbar,
}: DataTableProps<T>) {
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');
  const [search, setSearch] = useState('');
  const [internalPage, setInternalPage] = useState(1);

  // Filter by search
  const filtered = useMemo(() => {
    if (!search || searchFields.length === 0) return data;
    const q = search.toLowerCase();
    return data.filter(item =>
      searchFields.some(field => {
        const val = (item as Record<string, unknown>)[field];
        return val != null && String(val).toLowerCase().includes(q);
      })
    );
  }, [data, search, searchFields]);

  // Sort
  const sorted = useMemo(() => {
    if (!sortKey) return filtered;
    return [...filtered].sort((a, b) => {
      const aVal = (a as Record<string, unknown>)[sortKey] ?? '';
      const bVal = (b as Record<string, unknown>)[sortKey] ?? '';
      const cmp = String(aVal).localeCompare(String(bVal), 'zh-CN');
      return sortDir === 'asc' ? cmp : -cmp;
    });
  }, [filtered, sortKey, sortDir]);

  // Pagination
  const isExternal = externalPage != null && onPageChange != null;
  const totalItems = isExternal ? (externalTotal ?? data.length) : sorted.length;
  const totalPages = isExternal ? (externalTotalPages ?? 1) : Math.max(1, Math.ceil(sorted.length / pageSize));
  const currentPage = isExternal ? externalPage : internalPage;
  const paged = isExternal ? sorted : sorted.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  const handleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  };

  const goPage = (p: number) => {
    if (isExternal) onPageChange!(p);
    else setInternalPage(p);
  };

  const handleSearch = (val: string) => {
    setSearch(val);
    goPage(1);
  };

  // Loading state
  if (isLoading) {
    return (
      <div className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
        <div className="px-4 py-3 border-b border-background-100">
          <div className="skeleton h-8 w-48" />
        </div>
        <div className="divide-y divide-background-50">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="px-4 py-3 flex items-center gap-4">
              {columns.slice(0, 4).map((_col, j) => (
                <div key={j} className="skeleton h-4 rounded" style={{ width: `${60 + j * 40}px` }} />
              ))}
            </div>
          ))}
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
        <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
          <div className="w-14 h-14 rounded-full bg-red-50 flex items-center justify-center mb-3">
            <AlertTriangle className="w-7 h-7 text-red-400" />
          </div>
          <h3 className="text-base font-semibold text-foreground-700 mb-1">加载失败</h3>
          <p className="text-sm text-foreground-400 mb-4 max-w-sm">{error}</p>
          {onRetry && (
            <button
              onClick={onRetry}
              className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-background-100 border border-background-200 text-sm text-foreground-600 hover:bg-background-200 transition-colors duration-150"
            >
              <RotateCw className="w-4 h-4" />
              重试
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
      {/* Toolbar */}
      {(searchPlaceholder || toolbar) && (
        <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3 px-4 py-3 border-b border-background-100">
          {searchPlaceholder && (
            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-foreground-300" />
              <input
                type="text"
                value={search}
                onChange={e => handleSearch(e.target.value)}
                placeholder={searchPlaceholder}
                className="w-full h-8 pl-8 pr-3 rounded-md bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 transition-all duration-150"
              />
            </div>
          )}
          <div className="flex-1" />
          {toolbar}
        </div>
      )}

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-background-200/70 bg-background-50/50">
              {columns.map(col => (
                <th
                  key={col.key}
                  className={cn(
                    'text-left px-4 py-2.5 text-xs font-medium text-foreground-400 select-none',
                    col.sortable && 'cursor-pointer hover:text-foreground-600 transition-colors duration-150',
                    col.headerClassName
                  )}
                  onClick={() => col.sortable && handleSort(col.key)}
                >
                  <div className="flex items-center gap-1 whitespace-nowrap">
                    {col.header}
                    {col.sortable && (
                      sortKey === col.key
                        ? (sortDir === 'asc' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />)
                        : <ArrowUpDown className="w-3 h-3 text-foreground-200" />
                    )}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {paged.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-16">
                  <div className="flex flex-col items-center justify-center text-center">
                    <div className="w-14 h-14 rounded-full bg-background-100 flex items-center justify-center mb-3">
                      <Inbox className="w-7 h-7 text-foreground-300" />
                    </div>
                    <h3 className="text-base font-semibold text-foreground-600 mb-1">{search ? '没有找到匹配的结果' : emptyTitle}</h3>
                    {emptyDescription && <p className="text-sm text-foreground-400 max-w-sm mb-3">{emptyDescription}</p>}
                    {emptyAction && <div>{emptyAction}</div>}
                  </div>
                </td>
              </tr>
            ) : (
              paged.map((item, idx) => (
                <tr
                  key={String((item as Record<string, unknown>)[keyField])}
                  className="border-b border-background-100 last:border-b-0 hover:bg-background-50/70 transition-colors duration-150"
                >
                  {columns.map(col => (
                    <td key={col.key} className={cn('px-4 py-2.5', col.className)}>
                      {col.render(item, idx)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between px-4 py-2.5 border-t border-background-100">
          <span className="text-xs text-foreground-400">共 {totalItems} 条</span>
          <div className="flex items-center gap-0.5">
            <button
              onClick={() => goPage(Math.max(1, currentPage - 1))}
              disabled={currentPage <= 1}
              className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-400 hover:bg-background-100 disabled:opacity-30 transition-colors duration-150"
              aria-label="上一页"
            >
              <ChevronLeft className="w-3.5 h-3.5" />
            </button>
            {Array.from({ length: Math.min(totalPages, 5) }).map((_, i) => {
              let p: number;
              if (totalPages <= 5) {
                p = i + 1;
              } else if (currentPage <= 3) {
                p = i + 1;
              } else if (currentPage >= totalPages - 2) {
                p = totalPages - 4 + i;
              } else {
                p = currentPage - 2 + i;
              }
              return (
                <button
                  key={p}
                  onClick={() => goPage(p)}
                  className={cn(
                    'w-7 h-7 flex items-center justify-center rounded-md text-xs transition-colors duration-150',
                    p === currentPage
                      ? 'bg-primary-100 text-primary-700 font-medium'
                      : 'text-foreground-400 hover:bg-background-100'
                  )}
                >
                  {p}
                </button>
              );
            })}
            <button
              onClick={() => goPage(Math.min(totalPages, currentPage + 1))}
              disabled={currentPage >= totalPages}
              className="w-7 h-7 flex items-center justify-center rounded-md text-foreground-400 hover:bg-background-100 disabled:opacity-30 transition-colors duration-150"
              aria-label="下一页"
            >
              <ChevronRight className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
