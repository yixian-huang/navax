// ============================================================
// nav.ax App Themes Page — /app/themes
// Theme control + background image for the navigation page owner.
// Uses real ThemeRegistry (7 themes), consistent with admin panel.
// ============================================================

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Check, Paintbrush, Image as ImageIcon, Trash2, Upload } from 'lucide-react';
import { themeRegistry } from '@/themes/registry';
import { useToast } from '@/components/base/Toast';
import { useSaveStatus } from '@/hooks/useSaveStatus';
import { cn } from '@/lib/utils';
import type { ThemePackage } from '@/themes/types';
import { useMyPage, useUpdatePageSettings } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { assetsApi, getPublicConfig } from '@/api/assets';
import { ApiError } from '@/api/client';

const UPLOAD_ACCEPT = 'image/png,image/jpeg,image/gif';
const DEFAULT_MAX_UPLOAD_BYTES = 5 * 1024 * 1024;

import '@/themes/packages';

interface BgConfig {
  image: string;
  opacity: number;
}

export default function ThemesPage() {
  const { data: page, isLoading, isError, error, refetch } = useMyPage();
  const updateSettings = useUpdatePageSettings();
  const themes = useMemo(() => themeRegistry.list(), []);
  const [activeId, setActiveId] = useState('slate');
  const [pendingId, setPendingId] = useState<string | null>(null);
  const { toast } = useToast();
  const { markSaving, markSaved } = useSaveStatus();

  // Background image state
  const [bgConfig, setBgConfig] = useState<BgConfig>({ image: '', opacity: 0.15 });
  const [bgUrlInput, setBgUrlInput] = useState('');
  const [showBgUrlInput, setShowBgUrlInput] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [maxUploadBytes, setMaxUploadBytes] = useState(DEFAULT_MAX_UPLOAD_BYTES);

  useEffect(() => {
    getPublicConfig()
      .then(response => setMaxUploadBytes(response.data.limits.maxUploadBytes))
      .catch(() => { /* 保留默认上限，服务端仍会二次校验 */ });
  }, []);

  const seriousThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'serious'), [themes]);
  const cuteThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'cute'), [themes]);

  // Sync bgInput with current config when opening the input
  useEffect(() => {
    setBgUrlInput(bgConfig.image || '');
  }, [bgConfig.image, showBgUrlInput]);

  useEffect(() => {
    if (!page?.settings) return;
    setActiveId(page.settings.appearance.themeId);
    const background = page.settings.appearance.background;
    setBgConfig({
      image: background.type === 'image' ? background.value : '',
      opacity: background.opacity,
    });
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
      toast('success', `主题已切换为「${pkg?.meta.name || id}」`);
    } catch (cause) {
      themeRegistry.activate(activeId);
      toast('error', cause instanceof Error ? cause.message : '主题保存失败');
    } finally {
      setPendingId(null);
    }
  }, [activeId, page?.settings, toast, markSaving, markSaved, updateSettings]);

  const persistBackground = useCallback(async (next: BgConfig) => {
    if (!page?.settings) return;
    await updateSettings.mutateAsync({
      ...page.settings,
      appearance: {
        ...page.settings.appearance,
        background: {
          type: next.image ? 'image' : 'none',
          value: next.image,
          opacity: next.opacity,
        },
      },
    });
  }, [page?.settings, updateSettings]);

  const handleSaveBg = useCallback(async (image: string) => {
    const updated = { ...bgConfig, image };
    setBgConfig(updated);
    try {
      await persistBackground(updated);
      toast('success', '背景图已更新');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '背景图保存失败');
    }
  }, [bgConfig, persistBackground, toast]);

  const handleRemoveBg = useCallback(async () => {
    const updated = { image: '', opacity: 0.15 };
    setBgConfig(updated);
    setBgUrlInput('');
    try {
      await persistBackground(updated);
      toast('info', '背景图已清除');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '背景图清除失败');
    }
  }, [persistBackground, toast]);

  const handleUploadBg = useCallback(async (file: File) => {
    if (file.size > maxUploadBytes) {
      toast('error', `图片超过上限 ${Math.floor(maxUploadBytes / 1024 / 1024)}MB`);
      return;
    }
    setUploading(true);
    try {
      const response = await assetsApi.upload('background', file);
      await handleSaveBg(response.data.url);
    } catch (cause) {
      const message = cause instanceof ApiError && cause.status === 415
        ? '仅支持 PNG、JPEG 或 GIF 图片'
        : cause instanceof Error ? cause.message : '背景图上传失败';
      toast('error', message);
    } finally {
      setUploading(false);
    }
  }, [maxUploadBytes, handleSaveBg, toast]);

  const handleFilePicked = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = ''; // 允许重复选择同一文件
    if (file) void handleUploadBg(file);
  }, [handleUploadBg]);

  const handleOpacityChange = useCallback((opacity: number) => {
    setBgConfig(current => ({ ...current, opacity }));
  }, []);

  const handleOpacityCommit = useCallback(() => {
    void persistBackground(bgConfig).catch(cause => {
      toast('error', cause instanceof Error ? cause.message : '透明度保存失败');
    });
  }, [bgConfig, persistBackground, toast]);

  if (isLoading) return <LoadingSkeleton count={4} />;
  if (isError || !page?.settings) {
    return <ErrorState message={error instanceof Error ? error.message : '加载主题设置失败'} onRetry={() => refetch()} />;
  }

  const renderThemeCard = (pkg: ThemePackage) => {
    const isActive = activeId === pkg.id;
    const isPending = pendingId === pkg.id;

    return (
      <button
        key={pkg.id}
        onClick={() => handleActivate(pkg.id)}
        disabled={isPending}
        className={cn(
          'relative flex flex-col rounded-xl border-2 transition-all duration-200 text-left cursor-pointer',
          isActive && !isPending
            ? 'border-primary-500'
            : 'border-background-200/70 hover:border-background-300',
          isPending && 'opacity-70'
        )}
      >
        <div className="h-16 flex items-end rounded-t-[10px] overflow-hidden">
          {pkg.meta.swatches.map((c, i) => (
            <div
              key={i}
              className="flex-1 h-full"
              style={{ backgroundColor: c, opacity: i === 1 ? 0.85 : 1 }}
            />
          ))}
        </div>

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
          <span className={cn(
            'inline-block mt-2 text-[10px] font-medium px-2 py-0.5 rounded-full',
            pkg.meta.vibe === 'cute'
              ? 'bg-pink-50 text-pink-600'
              : 'bg-slate-100 text-slate-600'
          )}>
            {pkg.meta.vibe === 'cute' ? 'Kawaii' : 'Classic'}
          </span>
        </div>
      </button>
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
          选择导航站的全局主题风格，切换后立即对首页生效 · 共 {themes.length} 套主题
        </p>
      </div>

      {/* Background Image Section */}
      <div className="mb-8">
        <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
          <span className="w-1.5 h-4 rounded-full bg-primary-400" />
          页面背景
        </h3>
        <div className="bg-white rounded-xl border border-background-200/70 p-4 space-y-3">
          {/* Preview / Empty state */}
          {bgConfig.image ? (
            <div className="relative rounded-lg overflow-hidden h-28 group">
              <img
                src={bgConfig.image}
                alt="背景预览"
                className="w-full h-full object-cover"
              />
              <div
                className="absolute inset-0 pointer-events-none"
                style={{ backgroundColor: `rgba(255,255,255,${1 - bgConfig.opacity})` }}
              />
              <div className="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex items-center justify-center gap-2">
                <button
                  onClick={handleRemoveBg}
                  className="h-8 px-3 rounded-md bg-red-500/90 text-xs font-medium text-background-50 hover:bg-red-500 transition-colors duration-150 whitespace-nowrap"
                >
                  <Trash2 className="w-3.5 h-3.5 inline mr-1" />
                  移除
                </button>
              </div>
            </div>
          ) : (
            <div className="rounded-lg border-2 border-dashed border-background-200/70 h-28 flex items-center justify-center bg-background-50">
              <div className="text-center">
                <ImageIcon className="w-6 h-6 text-foreground-300 mx-auto mb-1" />
                <p className="text-xs text-foreground-400">给你的导航页加点氛围</p>
              </div>
            </div>
          )}

          <input
            ref={fileInputRef}
            type="file"
            accept={UPLOAD_ACCEPT}
            onChange={handleFilePicked}
            className="hidden"
            aria-hidden="true"
            tabIndex={-1}
          />

          {/* Set background */}
          {!showBgUrlInput ? (
            <div className="flex items-center gap-2">
              <button
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
                className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-60 disabled:cursor-not-allowed transition-colors duration-150 whitespace-nowrap inline-flex items-center gap-1.5"
              >
                {uploading ? (
                  <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                ) : (
                  <Upload className="w-3.5 h-3.5" />
                )}
                {uploading ? '上传中…' : bgConfig.image ? '上传新图片' : '上传图片'}
              </button>
              <button
                onClick={() => setShowBgUrlInput(true)}
                className="h-8 px-3 rounded-lg border border-background-200/70 text-xs text-foreground-600 hover:bg-background-50 transition-colors duration-150 whitespace-nowrap"
              >
                使用图片 URL
              </button>
              {bgConfig.image && (
                <button
                  onClick={handleRemoveBg}
                  className="h-8 px-3 rounded-lg text-xs text-red-500 hover:bg-red-50 transition-colors duration-150 whitespace-nowrap"
                >
                  清除
                </button>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={bgUrlInput}
                  onChange={e => setBgUrlInput(e.target.value)}
                  placeholder="粘贴图片 URL 或选下方预设..."
                  className="flex-1 h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
                />
                <button
                  onClick={() => {
                    if (bgUrlInput.trim()) handleSaveBg(bgUrlInput.trim());
                    setShowBgUrlInput(false);
                  }}
                  className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
                >
                  确定
                </button>
                <button
                  onClick={() => setShowBgUrlInput(false)}
                  className="h-9 px-3 rounded-lg text-xs text-foreground-500 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
                >
                  取消
                </button>
              </div>
              {/* Presets */}
              <div className="flex flex-wrap gap-1.5">
                {[
                  { label: '渐变暖调', url: 'https://readdy.ai/api/search-image?query=Abstract%20smooth%20gradient%20background%20with%20soft%20warm%20beige%20and%20cream%20tones%2C%20subtle%20organic%20curves%2C%20minimalist%20elegant%20texture%2C%20light%20airy%20atmosphere%2C%20no%20text%2C%20no%20objects%2C%20pure%20abstract%20wallpaper&width=1920&height=1080&seq=bg-preset-01&orientation=landscape' },
                  { label: '抽象纹理', url: 'https://readdy.ai/api/search-image?query=Subtle%20geometric%20texture%20background%20with%20warm%20beige%20and%20cream%20tones%2C%20delicate%20line%20patterns%2C%20minimalist%20design%2C%20soft%20natural%20light%2C%20no%20text%2C%20abstract%20wallpaper%20with%20gentle%20repeating%20shapes&width=1920&height=1080&seq=bg-preset-02&orientation=landscape' },
                  { label: '极简纯色', url: 'https://readdy.ai/api/search-image?query=Ultra%20minimalist%20solid%20color%20background%20with%20extremely%20subtle%20noise%20texture%2C%20warm%20off%20white%20tone%2C%20clean%20and%20simple%2C%20no%20patterns%2C%20no%20objects%2C%20pure%20background%20surface&width=1920&height=1080&seq=bg-preset-03&orientation=landscape' },
                  { label: '柔和雾化', url: 'https://readdy.ai/api/search-image?query=Soft%20dreamy%20atmospheric%20background%20with%20gentle%20blur%20and%20bokeh%20effect%2C%20warm%20cream%20and%20beige%20color%20palette%2C%20ethereal%20airy%20mood%2C%20no%20text%2C%20abstract%20wallpaper%20with%20soft%20focus&width=1920&height=1080&seq=bg-preset-04&orientation=landscape' },
                ].map(preset => (
                  <button
                    key={preset.label}
                    onClick={() => {
                      handleSaveBg(preset.url);
                      setShowBgUrlInput(false);
                    }}
                    className="h-7 px-2.5 rounded-md bg-background-50 border border-background-200/70 text-[10px] text-foreground-600 hover:bg-background-100 hover:border-background-300 transition-colors duration-150 whitespace-nowrap"
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Opacity slider */}
          {bgConfig.image && (
            <div className="space-y-1 pt-1">
              <div className="flex items-center justify-between">
                <span className="text-[10px] text-foreground-400">背景透明度</span>
                <span className="text-[10px] font-mono text-foreground-600">
                  {Math.round((1 - bgConfig.opacity) * 100)}% 不透明
                </span>
              </div>
              <input
                type="range"
                min={0}
                max={0.8}
                step={0.05}
                value={bgConfig.opacity}
                onChange={e => handleOpacityChange(Number(e.target.value))}
                onPointerUp={handleOpacityCommit}
                onKeyUp={handleOpacityCommit}
                className="w-full accent-primary-500 h-1"
              />
              <div className="flex justify-between text-[9px] text-foreground-300">
                <span>更明显</span>
                <span>更淡</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Classic themes */}
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

      {/* Kawaii themes */}
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
