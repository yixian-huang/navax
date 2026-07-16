/* eslint-disable react-refresh/only-export-components */
// ============================================================
// nav.ax IconRenderer — auto-detects icon type and renders accordingly
// Supports: emoji, image URL, Remix Icon (ri-*), plain text fallback
// ============================================================

import { cn } from '@/lib/utils';

type IconType = 'emoji' | 'image' | 'remix' | 'fallback';

export function detectIconType(icon: string): IconType {
  if (!icon || !icon.trim()) return 'fallback';
  const trimmed = icon.trim();

  // Image URL: starts with http:// or https://
  if (/^https?:\/\//.test(trimmed)) return 'image';

  // Remix Icon: starts with ri-
  if (/^ri-/.test(trimmed)) return 'remix';

  // Emoji detection: check if string is short and contains emoji characters
  const emojiRegex = /\p{Emoji}/u;
  if (emojiRegex.test(trimmed) && trimmed.length <= 8) return 'emoji';

  // Fallback: try as Remix Icon anyway
  return 'remix';
}

interface IconRendererProps {
  icon: string;
  className?: string;
  /** Container size in Tailwind classes, e.g. "w-6 h-6" */
  containerClassName?: string;
  /** img-only: alt text */
  alt?: string;
  size?: number;
}

export default function IconRenderer({ icon, className, containerClassName, alt, size }: IconRendererProps) {
  const type = detectIconType(icon);
  const defaultContainer = containerClassName || '';

  switch (type) {
    case 'image':
      return (
        <div className={cn('overflow-hidden rounded-md flex-shrink-0', defaultContainer)}>
          <img
            src={icon.trim()}
            alt={alt || 'icon'}
            className={cn('w-full h-full object-cover', className)}
            onError={(e) => {
              // Fallback to a generic icon on load failure
              const target = e.currentTarget;
              target.style.display = 'none';
              const fallback = target.parentElement?.querySelector('.icon-fallback');
              if (fallback) fallback.classList.remove('hidden');
            }}
          />
          <span className="icon-fallback hidden w-full h-full flex items-center justify-center bg-background-100 text-foreground-400">
            <i className="ri-link text-xs" />
          </span>
        </div>
      );

    case 'emoji':
      return (
        <span className={cn('inline-flex items-center justify-center flex-shrink-0', defaultContainer)}>
          <span className={cn('leading-none', className)} style={size ? { fontSize: size } : undefined} role="img" aria-label={alt || 'icon'}>
            {icon.trim()}
          </span>
        </span>
      );

    case 'remix':
    case 'fallback':
    default:
      return (
        <span className={cn('inline-flex items-center justify-center flex-shrink-0', defaultContainer)}>
          <i className={cn(icon.trim() || 'ri-link', className)} style={size ? { fontSize: size } : undefined} />
        </span>
      );
  }
}
