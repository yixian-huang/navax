// ============================================================
// nav.ax DensitySwitcher — ghost, almost invisible
// Icons must read as three distinct layouts at 16px.
// ============================================================

import { cn } from '@/lib/utils';
import type { Density } from '@/api/types';

interface DensitySwitcherProps {
  density: Density;
  onChange: (d: Density) => void;
}

// list = rows · compact = dense grid · comfortable = open grid
// (avoid ri-apps / ri-apps-2 which look identical at small size)
const options: { key: Density; icon: string; title: string }[] = [
  { key: 'list', icon: 'ri-list-check', title: '列表' },
  { key: 'compact', icon: 'ri-grid-fill', title: '紧凑' },
  { key: 'comfortable', icon: 'ri-layout-grid-line', title: '舒适' },
];

export default function DensitySwitcher({ density, onChange }: DensitySwitcherProps) {
  return (
    <div className="flex items-center wallpaper-type" role="radiogroup" aria-label="显示密度">
      {options.map(opt => (
        <button
          key={opt.key}
          type="button"
          role="radio"
          aria-checked={density === opt.key}
          title={opt.title}
          onClick={() => onChange(opt.key)}
          className={cn(
            'w-8 h-8 flex items-center justify-center rounded-lg transition-all duration-200 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-400/50',
            density === opt.key
              ? 'text-primary-500'
              : 'text-foreground-400 hover:text-foreground-700 hover:bg-background-50/30',
          )}
        >
          <i className={cn(opt.icon, 'text-base')} aria-hidden />
        </button>
      ))}
    </div>
  );
}
