// ============================================================
// Theme Package: Slate (default)
// Cool refined neutral. Editorial, quiet, expensive.
// Font: Fraunces heading / Inter body
// Shape: subtle 12-16px rounded, soft material shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const slateTheme: ThemePackage = {
  id: 'slate',
  meta: {
    name: 'Slate',
    subtitle: '冷调·编辑',
    description: '冷静中性，杂志般的编辑排版感。就像一本精致的独立刊物，安静但有分量。',
    swatches: ['#f4f5f7', '#3a4252', '#6b8f86'],
    vibe: 'serious',
  },
  css: `/* Slate — Cool Editorial */
:root,
[data-theme="slate"] {
  --font-heading: 'Fraunces', 'Noto Sans SC', Georgia, serif;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', monospace;

  --radius-none: 0;
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-xl: 16px;
  --radius-2xl: 20px;
  --radius-full: 9999px;

  --elevation-surface: 0 1px 2px rgb(15 23 32 / 0.04), 0 0 0 1px rgb(15 23 32 / 0.03);
  --elevation-raised: 0 2px 8px -2px rgb(15 23 32 / 0.06), 0 4px 16px -4px rgb(15 23 32 / 0.05);
  --elevation-float: 0 4px 12px -2px rgb(15 23 32 / 0.08), 0 12px 32px -8px rgb(15 23 32 / 0.08);
  --elevation-overlay: 0 8px 24px -4px rgb(15 23 32 / 0.10), 0 20px 48px -12px rgb(15 23 32 / 0.12);

  --background-50: 0.985 0.002 250;
  --background-100: 0.972 0.003 250;
  --background-200: 0.955 0.004 248;
  --background-300: 0.928 0.005 246;
  --background-400: 0.895 0.006 244;

  --foreground-50: 0.62 0.008 250;
  --foreground-100: 0.56 0.009 250;
  --foreground-200: 0.50 0.010 249;
  --foreground-300: 0.44 0.011 248;
  --foreground-400: 0.38 0.012 247;
  --foreground-500: 0.32 0.013 246;
  --foreground-600: 0.27 0.014 245;
  --foreground-700: 0.22 0.014 244;
  --foreground-800: 0.18 0.013 243;
  --foreground-900: 0.14 0.012 242;
  --foreground-950: 0.10 0.010 240;

  --primary-50: 0.60 0.020 245;
  --primary-100: 0.54 0.028 244;
  --primary-200: 0.48 0.036 243;
  --primary-300: 0.42 0.042 242;
  --primary-400: 0.37 0.046 241;
  --primary-500: 0.32 0.048 240;
  --primary-600: 0.28 0.046 239;
  --primary-700: 0.24 0.042 238;
  --primary-800: 0.20 0.036 237;
  --primary-900: 0.16 0.028 236;
  --primary-950: 0.13 0.022 235;

  --accent-50: 0.68 0.030 175;
  --accent-100: 0.62 0.040 174;
  --accent-200: 0.56 0.050 173;
  --accent-300: 0.51 0.058 172;
  --accent-400: 0.47 0.064 171;
  --accent-500: 0.44 0.066 170;
  --accent-600: 0.40 0.062 169;
  --accent-700: 0.35 0.055 168;
  --accent-800: 0.30 0.046 167;
  --accent-900: 0.26 0.038 166;
  --accent-950: 0.22 0.030 165;

  --secondary-50: 0.64 0.006 250;
  --secondary-100: 0.58 0.007 249;
  --secondary-200: 0.52 0.008 248;
  --secondary-300: 0.46 0.009 247;
  --secondary-400: 0.40 0.010 246;
  --secondary-500: 0.35 0.011 245;
  --secondary-600: 0.30 0.010 244;
  --secondary-700: 0.26 0.009 243;
  --secondary-800: 0.22 0.008 242;
  --secondary-900: 0.18 0.007 241;
  --secondary-950: 0.14 0.006 240;
}`,
};