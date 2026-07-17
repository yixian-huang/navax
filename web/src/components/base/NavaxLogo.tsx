// ============================================================
// nav.ax brand mark + wordmark
// Compass-node monogram: navigation paths meeting an axis point.
// Colors track theme tokens (primary / accent / background).
// ============================================================

import { cn } from '@/lib/utils';

type LogoSize = 'sm' | 'md' | 'lg';

interface NavaxLogoProps {
  /** sm = sidebar compact · md = shell header · lg = auth hero */
  size?: LogoSize;
  /** Hide the “nav.ax” wordmark (icon only). */
  markOnly?: boolean;
  className?: string;
  /** Extra classes on the wordmark span. */
  wordmarkClassName?: string;
}

const sizeMap = {
  sm: { mark: 22, gap: 'gap-1.5', text: 'text-sm', tracking: 'tracking-tight' },
  md: { mark: 28, gap: 'gap-2', text: 'text-base', tracking: 'tracking-tight' },
  lg: { mark: 36, gap: 'gap-2.5', text: 'text-2xl', tracking: 'tracking-tight' },
} as const;

/** Geometric mark: orbit + route nodes → destination (axis). */
function NavaxMark({ size, className }: { size: number; className?: string }) {
  const id = `navax-mark-${size}`;
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={cn('flex-shrink-0', className)}
      aria-hidden
    >
      <defs>
        <linearGradient id={`${id}-plate`} x1="4" y1="2" x2="28" y2="30" gradientUnits="userSpaceOnUse">
          <stop stopColor="oklch(var(--primary-500))" />
          <stop offset="1" stopColor="oklch(var(--primary-700))" />
        </linearGradient>
        <linearGradient id={`${id}-sheen`} x1="8" y1="4" x2="24" y2="20" gradientUnits="userSpaceOnUse">
          <stop stopColor="oklch(var(--background-50))" stopOpacity="0.28" />
          <stop offset="1" stopColor="oklch(var(--background-50))" stopOpacity="0" />
        </linearGradient>
        <filter id={`${id}-glow`} x="-40%" y="-40%" width="180%" height="180%">
          <feGaussianBlur stdDeviation="1.1" result="b" />
          <feMerge>
            <feMergeNode in="b" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Soft plate */}
      <rect x="1" y="1" width="30" height="30" rx="9" fill={`url(#${id}-plate)`} />
      <rect x="1" y="1" width="30" height="30" rx="9" fill={`url(#${id}-sheen)`} />

      {/* Orbit ring — the “axis” field */}
      <circle
        cx="16"
        cy="16"
        r="8.5"
        stroke="oklch(var(--background-50))"
        strokeOpacity="0.35"
        strokeWidth="1.25"
      />

      {/* Route: lower-left → hub → upper-right (navigation path) */}
      <path
        d="M9.5 21.5 L14.2 16.8 M17.8 13.2 L22.5 8.5"
        stroke="oklch(var(--background-50))"
        strokeOpacity="0.92"
        strokeWidth="1.75"
        strokeLinecap="round"
      />

      {/* Waypoint nodes */}
      <circle cx="9.5" cy="21.5" r="1.55" fill="oklch(var(--background-50))" fillOpacity="0.75" />
      <circle cx="16" cy="16" r="2.15" fill="oklch(var(--background-50))" />

      {/* Destination / axis point — brand accent */}
      <circle
        cx="22.5"
        cy="8.5"
        r="2.35"
        fill="oklch(var(--accent-400))"
        filter={`url(#${id}-glow)`}
      />
      <circle cx="22.5" cy="8.5" r="1" fill="oklch(var(--background-50))" fillOpacity="0.9" />
    </svg>
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
