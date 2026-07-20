// ============================================================
// nav.ax CategoryFolderWall — folder tiles + hover/tap site popover
// Desktop: hover opens, leave tile+panel closes (~120ms). Touch/click toggles.
// ============================================================

import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { cn } from '@/lib/utils';
import type { Site } from '@/api/types';

export interface CategoryFolderWallProps {
  categories: Array<{ id: string; name: string; sites: Site[] }>;
  onSiteOpen: (site: Site) => void;
  className?: string;
}

function getDomain(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url.replace(/^https?:\/\//, '').replace(/^www\./, '').split('/')[0] || '';
  }
}

/** Prefer site.icon when it is an http(s) URL; else Google favicon by domain. */
function siteFavicon(site: Site): string {
  const icon = (site.icon || '').trim();
  if (/^https?:\/\//i.test(icon)) return icon;
  const host = getDomain(site.url);
  if (!host) return '';
  return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=64`;
}

function canHoverOpen(): boolean {
  return (
    typeof window !== 'undefined'
    && window.matchMedia('(hover: hover) and (pointer: fine)').matches
  );
}

function FolderSiteIcon({
  site,
  size,
  className,
}: {
  site: Site;
  size: number;
  className?: string;
}) {
  const src = siteFavicon(site);
  if (!src) {
    return (
      <span
        className={cn('inline-block bg-background-100/60', className)}
        style={{ width: size, height: size }}
        aria-hidden
      />
    );
  }

  return (
    <img
      src={src}
      alt=""
      width={size}
      height={size}
      loading="lazy"
      decoding="async"
      className={cn('object-contain', className)}
      style={{ width: size, height: size }}
      onError={(e) => {
        const el = e.currentTarget;
        const host = getDomain(site.url);
        const fallback = host
          ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=64`
          : '';
        if (fallback && el.src !== fallback) {
          el.src = fallback;
        } else {
          el.style.visibility = 'hidden';
        }
      }}
    />
  );
}

type PanelPlacement = {
  top: number;
  left: number;
  transform: string;
  maxHeight: number;
};

function placePanel(tileEl: HTMLElement, panelW: number, panelMaxH: number): PanelPlacement {
  const rect = tileEl.getBoundingClientRect();
  const gap = 8;
  const margin = 8;
  const vw = window.innerWidth;
  const vh = window.innerHeight;

  const spaceBelow = vh - rect.bottom - gap - margin;
  const spaceAbove = rect.top - gap - margin;
  const placeAbove = spaceBelow < 160 && spaceAbove > spaceBelow;
  const maxHeight = Math.max(120, Math.min(panelMaxH, placeAbove ? spaceAbove : spaceBelow));

  let left = rect.left + rect.width / 2;
  const half = panelW / 2;
  if (left - half < margin) left = margin + half;
  if (left + half > vw - margin) left = vw - margin - half;

  if (placeAbove) {
    return {
      top: rect.top - gap,
      left,
      transform: 'translate(-50%, -100%)',
      maxHeight,
    };
  }
  return {
    top: rect.bottom + gap,
    left,
    transform: 'translate(-50%, 0)',
    maxHeight,
  };
}

interface FolderTileProps {
  category: { id: string; name: string; sites: Site[] };
  open: boolean;
  onToggle: () => void;
  onHoverOpen: () => void;
  onHoverIntentClose: () => void;
  onHoverStay: () => void;
  onSiteOpen: (site: Site) => void;
  onClose: () => void;
}

