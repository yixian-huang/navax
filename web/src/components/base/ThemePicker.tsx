// ============================================================
// ThemePicker — full theme switcher powered by ThemeRegistry.
// Renders available themes from the registry, grouped by vibe.
// Supports dynamic installation: newly registered themes
// appear automatically.
// ============================================================

import { useEffect, useState, useMemo } from 'react';
import { themeRegistry } from '@/themes/registry';
import type { ThemePackage } from '@/themes/types';

interface ThemePickerProps {
  active: string;
  onChange: (themeId: string) => void;
}

export default function ThemePicker({ active, onChange }: ThemePickerProps) {
  const [open, setOpen] = useState(true);
  const [mounted, setMounted] = useState(false);

  useEffect(() => { setMounted(true); }, []);

  const themes = useMemo(() => themeRegistry.list(), []);
  const seriousThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'serious'), [themes]);
  const cuteThemes = useMemo(() => themes.filter(t => t.meta.vibe === 'cute'), [themes]);

  const renderThemeButton = (t: ThemePackage) => {
    const isActive = active === t.id;
    return (
      <button
        key={t.id}
        onClick={() => onChange(t.id)}
        className="group w-full text-left p-3.5 rounded-xl transition-all duration-200 cursor-pointer flex items-start gap-3.5"
        style={{
          backgroundColor: isActive
            ? 'oklch(var(--primary-500) / 0.10)'
            : 'oklch(var(--background-100))',
          boxShadow: isActive
            ? 'var(--elevation-surface), 0 0 0 1px oklch(var(--primary-400) / 0.35)'
            : 'var(--elevation-surface)',
        }}
      >
        {/* Swatches */}
        <span className="flex-shrink-0 flex rounded-md overflow-hidden mt-0.5 shadow-surface">
          {t.meta.swatches.map((c, i) => (
            <span key={i} className="w-5 h-9" style={{ backgroundColor: c }} />
          ))}
        </span>
        {/* Info */}
        <span className="flex-1 min-w-0">
          <span className="flex items-baseline gap-1.5">
            <span className="text-sm font-semibold text-foreground-800">{t.meta.name}</span>
            <span className="text-[10px] text-foreground-400 tracking-wide font-medium">{t.meta.subtitle}</span>
          </span>
          <span className="block text-[11px] text-foreground-400 leading-snug mt-1">
            {t.meta.description}
          </span>
        </span>
        {/* Check */}
        {isActive && (
          <span className="flex-shrink-0 w-6 h-6 flex items-center justify-center rounded-full bg-primary-500 text-background-50 mt-0.5">
            <i className="ri-check-line text-xs" />
          </span>
        )}
      </button>
    );
  };

  return (
    <div className="fixed right-5 bottom-5 z-[60] flex flex-col items-end gap-3">
      {open && mounted && (
        <div
          className="w-80 p-5 rounded-2xl bg-background-50 rise-in"
          style={{ boxShadow: 'var(--elevation-overlay)' }}
        >
          <div className="flex items-center justify-between mb-1">
            <div className="flex items-center gap-2">
              <i className="ri-paint-brush-line text-primary-500 text-base" />
              <span className="font-heading text-sm font-semibold text-foreground-800 tracking-tight">
                主题风格
              </span>
            </div>
            <button
              onClick={() => setOpen(false)}
              className="w-7 h-7 flex items-center justify-center rounded-lg text-foreground-300 hover:text-foreground-600 hover:bg-background-200 transition-colors duration-150 cursor-pointer"
              aria-label="收起"
            >
              <i className="ri-close-line" />
            </button>
          </div>
          <p className="text-[11px] text-foreground-400 leading-relaxed mt-0.5 mb-4">
            点击切换主题，同一布局·多种风格。主题包独立加载，随时可扩展。
          </p>
          <div className="flex flex-col gap-3">
            {/* ---- Serious themes ---- */}
            {seriousThemes.length > 0 && (
              <>
                <span className="text-[10px] font-semibold uppercase tracking-[0.18em] text-foreground-300 pl-1">Classic</span>
                {seriousThemes.map(renderThemeButton)}
              </>
            )}

            {/* ---- Cute themes ---- */}
            {cuteThemes.length > 0 && (
              <>
                <span className="text-[10px] font-semibold uppercase tracking-[0.18em] text-foreground-300 pl-1 mt-2">Kawaii</span>
                {cuteThemes.map(renderThemeButton)}
              </>
            )}
          </div>
        </div>
      )}

      <button
        onClick={() => setOpen(o => !o)}
        className="w-12 h-12 flex items-center justify-center rounded-full bg-primary-500 text-background-50 hover:opacity-90 transition-opacity duration-200 cursor-pointer"
        style={{ boxShadow: 'var(--elevation-float)' }}
        aria-label="切换主题"
      >
        <i className={open ? 'ri-close-line text-lg' : 'ri-paint-brush-line text-lg'} />
      </button>
    </div>
  );
}