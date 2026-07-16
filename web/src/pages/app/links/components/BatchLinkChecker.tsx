// ============================================================
// nav.ax BatchLinkChecker — batch check managed links
// ============================================================

import { useState, useCallback } from 'react';
import { X, Search, RotateCw, CheckCircle2, AlertTriangle, Clock, ExternalLink, Link2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { navigationApi } from '@/api/navigation';
import type { LinkCheckResult } from '@/api/types';

interface ManagedLink {
  id: string;
  title: string;
  url: string;
}

interface DisplayResult extends LinkCheckResult, ManagedLink {}

interface BatchLinkCheckerProps {
  open: boolean;
  onClose: () => void;
  pageId: string;
  managedLinks: ManagedLink[];
}

export default function BatchLinkChecker({ open, onClose, pageId, managedLinks }: BatchLinkCheckerProps) {
  const [results, setResults] = useState<DisplayResult[]>([]);
  const [checking, setChecking] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');

  const handleCheck = useCallback(async () => {
    if (managedLinks.length === 0) return;

    setChecking(true);
    setResults([]);
    setErrorMessage('');
    try {
      const checked: LinkCheckResult[] = [];
      for (let start = 0; start < managedLinks.length; start += 50) {
        const siteIds = managedLinks.slice(start, start + 50).map(link => link.id);
        const response = await navigationApi.forPage(pageId).checkLinks(siteIds);
        checked.push(...response.data);
      }
      const linksById = new Map(managedLinks.map(link => [link.id, link]));
      setResults(checked.flatMap(result => {
        const link = linksById.get(result.siteId);
        return link ? [{ ...result, ...link }] : [];
      }));
    } catch (cause) {
      setErrorMessage(cause instanceof Error ? cause.message : '链接检测失败');
    } finally {
      setChecking(false);
    }
  }, [managedLinks, pageId]);

  const loadManagedLinks = useCallback(() => {
    setResults([]);
    setErrorMessage('');
  }, []);

  const okCount = results.filter(r => r.status === 'reachable').length;
  const failCount = results.filter(r => r.status !== 'reachable').length;

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative bg-white rounded-xl w-full max-w-2xl mx-4 max-h-[88vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-background-200/70 flex-shrink-0">
          <div>
            <h3 className="text-base font-semibold text-foreground-900">批量链接检测</h3>
            <p className="text-xs text-foreground-400 mt-0.5">
              检测当前管理的 {managedLinks.length} 个链接的可用性
            </p>
          </div>

          {errorMessage && <p className="mt-2 text-xs text-red-500">{errorMessage}</p>}
          <button
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Managed links summary */}
        <div className="px-5 py-3 border-b border-background-100 flex-shrink-0">
          <div className="flex items-center gap-3">
            <button
              onClick={handleCheck}
              disabled={checking || managedLinks.length === 0}
              className={cn(
                'h-9 px-4 rounded-lg text-sm font-medium transition-all duration-150 flex items-center gap-1.5 whitespace-nowrap',
                checking || managedLinks.length === 0
                  ? 'bg-background-100 text-foreground-300 cursor-not-allowed'
                  : 'bg-primary-500 text-background-50 dark:text-foreground-950 hover:bg-primary-600'
              )}
            >
              {checking ? (
                <>
                  <RotateCw className="w-4 h-4 animate-spin" />
                  检测中...
                </>
              ) : (
                <>
                  <Search className="w-4 h-4" />
                  检测全部 ({managedLinks.length})
                </>
              )}
            </button>

            <button
              onClick={loadManagedLinks}
              disabled={checking}
              className="h-9 px-3 rounded-lg border border-background-200/70 text-xs text-foreground-600 hover:bg-background-50 transition-colors duration-150 flex items-center gap-1.5 whitespace-nowrap"
            >
              <Link2 className="w-3.5 h-3.5" />
              重新加载链接
            </button>

            {results.length > 0 && !checking && (
              <div className="flex items-center gap-3 ml-auto">
                <span className="flex items-center gap-1 text-xs">
                  <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
                  <span className="text-green-600 font-medium">{okCount}</span>
                </span>
                {failCount > 0 && (
                  <span className="flex items-center gap-1 text-xs">
                    <AlertTriangle className="w-3.5 h-3.5 text-red-400" />
                    <span className="text-red-500 font-medium">{failCount}</span>
                  </span>
                )}
              </div>
            )}
          </div>

          {/* Link preview list */}
          {results.length === 0 && !checking && managedLinks.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {managedLinks.slice(0, 12).map((link, i) => (
                <span key={i} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-background-100 text-[10px] text-foreground-500">
                  {link.title}
                </span>
              ))}
              {managedLinks.length > 12 && (
                <span className="text-[10px] text-foreground-400 px-1">
                  ...还有 {managedLinks.length - 12} 个
                </span>
              )}
            </div>
          )}
        </div>

        {/* Results */}
        <div className="flex-1 overflow-y-auto px-5 pb-4">
          {results.length > 0 && (
            <div className="rounded-lg border border-background-200/70 overflow-hidden mt-3">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-background-200/70 bg-background-50">
                    <th className="text-left px-3 py-2 text-[10px] font-medium text-foreground-400 w-10">状态</th>
                    <th className="text-left px-3 py-2 text-[10px] font-medium text-foreground-400">网址</th>
                    <th className="text-left px-3 py-2 text-[10px] font-medium text-foreground-400 hidden sm:table-cell w-24">标题</th>
                    <th className="text-right px-3 py-2 text-[10px] font-medium text-foreground-400 w-20">响应时间</th>
                  </tr>
                </thead>
                <tbody>
                  {results.map(r => (
                    <tr
                      key={r.siteId}
                      className="border-b border-background-100 last:border-b-0 transition-colors duration-150"
                    >
                      <td className="px-3 py-2.5">
                        {r.status === 'reachable' && (
                          <div className="w-5 h-5 rounded-full bg-green-50 flex items-center justify-center">
                            <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
                          </div>
                        )}
                        {r.status === 'timeout' && (
                          <div className="w-5 h-5 rounded-full bg-yellow-50 flex items-center justify-center">
                            <Clock className="w-3.5 h-3.5 text-yellow-500" />
                          </div>
                        )}
                        {(r.status === 'unreachable' || r.status === 'blocked') && (
                          <div className="w-5 h-5 rounded-full bg-red-50 flex items-center justify-center">
                            <AlertTriangle className="w-3.5 h-3.5 text-red-400" />
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2.5 min-w-0">
                        <div className="flex items-center gap-1.5">
                          <a
                            href={r.url.startsWith('http') ? r.url : `https://${r.url}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-foreground-800 truncate hover:text-primary-600 transition-colors duration-150 font-mono"
                          >
                            {r.url}
                          </a>
                          <ExternalLink className="w-3 h-3 text-foreground-300 flex-shrink-0" />
                        </div>
                        {r.message && (
                          <p className="text-[10px] text-red-400 mt-0.5">{r.message}</p>
                        )}
                      </td>
                      <td className="px-3 py-2.5 hidden sm:table-cell">
                        {r.title && (
                          <span className="text-xs text-foreground-600">{r.title}</span>
                        )}
                      </td>
                      <td className="px-3 py-2.5 text-right">
                        {r.latencyMs != null && (
                          <span className={cn(
                            'text-xs font-mono',
                            r.latencyMs < 200 ? 'text-green-600' : r.latencyMs < 500 ? 'text-yellow-600' : 'text-red-500'
                          )}>
                            {r.latencyMs}ms
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {results.length === 0 && !checking && managedLinks.length === 0 && (
            <div className="py-8 text-center mt-4">
              <Search className="w-8 h-8 text-foreground-200 mx-auto mb-2" />
              <p className="text-sm text-foreground-400">当前没有管理的链接</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
