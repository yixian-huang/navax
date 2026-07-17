// ============================================================
// nav.ax DensitySwitcher — ghost, almost invisible
// Icons must read as three distinct layouts at 16px.
// On wallpaper: use --wp-ink for contrast.
// ============================================================

import { cn } from '@/lib/utils';
import type { Density } from '@/api/types';

interface DensitySwitcherProps {
  density: Density;
  onChange: (d: Density) => void;
}

const options: { key: Density; icon: string; title: string }[] = [
  { key: 'list', icon: 'ri-list-check', title: '列表' },
  { key: 'compact', icon: 'ri-grid-fill', title: '紧凑' },
  { key: 'comfortable', icon: 'ri-layout-grid-line', title: '舒适' },
];

export default function DensitySwitcher({ density, onChange }: DensitySwitcherProps) {
  return (
    <div
      className="density-switcher flex items-center wallpaper-type rounded-lg p-0.5"
      role="radiogroup"
      aria-label="显示密度"
    >
      {options.map(opt => (
        <button
          key={opt.key}
          type="button"
          role="radio"
          aria-checked={density === opt.key}
          aria-label={opt.title}
          title={opt.title}
          onClick={() => onChange(opt.key)}
          className={cn(
            'density-switcher-btn w-8 h-8 flex items-center justify-center rounded-lg transition-all duration-200 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-400/50',
            density === opt.key
              ? 'density-switcher-btn-active text-primary-500'
              : 'text-foreground-400 hover:text-foreground-700 hover:bg-background-50/30',
          )}
        >
          <i className={cn(opt.icon, 'text-base')} aria-hidden />
        </button>
      ))}
    </div>
  );
}
