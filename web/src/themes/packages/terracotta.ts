// ============================================================
// Theme Package: Terracotta (Mediterranean + Bauhaus)
// Sun-baked earthy clay, deep navy, warm amber.
// Bold, creative, organic. Collision of Mediterranean warmth
// and Bauhaus geometry.
// Font: DM Sans heading / Inter body
// Shape: mixed — some sharp, some pillow-soft large rounds
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const terracottaTheme: ThemePackage = {
  id: 'terracotta',
  meta: {
    name: 'Terracotta',
    subtitle: '赭石·大地',
    description: '日晒陶土底色，深海军蓝与暖琥珀。地中海与包豪斯的碰撞——热烈但不浮躁。',
    swatches: ['#faf3e5', '#2c3e68', '#d4873a'],
    vibe: 'serious',
  },
  css: `/* Terracotta — Mediterranean Bauhaus */
[data-theme="terracotta"] {
  --font-heading: 'DM Sans', 'Noto Sans SC', system-ui, sans-serif;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 6px;
  --radius-md: 12px;
  --radius-lg: 18px;
  --radius-xl: 24px;
  --radius-2xl: 32px;
  --radius-full: 9999px;

  --elevation-surface: 0 3px 8px rgb(180 80 40 / 0.06), 0 0 0 1px rgb(180 80 40 / 0.04);
  --elevation-raised: 0 6px 16px -3px rgb(180 80 40 / 0.08), 0 10px 28px -8px rgb(180 80 40 / 0.05);
  --elevation-float: 0 10px 24px -6px rgb(180 80 40 / 0.10), 0 18px 44px -12px rgb(180 80 40 / 0.07);
  --elevation-overlay: 0 14px 36px -8px rgb(180 80 40 / 0.12), 0 28px 64px -18px rgb(180 80 40 / 0.09);

  --background-50: 0.988 0.012 48;
  --background-100: 0.976 0.014 48;
  --background-200: 0.958 0.016 47;
  --background-300: 0.930 0.018 46;
  --background-400: 0.896 0.020 45;

  --foreground-50: 0.56 0.015 40;
  --foreground-100: 0.50 0.016 42;
  --foreground-200: 0.44 0.017 45;
  --foreground-300: 0.38 0.018 48;
  --foreground-400: 0.33 0.019 52;
  --foreground-500: 0.28 0.020 56;
  --foreground-600: 0.24 0.021 60;
  --foreground-700: 0.20 0.021 64;
  --foreground-800: 0.16 0.019 68;
  --foreground-900: 0.13 0.016 74;
  --foreground-950: 0.10 0.012 80;

  --primary-50: 0.58 0.06 240;
  --primary-100: 0.52 0.08 240;
  --primary-200: 0.47 0.10 240;
  --primary-300: 0.43 0.12 240;
  --primary-400: 0.40 0.135 240;
  --primary-500: 0.37 0.145 240;
  --primary-600: 0.34 0.145 241;
  --primary-700: 0.30 0.13 242;
  --primary-800: 0.26 0.108 243;
  --primary-900: 0.22 0.082 244;
  --primary-950: 0.18 0.058 245;

  --accent-50: 0.76 0.08 65;
  --accent-100: 0.72 0.10 64;
  --accent-200: 0.68 0.12 62;
  --accent-300: 0.65 0.135 60;
  --accent-400: 0.63 0.145 58;
  --accent-500: 0.61 0.15 56;
  --accent-600: 0.56 0.145 55;
  --accent-700: 0.50 0.125 54;
  --accent-800: 0.44 0.10 53;
  --accent-900: 0.38 0.075 52;
  --accent-950: 0.32 0.055 51;

  --secondary-50: 0.60 0.014 45;
  --secondary-100: 0.54 0.015 44;
  --secondary-200: 0.48 0.016 43;
  --secondary-300: 0.43 0.017 42;
  --secondary-400: 0.38 0.018 41;
  --secondary-500: 0.34 0.018 40;
  --secondary-600: 0.29 0.017 40;
  --secondary-700: 0.25 0.015 39;
  --secondary-800: 0.21 0.012 38;
  --secondary-900: 0.17 0.009 37;
  --secondary-950: 0.13 0.007 36;
}`,
};