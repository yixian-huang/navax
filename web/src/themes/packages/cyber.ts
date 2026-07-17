// ============================================================
// Theme Package: Cyber
// Near-black canvas, electric cyan primary, magenta accent.
// Neon-edge cards, scanline grain — HUD / cyberpunk dashboard.
// Font: Space Grotesk heading / Inter body / JetBrains Mono
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const cyberTheme: ThemePackage = {
  id: 'cyber',
  meta: {
    name: 'Cyber',
    subtitle: '霓虹·赛博',
    description: '近黑底上的电青与品红。细霓虹描边与扫描线，像未来都市的 HUD 面板。',
    swatches: ['#0a0e17', '#22d3ee', '#f472b6'],
    vibe: 'serious',
  },
  css: `/* Cyber — Neon HUD */
[data-theme="cyber"] {
  --font-heading: 'Space Grotesk', 'Noto Sans SC', system-ui, sans-serif;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'Space Grotesk', 'Noto Sans SC', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', monospace;

  --radius-none: 0;
  --radius-sm: 2px;
  --radius-md: 4px;
  --radius-lg: 6px;
  --radius-xl: 8px;
  --radius-2xl: 10px;
  --radius-full: 9999px;

  --elevation-surface: 0 0 0 1px oklch(0.75 0.14 210 / 0.18), 0 0 18px oklch(0.75 0.14 210 / 0.06);
  --elevation-raised: 0 0 0 1px oklch(0.78 0.14 210 / 0.28), 0 0 24px oklch(0.75 0.14 210 / 0.10);
  --elevation-float: 0 0 0 1px oklch(0.80 0.14 210 / 0.36), 0 8px 32px oklch(0.55 0.18 330 / 0.18);
  --elevation-overlay: 0 0 0 1px oklch(0.82 0.12 210 / 0.40), 0 16px 48px rgb(0 0 0 / 0.55);

  --background-50: 0.16 0.025 255;
  --background-100: 0.12 0.022 255;
  --background-200: 0.20 0.030 255;
  --background-300: 0.25 0.032 255;
  --background-400: 0.30 0.034 255;

  --foreground-50: 0.42 0.02 230;
  --foreground-100: 0.48 0.025 230;
  --foreground-200: 0.55 0.03 230;
  --foreground-300: 0.62 0.035 220;
  --foreground-400: 0.70 0.04 210;
  --foreground-500: 0.78 0.04 210;
  --foreground-600: 0.84 0.035 210;
  --foreground-700: 0.89 0.03 210;
  --foreground-800: 0.93 0.02 210;
  --foreground-900: 0.96 0.015 210;
  --foreground-950: 0.98 0.01 210;

  /* Primary — electric cyan */
  --primary-50: 0.45 0.08 210;
  --primary-100: 0.52 0.10 210;
  --primary-200: 0.60 0.12 210;
  --primary-300: 0.68 0.13 210;
  --primary-400: 0.76 0.14 210;
  --primary-500: 0.82 0.14 210;
  --primary-600: 0.86 0.12 210;
  --primary-700: 0.90 0.10 210;
  --primary-800: 0.93 0.07 210;
  --primary-900: 0.96 0.04 210;
  --primary-950: 0.98 0.02 210;

  /* Accent — neon magenta */
  --accent-50: 0.48 0.12 350;
  --accent-100: 0.54 0.14 350;
  --accent-200: 0.60 0.16 350;
  --accent-300: 0.66 0.17 350;
  --accent-400: 0.72 0.18 350;
  --accent-500: 0.76 0.17 350;
  --accent-600: 0.81 0.14 350;
  --accent-700: 0.86 0.11 350;
  --accent-800: 0.90 0.08 350;
  --accent-900: 0.94 0.05 350;
  --accent-950: 0.97 0.025 350;

  --secondary-50: 0.35 0.03 255;
  --secondary-100: 0.42 0.035 255;
  --secondary-200: 0.50 0.04 255;
  --secondary-300: 0.58 0.045 255;
  --secondary-400: 0.66 0.05 250;
  --secondary-500: 0.74 0.05 245;
  --secondary-600: 0.80 0.045 240;
  --secondary-700: 0.86 0.035 235;
  --secondary-800: 0.91 0.025 230;
  --secondary-900: 0.95 0.015 225;
  --secondary-950: 0.98 0.01 220;
}

[data-theme="cyber"] .material-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--elevation-raised), 0 0 20px oklch(0.75 0.14 210 / 0.12);
}

[data-theme="cyber"] body::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.35;
  mix-blend-mode: screen;
  background-image:
    repeating-linear-gradient(
      0deg,
      transparent,
      transparent 2px,
      oklch(0.70 0.12 210 / 0.03) 2px,
      oklch(0.70 0.12 210 / 0.03) 3px
    );
}

[data-theme="cyber"] .hairline {
  background: oklch(var(--primary-400) / 0.22);
}
[data-theme="cyber"] .hairline-gradient {
  background: linear-gradient(90deg, transparent, oklch(var(--primary-400) / 0.35), oklch(var(--accent-400) / 0.25), transparent);
}`,
};
