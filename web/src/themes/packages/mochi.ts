// ============================================================
// Theme Package: Mochi (Squishy Lavender Dream)
// Soft squishy lavender, warm coral, pillowy cream.
// Feels like a squishy mochi — gentle, round, comforting.
// Font: M PLUS Rounded 1c heading / Nunito body
// Shape: extra-large 26-34px rounds, lavender-tinted shadows
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const mochiTheme: ThemePackage = {
  id: 'mochi',
  meta: {
    name: 'Mochi',
    subtitle: '麻薯·软糯',
    description: '薰衣草紫 × 珊瑚橘，像一颗软糯麻薯——温柔、Q弹、治愈。薰衣草色投影，超级圆润。',
    swatches: ['#faf5f0', '#b39ddb', '#f08a7d'],
    vibe: 'cute',
  },
  css: `/* Mochi — Squishy Lavender Dream */

/* ---- Tokens ---- */
[data-theme="mochi"] {
  --font-heading: 'M PLUS Rounded 1c', 'Noto Sans SC', sans-serif;
  --font-body: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Nunito', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;

  --radius-none: 0;
  --radius-sm: 7px;
  --radius-md: 15px;
  --radius-lg: 24px;
  --radius-xl: 30px;
  --radius-2xl: 38px;
  --radius-full: 9999px;

  --elevation-surface: 0 2px 8px rgb(170 140 200 / 0.07), 0 0 0 1px rgb(170 140 200 / 0.04);
  --elevation-raised: 0 5px 16px -2px rgb(170 140 200 / 0.09), 0 10px 28px -6px rgb(170 140 200 / 0.05);
  --elevation-float: 0 10px 26px -6px rgb(170 140 200 / 0.11), 0 18px 44px -14px rgb(170 140 200 / 0.07);
  --elevation-overlay: 0 14px 36px -8px rgb(170 140 200 / 0.14), 0 28px 60px -18px rgb(170 140 200 / 0.10);

  --background-50: 0.988 0.004 83;
  --background-100: 0.976 0.005 83;
  --background-200: 0.956 0.006 82;
  --background-300: 0.928 0.007 80;
  --background-400: 0.895 0.008 78;

  --foreground-50: 0.64 0.012 290;
  --foreground-100: 0.58 0.013 290;
  --foreground-200: 0.52 0.014 288;
  --foreground-300: 0.46 0.014 286;
  --foreground-400: 0.40 0.013 284;
  --foreground-500: 0.34 0.012 282;
  --foreground-600: 0.28 0.011 280;
  --foreground-700: 0.22 0.010 280;
  --foreground-800: 0.18 0.008 278;
  --foreground-900: 0.14 0.007 275;
  --foreground-950: 0.10 0.006 272;

  --primary-50: 0.92 0.04 298;
  --primary-100: 0.86 0.06 298;
  --primary-200: 0.80 0.07 300;
  --primary-300: 0.74 0.08 300;
  --primary-400: 0.68 0.09 300;
  --primary-500: 0.62 0.10 300;
  --primary-600: 0.57 0.10 299;
  --primary-700: 0.50 0.09 298;
  --primary-800: 0.43 0.07 297;
  --primary-900: 0.36 0.05 296;
  --primary-950: 0.29 0.04 295;

  --accent-50: 0.92 0.04 28;
  --accent-100: 0.87 0.06 28;
  --accent-200: 0.82 0.08 27;
  --accent-300: 0.77 0.09 26;
  --accent-400: 0.72 0.10 25;
  --accent-500: 0.67 0.11 25;
  --accent-600: 0.61 0.11 24;
  --accent-700: 0.54 0.10 23;
  --accent-800: 0.47 0.08 22;
  --accent-900: 0.40 0.06 21;
  --accent-950: 0.33 0.04 20;

  --secondary-50: 0.76 0.007 288;
  --secondary-100: 0.68 0.008 287;
  --secondary-200: 0.60 0.009 286;
  --secondary-300: 0.52 0.010 285;
  --secondary-400: 0.44 0.010 284;
  --secondary-500: 0.37 0.010 283;
  --secondary-600: 0.31 0.009 282;
  --secondary-700: 0.25 0.008 281;
  --secondary-800: 0.20 0.007 280;
  --secondary-900: 0.16 0.005 279;
  --secondary-950: 0.12 0.004 278;
}

/* ---- Shared Kawaii Foundations ---- */
[data-theme="mochi"] body { position: relative; }
[data-theme="mochi"] body::before { content: ''; position: fixed; inset: 0; z-index: -1; pointer-events: none; }

[data-theme="mochi"] h1 { font-size: 2.8rem !important; line-height: 1.08 !important; font-weight: 800 !important; letter-spacing: -0.02em !important; }
[data-theme="mochi"] h2 { font-size: 1.25rem !important; font-weight: 700 !important; letter-spacing: -0.01em !important; }

[data-theme="mochi"] .material-card { border: 1.5px solid transparent; }
[data-theme="mochi"] .material-card:hover { transform: translateY(-5px) scale(1.02); }

[data-theme="mochi"] [role="tablist"] button {
  padding: 6px 18px !important; border-radius: 9999px !important;
  font-weight: 600 !important; font-size: 0.8rem !important;
  transition: all 0.25s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="mochi"] [role="tablist"] button[aria-selected="true"]:hover { transform: scale(1.05); }

[data-theme="mochi"] form[class*="relative"] > div { height: 4.5rem !important; border-radius: 2rem !important; }
[data-theme="mochi"] form input { font-size: 1.05rem !important; font-weight: 500 !important; }

[data-theme="mochi"] header nav a[href="/"] span:first-child { width: 2.5rem !important; height: 2.5rem !important; border-radius: 1rem !important; font-size: 1.1rem !important; font-weight: 700 !important; }

[data-theme="mochi"] header nav a[href="/login"] {
  border-radius: 9999px !important; padding-left: 1.25rem !important; padding-right: 1.25rem !important;
  font-weight: 600 !important; font-size: 0.8rem !important; height: 2.5rem !important;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) !important;
}
[data-theme="mochi"] header nav a[href="/login"]:hover { transform: scale(1.06); }

[data-theme="mochi"] footer a:hover { transform: translateY(-1px); }

[data-theme="mochi"] .material-card span[class*="w-11"] i { transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1); }
[data-theme="mochi"] .material-card:hover span[class*="w-11"] i { transform: scale(1.2); }

/* ---- Mochi-Specific Overrides ---- */
[data-theme="mochi"] body::before {
  background:
    radial-gradient(ellipse 70% 50% at 30% 20%, oklch(var(--primary-50) / 0.30) 0%, transparent 55%),
    radial-gradient(ellipse 60% 55% at 75% 65%, oklch(var(--accent-100) / 0.18) 0%, transparent 50%),
    radial-gradient(circle 6px at 18% 40%, oklch(var(--primary-300) / 0.18) 0%, transparent 100%),
    radial-gradient(circle 4px at 80% 15%, oklch(var(--primary-200) / 0.20) 0%, transparent 100%),
    radial-gradient(circle 5px at 40% 85%, oklch(var(--accent-200) / 0.16) 0%, transparent 100%),
    oklch(var(--background-100));
}

[data-theme="mochi"] .material-card {
  background: linear-gradient(170deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.35));
  border-color: oklch(var(--primary-200) / 0.3);
  box-shadow: 0 4px 14px rgb(170 140 200 / 0.07), 0 0 0 1px oklch(var(--primary-200) / 0.2), inset 0 2px 0 oklch(var(--primary-50) / 0.5);
}
[data-theme="mochi"] .material-card:hover {
  box-shadow: 0 8px 28px rgb(170 140 200 / 0.14), 0 0 0 1.5px oklch(var(--primary-400) / 0.35), inset 0 2px 0 oklch(var(--primary-50) / 0.7);
  transform: translateY(-4px) scale(1.03);
}

[data-theme="mochi"] .material-card::after {
  content: '♡'; position: absolute; top: 10px; right: 14px; font-size: 13px;
  color: oklch(var(--primary-300) / 0.45); pointer-events: none; opacity: 0;
  transition: opacity 0.3s ease, transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}
[data-theme="mochi"] .material-card:hover::after { opacity: 1; transform: scale(1.4); }

[data-theme="mochi"] form > div {
  background: linear-gradient(170deg, oklch(var(--background-50)), oklch(var(--primary-50) / 0.25)) !important;
  border: 1.5px solid oklch(var(--primary-200) / 0.22) !important;
}

[data-theme="mochi"] [role="tablist"] button[aria-selected="true"] {
  background: oklch(var(--primary-500)) !important; color: oklch(var(--background-50)) !important;
  box-shadow: 0 4px 14px oklch(var(--primary-400) / 0.4) !important;
}
[data-theme="mochi"] [role="tablist"] button[aria-selected="false"] { color: oklch(var(--foreground-400)) !important; }
[data-theme="mochi"] [role="tablist"] button[aria-selected="false"]:hover { background: oklch(var(--primary-100) / 0.45) !important; color: oklch(var(--primary-600)) !important; }

[data-theme="mochi"] [role="tablist"] + div,
[data-theme="mochi"] [role="tablist"] ~ div[class*="relative"] { display: none !important; }

[data-theme="mochi"] .material-card span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-200) / 0.45), oklch(var(--accent-200) / 0.4)) !important;
  color: oklch(var(--primary-700)) !important; border-radius: 1.1rem !important;
  width: 3rem !important; height: 3rem !important;
}
[data-theme="mochi"] .material-card:hover span[class*="w-11"] {
  background: linear-gradient(135deg, oklch(var(--primary-400)), oklch(var(--accent-400))) !important;
  color: oklch(var(--background-50)) !important;
}
[data-theme="mochi"] .material-card span[class*="w-11"] i { font-size: 1.4rem !important; }

[data-theme="mochi"] .material-card h3 { font-size: 0.95rem !important; font-weight: 700 !important; }

[data-theme="mochi"] header p[class*="tracking"] { color: oklch(var(--primary-400)) !important; }

/* ---- Kawaii Animations ---- */
[data-theme="mochi"] .rise-in { animation: floatUp 0.55s cubic-bezier(0.16, 1, 0.3, 1) both; }
[data-theme="mochi"] .material-card { animation: gentleFloat 3s ease-in-out infinite; position: relative; }

[data-theme="mochi"] .grid > .material-card:nth-child(1) { animation-delay: 0s; }
[data-theme="mochi"] .grid > .material-card:nth-child(2) { animation-delay: 0.15s; }
[data-theme="mochi"] .grid > .material-card:nth-child(3) { animation-delay: 0.3s; }
[data-theme="mochi"] .grid > .material-card:nth-child(4) { animation-delay: 0.45s; }
[data-theme="mochi"] .grid > .material-card:nth-child(5) { animation-delay: 0.6s; }
[data-theme="mochi"] .grid > .material-card:nth-child(6) { animation-delay: 0.75s; }
[data-theme="mochi"] .grid > .material-card:nth-child(7) { animation-delay: 0.9s; }
[data-theme="mochi"] .grid > .material-card:nth-child(8) { animation-delay: 1.05s; }

[data-theme="mochi"] .skeleton { background: linear-gradient(90deg, oklch(var(--primary-100) / 0.4) 25%, oklch(var(--primary-200) / 0.3) 50%, oklch(var(--primary-100) / 0.4) 75%); background-size: 200% 100%; }

[data-theme="mochi"] ::-webkit-scrollbar-thumb { background: oklch(var(--primary-200) / 0.5); border-radius: var(--radius-full); }
[data-theme="mochi"] ::-webkit-scrollbar-thumb:hover { background: oklch(var(--primary-300) / 0.65); }`,
};