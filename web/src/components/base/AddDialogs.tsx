// ============================================================
// nav.ax AddCategoryDialog & AddSiteDialog
// ============================================================

import { useDeferredValue, useState, useEffect, useRef } from 'react';
import { X, Plus, Sparkles, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { PlatformSite, Category } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';
import { FormField, FormInput, FormSelect, FormTextarea, SearchInput } from '@/components/base/FormField';
import { recognizeLink } from '@/lib/linkUtils';
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

// ---- Add Site Dialog ----
interface AddSiteDialogProps {
  open: boolean;
  onClose: () => void;
  categories: Category[];
  defaultCategoryId?: string;
  onConfirm: (data: { title: string; url: string; icon: string; description: string; categoryId: string }) => void;
}

export function AddSiteDialog({ open, onClose, categories, defaultCategoryId, onConfirm }: AddSiteDialogProps) {
  const [title, setTitle] = useState('');
  const [url, setUrl] = useState('');
  const [icon, setIcon] = useState('ri-link');
  const [description, setDescription] = useState('');
  const [categoryId, setCategoryId] = useState(defaultCategoryId || categories[0]?.id || '');
  const [tab, setTab] = useState<'manual' | 'directory'>('manual');
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [recognizing, setRecognizing] = useState(false);
  const [recognizedFavicon, setRecognizedFavicon] = useState('');
  const urlTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const manualEditRef = useRef(false);
  const directoryQuery = usePlatformDirectory(
    { search: deferredSearch || undefined, page: 1, pageSize: 50 },
    open && tab === 'directory',
  );

  useEffect(() => {
    if (manualEditRef.current) return;
    if (urlTimerRef.current) clearTimeout(urlTimerRef.current);

    if (!url.trim()) {
      setRecognizedFavicon('');
      return;
    }

    setRecognizing(true);
    urlTimerRef.current = setTimeout(() => {
      const info = recognizeLink(url);
      if (info) {
        setTitle(info.title);
        setIcon(info.icon);
        setDescription(info.description);
        setRecognizedFavicon(info.faviconUrl);
      }
      setRecognizing(false);
    }, 600);

    return () => {
      if (urlTimerRef.current) clearTimeout(urlTimerRef.current);
    };
  }, [url]);

  const handleTitleChange = (val: string) => {
    manualEditRef.current = true;
    setTitle(val);
  };

  const directorySites = directoryQuery.data?.items ?? [];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || !url.trim()) return;
    onConfirm({ title: title.trim(), url: url.trim(), icon: icon || 'ri-link', description: description.trim(), categoryId });
    reset();
    onClose();
  };

  const handleDirectoryPick = (site: PlatformSite) => {
    setTitle(site.title);
    setUrl(site.url);
    setIcon(site.icon);
    setDescription(site.description);
    setTab('manual');
  };

  const reset = () => {
    setTitle('');
    setUrl('');
    setIcon('ri-link');
    setDescription('');
    setSearch('');
    setTab('manual');
    setRecognizedFavicon('');
    manualEditRef.current = false;
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
        <div className="flex items-center justify-between mb-5">
          <h3 className="text-lg font-semibold text-foreground-900">添加站点</h3>
          <button onClick={handleClose} className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex items-center bg-background-100 rounded-lg p-0.5 mb-4">
          {([
            { key: 'manual' as const, label: '手动添加' },
            { key: 'directory' as const, label: '从平台推荐库' },
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

        {tab === 'manual' ? (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <FormField label="站点名称 *">
                <FormInput
                  type="text"
                  value={title}
                  onChange={e => handleTitleChange(e.target.value)}
                  placeholder="GitHub"
                  autoFocus
                />
              </FormField>
              <FormField label="分类">
                <FormSelect value={categoryId} onChange={e => setCategoryId(e.target.value)}>
                  {categories.map(c => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </FormSelect>
              </FormField>
            </div>

            <FormField label="网址 *">
              <div className="relative">
                <FormInput
                  type="url"
                  value={url}
                  onChange={e => setUrl(e.target.value)}
                  placeholder="https://github.com"
                  className="pr-8"
                />
                {recognizing && (
                  <Loader2 className="w-3.5 h-3.5 absolute right-2.5 top-1/2 -translate-y-1/2 text-primary-500 animate-spin" />
                )}
                {!recognizing && recognizedFavicon && (
                  <img
                    src={recognizedFavicon}
                    alt=""
                    className="w-4 h-4 absolute right-2.5 top-1/2 -translate-y-1/2 rounded-sm"
                    onError={e => { (e.target as HTMLImageElement).style.display = 'none'; }}
                  />
                )}
              </div>
              {!recognizing && recognizedFavicon && (
                <p className="text-[10px] text-accent-500 mt-1 flex items-center gap-1">
                  <Sparkles className="w-3 h-3" />
                  已自动识别站点信息，可手动修改
                </p>
              )}
            </FormField>

            <FormField label="图标">
              <FormInput
                type="text"
                value={icon}
                onChange={e => setIcon(e.target.value)}
                placeholder="ri-github-fill"
              />
            </FormField>

            <FormField label="简介">
              <FormTextarea
                value={description}
                onChange={e => setDescription(e.target.value)}
                maxLength={200}
                rows={2}
              />
            </FormField>

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={handleClose}
                className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
              >
                取消
              </button>
              <button
                type="submit"
                disabled={!title.trim() || !url.trim()}
                className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-all duration-150 flex items-center gap-1.5 whitespace-nowrap"
              >
                <Plus className="w-4 h-4" />
                添加
              </button>
            </div>
          </form>
        ) : (
          <div className="space-y-3">
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
                  <span className="text-xs text-foreground-300">{site.categoryName}</span>
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
