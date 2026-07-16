// ============================================================
// Theme Package: Pastel Sky (Candy Sky Breeze)
// Fresh sky-blue candy, pastel lemon, cloud-white.
// Like a clear spring sky — light, bouncy, full of hope.
// Font: M PLUS Rounded 1c heading / Nunito body
// Shape: bubbly 20-28px rounds, sky-blue tinted floating shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const pastelskyTheme: ThemePackage = {
  id: 'pastelsky',
  meta: {
    name: 'Pastel Sky',
    subtitle: '晴空·糖果',
    description: '晴空蓝 × 柠檬黄，像春日糖果色的天空。轻快、弹跳感、元气满满。',
    swatches: ['#f5f9fd', '#7eb8da', '#f5d77a'],
    vibe: 'cute',
  },
  css: `/* Pastel Sky — Candy Sky Breeze */

/* ---- Tokens ---- */
[data-theme="pastelsky"] {
  --font-heading: 'M PLUS Rounded 1c', 'Noto Sans SC', sans-serif;
  --font-body: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 6px;
  --radius-md: 12px;
  --radius-lg: 20px;
  --radius-xl: 26px;
  --radius-2xl: 34px;
  --radius-full: 9999px;

  --elevation-surface: 0 2px 8px rgb(160 200 220 / 0.07), 0 0 0 1px rgb(160 200 220 / 0.04);
  --elevation-raised: 0 5px 16px -2px rgb(160 200 220 / 0.09), 0 10px 28px -6px rgb(160 200 220 / 0.05);
  --elevation-float: 0 10px 26px -6px rgb(160 200 220 / 0.11), 0 18px 44px -14px rgb(160 200 220 / 0.07);
  --elevation-overlay: 0 14px 36px -8px rgb(160 200 220 / 0.14), 0 28px 60px -18px rgb(160 200 220 / 0.10);

  --background-50: 0.990 0.003 210;
  --background-100: 0.978 0.004 210;
  --background-200: 0.958 0.005 208;
  --background-300: 0.932 0.005 206;
  --background-400: 0.900 0.006 204;

  --foreground-50: 0.66 0.010 225;
  --foreground-100: 0.60 0.011 224;
  --foreground-200: 0.54 0.012 223;
  --foreground-300: 0.48 0.012 222;
  --foreground-400: 0.42 0.012 220;
  --foreground-500: 0.36 0.011 218;
  --foreground-600: 0.30 0.010 215;
  --foreground-700: 0.24 0.009 212;
  --foreground-800: 0.19 0.008 210;
  --foreground-900: 0.15 0.007 208;
  --foreground-950: 0.11 0.006 205;

  --primary-50: 0.93 0.03 220;
  --primary-100: 0.88 0.04 222;
  --primary-200: 0.83 0.05 223;
  --primary-300: 0.78 0.06 224;
  --primary-400: 0.72 0.07 225;
  --primary-500: 0.66 0.08 225;
  --primary-600: 0.60 0.08 225;
  --primary-700: 0.53 0.07 224;
  --primary-800: 0.46 0.06 223;
  --primary-900: 0.39 0.05 222;
  --primary-950: 0.32 0.04 220;

  --accent-50: 0.95 0.04 98;
  --accent-100: 0.91 0.06 98;
  --accent-200: 0.88 0.07 97;
  --accent-300: 0.85 0.08 96;
  --accent-400: 0.83 0.09 95;
  --accent-500: 0.81 0.10 95;
  --accent-600: 0.75 0.10 94;
  --accent-700: 0.67 0.09 93;
  --accent-800: 0.59 0.07 92;
  --accent-900: 0.51 0.05 91;
  --accent-950: 0.43 0.04 90;

  --secondary-50: 0.78 0.007 215;
  --secondary-100: 0.70 0.008 214;
  --secondary-200: 0.62 0.009 213;
  --secondary-300: 0.54 0.010 212;
  --secondary-400: 0.46 0.010 210;
  --secondary-500: 0.38 0.010 208;
  --secondary-600: 0.32 0.009 206;
  --secondary-700: 0.26 0.008 205;
  --secondary-800: 0.21 0.007 204;
  --secondary-900: 0.16 0.005 203;
  --secondary-950: 0.12 0.004 202;
}

/* ---- Shared Kawaii Foundations ---- */
[data-theme="pastelsky"] body { position: relative; }
[data-theme="pastelsky"] body::before { content: ''; position: fixed; inset: 0; z-index: -1; pointer-events: none; }

[data-theme="pastelsky"] h1 { font-size: 2.8rem !important; line-height: 1.08 !important; font-weight: 800 !important; letter-spacing: -0.02em !important; }
[data-theme="pastelsky"] h2 { font-size: 1.25rem !important; font-weight: 700 !important; letter-spacing: -0.01em !important; }

[data-theme="pastelsky"] .material-card { border: 1.5px solid transparent; }
[data-theme="pastelsky"] .material-card:hover { transform: translateY(-5px) scale(1.02); }

[data-theme="pastelsky"] [role="tablist"] button {
  padding: 6px 18px !important; border-radius: 9999px !important;
  font-weight: 600 !important; font-size: 0.8rem !important;
  transition: all 0.25s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="pastelsky"] [role="tablist"] button[aria-selected="true"]:hover { transform: scale(1.05); }

[data-theme="pastelsky"] form[class*="relative"] > div { height: 4.5rem !important; border-radius: 2rem !important; }
[data-theme="pastelsky"] form input { font-size: 1.05rem !important; font-weight: 500 !important; }

[data-theme="pastelsky"] header nav a[href="/"] span:first-child { width: 2.5rem !important; height: 2.5rem !important; border-radius: 1rem !important; font-size: 1.1rem !important; font-weight: 700 !important; }

[data-theme="pastelsky"] header nav a[href="/login"] {
  border-radius: 9999px !important; padding-left: 1.25rem !important; padding-right: 1.25rem !important;
  font-weight: 600 !important; font-size: 0.8rem !important; height: 2.5rem !important;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="pastelsky"] header nav a[href="/login"]:hover { transform: scale(1.06); }

[data-theme="pastelsky"] footer a:hover { transform: translateY(-1px); }

[data-theme="pastelsky"] .material-card span[class*="w-11"] i { transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1); }
[data-theme="pastelsky"] .material-card:hover span[class*="w-11"] i { transform: scale(1.2); }

/* ---- Pastel Sky-Specific Overrides ---- */
[data-theme="pastelsky"] body::before {
  background:
    radial-gradient(ellipse 75% 55% at 50% 10%, oklch(var(--primary-50) / 0.30) 0%, transparent 60%),
    radial-gradient(ellipse 60% 50% at 20% 80%, oklch(var(--accent-100) / 0.22) 0%, transparent 55%),
    radial-gradient(circle 7px at 15% 30%, oklch(var(--primary-300) / 0.16) 0%, transparent 100%),
    radial-gradient(circle 5px at 70% 20%, oklch(var(--accent-200) / 0.2) 0%, transparent 100%),
    radial-gradient(circle 4px at 45% 75%, oklch(var(--primary-200) / 0.18) 0%, transparent 100%),
    radial-gradient(circle 6px at 85% 60%, oklch(var(--accent-300) / 0.14) 0%, transparent 100%),
    oklch(var(--background-100));
}

[data-theme="pastelsky"] .material-card {
  background: linear-gradient(175deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.35));
  border-color: oklch(var(--primary-200) / 0.3);
  box-shadow: 0 3px 14px rgb(160 200 220 / 0.08), 0 0 0 1px oklch(var(--primary-200) / 0.22), inset 0 1px 0 oklch(var(--primary-50) / 0.55);
}
[data-theme="pastelsky"] .material-card:hover {
  box-shadow: 0 10px 32px rgb(160 200 220 / 0.16), 0 0 0 1.5px oklch(var(--primary-400) / 0.38), inset 0 1px 0 oklch(var(--primary-50) / 0.75);
  transform: translateY(-6px) scale(1.015);
}

[data-theme="pastelsky"] .material-card::after {
  content: '☆'; position: absolute; top: 10px; right: 14px; font-size: 12px;
  color: oklch(var(--accent-300) / 0.5); pointer-events: none; opacity: 0;
  transition: opacity 0.3s ease, transform 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}
[data-theme="pastelsky"] .material-card:hover::after { opacity: 1; transform: rotate(25deg) scale(1.35); }

[data-theme="pastelsky"] form > div {
  background: linear-gradient(175deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.25)) !important;
  border: 1.5px solid oklch(var(--primary-200) / 0.22) !important;
}

[data-theme="pastelsky"] [role="tablist"] button[aria-selected="true"] {
  background: oklch(var(--primary-500)) !important; color: oklch(var(--background-50)) !important;
  box-shadow: 0 4px 14px oklch(var(--primary-400) / 0.4) !important;
}
[data-theme="pastelsky"] [role="tablist"] button[aria-selected="false"] { color: oklch(var(--foreground-400)) !important; }
[data-theme="pastelsky"] [role="tablist"] button[aria-selected="false"]:hover { background: oklch(var(--primary-100) / 0.45) !important; color: oklch(var(--primary-600)) !important; }

[data-theme="pastelsky"] [role="tablist"] + div,
[data-theme="pastelsky"] [role="tablist"] ~ div[class*="relative"] { display: none !important; }

[data-theme="pastelsky"] .material-card span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-200) / 0.45), oklch(var(--accent-200) / 0.4)) !important;
  color: oklch(var(--primary-700)) !important; border-radius: 1rem !important;
  width: 3rem !important; height: 3rem !important;
}
[data-theme="pastelsky"] .material-card:hover span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-400)), oklch(var(--accent-400))) !important;
  color: oklch(var(--background-50)) !important;
}
[data-theme="pastelsky"] .material-card span[class*="w-11"] i { font-size: 1.4rem !important; }

[data-theme="pastelsky"] .material-card h3 { font-size: 0.95rem !important; font-weight: 700 !important; }

[data-theme="pastelsky"] header p[class*="tracking"] { color: oklch(var(--primary-400)) !important; }

/* ---- Kawaii Animations ---- */
[data-theme="pastelsky"] .rise-in { animation: floatUp 0.55s cubic-bezier(0.16, 1, 0.3, 1) both; }
[data-theme="pastelsky"] .material-card { animation: gentleFloat 3s ease-in-out infinite; position: relative; }

[data-theme="pastelsky"] .grid > .material-card:nth-child(1) { animation-delay: 0s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(2) { animation-delay: 0.15s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(3) { animation-delay: 0.3s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(4) { animation-delay: 0.45s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(5) { animation-delay: 0.6s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(6) { animation-delay: 0.75s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(7) { animation-delay: 0.9s; }
[data-theme="pastelsky"] .grid > .material-card:nth-child(8) { animation-delay: 1.05s; }

[data-theme="pastelsky"] .skeleton { background: linear-gradient(90deg, oklch(var(--primary-100) / 0.4) 25%, oklch(var(--primary-200) / 0.3) 50%, oklch(var(--primary-100) / 0.4) 75%); background-size: 200% 100%; }

[data-theme="pastelsky"] ::-webkit-scrollbar-thumb { background: oklch(var(--primary-200) / 0.5); border-radius: var(--radius-full); }
[data-theme="pastelsky"] ::-webkit-scrollbar-thumb:hover { background: oklch(var(--primary-300) / 0.65); }`,
};