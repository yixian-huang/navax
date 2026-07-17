// ============================================================
// nav.ax Admin Themes Page — /admin/themes
// Platform theme enablement + instance background presets.
// ============================================================

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Check, Paintbrush, Upload, Trash2, Film, Image as ImageIcon, Library } from 'lucide-react';
import { Link } from 'react-router-dom';
import { themeRegistry } from '@/themes/registry';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { ThemePackage } from '@/themes/types';
import { useAdminThemes, useUpdateAdminThemeState } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { backgroundsApi } from '@/api/backgrounds';
import { ApiError } from '@/api/client';
import type { BackgroundMedia } from '@/api/types';

import '@/themes/packages';

const UPLOAD_ACCEPT = 'image/png,image/jpeg,image/jpg,image/gif,image/webp,video/mp4,video/webm';
const MAX_INSTANCE_PRESETS = 12;
const MAX_UPLOAD_BYTES = 40 * 1024 * 1024;

export default function AdminThemesPage() {
  const { data: platformThemes, isLoading, isError, error, refetch } = useAdminThemes();
  const updateTheme = useUpdateAdminThemeState();
  const themes = useMemo(() => themeRegistry.list(), []);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const { toast } = useToast();
  const activeId = platformThemes?.find(theme => theme.default || theme.isDefault)?.id ?? 'slate';
  const enabledMap = useMemo(() => {
    const map = new Map<string, boolean>();
    for (const theme of platformThemes ?? []) {
      map.set(theme.id, theme.enabled !== false);
    }
    return map;
  }, [platformThemes]);

  const seriousThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'serious'), [themes]);
  const cuteThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'cute'), [themes]);

  // Instance background presets (站长精选)
  const [presets, setPresets] = useState<BackgroundMedia[]>([]);
  const [presetsLoading, setPresetsLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadPresets = useCallback(async () => {
    setPresetsLoading(true);
    try {
      const res = await backgroundsApi.listPresets(true);
      setPresets(res.data ?? []);
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '加载站长精选失败');
    } finally {
      setPresetsLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void loadPresets();
  }, [loadPresets]);

  const handleActivate = useCallback(async (id: string) => {
    if (id === activeId) return;
    setPendingId(id);
    themeRegistry.activate(id);
    try {
      await updateTheme.mutateAsync({ themeId: id, data: { enabled: true, default: true } });
      setPendingId(null);
      const pkg = themeRegistry.get(id);
      toast('success', `默认主题已切换为「${pkg?.meta.name || id}」`);
    } catch (cause) {
      themeRegistry.activate(activeId);
      toast('error', cause instanceof Error ? cause.message : '主题切换失败');
      setPendingId(null);
    }
  }, [activeId, toast, updateTheme]);

  const handleToggleEnabled = useCallback(async (id: string, enabled: boolean) => {
    if (id === activeId && !enabled) {
      toast('error', '默认主题不可停用，请先切换默认主题');
      return;
    }
    setPendingId(id);
    try {
      await updateTheme.mutateAsync({ themeId: id, data: { enabled } });
      toast('success', enabled ? '主题已启用' : '主题已停用');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '更新主题状态失败');
    } finally {
      setPendingId(null);
    }
  }, [activeId, toast, updateTheme]);

  const handleUploadPreset = useCallback(async (file: File) => {
    if (presets.length >= MAX_INSTANCE_PRESETS) {
      toast('error', `站长精选最多 ${MAX_INSTANCE_PRESETS} 个`);
      return;
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      toast('error', `文件超过上限 ${Math.floor(MAX_UPLOAD_BYTES / 1024 / 1024)}MB`);
      return;
    }
    setUploading(true);
    try {
      const res = await backgroundsApi.uploadPreset(file);
      setPresets(current => [...current, res.data]);
      toast('success', '已加入站长精选');
    } catch (cause) {
      const message = cause instanceof ApiError && cause.status === 503
        ? '服务器未安装 ffmpeg，暂不支持视频'
        : cause instanceof ApiError && cause.status === 415
          ? '仅支持 PNG/JPEG/GIF/WebP 或 MP4/WebM'
          : cause instanceof Error ? cause.message : '上传失败';
      toast('error', message);
    } finally {
      setUploading(false);
    }
  }, [presets.length, toast]);

  const handleDeletePreset = useCallback(async (media: BackgroundMedia) => {
    setDeletingId(media.id);
    try {
      await backgroundsApi.deletePreset(media.id);
      setPresets(current => current.filter(item => item.id !== media.id));
      toast('info', '已删除（引用该背景的草稿会自动清空）');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '删除失败');
    } finally {
      setDeletingId(null);
    }
  }, [toast]);

  if (isLoading) return <LoadingSkeleton count={4} />;
  if (isError) {
    return <ErrorState message={error instanceof Error ? error.message : '加载主题失败'} onRetry={() => refetch()} />;
  }

  const renderThemeCard = (pkg: ThemePackage) => {
    const isActive = activeId === pkg.id;
    const isPending = pendingId === pkg.id;
    const isEnabled = enabledMap.get(pkg.id) ?? true;

    return (
      <div
        key={pkg.id}
        className={cn(
          'relative flex flex-col rounded-xl border-2 transition-all duration-200 text-left',
          isActive && !isPending
            ? 'border-primary-500'
            : 'border-background-200/70',
          !isEnabled && 'opacity-60',
          isPending && 'opacity-70',
        )}
      >
        <button
          type="button"
          onClick={() => handleActivate(pkg.id)}
          disabled={isPending}
          className="h-16 flex items-end rounded-t-[10px] overflow-hidden cursor-pointer"
          aria-label={`设为默认主题 ${pkg.meta.name}`}
        >
          {pkg.meta.swatches.map((c, i) => (
            <div
              key={i}
              className="flex-1 h-full"
              style={{ backgroundColor: c, opacity: i === 1 ? 0.85 : 1 }}
            />
          ))}
        </button>

        <div className="p-3.5 bg-background-50 rounded-b-[10px]">
          <div className="flex items-center justify-between mb-1">
            <div className="flex items-center gap-1.5">
              <span className="text-sm font-semibold text-foreground-900">{pkg.meta.name}</span>
              <span className="text-[10px] text-foreground-400 tracking-wide">{pkg.meta.subtitle}</span>
            </div>
            {isActive && !isPending && (
              <div className="w-6 h-6 rounded-full bg-primary-500 flex items-center justify-center flex-shrink-0">
                <Check className="w-3.5 h-3.5 text-background-50" />
              </div>
            )}
            {isPending && (
              <div className="w-6 h-6 rounded-full border-2 border-primary-400 border-t-transparent animate-spin flex-shrink-0" />
            )}
          </div>
          <p className="text-[11px] text-foreground-400 leading-relaxed">
            {pkg.meta.description}
          </p>
          <div className="mt-2 flex items-center justify-between gap-2">
            <span className={cn(
              'inline-block text-[10px] font-medium px-2 py-0.5 rounded-full',
              pkg.meta.vibe === 'cute'
                ? 'bg-pink-50 text-pink-600'
                : 'bg-slate-100 text-slate-600',
            )}>
              {pkg.meta.vibe === 'cute' ? 'Kawaii' : 'Classic'}
            </span>
            <button
              type="button"
              role="switch"
              aria-checked={isEnabled}
              aria-label={`${isEnabled ? '停用' : '启用'}主题 ${pkg.meta.name}`}
              disabled={isPending || isActive}
              onClick={() => handleToggleEnabled(pkg.id, !isEnabled)}
              className={cn(
                'relative w-9 h-5 rounded-full transition-colors disabled:opacity-40',
                isEnabled ? 'bg-primary-500' : 'bg-background-300',
              )}
            >
              <span className={cn(
                'absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                isEnabled && 'translate-x-4',
              )} />
            </button>
          </div>
        </div>
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
          <h1 className="text-xl font-bold font-heading text-foreground-950">平台主题库</h1>
        </div>
        <p className="text-xs text-foreground-400 mt-0.5">
          管理实例主题包与「站长精选」背景。用户在工作台
          <Link to="/app/themes" className="text-primary-600 hover:underline mx-0.5">主题设置</Link>
          中选用背景 · 共 {themes.length} 套主题
        </p>
      </div>

      {/* 站长精选背景库 */}
      <div className="mb-8">
        <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
          <span className="w-1.5 h-4 rounded-full bg-primary-400" />
          <Library className="w-4 h-4 text-primary-500" />
          站长精选背景
          <span className="text-[11px] font-normal text-foreground-400">
            最多 {MAX_INSTANCE_PRESETS} · 当前 {presets.length}
          </span>
        </h3>
        <div className="bg-white rounded-xl border border-background-200/70 p-4 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-[11px] text-foreground-400">
              上传后所有用户可在工作台「主题设置 → 站长精选」中选用。支持图片与 ≤15s 视频（MP4/WebM）。
            </p>
            <button
              type="button"
              disabled={uploading || presets.length >= MAX_INSTANCE_PRESETS}
              onClick={() => fileInputRef.current?.click()}
              className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-60 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap inline-flex items-center gap-1.5"
            >
              {uploading ? (
                <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
              ) : (
                <Upload className="w-3.5 h-3.5" />
              )}
              {uploading ? '处理中…' : '上传预设'}
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept={UPLOAD_ACCEPT}
              className="hidden"
              onChange={(event) => {
                const file = event.target.files?.[0];
                event.target.value = '';
                if (file) void handleUploadPreset(file);
              }}
            />
          </div>

          {presetsLoading ? (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="skeleton aspect-[16/10] rounded-lg" />
              ))}
            </div>
          ) : presets.length === 0 ? (
            <div className="rounded-lg border-2 border-dashed border-background-200/70 py-10 flex flex-col items-center justify-center bg-background-50">
              <ImageIcon className="w-7 h-7 text-foreground-300 mb-2" />
              <p className="text-sm text-foreground-600 font-medium">还没有站长精选</p>
              <p className="text-xs text-foreground-400 mt-1 mb-3">点「上传预设」添加第一张背景图或短视频</p>
              <button
                type="button"
                disabled={uploading}
                onClick={() => fileInputRef.current?.click()}
                className="h-8 px-3 rounded-lg border border-background-200 text-xs text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1.5"
              >
                <Upload className="w-3.5 h-3.5" />
                上传预设
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
              {presets.map(media => {
                const thumb = media.mediaKind === 'video' ? (media.posterUrl || media.url) : media.url;
                const busy = deletingId === media.id;
                return (
                  <div
                    key={media.id}
                    className="relative group rounded-lg overflow-hidden border border-background-200/70 aspect-[16/10] bg-background-100"
                  >
                    {media.mediaKind === 'video' && !media.posterUrl ? (
                      <div className="w-full h-full flex items-center justify-center bg-slate-800 text-white/80">
                        <Film className="w-6 h-6" />
                      </div>
                    ) : (
                      <img src={thumb} alt="" className="w-full h-full object-cover" referrerPolicy="no-referrer" />
                    )}
                    {media.mediaKind === 'video' && (
                      <span className="absolute left-1.5 top-1.5 inline-flex items-center gap-0.5 rounded-md bg-black/55 px-1.5 py-0.5 text-[10px] font-medium text-white">
                        <Film className="w-3 h-3" />
                        视频
                      </span>
                    )}
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => void handleDeletePreset(media)}
                      className="absolute right-1.5 bottom-1.5 h-7 w-7 rounded-md bg-black/55 text-white opacity-0 group-hover:opacity-100 focus:opacity-100 disabled:opacity-40 flex items-center justify-center transition-opacity"
                      aria-label="删除预设"
                    >
                      {busy ? (
                        <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                      ) : (
                        <Trash2 className="w-3.5 h-3.5" />
                      )}
                    </button>
                  </div>
                );
              })}
              {presets.length < MAX_INSTANCE_PRESETS && (
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
      </div>

      {seriousThemes.length > 0 && (
        <div className="mb-8">
          <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
            <span className="w-1.5 h-4 rounded-full bg-slate-400" />
            Classic 经典
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
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
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {cuteThemes.map(renderThemeCard)}
          </div>
        </div>
      )}
    </div>
  );
}
