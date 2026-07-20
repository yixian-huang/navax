// ============================================================
// nav.ax SiteCard — Refined Neutral / Material
// Comfortable: icon + title side-by-side; list: low-opacity row wash on wallpaper.
// ============================================================

import { useCallback } from 'react';
import { cn } from '@/lib/utils';
import type { Site, Density } from '@/api/types';
import { useContextMenu, createSiteContextActions } from '@/components/base/ContextMenu';
import { useToast } from '@/components/base/Toast';
import IconRenderer from '@/components/base/IconRenderer';

interface SiteCardProps {
  site: Site;
  density: Density;
  onOpen?: (site: Site) => void;
  onEdit?: (site: Site) => void;
  onDelete?: (site: Site) => void;
  searchQuery?: string;
}

function getDomain(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url.replace(/^https?:\/\//, '').replace(/^www\./, '').split('/')[0];
  }
}

function getFaviconUrl(url: string): string {
  return `https://www.google.com/s2/favicons?domain=${getDomain(url)}&sz=64`;
}

/** Prefer stored image icon; fall back to domain favicon (never leave empty). */
function resolveSiteIcon(site: Site): string {
  const icon = (site.icon || '').trim();
  if (/^https?:\/\//i.test(icon)) return icon;
  return getFaviconUrl(site.url);
}

function SiteIcon({ site, size, className }: { site: Site; size: number; className?: string }) {
  const src = resolveSiteIcon(site);
  if (/^https?:\/\//i.test(src)) {
    return (
      <img
        src={src}
        alt=""
        width={size}
        height={size}
        loading="lazy"
        className={cn('object-contain', className)}
        style={{ width: size, height: size }}
        onError={e => {
          const el = e.currentTarget;
          const fallback = getFaviconUrl(site.url);
          if (el.src !== fallback) el.src = fallback;
          else el.style.visibility = 'hidden';
        }}
      />
    );
  }
  return <IconRenderer icon={src || 'ri-link'} size={size} className={className} />;
}

function HighlightText({ text, query }: { text: string; query: string }) {
  if (!query || !text) return <span>{text}</span>;

  const escaped = query.replace(/[.*+?^${}()|[\]\\\\]/g, '\\\\$&');
  const lowerQuery = query.toLowerCase();
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'));

  if (parts.length === 1) return <span>{text}</span>;

  return (
    <span>
      {parts.map((part, i) => {
        const key = `${part}-${i}`;
        const isMatch = part.toLowerCase() === lowerQuery;
        return isMatch
          ? <mark key={key} className="bg-accent-200/40 text-foreground-950 rounded-sm px-0.5">{part}</mark>
          : <span key={key}>{part}</span>;
      })}
    </span>
  );
}

function siteDescription(site: Site): string {
  return (site.description || '').trim();
}

function tooltipFor(site: Site, domain: string): string {
  const desc = siteDescription(site);
  if (desc) return `${site.title} — ${desc}`;
  return `${site.title} · ${domain}`;
}

function CardWrapper({
  site,
  onOpen,
  onEdit,
  onDelete,
  children,
  className,
  title,
}: {
  site: Site;
  onOpen?: (site: Site) => void;
  onEdit?: (site: Site) => void;
  onDelete?: (site: Site) => void;
  children: React.ReactNode;
  className?: string;
  title?: string;
}) {
  const { handleContextMenu, portal } = useContextMenu();
  const { toast } = useToast();

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    if (onOpen) onOpen(site);
    else window.open(site.url, '_blank', 'noopener,noreferrer');
  }, [site, onOpen]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (onOpen) onOpen(site);
      else window.open(site.url, '_blank', 'noopener,noreferrer');
    }
  }, [site, onOpen]);

  const handleCopyLink = useCallback(() => {
    navigator.clipboard.writeText(site.url).then(() => {
      toast('success', '链接已复制');
    }).catch(() => {
      toast('error', '复制失败');
    });
  }, [site.url, toast]);

  const onContextMenu = useCallback((e: React.MouseEvent) => {
    handleContextMenu(e, createSiteContextActions(site, {
      onOpen: () => {
        if (onOpen) onOpen(site);
        else window.open(site.url, '_blank', 'noopener,noreferrer');
      },
      onCopyLink: handleCopyLink,
      onEdit: onEdit ? () => onEdit(site) : undefined,
      onDelete: onDelete ? () => onDelete(site) : undefined,
    }));
  }, [handleContextMenu, site, handleCopyLink, onOpen, onEdit, onDelete]);

  return (
    <>
      <a
        href={site.url}
        target="_blank"
        rel="nofollow noopener noreferrer"
        title={title}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        onContextMenu={onContextMenu}
        tabIndex={0}
        className={cn(
          'group cursor-pointer min-w-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50',
          className,
        )}
      >
        {children}
      </a>
      {portal}
    </>
  );
}

