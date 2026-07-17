// ============================================================
// nav.ax AddCategoryDialog & AddSiteDialog
// ============================================================

import { useDeferredValue, useState, useEffect, useRef, useMemo } from 'react';
import { X, Plus, Sparkles, Loader2, ChevronDown, Link2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { PlatformSite, Category } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';
import { FormField, FormInput, FormSelect, FormTextarea, SearchInput } from '@/components/base/FormField';
import { isPlausibleUrl, normalizeUrl, recognizeLink } from '@/lib/linkUtils';
import { linkPreviewApi } from '@/api/linkPreview';
import { usePlatformDirectory } from '@/hooks/useQueries';

// ---- Add Category Dialog ----
interface AddCategoryDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: (name: string, icon: string) => void;
}

export function AddCategoryDialog({ open, onClose, onConfirm }: AddCategoryDialogProps) {
  const [name, setName] = useState('');
  const [icon, setIcon] = useState('ri-code-s-slash-line');

  const iconOptions = [
    'ri-code-s-slash-line', 'ri-palette-line', 'ri-rocket-line', 'ri-book-open-line',
    'ri-chat-3-line', 'ri-newspaper-line', 'ri-shopping-bag-line', 'ri-gamepad-line',
    'ri-cloud-line', 'ri-database-2-line', 'ri-global-line', 'ri-heart-line',
  ];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    onConfirm(name.trim(), icon);
    setName('');
    setIcon('ri-code-s-slash-line');
    onClose();
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative bg-background-50 rounded-xl p-6 w-full max-w-md mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-5">
          <h3 className="text-lg font-semibold text-foreground-900">新建分类</h3>
          <button onClick={onClose} className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <FormField label="分类名称">
            <FormInput
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              autoFocus
              placeholder="例如：开发工具"
            />
          </FormField>

          <FormField label="选择图标">
            <div className="grid grid-cols-6 gap-2">
              {iconOptions.map(opt => (
                <button
                  key={opt}
                  type="button"
                  onClick={() => setIcon(opt)}
                  className={cn(
                    'w-10 h-10 rounded-lg flex items-center justify-center transition-all duration-150 cursor-pointer',
                    icon === opt
                      ? 'bg-primary-100 text-primary-600 ring-1 ring-primary-300'
                      : 'bg-background-50 text-foreground-400 hover:bg-background-100',
                  )}
                >
                  <i className={cn(opt, 'text-lg')} />
                </button>
              ))}
            </div>
          </FormField>

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={!name.trim()}
              className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-all duration-150 flex items-center gap-1.5 whitespace-nowrap"
            >
              <Plus className="w-4 h-4" />
              创建
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ---- Add Site Dialog (URL-first) ----
interface AddSiteDialogProps {
  open: boolean;
  onClose: () => void;
  categories: Category[];
  defaultCategoryId?: string;
  onConfirm: (data: { title: string; url: string; icon: string; description: string; categoryId: string }) => void;
}

function pickDefaultCategoryId(categories: Category[], preferred?: string): string {
  if (preferred && categories.some(c => c.id === preferred)) return preferred;
  const uncategorized = categories.find(c => c.name === '未分类');
  if (uncategorized) return uncategorized.id;
  return categories[0]?.id || '';
}

export function AddSiteDialog({ open, onClose, categories, defaultCategoryId, onConfirm }: AddSiteDialogProps) {
  const [title, setTitle] = useState('');
  const [url, setUrl] = useState('');
  const [icon, setIcon] = useState('');
  const [description, setDescription] = useState('');
  const [categoryId, setCategoryId] = useState(() => pickDefaultCategoryId(categories, defaultCategoryId));
  const [tab, setTab] = useState<'quick' | 'directory'>('quick');
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [recognizing, setRecognizing] = useState(false);
  const [recognizedFavicon, setRecognizedFavicon] = useState('');
  const [showMore, setShowMore] = useState(false);
  const urlTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const titleTouchedRef = useRef(false);
  const descriptionTouchedRef = useRef(false);
  const iconTouchedRef = useRef(false);
  const urlInputRef = useRef<HTMLInputElement>(null);
  const directoryQuery = usePlatformDirectory(
    { search: deferredSearch || undefined, page: 1, pageSize: 50 },
    open && tab === 'directory',
  );

  // Sync category when dialog opens / categories load.
  useEffect(() => {
    if (!open) return;
    setCategoryId(pickDefaultCategoryId(categories, defaultCategoryId));
    // Focus URL on open
    requestAnimationFrame(() => urlInputRef.current?.focus());
  }, [open, categories, defaultCategoryId]);

  useEffect(() => {
    if (urlTimerRef.current) clearTimeout(urlTimerRef.current);
    let cancelled = false;

    if (!url.trim() || !isPlausibleUrl(url)) {
      setRecognizing(false);
      if (!titleTouchedRef.current) setTitle('');
      if (!descriptionTouchedRef.current) setDescription('');
      if (!iconTouchedRef.current) setIcon('');
      setRecognizedFavicon('');
      return;
    }

    setRecognizing(true);
    // Instant client-side guess, then upgrade from server preview.
    const local = recognizeLink(normalizeUrl(url));
    if (local) {
      if (!titleTouchedRef.current) setTitle(local.title);
      if (!descriptionTouchedRef.current) setDescription(local.description);
      if (!iconTouchedRef.current) setIcon(local.icon);
      setRecognizedFavicon(local.faviconUrl);
    }

    urlTimerRef.current = setTimeout(() => {
      void linkPreviewApi
        .preview(normalizeUrl(url))
        .then(response => {
          if (cancelled) return;
          const data = response.data;
          if (!titleTouchedRef.current && data.title) setTitle(data.title);
          if (!descriptionTouchedRef.current) setDescription(data.description || '');
          if (!iconTouchedRef.current && data.faviconUrl) setIcon(data.faviconUrl);
          if (data.faviconUrl) setRecognizedFavicon(data.faviconUrl);
        })
        .catch(() => {
          // Keep client-side recognition; silent on network / SSRF / rate-limit.
        })
        .finally(() => {
          if (!cancelled) setRecognizing(false);
        });
    }, 400);

    return () => {
      cancelled = true;
      if (urlTimerRef.current) clearTimeout(urlTimerRef.current);
    };
  }, [url]);

  const directorySites = directoryQuery.data?.items ?? [];

  const canSubmit = useMemo(() => {
    if (!isPlausibleUrl(url)) return false;
    if (!categoryId) return false;
    // Title optional in UI — we fill from domain on submit if empty
    return true;
  }, [url, categoryId]);

  const previewTitle = title.trim() || (isPlausibleUrl(url) ? (recognizeLink(normalizeUrl(url))?.title ?? '') : '');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    const finalUrl = normalizeUrl(url);
    const info = recognizeLink(finalUrl);
    const finalTitle = title.trim() || info?.title || finalUrl;
    const finalIcon = icon.trim() || info?.faviconUrl || info?.icon || 'ri-link';
    const finalDescription = description.trim();
    onConfirm({
      title: finalTitle,
      url: finalUrl,
      icon: finalIcon,
      description: finalDescription,
      categoryId,
    });
    reset();
    onClose();
  };

  const handleDirectoryPick = (site: PlatformSite) => {
    // One-click add from directory with smart defaults.
    const targetCategory = categoryId || pickDefaultCategoryId(categories, defaultCategoryId);
    if (!targetCategory) return;
    onConfirm({
      title: site.title,
      url: site.url,
      icon: site.icon || '',
      description: site.description || '',
      categoryId: targetCategory,
    });
    reset();
    onClose();
  };

  const reset = () => {
    setTitle('');
    setUrl('');
    setIcon('');
    setDescription('');
    setSearch('');
    setTab('quick');
    setShowMore(false);
    setRecognizedFavicon('');
    titleTouchedRef.current = false;
    descriptionTouchedRef.current = false;
    iconTouchedRef.current = false;
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={handleClose} />
      <div className="relative bg-background-50 rounded-xl p-6 w-full max-w-lg mx-4 max-h-[85vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-foreground-900">添加站点</h3>
            <p className="text-[11px] text-foreground-400 mt-0.5">粘贴链接即可，名称与图标会自动填充</p>
          </div>
          <button onClick={handleClose} className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="flex items-center bg-background-100 rounded-lg p-0.5 mb-4">
          {([
            { key: 'quick' as const, label: '快速添加' },
            { key: 'directory' as const, label: '从推荐库' },
          ]).map(t => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={cn(
                'flex-1 h-8 rounded-md text-xs font-medium transition-all duration-200 whitespace-nowrap cursor-pointer',
                tab === t.key
                  ? 'bg-background-50 text-foreground-900 shadow-raised'
                  : 'text-foreground-400 hover:text-foreground-600',
              )}
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab === 'quick' ? (
          <form onSubmit={handleSubmit} className="space-y-3">
            {/* Primary: URL */}
            <FormField label="链接">
              <div className="relative">
                <Link2 className="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-foreground-300 pointer-events-none" />
                <FormInput
                  ref={urlInputRef}
                  type="text"
                  inputMode="url"
                  autoComplete="url"
                  value={url}
                  onChange={e => setUrl(e.target.value)}
                  placeholder="粘贴或输入 URL，如 github.com"
                  className="pl-9 pr-9"
                  autoFocus
                />
                {recognizing && (
                  <Loader2 className="w-3.5 h-3.5 absolute right-2.5 top-1/2 -translate-y-1/2 text-primary-500 animate-spin" />
                )}
                {!recognizing && recognizedFavicon && (
                  <img
                    src={recognizedFavicon}
                    alt=""
                    width={16}
                    height={16}
                    className="w-4 h-4 absolute right-2.5 top-1/2 -translate-y-1/2 rounded-sm object-contain"
                    onError={e => { (e.target as HTMLImageElement).style.display = 'none'; }}
                  />
                )}
              </div>
            </FormField>

            {/* Live preview chip */}
            {isPlausibleUrl(url) && previewTitle && (
              <div className="flex items-center gap-2.5 rounded-lg border border-background-200/70 bg-background-50 px-3 py-2">
                <div className="w-8 h-8 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0 overflow-hidden">
                  {(icon || recognizedFavicon) ? (
                    <IconRenderer icon={icon || recognizedFavicon} size={20} alt="" />
                  ) : (
                    <Link2 className="w-4 h-4 text-foreground-300" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-foreground-900 truncate">{previewTitle}</div>
                  <div className="text-[11px] text-foreground-400 truncate">{normalizeUrl(url)}</div>
                </div>
                {recognizing ? (
                  <span className="text-[10px] text-foreground-400 inline-flex items-center gap-0.5 flex-shrink-0">
                    <Loader2 className="w-3 h-3 animate-spin" />
                    抓取中
                  </span>
                ) : (
                  <span className="text-[10px] text-accent-600 inline-flex items-center gap-0.5 flex-shrink-0">
                    <Sparkles className="w-3 h-3" />
                    已识别
                  </span>
                )}
              </div>
            )}

            {/* Category — compact, smart default */}
            <FormField label="放到分类">
              <FormSelect value={categoryId} onChange={e => setCategoryId(e.target.value)}>
                {categories.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </FormSelect>
            </FormField>

            {/* Advanced — collapsed */}
            <div className="rounded-lg border border-background-200/60 overflow-hidden">
              <button
                type="button"
                onClick={() => setShowMore(v => !v)}
                className="w-full flex items-center justify-between px-3 py-2 text-xs text-foreground-500 hover:bg-background-50 transition-colors"
              >
                <span>更多选项（名称 / 简介 / 图标）</span>
                <ChevronDown className={cn('w-3.5 h-3.5 transition-transform', showMore && 'rotate-180')} />
              </button>
              {showMore && (
                <div className="px-3 pb-3 space-y-3 border-t border-background-100 pt-3">
                  <FormField label="名称">
                    <FormInput
                      type="text"
                      value={title}
                      onChange={e => {
                        titleTouchedRef.current = true;
                        setTitle(e.target.value);
                      }}
                      placeholder="留空则用自动识别"
                    />
                  </FormField>
                  <FormField label="简介">
                    <FormTextarea
                      value={description}
                      onChange={e => {
                        descriptionTouchedRef.current = true;
                        setDescription(e.target.value);
                      }}
                      maxLength={200}
                      rows={2}
                      placeholder="可选"
                    />
                  </FormField>
                  <FormField label="图标">
                    <div className="flex items-center gap-2">
                      <div className="w-9 h-9 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0 overflow-hidden">
                        <IconRenderer icon={icon || recognizedFavicon || 'ri-link'} size={20} />
                      </div>
                      <FormInput
                        type="text"
                        value={icon}
                        onChange={e => {
                          iconTouchedRef.current = true;
                          setIcon(e.target.value);
                        }}
                        placeholder="自动 favicon，或填 Remix 名 / 图片 URL"
                        className="flex-1"
                      />
                    </div>
                    <p className="text-[10px] text-foreground-300 mt-1">默认使用站点 favicon，一般无需修改</p>
                  </FormField>
                </div>
              )}
            </div>

            <div className="flex justify-end gap-3 pt-1">
              <button
                type="button"
                onClick={handleClose}
                className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
              >
                取消
              </button>
              <button
                type="submit"
                disabled={!canSubmit}
                className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-all duration-150 flex items-center gap-1.5 whitespace-nowrap"
              >
                <Plus className="w-4 h-4" />
                添加
              </button>
            </div>
          </form>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <span className="text-[11px] text-foreground-400 flex-shrink-0">加入分类</span>
              <FormSelect value={categoryId} onChange={e => setCategoryId(e.target.value)} className="flex-1">
                {categories.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </FormSelect>
            </div>
            <SearchInput
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="搜索平台推荐站点..."
            />
            <div className="max-h-64 overflow-y-auto space-y-1">
              {directorySites.map(site => (
                <button
                  key={site.id}
                  type="button"
                  onClick={() => handleDirectoryPick(site)}
                  className="w-full flex items-center gap-3 p-3 rounded-lg hover:bg-background-50 transition-colors duration-150 text-left cursor-pointer"
                >
                  <div className="w-8 h-8 rounded-lg bg-background-100 flex items-center justify-center flex-shrink-0">
                    <IconRenderer icon={site.icon} className="text-base text-foreground-500" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-foreground-900">{site.title}</div>
                    <div className="text-xs text-foreground-400 truncate">{site.description}</div>
                  </div>
                  <span className="text-[10px] text-primary-600 font-medium flex-shrink-0">添加</span>
                </button>
              ))}
              {directoryQuery.isLoading && (
                <div className="py-8 flex items-center justify-center gap-2 text-sm text-foreground-400">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  正在加载推荐站点
                </div>
              )}
              {directoryQuery.isError && (
                <div className="py-8 text-center text-sm text-red-500">推荐目录加载失败，请稍后重试</div>
              )}
              {!directoryQuery.isLoading && !directoryQuery.isError && directorySites.length === 0 && (
                <div className="py-8 text-center text-sm text-foreground-400">没有匹配的站点</div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
