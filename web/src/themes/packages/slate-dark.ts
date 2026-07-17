// ============================================================
// Theme Package: Slate Dark — v2 "Flat Editorial" 暗色版
// 与 slate 同一套扁平棱角语言：墨底、亮色低透明度描边、
// 苔绿点缀、颗粒改 screen 混合。
// 新文件，放到 web/src/themes/packages/slate-dark.ts
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const slateDarkTheme: ThemePackage = {
  id: 'slate-dark',
  meta: {
    name: 'Slate Dark',
    subtitle: '冷调·暗夜',
    description: '墨色底，纸白文字，苔绿点缀。与 Slate 同一套直角与实线描边语言的暗色版。',
    swatches: ['#1b1e26', '#e8eaf0', '#6f9c7d'],
    vibe: 'serious',
  },
  css: `/* Slate Dark — Flat Editorial (dark) */
[data-theme="slate-dark"] {
  --font-heading: 'Space Grotesk', 'Noto Sans SC', system-ui, sans-serif;
  --font-body: 'Noto Sans SC', 'Space Grotesk', system-ui, sans-serif;
  --font-label: 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', monospace;

  --radius-none: 0;
  --radius-sm: 0;
  --radius-md: 0;
  --radius-lg: 0;
  --radius-xl: 0;
  --radius-2xl: 0;
  --radius-full: 9999px;

  /* 亮色低透明度描边，替代阴影 */
  --elevation-surface: 0 0 0 1px oklch(0.95 0.01 255 / 0.10);
  --elevation-raised: 0 0 0 1px oklch(0.95 0.01 255 / 0.12);
  --elevation-float: 0 0 0 1px oklch(0.95 0.01 255 / 0.34);
  --elevation-overlay: 0 0 0 1px oklch(0.95 0.01 255 / 0.14);

  /* body 用 background-100（页面底）；卡片用 background-50 */
  --background-50: 0.205 0.012 255;
  --background-100: 0.165 0.010 255;
  --background-200: 0.260 0.012 253;
  --background-300: 0.300 0.012 253;
  --background-400: 0.340 0.012 253;

  --foreground-50: 0.50 0.008 253;
  --foreground-100: 0.54 0.008 253;
  --foreground-200: 0.58 0.008 253;
  --foreground-300: 0.62 0.008 253;
  --foreground-400: 0.66 0.008 253;
  --foreground-500: 0.70 0.008 253;
  --foreground-600: 0.75 0.008 254;
  --foreground-700: 0.80 0.008 254;
  --foreground-800: 0.85 0.007 255;
  --foreground-900: 0.90 0.007 255;
  --foreground-950: 0.95 0.006 255;

  /* Primary — 亮墨蓝（active 文字、实心按钮=亮底深字，天然反色） */
  --primary-50: 0.45 0.030 257;
  --primary-100: 0.50 0.036 257;
  --primary-200: 0.55 0.042 257;
  --primary-300: 0.60 0.046 257;
  --primary-400: 0.66 0.050 257;
  --primary-500: 0.72 0.052 257;
  --primary-600: 0.78 0.046 257;
  --primary-700: 0.83 0.040 256;
  --primary-800: 0.88 0.032 256;
  --primary-900: 0.92 0.024 255;
  --primary-950: 0.95 0.016 255;

  /* Accent — 苔绿，暗底上提亮 */
  --accent-50: 0.44 0.040 155;
  --accent-100: 0.49 0.048 155;
  --accent-200: 0.54 0.055 155;
  --accent-300: 0.59 0.062 155;
  --accent-400: 0.64 0.066 155;
  --accent-500: 0.68 0.070 155;
  --accent-600: 0.73 0.064 154;
  --accent-700: 0.78 0.056 154;
  --accent-800: 0.83 0.046 153;
  --accent-900: 0.88 0.036 153;
  --accent-950: 0.92 0.026 152;

  --secondary-50: 0.48 0.006 253;
  --secondary-100: 0.52 0.006 253;
  --secondary-200: 0.56 0.007 253;
  --secondary-300: 0.60 0.007 252;
  --secondary-400: 0.64 0.008 252;
  --secondary-500: 0.68 0.008 251;
  --secondary-600: 0.73 0.008 251;
  --secondary-700: 0.78 0.007 250;
  --secondary-800: 0.83 0.007 250;
  --secondary-900: 0.88 0.006 249;
  --secondary-950: 0.92 0.006 248;
}

/* 扁平化补充：网格项与列表面板同色同描边 */
[data-theme="slate-dark"] .material-card {
  background: oklch(var(--background-50));
  box-shadow: none;
  border: 1px solid oklch(0.95 0.01 255 / 0.10);
}
[data-theme="slate-dark"] .material-card:hover {
  transform: none;
  box-shadow: none;
  border-color: oklch(0.95 0.01 255 / 0.18);
  background: oklch(var(--background-50));
}

/* theme 在 html，wallpaper 在 PublicShell — 用后代选择器 */
[data-theme="slate-dark"] [data-wallpaper] .material-card {
  -webkit-backdrop-filter: none !important;
  backdrop-filter: none !important;
  box-shadow: none !important;
}
[data-theme="slate-dark"] [data-wallpaper][data-wallpaper-tone="light"] .material-card {
  background: oklch(1 0 0 / 0.2) !important;
  border-color: color-mix(in oklch, var(--wp-ink) 14%, transparent);
}
[data-theme="slate-dark"] [data-wallpaper][data-wallpaper-tone="dark"] .material-card {
  background: oklch(0.12 0.02 260 / 0.34) !important;
  border-color: color-mix(in oklch, var(--wp-ink) 14%, transparent);
}
[data-theme="slate-dark"] [data-wallpaper][data-wallpaper-tone="light"] .material-card:hover {
  background: oklch(1 0 0 / 0.3) !important;
}
[data-theme="slate-dark"] [data-wallpaper][data-wallpaper-tone="dark"] .material-card:hover {
  background: oklch(0.12 0.02 260 / 0.48) !important;
}
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-title,
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-title *,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-title,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-title * {
  color: var(--wp-ink) !important;
  text-shadow: var(--wp-shadow);
}
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-domain,
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-domain *,
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-desc,
[data-theme="slate-dark"] [data-wallpaper] .material-card .site-card-desc *,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-domain,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-domain *,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-desc,
[data-theme="slate-dark"] [data-wallpaper] .wallpaper-ink-scope .material-card .site-card-desc * {
  color: var(--wp-ink-muted) !important;
  text-shadow: var(--wp-shadow);
}

/* 纸纹颗粒 — 暗底用 screen 混合、低透明度 */
[data-theme="slate-dark"] body::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.3;
  mix-blend-mode: screen;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.05'/%3E%3C/svg%3E");
}`,
};