export default function SiteCard({ site, density, onOpen, onEdit, onDelete, searchQuery }: SiteCardProps) {
  const domain = getDomain(site.url);
  const q = searchQuery || '';
  const desc = siteDescription(site);
  const tip = tooltipFor(site, domain);
  const shared = { site, onOpen, onEdit, onDelete, title: tip };

  if (density === 'list') {
    const secondary = desc || domain;
    return (
      <CardWrapper
        {...shared}
        className="site-card-list flex items-center gap-2.5 px-2.5 py-2 rounded-lg transition-colors duration-200 focus-visible:outline-offset-[-2px] hover:bg-background-100/60"
      >
        <span className="site-card-favicon flex h-8 w-8 flex-shrink-0 items-center justify-center">
          <SiteIcon site={site} size={18} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="site-card-title block text-sm font-medium text-foreground-800 truncate group-hover:text-accent-500 transition-colors duration-200">
            <HighlightText text={site.title} query={q} />
          </span>
          <span
            className={cn(
              'block text-[11px] truncate max-w-full',
              desc
                ? 'site-card-list-desc text-foreground-500'
                : 'site-card-list-domain text-foreground-400 font-mono',
            )}
          >
            <HighlightText text={secondary} query={q} />
          </span>
        </span>
        <i className="ri-arrow-right-up-line text-sm text-foreground-300 opacity-0 group-hover:opacity-100 transition-all duration-200 flex-shrink-0" />
      </CardWrapper>
    );
  }

  if (density === 'compact') {
    return (
      <CardWrapper {...shared} className="material-card flex flex-col items-center gap-1.5 p-2 min-h-[3.75rem]">
        <SiteIcon site={site} size={22} />
        <span className="site-card-title text-[10px] font-medium text-foreground-700 text-center line-clamp-2 w-full leading-tight group-hover:text-accent-500 transition-colors duration-200">
          <HighlightText text={site.title} query={q} />
        </span>
      </CardWrapper>
    );
  }

  // Comfortable: icon + title only; domain/desc live in CardWrapper title tooltip.
  return (
    <CardWrapper
      {...shared}
      className="material-card site-card-comfortable flex items-center gap-2.5 px-2.5 py-2 min-h-[3.25rem]"
    >
      <span className="site-card-favicon flex h-7 w-7 flex-shrink-0 items-center justify-center">
        <SiteIcon site={site} size={28} />
      </span>
      <div className="min-w-0 flex-1 flex items-center gap-1.5">
        <h3 className="site-card-title min-w-0 flex-1 text-[13px] font-semibold text-foreground-900 line-clamp-1 group-hover:text-accent-500 transition-colors duration-200">
          <HighlightText text={site.title} query={q} />
        </h3>
        <i className="ri-arrow-right-up-line text-sm text-foreground-300 opacity-0 -translate-x-0.5 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300 flex-shrink-0" />
      </div>
    </CardWrapper>
  );
}
