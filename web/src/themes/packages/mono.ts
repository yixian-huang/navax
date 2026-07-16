// ============================================================
// Theme Package: Mono
// Pure grayscale. Flat, sharp, dense. Utilitarian and brutal.
// Font: Space Mono heading / Inter body
// Shape: zero radius, no shadows, no decoration.
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const monoTheme: ThemePackage = {
  id: 'mono',
  meta: {
    name: 'Mono',
    subtitle: '单色·极简',
    description: '纯粹的黑白灰。零圆角、零阴影、零装饰，信息密度拉满，只为效率而生。',
    swatches: ['#ffffff', '#888888', '#111111'],
    vibe: 'serious',
  },
  css: `/* Mono — Pure Monochrome */
[data-theme="mono"] {
  --font-heading: 'Space Mono', 'JetBrains Mono', 'Noto Sans SC', monospace;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'Space Mono', 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 0;
  --radius-md: 0;
  --radius-lg: 0;
  --radius-xl: 0;
  --radius-2xl: 0;
  --radius-full: 0;

  --elevation-surface: none;
  --elevation-raised: none;
  --elevation-float: none;
  --elevation-overlay: none;

  --background-50: 1 0 0;
  --background-100: 0.975 0 0;
  --background-200: 0.948 0 0;
  --background-300: 0.915 0 0;
  --background-400: 0.875 0 0;

  --foreground-50: 0.70 0 0;
  --foreground-100: 0.64 0 0;
  --foreground-200: 0.58 0 0;
  --foreground-300: 0.52 0 0;
  --foreground-400: 0.45 0 0;
  --foreground-500: 0.38 0 0;
  --foreground-600: 0.32 0 0;
  --foreground-700: 0.25 0 0;
  --foreground-800: 0.18 0 0;
  --foreground-900: 0.11 0 0;
  --foreground-950: 0.04 0 0;

  --primary-50: 0.65 0 0;
  --primary-100: 0.59 0 0;
  --primary-200: 0.53 0 0;
  --primary-300: 0.46 0 0;
  --primary-400: 0.39 0 0;
  --primary-500: 0.30 0 0;
  --primary-600: 0.26 0 0;
  --primary-700: 0.21 0 0;
  --primary-800: 0.16 0 0;
  --primary-900: 0.10 0 0;
  --primary-950: 0.04 0 0;

  --accent-50: 0.75 0 0;
  --accent-100: 0.69 0 0;
  --accent-200: 0.63 0 0;
  --accent-300: 0.57 0 0;
  --accent-400: 0.50 0 0;
  --accent-500: 0.43 0 0;
  --accent-600: 0.38 0 0;
  --accent-700: 0.33 0 0;
  --accent-800: 0.27 0 0;
  --accent-900: 0.21 0 0;
  --accent-950: 0.15 0 0;

  --secondary-50: 0.92 0 0;
  --secondary-100: 0.88 0 0;
  --secondary-200: 0.84 0 0;
  --secondary-300: 0.78 0 0;
  --secondary-400: 0.72 0 0;
  --secondary-500: 0.65 0 0;
  --secondary-600: 0.57 0 0;
  --secondary-700: 0.48 0 0;
  --secondary-800: 0.39 0 0;
  --secondary-900: 0.30 0 0;
  --secondary-950: 0.21 0 0;
}`,
};