function FolderTile({
  category,
  open,
  onToggle,
  onHoverOpen,
  onHoverIntentClose,
  onHoverStay,
  onSiteOpen,
  onClose,
}: FolderTileProps) {
  const panelId = useId();
  const tileRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const preview = category.sites.slice(0, 4);
  const count = category.sites.length;
  const [placement, setPlacement] = useState<PanelPlacement | null>(null);

  useLayoutEffect(() => {
    if (!open || !tileRef.current) {
      setPlacement(null);
      return;
    }
    const update = () => {
      if (!tileRef.current) return;
      setPlacement(placePanel(tileRef.current, 224, 256));
    };
    update();
    window.addEventListener('resize', update);
    window.addEventListener('scroll', update, true);
    return () => {
      window.removeEventListener('resize', update);
      window.removeEventListener('scroll', update, true);
    };
  }, [open, count]);

  useEffect(() => {
    if (!open || !panelRef.current) return;
    const panel = panelRef.current;
    const focusable = panel.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])',
    );
    const list = Array.from(focusable);
    const first = list[0] ?? panel;
    first.focus();

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
        buttonRef.current?.focus();
        return;
      }
      if (e.key !== 'Tab' || list.length === 0) return;
      const active = document.activeElement as HTMLElement | null;
      const idx = list.indexOf(active as HTMLElement);
      if (e.shiftKey) {
        if (idx <= 0) {
          e.preventDefault();
          list[list.length - 1].focus();
        }
      } else if (idx === list.length - 1 || idx === -1) {
        e.preventDefault();
        list[0].focus();
      }
    };
    panel.addEventListener('keydown', onKeyDown);
    return () => panel.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

  return (
    <div
      ref={tileRef}
      className="relative"
      data-folder-tile={category.id}
      onMouseEnter={() => {
        if (!canHoverOpen()) return;
        onHoverStay();
        onHoverOpen();
      }}
      onMouseLeave={() => {
        if (!canHoverOpen()) return;
        onHoverIntentClose();
      }}
    >
      <button
        ref={buttonRef}
        type="button"
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-controls={panelId}
        aria-label={`${category.name}，${count} 个站点`}
        onClick={onToggle}
        className={cn(
          'w-full aspect-square rounded-2xl p-3 flex flex-col items-center justify-center gap-2',
          'bg-background-50/70 border border-background-200/50 hover:border-primary-300/60',
          'transition-colors duration-150',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50',
          open && 'border-primary-300/60',
        )}
      >
        <div className="grid grid-cols-2 gap-1 w-10 h-10" aria-hidden>
          {Array.from({ length: 4 }).map((_, i) => {
            const site = preview[i];
            return (
              <div
                key={site?.id ?? `empty-${category.id}-${i}`}
                className="rounded-md bg-background-100/80 overflow-hidden flex items-center justify-center"
              >
                {site ? (
                  <FolderSiteIcon site={site} size={16} className="w-full h-full" />
                ) : null}
              </div>
            );
          })}
        </div>
        <span className="text-xs font-medium text-foreground-800 truncate max-w-full">
          {category.name}
        </span>
        <span className="text-[10px] text-foreground-400 tabular-nums">{count}</span>
      </button>

      {open && placement ? (
        <div
          ref={panelRef}
          id={panelId}
          role="dialog"
          aria-label={category.name}
          tabIndex={-1}
          style={{
            position: 'fixed',
            top: placement.top,
            left: placement.left,
            transform: placement.transform,
            maxHeight: placement.maxHeight,
            width: 224,
          }}
          className={cn(
            'z-50 overflow-auto rounded-xl border border-background-200/60',
            'bg-background-0/95 backdrop-blur-md p-3 shadow-lg',
            'focus:outline-none',
          )}
          onMouseEnter={() => {
            if (!canHoverOpen()) return;
            onHoverStay();
          }}
          onMouseLeave={() => {
            if (!canHoverOpen()) return;
            onHoverIntentClose();
          }}
        >
          {count === 0 ? (
            <p className="text-xs text-foreground-400 text-center py-4">暂无站点</p>
          ) : (
            <div className="grid grid-cols-4 gap-2">
              {category.sites.map((site) => (
                <a
                  key={site.id}
                  href={site.url}
                  title={site.title}
                  target="_blank"
                  rel="nofollow noopener noreferrer"
                  className={cn(
                    'flex flex-col items-center gap-1 min-h-[44px] min-w-[40px] rounded-lg p-1',
                    'hover:bg-background-100/80',
                    'focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-400/50',
                  )}
                  onClick={(e) => {
                    e.preventDefault();
                    onSiteOpen(site);
                  }}
                >
                  <FolderSiteIcon site={site} size={28} className="w-7 h-7" />
                  <span className="text-[9px] text-foreground-700 line-clamp-1 w-full text-center">
                    {site.title}
                  </span>
                </a>
              ))}
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}

export default function CategoryFolderWall({
  categories,
  onSiteOpen,
  className,
}: CategoryFolderWallProps) {
  const [openId, setOpenId] = useState<string | null>(null);
  const closeTimer = useRef<number | null>(null);
  const wallRef = useRef<HTMLDivElement>(null);

  const clearCloseTimer = useCallback(() => {
    if (closeTimer.current != null) {
      window.clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  }, []);

  const scheduleClose = useCallback(() => {
    clearCloseTimer();
    closeTimer.current = window.setTimeout(() => {
      setOpenId(null);
      closeTimer.current = null;
    }, 120);
  }, [clearCloseTimer]);

  const closeOpen = useCallback(() => {
    clearCloseTimer();
    setOpenId(null);
  }, [clearCloseTimer]);

  useEffect(() => () => clearCloseTimer(), [clearCloseTimer]);

  useEffect(() => {
    if (!openId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        clearCloseTimer();
        setOpenId(null);
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [openId, clearCloseTimer]);

  useEffect(() => {
    if (!openId) return;
    const onPointerDown = (e: PointerEvent) => {
      const root = wallRef.current;
      if (!root) return;
      const target = e.target as Node | null;
      if (!target) return;
      const openTile = root.querySelector(
        `[data-folder-tile="${CSS.escape(openId)}"]`,
      );
      // Fixed popover is portaled via fixed coords but still inside tile DOM tree
      if (openTile && !openTile.contains(target)) {
        clearCloseTimer();
        setOpenId(null);
      }
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  }, [openId, clearCloseTimer]);

  return (
    <div
      ref={wallRef}
      className={cn(
        'grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 gap-3',
        className,
      )}
    >
      {categories.map((cat) => (
        <FolderTile
          key={cat.id}
          category={cat}
          open={openId === cat.id}
          onToggle={() => {
            clearCloseTimer();
            setOpenId((prev) => (prev === cat.id ? null : cat.id));
          }}
          onHoverOpen={() => {
            clearCloseTimer();
            setOpenId(cat.id);
          }}
          onHoverIntentClose={scheduleClose}
          onHoverStay={clearCloseTimer}
          onSiteOpen={onSiteOpen}
          onClose={closeOpen}
        />
      ))}
    </div>
  );
}
