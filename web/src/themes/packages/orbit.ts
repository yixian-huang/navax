// ============================================================
// Theme Package: Orbit
// Deep indigo void, electric blue orbits, soft violet accents.
// Mission-control / deep-space navigation aesthetic.
// Font: Space Grotesk heading / Inter body
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const orbitTheme: ThemePackage = {
  id: 'orbit',
  meta: {
    name: 'Orbit',
    subtitle: '轨道·深空',
    description: '靛紫虚空与电蓝轨迹。像深空任务控制台：冷静、精密、略带科幻。',
    swatches: ['#0b1020', '#60a5fa', '#a78bfa'],
    vibe: 'serious',
  },
  css: `/* Orbit — Deep Space Mission Control */
[data-theme="orbit"] {
  --font-heading: 'Space Grotesk', 'Noto Sans SC', system-ui, sans-serif;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Space Grotesk', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', monospace;

  --radius-none: 0;
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-xl: 16px;
  --radius-2xl: 20px;
  --radius-full: 9999px;

  --elevation-surface: 0 2px 12px rgb(0 0 0 / 0.35), 0 0 0 1px oklch(0.70 0.12 255 / 0.12);
  --elevation-raised: 0 6px 20px rgb(0 0 0 / 0.45), 0 0 0 1px oklch(0.72 0.13 255 / 0.18);
  --elevation-float: 0 12px 36px rgb(0 0 0 / 0.55), 0 0 0 1px oklch(0.75 0.12 280 / 0.22);
  --elevation-overlay: 0 20px 50px rgb(0 0 0 / 0.65), 0 0 0 1px oklch(0.78 0.10 255 / 0.20);

  --background-50: 0.18 0.035 270;
  --background-100: 0.135 0.030 270;
  --background-200: 0.22 0.040 270;
  --background-300: 0.27 0.042 268;
  --background-400: 0.32 0.044 266;

  --foreground-50: 0.42 0.02 265;
  --foreground-100: 0.48 0.025 265;
  --foreground-200: 0.55 0.03 265;
  --foreground-300: 0.62 0.03 260;
  --foreground-400: 0.70 0.03 255;
  --foreground-500: 0.77 0.03 255;
  --foreground-600: 0.83 0.025 255;
  --foreground-700: 0.88 0.02 255;
  --foreground-800: 0.92 0.015 255;
  --foreground-900: 0.96 0.01 255;
  --foreground-950: 0.98 0.008 255;

  /* Primary — electric sky blue */
  --primary-50: 0.48 0.08 255;
  --primary-100: 0.55 0.10 255;
  --primary-200: 0.62 0.12 255;
  --primary-300: 0.68 0.13 255;
  --primary-400: 0.74 0.13 255;
  --primary-500: 0.78 0.13 255;
  --primary-600: 0.83 0.11 255;
  --primary-700: 0.88 0.09 255;
  --primary-800: 0.92 0.06 255;
  --primary-900: 0.95 0.04 255;
  --primary-950: 0.97 0.02 255;

  /* Accent — soft violet */
  --accent-50: 0.48 0.10 295;
  --accent-100: 0.54 0.12 295;
  --accent-200: 0.60 0.14 295;
  --accent-300: 0.66 0.15 295;
  --accent-400: 0.72 0.15 295;
  --accent-500: 0.76 0.14 295;
  --accent-600: 0.81 0.12 295;
  --accent-700: 0.86 0.09 295;
  --accent-800: 0.90 0.06 295;
  --accent-900: 0.94 0.04 295;
  --accent-950: 0.97 0.02 295;

  --secondary-50: 0.36 0.03 270;
  --secondary-100: 0.42 0.035 270;
  --secondary-200: 0.50 0.04 268;
  --secondary-300: 0.58 0.04 266;
  --secondary-400: 0.66 0.04 264;
  --secondary-500: 0.74 0.035 262;
  --secondary-600: 0.80 0.03 260;
  --secondary-700: 0.86 0.025 258;
  --secondary-800: 0.91 0.018 256;
  --secondary-900: 0.95 0.012 254;
  --secondary-950: 0.98 0.008 252;
}

[data-theme="orbit"] .material-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--elevation-raised), 0 0 28px oklch(0.70 0.12 255 / 0.14);
}

[data-theme="orbit"] body::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.45;
  mix-blend-mode: screen;
  background-image:
    radial-gradient(1.2px 1.2px at 12% 18%, oklch(0.92 0.04 255 / 0.55), transparent),
    radial-gradient(1px 1px at 38% 62%, oklch(0.90 0.05 295 / 0.40), transparent),
    radial-gradient(1.4px 1.4px at 72% 28%, oklch(0.95 0.03 255 / 0.50), transparent),
    radial-gradient(1px 1px at 88% 74%, oklch(0.88 0.06 295 / 0.35), transparent),
    radial-gradient(1.1px 1.1px at 55% 10%, oklch(0.93 0.04 255 / 0.40), transparent),
    radial-gradient(0.9px 0.9px at 22% 88%, oklch(0.90 0.04 270 / 0.30), transparent);
}

[data-theme="orbit"] .hairline {
  background: oklch(var(--primary-400) / 0.18);
}
[data-theme="orbit"] .hairline-gradient {
  background: linear-gradient(90deg, transparent, oklch(var(--primary-400) / 0.30), oklch(var(--accent-400) / 0.22), transparent);
}`,
};
