import { useMemo, useRef, useState } from 'react';
import { AlertTriangle, CheckCircle2, Download, FileText, Loader2, Upload } from 'lucide-react';
import { useMyPage } from '@/hooks/useQueries';
import { navigationApi } from '@/api/navigation';
import type { ExportFormat, ImportFormat, ImportPreview, ImportResult } from '@/api/types';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { draftSaveToastMessage } from '@/lib/publish-state';

function createIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function inferFormat(file: File): ImportFormat {
  return /\.html?$/i.test(file.name) ? 'bookmarks-html' : 'navax-json';
}

const IMPORT_SITES_ENABLED_KEY = 'navax.import.sitesEnabled';

function readRememberedImportSitesEnabled(): boolean {
  try {
    const raw = localStorage.getItem(IMPORT_SITES_ENABLED_KEY);
    if (raw === 'true') return true;
    if (raw === 'false') return false;
  } catch {
    /* ignore */
  }
  // 8-C default: import as hidden
  return false;
}

export default function ImportExportPage() {
  const pageQuery = useMyPage();
  const { toast } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const idempotencyKeyRef = useRef('');
  const [file, setFile] = useState<File | null>(null);
  const [format, setFormat] = useState<ImportFormat>('bookmarks-html');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [selectedSiteIds, setSelectedSiteIds] = useState<Set<string>>(new Set());
  const [mode, setMode] = useState<'merge' | 'replace'>('merge');
  const [sitesEnabled, setSitesEnabled] = useState(readRememberedImportSitesEnabled);
  const [previewing, setPreviewing] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [exporting, setExporting] = useState<ExportFormat | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);

  const selectableSites = useMemo(
    () => preview?.categories.flatMap(category => category.sites).filter(site => site.valid && !site.duplicate) ?? [],
    [preview],
  );

  if (pageQuery.isLoading) return <LoadingSkeleton count={4} />;
  if (pageQuery.isError || !pageQuery.data) {
    return <ErrorState message={pageQuery.error?.message || '加载页面失败'} onRetry={() => pageQuery.refetch()} />;
  }

  const page = pageQuery.data;
  const pageApi = navigationApi.forPage(page.id);

  const handleFile = (nextFile: File | null) => {
    setFile(nextFile);
    setPreview(null);
    setResult(null);
    setSelectedSiteIds(new Set());
    idempotencyKeyRef.current = '';
    if (nextFile) setFormat(inferFormat(nextFile));
  };

  const handlePreview = async () => {
    if (!file) return;
    setPreviewing(true);
    setResult(null);
    try {
      const response = await pageApi.previewImport(format, file);
      setPreview(response.data);
      setSelectedSiteIds(new Set(
        response.data.categories.flatMap(category => category.sites)
          .filter(site => site.valid && !site.duplicate)
          .map(site => site.sourceId),
      ));
      idempotencyKeyRef.current = createIdempotencyKey();
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '导入预检失败');
    } finally {
      setPreviewing(false);
    }
  };

  const toggleSite = (sourceId: string) => {
    setSelectedSiteIds(current => {
      const next = new Set(current);
      if (next.has(sourceId)) next.delete(sourceId);
      else next.add(sourceId);
      return next;
    });
  };

  const handleCommit = async () => {
    if (!preview || selectedSiteIds.size === 0) return;
    setCommitting(true);
    try {
      // Bookmarks: always send sitesEnabled (UI choice). JSON: omit so file values apply.
      const commitBody = {
        importToken: preview.importToken,
        mode,
        selectedSiteIds: [...selectedSiteIds],
        expectedRevision: page.draftRevision ?? 0,
        ...(format === 'bookmarks-html' ? { sitesEnabled } : {}),
      };
      if (format === 'bookmarks-html') {
        try {
          localStorage.setItem(IMPORT_SITES_ENABLED_KEY, String(sitesEnabled));
        } catch {
          /* ignore */
        }
      }
      const response = await pageApi.commitImport(
        commitBody,
        idempotencyKeyRef.current || createIdempotencyKey(),
      );
      setResult(response.data);
      const refreshed = await pageQuery.refetch();
      const publication = refreshed.data?.publication ?? page.publication;
      const hiddenNote = format === 'bookmarks-html' && !sitesEnabled ? '（默认隐藏，可在链接管理中上架）' : '';
      toast(
        'success',
        `已导入 ${response.data.sitesCreated} 个站点${hiddenNote} · ${draftSaveToastMessage(publication)}`,
      );
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '导入提交失败';
      const detail = cause && typeof cause === 'object' && 'detail' in cause
        ? String((cause as { detail?: string }).detail || '')
        : '';
      toast('error', detail && detail !== message ? `${message}：${detail}` : message);
    } finally {
      setCommitting(false);
    }
  };

  const handleExport = async (exportFormat: ExportFormat) => {
    setExporting(exportFormat);
    try {
      const attachment = await pageApi.exportPage(exportFormat);
      const url = URL.createObjectURL(attachment.blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = attachment.filename;
      anchor.click();
      URL.revokeObjectURL(url);
      toast('success', '导出文件已生成');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '导出失败');
    } finally {
      setExporting(null);
    }
  };

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold font-heading text-foreground-950">导入导出</h1>
        <p className="text-sm text-foreground-400 mt-1">由服务端预检并原子导入，或下载标准附件备份</p>
      </header>

      <section className="bg-white rounded-xl border border-background-200/70 p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-primary-50 flex items-center justify-center">
            <Upload className="w-5 h-5 text-primary-600" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground-900">导入导航数据</h2>
            <p className="text-xs text-foreground-400">支持浏览器书签 HTML 与 nav.ax JSON</p>
          </div>
        </div>

        <input
          ref={fileInputRef}
          type="file"
          accept=".html,.htm,.json"
          className="hidden"
          onChange={event => handleFile(event.target.files?.[0] ?? null)}
        />
        <div className="flex flex-col sm:flex-row gap-2">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="h-10 px-4 rounded-lg border border-background-200 text-sm text-foreground-700 text-left flex-1"
          >
            {file?.name ?? '选择 .html、.htm 或 .json 文件'}
          </button>
          <select
            value={format}
            onChange={event => setFormat(event.target.value as ImportFormat)}
            className="h-10 px-3 rounded-lg border border-background-200 text-sm bg-background-50"
          >
            <option value="bookmarks-html">浏览器书签 HTML</option>
            <option value="navax-json">nav.ax JSON</option>
          </select>
          <button
            type="button"
            onClick={handlePreview}
            disabled={!file || previewing}
            className="h-10 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center justify-center gap-2"
          >
            {previewing && <Loader2 className="w-4 h-4 animate-spin" />}
            预检文件
          </button>
        </div>

        {preview && (
          <div className="space-y-4 border-t border-background-100 pt-4">
            <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
              {[
                ['分类', preview.totals.categories],
                ['站点', preview.totals.sites],
                ['重复', preview.totals.duplicates],
                ['无效', preview.totals.invalid],
                ['已截断', preview.totals.truncated],
              ].map(([label, value]) => (
                <div key={label} className="rounded-lg bg-background-50 p-3">
                  <div className="text-lg font-semibold text-foreground-900">{value}</div>
                  <div className="text-xs text-foreground-400">{label}</div>
                </div>
              ))}
            </div>

            <div className="max-h-80 overflow-y-auto rounded-lg border border-background-200/70 divide-y divide-background-100">
              {preview.categories.map(category => (
                <div key={category.sourceId} className="p-3">
                  <div className="text-sm font-medium text-foreground-800 mb-2 flex items-center gap-2 min-w-0">
                    <span className="truncate">{category.name}</span>
                    {category.truncated && (
                      <span className="shrink-0 text-[10px] font-medium text-amber-700 bg-amber-50 border border-amber-100 px-1.5 py-0.5 rounded">
                        分类名已截断
                      </span>
                    )}
                  </div>
                  <div className="space-y-1.5">
                    {category.sites.map(site => {
                      const selectable = site.valid && !site.duplicate;
                      return (
                        <label key={site.sourceId} className="flex items-start gap-2 text-xs">
                          <input
                            type="checkbox"
                            checked={selectedSiteIds.has(site.sourceId)}
                            disabled={!selectable}
                            onChange={() => toggleSite(site.sourceId)}
                            className="mt-0.5"
                          />
                          <span className="min-w-0 flex-1">
                            <span className="block text-foreground-700 truncate">{site.title}</span>
                            <span className="block text-foreground-400 truncate">{site.url}</span>
                          </span>
                          <span className="shrink-0 flex flex-col items-end gap-0.5">
                            {site.truncated && <span className="text-amber-600">已截断</span>}
                            {site.duplicate && <span className="text-accent-600">重复</span>}
                            {!site.valid && <span className="text-red-500">{site.error || '无效'}</span>}
                          </span>
                        </label>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>

            <div className="flex flex-col sm:flex-row sm:items-center gap-3 flex-wrap">
              <label className="text-xs text-foreground-500">
                导入模式
                <select
                  value={mode}
                  onChange={event => setMode(event.target.value as 'merge' | 'replace')}
                  className="ml-2 h-9 px-3 rounded-lg border border-background-200 bg-background-50 text-sm"
                >
                  <option value="merge">合并到现有数据</option>
                  <option value="replace">替换现有数据</option>
                </select>
              </label>
              {format === 'bookmarks-html' && (
                <label className="text-xs text-foreground-500 inline-flex items-center gap-2">
                  导入后状态
                  <select
                    value={sitesEnabled ? 'enabled' : 'hidden'}
                    onChange={event => setSitesEnabled(event.target.value === 'enabled')}
                    className="h-9 px-3 rounded-lg border border-background-200 bg-background-50 text-sm"
                  >
                    <option value="hidden">隐藏（推荐精品站）</option>
                    <option value="enabled">直接上架</option>
                  </select>
                </label>
              )}
              {format === 'navax-json' && (
                <span className="text-xs text-foreground-400">JSON 将按文件中的 enabled 还原</span>
              )}
              {mode === 'replace' && (
                <span className="inline-flex items-center gap-1 text-xs text-red-500">
                  <AlertTriangle className="w-3.5 h-3.5" />现有分类和站点将被替换
                </span>
              )}
              <button
                type="button"
                onClick={handleCommit}
                disabled={committing || result !== null || selectedSiteIds.size === 0 || selectableSites.length === 0}
                className="sm:ml-auto h-10 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center justify-center gap-2"
              >
                {committing && <Loader2 className="w-4 h-4 animate-spin" />}
                导入已选 {selectedSiteIds.size} 项
              </button>
            </div>
          </div>
        )}

        {result && (
          <div className="rounded-lg bg-green-50 border border-green-100 p-3 flex items-start gap-2 text-sm text-green-700">
            <CheckCircle2 className="w-4 h-4 mt-0.5" />
            新建 {result.categoriesCreated} 个分类、{result.sitesCreated} 个站点；跳过 {result.duplicatesSkipped} 个重复项和 {result.invalidSkipped} 个无效项。
          </div>
        )}
      </section>

      <section className="bg-white rounded-xl border border-background-200/70 p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-accent-50 flex items-center justify-center">
            <Download className="w-5 h-5 text-accent-600" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground-900">下载附件备份</h2>
            <p className="text-xs text-foreground-400">文件名和内容类型由服务端响应决定</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => handleExport('navax-json')}
            disabled={exporting !== null}
            className="h-10 px-4 rounded-lg bg-accent-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center gap-2"
          >
            {exporting === 'navax-json' ? <Loader2 className="w-4 h-4 animate-spin" /> : <FileText className="w-4 h-4" />}
            nav.ax JSON
          </button>
          <button
            type="button"
            onClick={() => handleExport('bookmarks-html')}
            disabled={exporting !== null}
            className="h-10 px-4 rounded-lg border border-background-200 text-sm text-foreground-700 disabled:opacity-50 inline-flex items-center gap-2"
          >
            {exporting === 'bookmarks-html' ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4" />}
            浏览器书签 HTML
          </button>
        </div>
      </section>
    </div>
  );
}
