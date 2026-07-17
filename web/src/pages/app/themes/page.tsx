// ============================================================
// nav.ax App Themes Page — /app/themes
// Theme control + background media library for the page owner.
// ============================================================

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import {
  Check,
  Paintbrush,
  Image as ImageIcon,
  Trash2,
  Upload,
  Film,
  Link2,
  Library,
  UserRound,
} from 'lucide-react';
import { themeRegistry } from '@/themes/registry';
import { useToast } from '@/components/base/Toast';
import { useSaveStatus } from '@/hooks/useSaveStatus';
import { cn } from '@/lib/utils';
import { draftSaveToastMessage } from '@/lib/publish-state';
import type { ThemePackage } from '@/themes/types';
import { useMyPage, useThemes, useUpdatePageSettings } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { getPublicConfig } from '@/api/assets';
import { backgroundsApi } from '@/api/backgrounds';
import { ApiError } from '@/api/client';
import { useAuth } from '@/hooks/useAuth';
import type { BackgroundMedia } from '@/api/types';

const UPLOAD_ACCEPT = 'image/png,image/jpeg,image/jpg,image/gif,image/webp,video/mp4,video/webm';
const DEFAULT_MAX_UPLOAD_BYTES = 40 * 1024 * 1024;
const MAX_USER_LIBRARY = 3;
const MAX_INSTANCE_PRESETS = 12;

import '@/themes/packages';

type BgSourceTab = 'presets' | 'mine' | 'url';

interface BgConfig {
  type: 'none' | 'image' | 'video';
  value: string;
  opacity: number;
  mediaId?: string | null;
  poster?: string | null;
}

function emptyBg(opacity = 0.8): BgConfig {
  return { type: 'none', value: '', opacity, mediaId: null, poster: null };
}

