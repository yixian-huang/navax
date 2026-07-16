// ============================================================
// nav.ax DensitySwitcher — ghost, almost invisible
// ============================================================

import { cn } from '@/lib/utils';
import type { Density } from '@/api/types';

interface DensitySwitcherProps {
  density: Density;
  onChange: (d: Density) => void;
}

const options: { key: Density; icon: string; title: string }[] = [
  { key: 'list', icon: 'ri-list-check', title: '列表' },
  { key: 'compact', icon: 'ri-apps-2-line', title: '紧凑' },
  { key: 'comfortable', icon: 'ri-apps-line', title: '舒适' },
];

export default function DensitySwitcher({ density, onChange }: DensitySwitcherProps) {
  return (
    <div className="flex items-center" role="radiogroup" aria-label="显示密度">
      {options.map(opt => (
        <button
          key={opt.key}
          role="radio"
          aria-checked={density === opt.key}
          title={opt.title}
          onClick={() => onChange(opt.key)}
          className={cn(
            'w-8 h-8 flex items-center justify-center rounded-lg transition-all duration-200 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-400/50',
            density === opt.key
              ? 'text-primary-500'
              : 'text-foreground-300 hover:text-foreground-600 hover:bg-background-200'
          )}
        >
          <i className={cn(opt.icon, 'text-base')} />
        </button>
      ))}
    </div>
  );
}
