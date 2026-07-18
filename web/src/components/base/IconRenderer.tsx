/* eslint-disable react-refresh/only-export-components */
// ============================================================
// nav.ax IconRenderer — auto-detects icon type and renders accordingly
// Supports: emoji, image URL, Remix Icon (ri-*), plain text fallback
// ============================================================

import { cn } from '@/lib/utils';
import { resolveSiteIcon } from '@/lib/linkUtils';

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
  /** When icon is empty (e.g. imported bookmarks), derive favicon from this URL. */
  url?: string;
  className?: string;
  /** Container size in Tailwind classes, e.g. "w-6 h-6" */
  containerClassName?: string;
  /** img-only: alt text */
  alt?: string;
  /** Pixel size for fixed square icons (img + emoji/remix font size). */
  size?: number;
}

export default function IconRenderer({ icon, url, className, containerClassName, alt, size }: IconRendererProps) {
  const resolved = resolveSiteIcon(icon, url);
  const type = detectIconType(resolved);
  const defaultContainer = containerClassName || '';
  // Prefer explicit size so remote favicons never blow out layout.
  const boxStyle = size
    ? { width: size, height: size, minWidth: size, minHeight: size }
    : undefined;

  switch (type) {
    case 'image':
      return (
        <div
          className={cn(
            'overflow-hidden rounded-md flex-shrink-0 inline-flex items-center justify-center',
            // Without size prop, containerClassName must supply w/h; default to 1rem box.
            !size && !defaultContainer && 'w-4 h-4',
            defaultContainer,
          )}
          style={boxStyle}
        >
          <img
            src={resolved}
            alt={alt || 'icon'}
            width={size || 16}
            height={size || 16}
            decoding="async"
            loading="lazy"
            className={cn(
              'block max-w-full max-h-full w-full h-full object-contain',
              className,
            )}
            onError={(e) => {
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
        <span
          className={cn('inline-flex items-center justify-center flex-shrink-0', defaultContainer)}
          style={boxStyle}
        >
          <span
            className={cn('leading-none', className)}
            style={size ? { fontSize: size } : undefined}
            role="img"
            aria-label={alt || 'icon'}
          >
            {resolved}
          </span>
        </span>
      );

    case 'remix':
    case 'fallback':
    default:
      return (
        <span
          className={cn('inline-flex items-center justify-center flex-shrink-0', defaultContainer)}
          style={boxStyle}
        >
          <i
            className={cn(resolved || 'ri-link', className)}
            style={size ? { fontSize: size } : undefined}
          />
        </span>
      );
  }
}
