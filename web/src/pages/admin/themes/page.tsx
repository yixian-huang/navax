// ============================================================
// nav.ax Admin Themes Page — /admin/themes
// Controls the public-facing navigation theme.
// Uses the real ThemeRegistry (7 themes), not mock data.
// ============================================================

import { useState, useMemo, useCallback } from 'react';
import { Check, Paintbrush } from 'lucide-react';
import { themeRegistry } from '@/themes/registry';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { ThemePackage } from '@/themes/types';
import { useAdminThemes, useUpdateAdminThemeState } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';

// Import all theme packages so they're registered
import '@/themes/packages';

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
          isPending && 'opacity-70'
        )}
      >
        {/* Preview bar — swatches from theme metadata */}
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

        {/* Info */}
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
                : 'bg-slate-100 text-slate-600'
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
                isEnabled ? 'bg-primary-500' : 'bg-background-300'
              )}
            >
              <span className={cn(
                'absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                isEnabled && 'translate-x-4'
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
          控制实例内用户可选哪些主题，以及新建导航页的默认主题。各导航页自己的外观请在工作台「主题设置」中配置 · 共 {themes.length} 套
        </p>
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
