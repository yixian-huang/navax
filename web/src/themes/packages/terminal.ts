// ============================================================
// Theme Package: Terminal
// CRT black, phosphor green primary, amber secondary.
// Classic hacker / systems console — dense, monospaced feel.
// Font: JetBrains Mono / Space Mono for headings
// ============================================================

import type { ThemePackage } from '@/themes/types';

export const terminalTheme: ThemePackage = {
  id: 'terminal',
  meta: {
    name: 'Terminal',
    subtitle: '终端·磷光',
    description: 'CRT 黑底与磷光绿。等宽字感与锐利直角，像深夜里亮着的系统控制台。',
    swatches: ['#0c120c', '#4ade80', '#fbbf24'],
    vibe: 'serious',
  },
  css: `/* Terminal — Phosphor Console */
[data-theme="terminal"] {
  --font-heading: 'JetBrains Mono', 'Space Mono', 'Noto Sans SC', monospace;
  --font-body: 'Inter', 'Noto Sans SC', system-ui, sans-serif;
  --font-label: 'JetBrains Mono', 'Noto Sans SC', monospace;
  --font-mono: 'JetBrains Mono', 'Space Mono', monospace;

  --radius-none: 0;
  --radius-sm: 0;
  --radius-md: 2px;
  --radius-lg: 3px;
  --radius-xl: 4px;
  --radius-2xl: 6px;
  --radius-full: 9999px;

  --elevation-surface: 0 0 0 1px oklch(0.75 0.16 145 / 0.16);
  --elevation-raised: 0 0 0 1px oklch(0.78 0.16 145 / 0.28), 0 0 16px oklch(0.70 0.16 145 / 0.10);
  --elevation-float: 0 0 0 1px oklch(0.80 0.16 145 / 0.34), 0 8px 24px rgb(0 0 0 / 0.50);
  --elevation-overlay: 0 0 0 1px oklch(0.82 0.14 145 / 0.38), 0 16px 40px rgb(0 0 0 / 0.60);

  --background-50: 0.155 0.025 145;
  --background-100: 0.11 0.020 145;
  --background-200: 0.195 0.030 145;
  --background-300: 0.24 0.032 145;
  --background-400: 0.29 0.034 145;

  --foreground-50: 0.40 0.04 145;
  --foreground-100: 0.48 0.05 145;
  --foreground-200: 0.56 0.06 145;
  --foreground-300: 0.64 0.08 145;
  --foreground-400: 0.72 0.10 145;
  --foreground-500: 0.78 0.12 145;
  --foreground-600: 0.84 0.11 145;
  --foreground-700: 0.89 0.09 145;
  --foreground-800: 0.93 0.06 145;
  --foreground-900: 0.96 0.04 145;
  --foreground-950: 0.98 0.02 145;

  /* Primary — phosphor green */
  --primary-50: 0.45 0.10 145;
  --primary-100: 0.52 0.13 145;
  --primary-200: 0.60 0.15 145;
  --primary-300: 0.68 0.16 145;
  --primary-400: 0.75 0.17 145;
  --primary-500: 0.80 0.17 145;
  --primary-600: 0.85 0.14 145;
  --primary-700: 0.89 0.11 145;
  --primary-800: 0.93 0.08 145;
  --primary-900: 0.96 0.05 145;
  --primary-950: 0.98 0.025 145;

  /* Accent — amber status LED */
  --accent-50: 0.55 0.10 85;
  --accent-100: 0.62 0.12 85;
  --accent-200: 0.68 0.13 85;
  --accent-300: 0.74 0.14 85;
  --accent-400: 0.80 0.14 85;
  --accent-500: 0.84 0.14 85;
  --accent-600: 0.88 0.12 85;
  --accent-700: 0.91 0.09 85;
  --accent-800: 0.94 0.06 85;
  --accent-900: 0.96 0.04 85;
  --accent-950: 0.98 0.02 85;

  --secondary-50: 0.32 0.03 145;
  --secondary-100: 0.40 0.04 145;
  --secondary-200: 0.48 0.05 145;
  --secondary-300: 0.56 0.06 145;
  --secondary-400: 0.64 0.07 145;
  --secondary-500: 0.72 0.08 145;
  --secondary-600: 0.79 0.07 145;
  --secondary-700: 0.85 0.06 145;
  --secondary-800: 0.90 0.04 145;
  --secondary-900: 0.94 0.03 145;
  --secondary-950: 0.97 0.015 145;
}

[data-theme="terminal"] .material-card:hover {
  transform: none;
  box-shadow: var(--elevation-raised);
}

[data-theme="terminal"] h1,
[data-theme="terminal"] h2 {
  letter-spacing: -0.02em;
  text-transform: none;
}

[data-theme="terminal"] body::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.28;
  mix-blend-mode: screen;
  background-image:
    repeating-linear-gradient(
      0deg,
      transparent,
      transparent 3px,
      oklch(0.70 0.14 145 / 0.04) 3px,
      oklch(0.70 0.14 145 / 0.04) 4px
    ),
    radial-gradient(ellipse at center, transparent 50%, oklch(0.10 0.02 145 / 0.45) 100%);
}

[data-theme="terminal"] .hairline {
  background: oklch(var(--primary-400) / 0.20);
}
[data-theme="terminal"] .hairline-gradient {
  background: linear-gradient(90deg, transparent, oklch(var(--primary-400) / 0.32), oklch(var(--accent-400) / 0.20), transparent);
}`,
};
