// ============================================================
// Theme Package: Noir (Dark Luxury)
// Deep black canvas, metallic champagne gold, garnet accent.
// Dramatic, confident, editorial. High contrast luxury.
// Font: Fraunces heading / Inter body
// Shape: tight 6-10px rounded, crisp sharp shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const noirTheme: ThemePackage = {
  id: 'noir',
  meta: {
    name: 'Noir',
    subtitle: '暗夜·奢感',
    description: '深黑画布上浮现金色与石榴红。高对比、戏剧性，像高级腕表的暗夜橱窗。',
    swatches: ['#1a1d24', '#d9b871', '#c04a3a'],
    vibe: 'serious',
  },
  css: `/* Noir — Dark Luxury */
[data-theme="noir"] {
  --font-heading: 'Fraunces', 'Noto Sans SC', Georgia, serif;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 3px;
  --radius-md: 6px;
  --radius-lg: 10px;
  --radius-xl: 14px;
  --radius-2xl: 18px;
  --radius-full: 9999px;

  --elevation-surface: 0 2px 8px rgb(0 0 0 / 0.30), 0 0 0 1px rgb(255 255 255 / 0.04);
  --elevation-raised: 0 4px 16px rgb(0 0 0 / 0.40), 0 0 0 1px rgb(255 255 255 / 0.06);
  --elevation-float: 0 8px 28px rgb(0 0 0 / 0.50), 0 0 0 1px rgb(255 255 255 / 0.08);
  --elevation-overlay: 0 12px 40px rgb(0 0 0 / 0.60), 0 0 0 1px rgb(255 255 255 / 0.10);

  --background-50: 0.13 0.004 260;
  --background-100: 0.10 0.003 260;
  --background-200: 0.17 0.005 260;
  --background-300: 0.22 0.006 260;
  --background-400: 0.28 0.007 260;

  --foreground-50: 0.30 0.004 260;
  --foreground-100: 0.36 0.005 260;
  --foreground-200: 0.44 0.005 260;
  --foreground-300: 0.52 0.005 260;
  --foreground-400: 0.60 0.005 260;
  --foreground-500: 0.68 0.004 260;
  --foreground-600: 0.76 0.004 260;
  --foreground-700: 0.83 0.003 260;
  --foreground-800: 0.89 0.002 260;
  --foreground-900: 0.94 0.001 260;
  --foreground-950: 0.98 0.001 260;

  --primary-50: 0.52 0.04 82;
  --primary-100: 0.58 0.06 83;
  --primary-200: 0.65 0.08 83;
  --primary-300: 0.72 0.09 84;
  --primary-400: 0.78 0.095 84;
  --primary-500: 0.84 0.098 85;
  --primary-600: 0.87 0.088 85;
  --primary-700: 0.90 0.072 86;
  --primary-800: 0.93 0.052 86;
  --primary-900: 0.95 0.034 87;
  --primary-950: 0.97 0.020 88;

  --accent-50: 0.44 0.05 35;
  --accent-100: 0.50 0.06 34;
  --accent-200: 0.57 0.07 33;
  --accent-300: 0.63 0.08 33;
  --accent-400: 0.69 0.09 32;
  --accent-500: 0.74 0.095 32;
  --accent-600: 0.79 0.082 32;
  --accent-700: 0.84 0.064 33;
  --accent-800: 0.88 0.046 33;
  --accent-900: 0.92 0.030 34;
  --accent-950: 0.96 0.016 34;

  --secondary-50: 0.30 0.004 260;
  --secondary-100: 0.36 0.005 260;
  --secondary-200: 0.44 0.005 260;
  --secondary-300: 0.52 0.005 260;
  --secondary-400: 0.60 0.005 260;
  --secondary-500: 0.68 0.004 260;
  --secondary-600: 0.76 0.004 260;
  --secondary-700: 0.83 0.003 260;
  --secondary-800: 0.89 0.002 260;
  --secondary-900: 0.94 0.001 260;
  --secondary-950: 0.98 0.001 260;
}

/* Noir hairline overrides — lighter, more translucent on dark bg */
[data-theme="noir"] .hairline {
  background: oklch(var(--secondary-200) / 0.15);
}
[data-theme="noir"] .hairline-gradient {
  background: linear-gradient(90deg, transparent, oklch(var(--secondary-200) / 0.25), transparent);
}`,
};