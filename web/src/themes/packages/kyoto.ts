// ============================================================
// Theme Package: Kyoto (Wabi-Sabi)
// Warm rice-paper cream, deep forest green, rustic terracotta.
// Zen, organic, softly rounded. Feels like a tea house.
// Font: Playfair Display heading / DM Sans body
// Shape: generous 20-24px rounds, softer organic shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const kyotoTheme: ThemePackage = {
  id: 'kyoto',
  meta: {
    name: 'Kyoto',
    subtitle: '侘寂·禅意',
    description: '米纸奶白底，深林绿与赭石点缀。像一间日式茶室——温润、安静、有呼吸感。',
    swatches: ['#f8f3e8', '#3d6045', '#c4814a'],
    vibe: 'serious',
  },
  css: `/* Kyoto — Wabi-Sabi Tea House */
[data-theme="kyoto"] {
  --font-heading: 'Playfair Display', 'Noto Sans SC', Georgia, serif;
  --font-body: 'DM Sans', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'DM Sans', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 5px;
  --radius-md: 10px;
  --radius-lg: 16px;
  --radius-xl: 22px;
  --radius-2xl: 28px;
  --radius-full: 9999px;

  --elevation-surface: 0 2px 6px rgb(80 50 30 / 0.05), 0 0 0 1px rgb(80 50 30 / 0.03);
  --elevation-raised: 0 4px 12px -2px rgb(80 50 30 / 0.06), 0 8px 24px -6px rgb(80 50 30 / 0.04);
  --elevation-float: 0 8px 20px -4px rgb(80 50 30 / 0.08), 0 16px 40px -10px rgb(80 50 30 / 0.06);
  --elevation-overlay: 0 12px 32px -6px rgb(80 50 30 / 0.10), 0 24px 60px -16px rgb(80 50 30 / 0.08);

  --background-50: 0.990 0.008 78;
  --background-100: 0.978 0.010 78;
  --background-200: 0.958 0.012 77;
  --background-300: 0.928 0.014 76;
  --background-400: 0.892 0.016 74;

  --foreground-50: 0.58 0.015 95;
  --foreground-100: 0.52 0.016 100;
  --foreground-200: 0.46 0.017 105;
  --foreground-300: 0.40 0.018 110;
  --foreground-400: 0.35 0.019 115;
  --foreground-500: 0.30 0.020 120;
  --foreground-600: 0.25 0.020 125;
  --foreground-700: 0.21 0.019 130;
  --foreground-800: 0.17 0.017 135;
  --foreground-900: 0.13 0.015 144;
  --foreground-950: 0.10 0.012 152;

  --primary-50: 0.62 0.06 156;
  --primary-100: 0.56 0.08 155;
  --primary-200: 0.51 0.10 154;
  --primary-300: 0.47 0.115 153;
  --primary-400: 0.44 0.125 152;
  --primary-500: 0.41 0.13 152;
  --primary-600: 0.37 0.125 151;
  --primary-700: 0.33 0.11 150;
  --primary-800: 0.29 0.09 148;
  --primary-900: 0.25 0.07 145;
  --primary-950: 0.21 0.05 142;

  --accent-50: 0.74 0.06 58;
  --accent-100: 0.70 0.08 57;
  --accent-200: 0.66 0.095 56;
  --accent-300: 0.63 0.105 55;
  --accent-400: 0.60 0.11 55;
  --accent-500: 0.58 0.115 54;
  --accent-600: 0.53 0.11 53;
  --accent-700: 0.47 0.095 52;
  --accent-800: 0.41 0.08 51;
  --accent-900: 0.35 0.062 50;
  --accent-950: 0.29 0.046 49;

  --secondary-50: 0.60 0.012 86;
  --secondary-100: 0.54 0.013 85;
  --secondary-200: 0.48 0.014 83;
  --secondary-300: 0.43 0.015 81;
  --secondary-400: 0.38 0.016 79;
  --secondary-500: 0.34 0.016 77;
  --secondary-600: 0.29 0.015 75;
  --secondary-700: 0.25 0.013 73;
  --secondary-800: 0.21 0.011 71;
  --secondary-900: 0.17 0.009 69;
  --secondary-950: 0.13 0.007 67;
}`,
};