export default function ThemesPage() {
  const { data: page, isLoading: pageLoading, isError, error, refetch } = useMyPage();
  const enabledThemesQuery = useThemes();
  const isLoading = pageLoading || enabledThemesQuery.isLoading;
  const updateSettings = useUpdatePageSettings();
  const { isAdmin } = useAuth();
  const [activeId, setActiveId] = useState('slate');
  const [pendingId, setPendingId] = useState<string | null>(null);
  const { toast } = useToast();
  const { markSaving, markSaved } = useSaveStatus();

  const [bgConfig, setBgConfig] = useState<BgConfig>(emptyBg());
  const bgConfigRef = useRef(bgConfig);
  bgConfigRef.current = bgConfig;

  const [sourceTab, setSourceTab] = useState<BgSourceTab>('presets');
  const [presets, setPresets] = useState<BackgroundMedia[]>([]);
  const [mine, setMine] = useState<BackgroundMedia[]>([]);
  const [libraryLoading, setLibraryLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const presetFileInputRef = useRef<HTMLInputElement>(null);
  const [maxUploadBytes, setMaxUploadBytes] = useState(DEFAULT_MAX_UPLOAD_BYTES);
  const [bgUrlInput, setBgUrlInput] = useState('');

  useEffect(() => {
    getPublicConfig()
      .then(response => {
        // Video uploads allow larger payloads than legacy image-only limit.
        const limit = response.data.limits.maxUploadBytes;
        setMaxUploadBytes(Math.max(limit, DEFAULT_MAX_UPLOAD_BYTES));
      })
      .catch(() => { /* keep default */ });
  }, []);

  const loadLibrary = useCallback(async () => {
    setLibraryLoading(true);
    try {
      const [presetRes, mineRes] = await Promise.all([
        backgroundsApi.listPresets(isAdmin),
        backgroundsApi.listMine(),
      ]);
      setPresets(presetRes.data ?? []);
      setMine(mineRes.data ?? []);
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '加载背景库失败');
    } finally {
      setLibraryLoading(false);
    }
  }, [isAdmin, toast]);

  useEffect(() => {
    void loadLibrary();
  }, [loadLibrary]);

  const themes = useMemo(() => {
    const enabledIds = new Set((enabledThemesQuery.data ?? []).map(theme => theme.id));
    const registered = themeRegistry.list();
    if (enabledIds.size === 0) return registered;
    return registered.filter(pkg => enabledIds.has(pkg.id));
  }, [enabledThemesQuery.data]);

  const seriousThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'serious'), [themes]);
  const cuteThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'cute'), [themes]);

  useEffect(() => {
    if (!page?.settings) return;
    setActiveId(page.settings.appearance.themeId);
    const background = page.settings.appearance.background;
    if (background.type === 'image' || background.type === 'video') {
      setBgConfig({
        type: background.type,
        value: background.value,
        opacity: background.opacity,
        mediaId: background.mediaId ?? null,
        poster: background.poster ?? null,
      });
    } else {
      setBgConfig(emptyBg(background.opacity ?? 0.8));
    }
  }, [page?.settings]);

  const handleActivate = useCallback(async (id: string) => {
    if (id === activeId || !page?.settings) return;
    setPendingId(id);
    markSaving();
    themeRegistry.activate(id);
    try {
      await updateSettings.mutateAsync({
        ...page.settings,
        appearance: { ...page.settings.appearance, themeId: id },
      });
      setActiveId(id);
      markSaved();
      const pkg = themeRegistry.get(id);
      const name = pkg?.meta.name || id;
      toast('success', draftSaveToastMessage(page.publication, `主题已写入草稿：「${name}」`));
    } catch (cause) {
      themeRegistry.activate(activeId);
      toast('error', cause instanceof Error ? cause.message : '主题保存失败');
    } finally {
      setPendingId(null);
    }
  }, [activeId, page?.settings, page?.publication, toast, markSaving, markSaved, updateSettings]);

  const persistBackground = useCallback(async (next: BgConfig) => {
    if (!page?.settings) return;
    await updateSettings.mutateAsync({
      ...page.settings,
      appearance: {
        ...page.settings.appearance,
        background: {
          type: next.type === 'none' || !next.value ? 'none' : next.type,
          value: next.type === 'none' ? '' : next.value,
          opacity: next.opacity,
          mediaId: next.mediaId ?? null,
          poster: next.poster ?? null,
        },
      },
    });
  }, [page?.settings, updateSettings]);

  const applyMedia = useCallback(async (media: BackgroundMedia) => {
    const next: BgConfig = {
      type: media.mediaKind === 'video' ? 'video' : 'image',
      value: media.url,
      opacity: bgConfigRef.current.opacity,
      mediaId: media.id,
      poster: media.posterUrl ?? null,
    };
    setBgConfig(next);
    bgConfigRef.current = next;
    try {
      await persistBackground(next);
      toast(
        'success',
        draftSaveToastMessage(
          page?.publication,
          page?.publication?.published
            ? '背景已写入草稿 · 请到「发布」页点「发布更新」后访客才能看到'
            : '背景已写入草稿 · 发布后生效',
        ),
      );
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '背景保存失败');
    }
  }, [persistBackground, toast, page?.publication]);

  const handleRemoveBg = useCallback(async () => {
    const updated = emptyBg(bgConfigRef.current.opacity);
    setBgConfig(updated);
    bgConfigRef.current = updated;
    setBgUrlInput('');
    try {
      await persistBackground(updated);
      toast('info', draftSaveToastMessage(page?.publication, '背景已从草稿清除'));
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '背景清除失败');
    }
  }, [persistBackground, toast, page?.publication]);

  const handleSaveUrl = useCallback(async (url: string) => {
    const next: BgConfig = {
      type: 'image',
      value: url,
      opacity: bgConfigRef.current.opacity,
      mediaId: null,
      poster: null,
    };
    setBgConfig(next);
    bgConfigRef.current = next;
    try {
      await persistBackground(next);
      toast('success', draftSaveToastMessage(page?.publication));
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '背景图保存失败');
    }
  }, [persistBackground, toast, page?.publication]);

  const uploadErrorMessage = (cause: unknown) => {
    if (cause instanceof ApiError) {
      if (cause.status === 415) return '仅支持 PNG、JPEG、GIF、WebP 图片或 MP4/WebM 视频';
      if (cause.status === 503) return '服务器未安装 ffmpeg 或存储暂不可用，视频背景暂不可用';
      if (cause.status === 422) return cause.message || '文件不符合要求（配额/尺寸/时长）';
    }
    return cause instanceof Error ? cause.message : '上传失败';
  };

  const handleUploadMine = useCallback(async (file: File) => {
    if (mine.length >= MAX_USER_LIBRARY) {
      toast('error', `我的背景最多 ${MAX_USER_LIBRARY} 个，请先删除后再上传`);
      return;
    }
    if (file.size > maxUploadBytes) {
      toast('error', `文件超过上限 ${Math.floor(maxUploadBytes / 1024 / 1024)}MB`);
      return;
    }
    setUploading(true);
    try {
      const response = await backgroundsApi.uploadMine(file);
      const media = response.data;
      setMine(current => [media, ...current]);
      await applyMedia(media);
    } catch (cause) {
      toast('error', uploadErrorMessage(cause));
    } finally {
      setUploading(false);
    }
  }, [mine.length, maxUploadBytes, applyMedia, toast]);

  const handleUploadPreset = useCallback(async (file: File) => {
    if (presets.length >= MAX_INSTANCE_PRESETS) {
      toast('error', `站长预设最多 ${MAX_INSTANCE_PRESETS} 个`);
      return;
    }
    if (file.size > maxUploadBytes) {
      toast('error', `文件超过上限 ${Math.floor(maxUploadBytes / 1024 / 1024)}MB`);
      return;
    }
    setUploading(true);
    try {
      const response = await backgroundsApi.uploadPreset(file);
      setPresets(current => [...current, response.data]);
      toast('success', '预设背景已添加');
    } catch (cause) {
      toast('error', uploadErrorMessage(cause));
    } finally {
      setUploading(false);
    }
  }, [presets.length, maxUploadBytes, toast]);

  const handleDeleteMedia = useCallback(async (media: BackgroundMedia, scope: 'preset' | 'mine') => {
    setDeletingId(media.id);
    try {
      if (scope === 'preset') {
        await backgroundsApi.deletePreset(media.id);
        setPresets(current => current.filter(item => item.id !== media.id));
      } else {
        await backgroundsApi.deleteMine(media.id);
        setMine(current => current.filter(item => item.id !== media.id));
      }
      // Server auto-clears drafts that referenced this media.
      const selected =
        bgConfigRef.current.mediaId === media.id
        || bgConfigRef.current.value === media.url;
      if (selected) {
        const cleared = emptyBg(bgConfigRef.current.opacity);
        setBgConfig(cleared);
        bgConfigRef.current = cleared;
      }
      await refetch();
      toast('info', selected ? '已删除并清空当前背景引用' : '已删除');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '删除失败');
    } finally {
      setDeletingId(null);
    }
  }, [refetch, toast]);

  const handleFilePicked = useCallback((event: React.ChangeEvent<HTMLInputElement>, target: 'mine' | 'preset') => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (target === 'preset') void handleUploadPreset(file);
    else void handleUploadMine(file);
  }, [handleUploadMine, handleUploadPreset]);

  const handleOpacityChange = useCallback((opacity: number) => {
    setBgConfig(current => {
      const next = { ...current, opacity };
      bgConfigRef.current = next;
      return next;
    });
  }, []);

  const handleOpacityCommit = useCallback(() => {
    const next = bgConfigRef.current;
    void persistBackground(next)
      .then(() => {
        toast('success', draftSaveToastMessage(page?.publication));
      })
      .catch(cause => {
        toast('error', cause instanceof Error ? cause.message : '透明度保存失败');
      });
  }, [persistBackground, toast, page?.publication]);

  if (isLoading) return <LoadingSkeleton count={4} />;
  if (isError || !page?.settings) {
    return <ErrorState message={error instanceof Error ? error.message : '加载主题设置失败'} onRetry={() => refetch()} />;
  }

  const hasBackground = bgConfig.type !== 'none' && Boolean(bgConfig.value);
  const previewSrc = bgConfig.type === 'video'
    ? (bgConfig.poster || bgConfig.value)
    : bgConfig.value;

  const renderThemeCard = (pkg: ThemePackage) => {
    const isActive = activeId === pkg.id;
    const isPending = pendingId === pkg.id;

    return (
      <button
        key={pkg.id}
        onClick={() => handleActivate(pkg.id)}
        disabled={isPending}
        className={cn(
          'relative flex flex-col rounded-lg border transition-all duration-200 text-left cursor-pointer',
          isActive && !isPending
            ? 'border-primary-500 ring-1 ring-primary-500/30'
            : 'border-background-200/70 hover:border-background-300',
          isPending && 'opacity-70',
        )}
      >
        <div className="h-10 flex items-end rounded-t-[7px] overflow-hidden">
          {pkg.meta.swatches.map((c, i) => (
            <div
              key={i}
              className="flex-1 h-full"
              style={{ backgroundColor: c, opacity: i === 1 ? 0.85 : 1 }}
            />
          ))}
        </div>

        <div className="px-2.5 py-2 bg-background-50 rounded-b-[7px]">
          <div className="flex items-center justify-between gap-1.5">
            <div className="min-w-0 flex items-baseline gap-1">
              <span className="text-xs font-semibold text-foreground-900 truncate">{pkg.meta.name}</span>
              <span className="text-[9px] text-foreground-400 tracking-wide truncate hidden sm:inline">
                {pkg.meta.subtitle}
              </span>
            </div>
            {isActive && !isPending && (
              <div className="w-4 h-4 rounded-full bg-primary-500 flex items-center justify-center flex-shrink-0">
                <Check className="w-2.5 h-2.5 text-background-50" />
              </div>
            )}
            {isPending && (
              <div className="w-4 h-4 rounded-full border-2 border-primary-400 border-t-transparent animate-spin flex-shrink-0" />
            )}
          </div>
          <p className="mt-0.5 text-[10px] text-foreground-400 leading-snug line-clamp-2">
            {pkg.meta.description}
          </p>
          <span className={cn(
            'inline-block mt-1.5 text-[9px] font-medium px-1.5 py-px rounded-full',
            pkg.meta.vibe === 'cute'
              ? 'bg-pink-50 text-pink-600'
              : 'bg-slate-100 text-slate-600',
          )}>
            {pkg.meta.vibe === 'cute' ? 'Kawaii' : 'Classic'}
          </span>
        </div>
      </button>
    );
  };

  const renderMediaTile = (media: BackgroundMedia, scope: 'preset' | 'mine') => {
    const selected = bgConfig.mediaId === media.id || bgConfig.value === media.url;
    const thumb = media.mediaKind === 'video' ? (media.posterUrl || media.url) : media.url;
    const canDelete = scope === 'mine' || isAdmin;
    const busy = deletingId === media.id;

    return (
      <div
        key={media.id}
        className={cn(
          'relative group rounded-lg overflow-hidden border-2 aspect-[16/10] bg-background-100',
          selected ? 'border-primary-500' : 'border-transparent hover:border-background-300',
        )}
      >
        <button
          type="button"
          onClick={() => void applyMedia(media)}
          className="absolute inset-0 w-full h-full text-left"
          aria-label={`选用背景 ${media.mediaKind}`}
        >
          {media.mediaKind === 'video' && !media.posterUrl ? (
            <div className="w-full h-full flex items-center justify-center bg-slate-800 text-white/80">
              <Film className="w-6 h-6" />
            </div>
          ) : (
            <img
              src={thumb}
              alt=""
              referrerPolicy="no-referrer"
              className="w-full h-full object-cover"
            />
          )}
          {media.mediaKind === 'video' && (
            <span className="absolute left-1.5 top-1.5 inline-flex items-center gap-0.5 rounded-md bg-black/55 px-1.5 py-0.5 text-[10px] font-medium text-white">
              <Film className="w-3 h-3" />
              视频
            </span>
          )}
          {selected && (
            <span className="absolute right-1.5 top-1.5 w-5 h-5 rounded-full bg-primary-500 flex items-center justify-center">
              <Check className="w-3 h-3 text-background-50" />
            </span>
          )}
        </button>
        {canDelete && (
          <button
            type="button"
            disabled={busy}
            onClick={(event) => {
              event.stopPropagation();
              void handleDeleteMedia(media, scope);
            }}
            className="absolute right-1.5 bottom-1.5 h-7 w-7 rounded-md bg-black/55 text-white opacity-0 group-hover:opacity-100 focus:opacity-100 disabled:opacity-40 flex items-center justify-center transition-opacity"
            aria-label="删除背景"
          >
            {busy ? (
              <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
            ) : (
              <Trash2 className="w-3.5 h-3.5" />
            )}
          </button>
        )}
      </div>
    );
  };

  return (
    <div>
      <div className="mb-6">
        <div className="flex items-center gap-2.5 mb-1">
          <div className="w-8 h-8 rounded-lg bg-primary-500 flex items-center justify-center">
            <Paintbrush className="w-4 h-4 text-background-50" />
          </div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">主题设置</h1>
        </div>
        <p className="text-sm text-foreground-400 mt-1">
          选择当前导航页的主题与背景（含「站长精选」）。上传与改动先写入草稿；公开首页要看到效果，请到「发布」页点「发布更新」。
          管理员请确认顶部为「管理主站」（system），不是「我的导航」。共 {themes.length} 套主题
        </p>
      </div>

      {/* Background library */}
      <div className="mb-8">
        <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
          <span className="w-1.5 h-4 rounded-full bg-primary-400" />
          页面背景 · 站长精选 / 我的上传
        </h3>
        <div className="bg-white rounded-xl border border-background-200/70 p-4 space-y-4">
          {/* Current preview */}
          {hasBackground ? (
            <div className="relative rounded-lg overflow-hidden h-28 group">
              {bgConfig.type === 'video' ? (
                <video
                  src={bgConfig.value}
                  poster={bgConfig.poster ?? undefined}
                  className="w-full h-full object-cover"
                  style={{ opacity: Math.min(1, Math.max(0.25, bgConfig.opacity)) }}
                  muted
                  loop
                  playsInline
                  autoPlay
                />
              ) : (
                <img
                  src={previewSrc}
                  alt="背景预览"
                  referrerPolicy="no-referrer"
                  className="w-full h-full object-cover"
                  style={{ opacity: Math.min(1, Math.max(0.25, bgConfig.opacity)) }}
                />
              )}
              <div
                className="absolute inset-0 pointer-events-none"
                style={{
                  background: [
                    'radial-gradient(ellipse 90% 75% at 50% 35%, transparent 35%, rgba(15, 23, 42, 0.22) 100%)',
                    'linear-gradient(to bottom, rgba(255,255,255,0.10) 0%, transparent 40%, rgba(15,23,42,0.14) 100%)',
                  ].join(', '),
                }}
              />
              <div className="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex items-center justify-center gap-2">
                <button
                  onClick={() => void handleRemoveBg()}
                  className="h-8 px-3 rounded-md bg-red-500/90 text-xs font-medium text-background-50 hover:bg-red-500 transition-colors duration-150 whitespace-nowrap"
                >
                  <Trash2 className="w-3.5 h-3.5 inline mr-1" />
                  移除
                </button>
              </div>
              {bgConfig.type === 'video' && (
                <span className="absolute left-2 top-2 inline-flex items-center gap-1 rounded-md bg-black/55 px-2 py-0.5 text-[10px] font-medium text-white">
                  <Film className="w-3 h-3" />
                  视频背景
                </span>
              )}
            </div>
          ) : (
            <div className="rounded-lg border-2 border-dashed border-background-200/70 h-28 flex items-center justify-center bg-background-50">
              <div className="text-center">
                <ImageIcon className="w-6 h-6 text-foreground-300 mx-auto mb-1" />
                <p className="text-xs text-foreground-400">从下方库中选用，或上传你的图/短视频</p>
              </div>
            </div>
          )}

          {/* Source tabs — full width on mobile so labels are easy to spot */}
          <div className="flex items-center gap-1 p-1 rounded-xl bg-background-100 w-full sm:w-fit">
            {([
              { id: 'presets' as const, label: '站长精选', icon: Library },
              { id: 'mine' as const, label: '我的上传', icon: UserRound },
              { id: 'url' as const, label: '外链 URL', icon: Link2 },
            ]).map(tab => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setSourceTab(tab.id)}
                className={cn(
                  'flex-1 sm:flex-none h-9 px-3.5 rounded-lg text-xs font-medium inline-flex items-center justify-center gap-1.5 transition-colors whitespace-nowrap',
                  sourceTab === tab.id
                    ? 'bg-background-50 text-foreground-900 shadow-sm ring-1 ring-primary-200/80'
                    : 'text-foreground-500 hover:text-foreground-700',
                )}
              >
                <tab.icon className="w-3.5 h-3.5" />
                {tab.label}
              </button>
            ))}
          </div>

          <input
            ref={fileInputRef}
            type="file"
            accept={UPLOAD_ACCEPT}
            onChange={event => handleFilePicked(event, 'mine')}
            className="hidden"
            aria-hidden="true"
            tabIndex={-1}
          />
          <input
            ref={presetFileInputRef}
            type="file"
            accept={UPLOAD_ACCEPT}
            onChange={event => handleFilePicked(event, 'preset')}
            className="hidden"
            aria-hidden="true"
            tabIndex={-1}
          />

          {sourceTab === 'presets' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-[11px] text-foreground-400">
                  实例预设（最多 {MAX_INSTANCE_PRESETS}）· 当前 {presets.length}
                </p>
                {isAdmin && (
                  <button
                    type="button"
                    disabled={uploading || presets.length >= MAX_INSTANCE_PRESETS}
                    onClick={() => presetFileInputRef.current?.click()}
                    className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-60 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap inline-flex items-center gap-1.5"
                  >
                    {uploading ? (
                      <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                    ) : (
                      <Upload className="w-3.5 h-3.5" />
                    )}
                    上传预设
                  </button>
                )}
              </div>
              {libraryLoading ? (
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
                  {Array.from({ length: 4 }).map((_, i) => (
                    <div key={i} className="skeleton aspect-[16/10] rounded-lg" />
                  ))}
                </div>
              ) : presets.length === 0 ? (
                <div className="rounded-lg border border-dashed border-background-200/80 py-8 px-4 text-center bg-background-50/80">
                  <Library className="w-6 h-6 text-foreground-300 mx-auto mb-2" />
                  <p className="text-sm text-foreground-600 font-medium">站长精选暂无内容</p>
                  <p className="text-xs text-foreground-400 mt-1">
                    {isAdmin
                      ? '点右上角「上传预设」，或到管理端「平台主题库」维护实例背景库'
                      : '站长尚未上传精选背景，可先用「我的上传」或外链'}
                  </p>
                  {isAdmin && (
                    <Link
                      to="/admin/themes"
                      className="inline-block mt-3 text-xs text-primary-600 hover:underline"
                    >
                      去管理端上传站长精选 →
                    </Link>
                  )}
                </div>
              ) : (
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
                  {presets.map(media => renderMediaTile(media, 'preset'))}
                </div>
              )}
            </div>
          )}

          {sourceTab === 'mine' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-[11px] text-foreground-400">
                  私有库（最多 {MAX_USER_LIBRARY}）· 当前 {mine.length}
                  · 支持 PNG/JPEG/GIF/WebP 与 MP4/WebM（≤15 秒）
                </p>
                <button
                  type="button"
                  disabled={uploading || mine.length >= MAX_USER_LIBRARY}
                  onClick={() => fileInputRef.current?.click()}
                  className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-60 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap inline-flex items-center gap-1.5"
                >
                  {uploading ? (
                    <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                  ) : (
                    <Upload className="w-3.5 h-3.5" />
                  )}
                  {uploading ? '处理中…' : '上传'}
                </button>
              </div>
              {libraryLoading ? (
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="skeleton aspect-[16/10] rounded-lg" />
                  ))}
                </div>
              ) : (
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  {mine.map(media => renderMediaTile(media, 'mine'))}
                  {mine.length < MAX_USER_LIBRARY && (
                    <button
                      type="button"
                      disabled={uploading}
                      onClick={() => fileInputRef.current?.click()}
                      className="aspect-[16/10] rounded-lg border-2 border-dashed border-background-200/70 text-foreground-400 hover:border-primary-300 hover:text-primary-600 flex flex-col items-center justify-center gap-1 text-xs transition-colors"
                    >
                      <Upload className="w-4 h-4" />
                      添加
                    </button>
                  )}
                </div>
              )}
            </div>
          )}

          {sourceTab === 'url' && (
            <div className="space-y-2">
              <p className="text-[11px] text-foreground-400">粘贴外链图片 URL（不经过媒体库压缩与配额）</p>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={bgUrlInput}
                  onChange={e => setBgUrlInput(e.target.value)}
                  placeholder="https://…"
                  className="flex-1 h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
                />
                <button
                  type="button"
                  onClick={() => {
                    if (bgUrlInput.trim()) void handleSaveUrl(bgUrlInput.trim());
                  }}
                  className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
                >
                  应用
                </button>
              </div>
            </div>
          )}

          {hasBackground && (
            <div className="space-y-1 pt-1">
              <div className="flex items-center justify-between">
                <span className="text-[10px] text-foreground-400">背景可见度</span>
                <span className="text-[10px] font-mono text-foreground-600">
                  {Math.round(bgConfig.opacity * 100)}%
                </span>
              </div>
              <input
                type="range"
                min={0.2}
                max={1}
                step={0.05}
                value={bgConfig.opacity}
                onChange={e => handleOpacityChange(Number(e.target.value))}
                onPointerUp={handleOpacityCommit}
                onKeyUp={handleOpacityCommit}
                className="w-full accent-primary-500 h-1"
              />
              <div className="flex justify-between text-[9px] text-foreground-300">
                <span>更淡</span>
                <span>更明显</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {seriousThemes.length > 0 && (
        <div className="mb-8">
          <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
            <span className="w-1.5 h-4 rounded-full bg-slate-400" />
            Classic 经典
          </h3>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
            {seriousThemes.map(renderThemeCard)}
          </div>
        </div>
      )}

      {cuteThemes.length > 0 && (
        <div>
          <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
            <span className="w-1.5 h-4 rounded-full bg-pink-400" />
            Kawaii 可爱
          </h3>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
            {cuteThemes.map(renderThemeCard)}
          </div>
        </div>
      )}
    </div>
  );
}
