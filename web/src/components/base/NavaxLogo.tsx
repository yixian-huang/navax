// ============================================================
// nav.ax brand mark + wordmark
// Site-grid monogram from design export (logo-export / public marks).
// ============================================================

import { cn } from '@/lib/utils';

type LogoSize = 'sm' | 'md' | 'lg';
type MarkVariant = 'auto' | 'light' | 'dark';

interface NavaxLogoProps {
  /** sm = sidebar compact · md = shell header · lg = auth hero */
  size?: LogoSize;
  /** Hide the “nav.ax” wordmark (icon only). */
  markOnly?: boolean;
  className?: string;
  /** Extra classes on the wordmark span. */
  wordmarkClassName?: string;
  /**
   * Mark surface variant:
   * - auto (default): light UI uses mark-light; dark themes / dark wallpaper use mark-dark
   * - light / dark: force the design mark for that surface
   */
  markVariant?: MarkVariant;
}

const sizeMap = {
  sm: { mark: 22, gap: 'gap-1.5', text: 'text-sm', tracking: 'tracking-tight' },
  md: { mark: 28, gap: 'gap-2', text: 'text-base', tracking: 'tracking-tight' },
  lg: { mark: 36, gap: 'gap-2.5', text: 'text-2xl', tracking: 'tracking-tight' },
} as const;

/**
 * 2×2 site grid mark (design export).
 * light = ink squares on transparent (for light UI)
 * dark = paper squares on transparent (for dark UI)
 */
function NavaxMark({
  size,
  className,
  variant = 'auto',
}: {
  size: number;
  className?: string;
  variant?: MarkVariant;
}) {
  return (
    <span
      className={cn(
        'navax-mark relative inline-block flex-shrink-0',
        variant === 'light' && 'navax-mark--force-light',
        variant === 'dark' && 'navax-mark--force-dark',
        className,
      )}
      style={{ width: size, height: size }}
      aria-hidden
    >
      <img
        src="/mark-light.svg"
        alt=""
        width={size}
        height={size}
        draggable={false}
        className="navax-mark-light block h-full w-full select-none"
      />
      <img
        src="/mark-dark.svg"
        alt=""
        width={size}
        height={size}
        draggable={false}
        className="navax-mark-dark absolute inset-0 hidden h-full w-full select-none"
      />
    </span>
  );
}

/**
 * Brand logo for shell headers, auth screens, and sidebars.
 * Prefer this over raw “nav.ax” text for any primary brand placement.
 */
export default function NavaxLogo({
  size = 'md',
  markOnly = false,
  className,
  wordmarkClassName,
  markVariant = 'auto',
}: NavaxLogoProps) {
  const s = sizeMap[size];

  return (
    <span
      className={cn(
        'inline-flex items-center select-none',
        s.gap,
        className,
      )}
    >
      <NavaxMark
        size={s.mark}
        variant={markVariant}
        className="transition-transform duration-300 ease-out group-hover:scale-[1.04] group-hover:rotate-[-2deg]"
      />
      {!markOnly ? (
        <span
          className={cn(
            'navax-wordmark font-heading font-semibold leading-none',
            s.text,
            s.tracking,
            wordmarkClassName,
          )}
        >
          <span className="navax-wordmark-nav text-foreground-900">nav</span>
          <span className="navax-wordmark-dot text-accent-500" aria-hidden>.</span>
          <span className="navax-wordmark-ax text-foreground-700 font-medium">ax</span>
        </span>
      ) : (
        <span className="sr-only">nav.ax</span>
      )}
    </span>
  );
}

export { NavaxMark };
