// ============================================================
// Theme Package: Sakura (Cherry Blossom Magic)
// Dreamy cherry-blossom pink, soft mint accent.
// Magical-girl aesthetic — rounded, airy, gentle.
// Font: M PLUS Rounded 1c heading / Nunito body
// Shape: large bubbly 22-30px rounds, pink-tinted soft shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const sakuraTheme: ThemePackage = {
  id: 'sakura',
  meta: {
    name: 'Sakura',
    subtitle: '樱花·魔法',
    description: '梦幻樱花粉 × 薄荷绿点缀，像魔法少女变身一样轻盈甜美。泡泡圆角，空气感十足。',
    swatches: ['#fef5f7', '#e88da5', '#8ecfba'],
    vibe: 'cute',
  },
  css: `/* Sakura — Cherry Blossom Dream */

/* ---- Tokens ---- */
[data-theme="sakura"] {
  --font-heading: 'M PLUS Rounded 1c', 'Noto Sans SC', sans-serif;
  --font-body: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 7px;
  --radius-md: 14px;
  --radius-lg: 22px;
  --radius-xl: 28px;
  --radius-2xl: 36px;
  --radius-full: 9999px;

  --elevation-surface: 0 3px 10px rgb(220 150 170 / 0.08), 0 0 0 1px rgb(220 150 170 / 0.05);
  --elevation-raised: 0 6px 18px -3px rgb(220 150 170 / 0.10), 0 10px 30px -8px rgb(220 150 170 / 0.06);
  --elevation-float: 0 10px 28px -6px rgb(220 150 170 / 0.12), 0 18px 48px -14px rgb(220 150 170 / 0.08);
  --elevation-overlay: 0 14px 40px -10px rgb(220 150 170 / 0.14), 0 28px 64px -20px rgb(220 150 170 / 0.10);

  --background-50: 0.990 0.003 12;
  --background-100: 0.978 0.004 12;
  --background-200: 0.958 0.005 11;
  --background-300: 0.932 0.006 10;
  --background-400: 0.900 0.007 9;

  --foreground-50: 0.68 0.008 20;
  --foreground-100: 0.62 0.009 20;
  --foreground-200: 0.56 0.010 19;
  --foreground-300: 0.50 0.011 18;
  --foreground-400: 0.44 0.012 17;
  --foreground-500: 0.38 0.012 16;
  --foreground-600: 0.32 0.012 15;
  --foreground-700: 0.26 0.011 14;
  --foreground-800: 0.20 0.010 13;
  --foreground-900: 0.15 0.008 12;
  --foreground-950: 0.10 0.006 10;

  --primary-50: 0.96 0.02 6;
  --primary-100: 0.92 0.04 6;
  --primary-200: 0.88 0.06 6;
  --primary-300: 0.84 0.08 5;
  --primary-400: 0.78 0.12 5;
  --primary-500: 0.72 0.15 5;
  --primary-600: 0.66 0.15 4;
  --primary-700: 0.58 0.13 4;
  --primary-800: 0.50 0.10 3;
  --primary-900: 0.42 0.07 2;
  --primary-950: 0.34 0.05 1;

  --accent-50: 0.94 0.03 168;
  --accent-100: 0.90 0.04 168;
  --accent-200: 0.86 0.05 167;
  --accent-300: 0.81 0.06 166;
  --accent-400: 0.77 0.07 165;
  --accent-500: 0.73 0.08 165;
  --accent-600: 0.67 0.08 164;
  --accent-700: 0.59 0.07 163;
  --accent-800: 0.51 0.06 162;
  --accent-900: 0.43 0.04 161;
  --accent-950: 0.35 0.03 160;

  --secondary-50: 0.80 0.006 20;
  --secondary-100: 0.72 0.007 19;
  --secondary-200: 0.64 0.008 18;
  --secondary-300: 0.56 0.009 17;
  --secondary-400: 0.48 0.010 16;
  --secondary-500: 0.40 0.010 15;
  --secondary-600: 0.34 0.009 14;
  --secondary-700: 0.28 0.008 14;
  --secondary-800: 0.22 0.007 13;
  --secondary-900: 0.17 0.006 12;
  --secondary-950: 0.13 0.005 11;
}

/* ---- Shared Kawaii Foundations ---- */
[data-theme="sakura"] body { position: relative; }
[data-theme="sakura"] body::before { content: ''; position: fixed; inset: 0; z-index: -1; pointer-events: none; }

[data-theme="sakura"] h1 { font-size: 2.8rem !important; line-height: 1.08 !important; font-weight: 800 !important; letter-spacing: -0.02em !important; }
[data-theme="sakura"] h2 { font-size: 1.25rem !important; font-weight: 700 !important; letter-spacing: -0.01em !important; }

[data-theme="sakura"] .material-card { border: 1.5px solid transparent; }
[data-theme="sakura"] .material-card:hover { transform: translateY(-5px) scale(1.02); }

[data-theme="sakura"] [role="tablist"] button {
  padding: 6px 18px !important; border-radius: 9999px !important;
  font-weight: 600 !important; font-size: 0.8rem !important;
  transition: all 0.25s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="sakura"] [role="tablist"] button[aria-selected="true"]:hover { transform: scale(1.05); }

[data-theme="sakura"] form[class*="relative"] > div { height: 4.5rem !important; border-radius: 2rem !important; }
[data-theme="sakura"] form input { font-size: 1.05rem !important; font-weight: 500 !important; }

[data-theme="sakura"] header nav a[href="/"] span:first-child { width: 2.5rem !important; height: 2.5rem !important; border-radius: 1rem !important; font-size: 1.1rem !important; font-weight: 700 !important; }

[data-theme="sakura"] header nav a[href="/login"] {
  border-radius: 9999px !important; padding-left: 1.25rem !important; padding-right: 1.25rem !important;
  font-weight: 600 !important; font-size: 0.8rem !important; height: 2.5rem !important;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="sakura"] header nav a[href="/login"]:hover { transform: scale(1.06); }

[data-theme="sakura"] footer a:hover { transform: translateY(-1px); }

[data-theme="sakura"] .material-card span[class*="w-11"] i { transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1); }
[data-theme="sakura"] .material-card:hover span[class*="w-11"] i { transform: scale(1.2); }

/* ---- Sakura-Specific Overrides ---- */
[data-theme="sakura"] body::before {
  background:
    radial-gradient(ellipse 80% 60% at 15% 30%, oklch(var(--primary-50) / 0.35) 0%, transparent 60%),
    radial-gradient(ellipse 60% 50% at 85% 70%, oklch(var(--accent-100) / 0.20) 0%, transparent 55%),
    radial-gradient(circle 4px at 12% 18%, oklch(var(--primary-300) / 0.25) 0%, transparent 100%),
    radial-gradient(circle 3px at 88% 25%, oklch(var(--primary-200) / 0.22) 0%, transparent 100%),
    radial-gradient(circle 5px at 25% 75%, oklch(var(--accent-200) / 0.18) 0%, transparent 100%),
    radial-gradient(circle 3px at 72% 82%, oklch(var(--primary-300) / 0.20) 0%, transparent 100%),
    radial-gradient(circle 4px at 55% 12%, oklch(var(--accent-200) / 0.22) 0%, transparent 100%),
    oklch(var(--background-100));
}

[data-theme="sakura"] .material-card {
  background: linear-gradient(160deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.4));
  border-color: oklch(var(--primary-200) / 0.35);
  box-shadow: 0 4px 16px rgb(220 150 170 / 0.08), 0 0 0 1px oklch(var(--primary-200) / 0.25), inset 0 1px 0 oklch(var(--primary-50) / 0.6);
}
[data-theme="sakura"] .material-card:hover {
  box-shadow: 0 8px 32px rgb(220 150 170 / 0.16), 0 0 0 1.5px oklch(var(--primary-400) / 0.4), inset 0 1px 0 oklch(var(--primary-50) / 0.8);
}

[data-theme="sakura"] .material-card::after {
  content: '✿'; position: absolute; top: 10px; right: 14px; font-size: 12px;
  color: oklch(var(--primary-300) / 0.5); pointer-events: none; opacity: 0;
  transition: opacity 0.3s ease, transform 0.3s ease; transform: rotate(-15deg);
}
[data-theme="sakura"] .material-card:hover::after { opacity: 1; transform: rotate(0deg) scale(1.3); }

[data-theme="sakura"] form > div {
  background: linear-gradient(160deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.3)) !important;
  border: 1.5px solid oklch(var(--primary-200) / 0.25) !important;
}

[data-theme="sakura"] [role="tablist"] button[aria-selected="true"] {
  background: oklch(var(--primary-500)) !important; color: oklch(var(--background-50)) !important;
  box-shadow: 0 4px 14px oklch(var(--primary-400) / 0.45) !important;
}
[data-theme="sakura"] [role="tablist"] button[aria-selected="false"] { color: oklch(var(--foreground-400)) !important; }
[data-theme="sakura"] [role="tablist"] button[aria-selected="false"]:hover { background: oklch(var(--primary-100) / 0.5) !important; color: oklch(var(--primary-600)) !important; }

[data-theme="sakura"] [role="tablist"] + div,
[data-theme="sakura"] [role="tablist"] ~ div[class*="relative"] { display: none !important; }

[data-theme="sakura"] .material-card span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-200) / 0.5), oklch(var(--accent-200) / 0.5)) !important;
  color: oklch(var(--primary-700)) !important; border-radius: 1rem !important;
  width: 3rem !important; height: 3rem !important;
}
[data-theme="sakura"] .material-card:hover span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-400)), oklch(var(--primary-500))) !important;
  color: oklch(var(--background-50)) !important;
}
[data-theme="sakura"] .material-card span[class*="w-11"] i { font-size: 1.4rem !important; }

[data-theme="sakura"] .material-card h3 { font-size: 0.95rem !important; font-weight: 700 !important; }

[data-theme="sakura"] header p[class*="tracking"] { color: oklch(var(--primary-400)) !important; }

/* ---- Kawaii Animations ---- */
[data-theme="sakura"] .rise-in { animation: floatUp 0.55s cubic-bezier(0.16, 1, 0.3, 1) both; }
[data-theme="sakura"] .material-card { animation: gentleFloat 3s ease-in-out infinite; position: relative; }

[data-theme="sakura"] .grid > .material-card:nth-child(1) { animation-delay: 0s; }
[data-theme="sakura"] .grid > .material-card:nth-child(2) { animation-delay: 0.15s; }
[data-theme="sakura"] .grid > .material-card:nth-child(3) { animation-delay: 0.3s; }
[data-theme="sakura"] .grid > .material-card:nth-child(4) { animation-delay: 0.45s; }
[data-theme="sakura"] .grid > .material-card:nth-child(5) { animation-delay: 0.6s; }
[data-theme="sakura"] .grid > .material-card:nth-child(6) { animation-delay: 0.75s; }
[data-theme="sakura"] .grid > .material-card:nth-child(7) { animation-delay: 0.9s; }
[data-theme="sakura"] .grid > .material-card:nth-child(8) { animation-delay: 1.05s; }

[data-theme="sakura"] .skeleton { background: linear-gradient(90deg, oklch(var(--primary-100) / 0.4) 25%, oklch(var(--primary-200) / 0.3) 50%, oklch(var(--primary-100) / 0.4) 75%); background-size: 200% 100%; }

[data-theme="sakura"] ::-webkit-scrollbar-thumb { background: oklch(var(--primary-200) / 0.5); border-radius: var(--radius-full); }
[data-theme="sakura"] ::-webkit-scrollbar-thumb:hover { background: oklch(var(--primary-300) / 0.65); }`,
};