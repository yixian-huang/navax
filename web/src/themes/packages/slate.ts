// ============================================================
// Theme Package: Slate (default) — v2 "Flat Editorial"
// 冷调纸感底色 + 墨色文字 + 单一紫檀点缀。
// 扁平：1px 实线描边取代阴影；全部直角；纸纹颗粒增加质感。
// Font: Space Grotesk heading / Noto Sans SC body
// Shape: 直角（radius 0），无投影，无悬浮位移
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const slateTheme: ThemePackage = {
  id: 'slate',
  meta: {
    name: 'Slate',
    subtitle: '冷调·扁平',
    description: '冷调纸感底色，墨色文字，一抹苔绿点缀。直角与实线描边，无阴影——扁平、锐利、有质感。',
    swatches: ['#f6f7f9', '#232833', '#4a6b52'],
    vibe: 'serious',
  },
  css: `/* Slate — Quiet Editorial (v2) */
:root,
[data-theme="slate"] {
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

  /* 扁平：纯 1px 描边（无模糊投影），hover 用 float 加深描边 */
  --elevation-surface: 0 0 0 1px oklch(0.20 0.015 255 / 0.12);
  --elevation-raised: 0 0 0 1px oklch(0.20 0.015 255 / 0.14);
  --elevation-float: 0 0 0 1px oklch(0.20 0.015 255 / 0.55);
  --elevation-overlay: 0 0 0 1px oklch(0.20 0.015 255 / 0.16);

  --background-50: 0.992 0.002 255;
  --background-100: 0.976 0.003 255;
  --background-200: 0.956 0.004 253;
  --background-300: 0.930 0.005 251;
  --background-400: 0.896 0.006 249;

  --foreground-50: 0.63 0.007 255;
  --foreground-100: 0.57 0.008 254;
  --foreground-200: 0.51 0.009 254;
  --foreground-300: 0.45 0.010 253;
  --foreground-400: 0.39 0.011 253;
  --foreground-500: 0.33 0.012 253;
  --foreground-600: 0.28 0.013 252;
  --foreground-700: 0.23 0.013 252;
  --foreground-800: 0.19 0.013 252;
  --foreground-900: 0.15 0.012 251;
  --foreground-950: 0.11 0.010 250;

  /* Primary — 墨蓝 ink */
  --primary-50: 0.62 0.030 257;
  --primary-100: 0.55 0.038 257;
  --primary-200: 0.49 0.046 257;
  --primary-300: 0.43 0.052 257;
  --primary-400: 0.38 0.056 257;
  --primary-500: 0.33 0.058 257;
  --primary-600: 0.28 0.054 256;
  --primary-700: 0.24 0.048 256;
  --primary-800: 0.20 0.040 255;
  --primary-900: 0.16 0.032 255;
  --primary-950: 0.12 0.024 254;

  /* Accent — 苔绿 moss（#4A6B52 附近），唯一点缀色，克制使用 */
  --accent-50: 0.66 0.040 155;
  --accent-100: 0.61 0.048 155;
  --accent-200: 0.56 0.055 155;
  --accent-300: 0.52 0.060 155;
  --accent-400: 0.49 0.062 155;
  --accent-500: 0.46 0.063 155;
  --accent-600: 0.41 0.058 154;
  --accent-700: 0.36 0.052 154;
  --accent-800: 0.31 0.044 153;
  --accent-900: 0.26 0.036 153;
  --accent-950: 0.21 0.028 152;

  --secondary-50: 0.64 0.006 253;
  --secondary-100: 0.58 0.007 253;
  --secondary-200: 0.52 0.008 252;
  --secondary-300: 0.46 0.009 252;
  --secondary-400: 0.40 0.010 251;
  --secondary-500: 0.35 0.011 251;
  --secondary-600: 0.30 0.010 250;
  --secondary-700: 0.26 0.009 250;
  --secondary-800: 0.22 0.008 249;
  --secondary-900: 0.18 0.007 249;
  --secondary-950: 0.14 0.006 248;
}

/* 扁平化补充：material-card 不再上浮 */
[data-theme="slate"] .material-card:hover {
  transform: none;
}

/* 纸纹颗粒 — 极细噪点叠加，赋予"印刷品"质感 */
[data-theme="slate"] body::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.55;
  mix-blend-mode: multiply;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.05'/%3E%3C/svg%3E");
}`,
};
