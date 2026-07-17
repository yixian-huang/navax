// ============================================================
// nav.ax SiteCard — Refined Neutral / Material
// Shared wrapper with three density views, context menu, visits.
// Description: compact hidden; comfortable/list one muted line when present.
// ============================================================

import { useCallback } from 'react';
import { cn } from '@/lib/utils';
import type { Site, Density } from '@/api/types';
import { useContextMenu, createSiteContextActions } from '@/components/base/ContextMenu';
import { useToast } from '@/components/base/Toast';

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

function Favicon({ url, className }: { url: string; className: string }) {
  return (
    <img
      src={getFaviconUrl(url)}
      alt=""
      loading="lazy"
      className={`${className} object-contain`}
      onError={e => { (e.target as HTMLImageElement).style.visibility = 'hidden'; }}
    />
  );
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

/** Prefer non-empty description; used for secondary line + native tooltip. */
function siteDescription(site: Site): string {
  return (site.description || '').trim();
}

function tooltipFor(site: Site, domain: string): string {
  const desc = siteDescription(site);
  if (desc) return `${site.title} — ${desc}`;
  return `${site.title} · ${domain}`;
}

// ============================================================
// CardWrapper — shared click/keyboard/context-menu behavior
// ============================================================
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
  /** Native browser tooltip (full title + description). */
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

// ============================================================
// SiteCard
// ============================================================
export default function SiteCard({ site, density, onOpen, onEdit, onDelete, searchQuery }: SiteCardProps) {
  const domain = getDomain(site.url);
  const q = searchQuery || '';
  const desc = siteDescription(site);
  const tip = tooltipFor(site, domain);

  const shared = { site, onOpen, onEdit, onDelete, title: tip };

  if (density === 'list') {
    // Second line: description preferred, else domain (layout height unchanged).
    const secondary = desc || domain;
    return (
      <CardWrapper
        {...shared}
        className="site-card-list flex items-center gap-3.5 px-3 py-2.5 rounded-lg hover:bg-background-100 transition-colors duration-200 focus-visible:outline-offset-[-2px]"
      >
        <Favicon url={site.url} className="w-5 h-5 flex-shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium text-foreground-800 truncate group-hover:text-accent-500 transition-colors duration-200">
            <HighlightText text={site.title} query={q} />
          </span>
          <span
            className={cn(
              'block text-[11px] truncate site-card-list-domain',
              desc
                ? 'text-foreground-400 site-card-list-desc'
                : 'text-foreground-300 font-mono',
            )}
          >
            <HighlightText text={secondary} query={q} />
          </span>
        </span>
        <i className="ri-arrow-right-up-line text-sm text-foreground-200 opacity-0 group-hover:opacity-100 transition-all duration-200 flex-shrink-0" />
      </CardWrapper>
    );
  }

  if (density === 'compact') {
    // Keep dense icon grid; description only via native title tooltip.
    return (
      <CardWrapper {...shared} className="material-card flex flex-col items-center gap-2 p-3">
        <Favicon url={site.url} className="w-6 h-6" />
        <span className="text-[11px] font-medium text-foreground-700 text-center truncate w-full leading-tight group-hover:text-accent-500 transition-colors duration-200">
          <HighlightText text={site.title} query={q} />
        </span>
      </CardWrapper>
    );
  }

  // Comfortable: domain always; description only when present (no empty row).
  return (
    <CardWrapper {...shared} className="material-card flex flex-col gap-3 p-4">
      <div className="flex items-center justify-between">
        <Favicon url={site.url} className="w-7 h-7" />
        <div className="flex items-center gap-1.5">
          <i className="ri-arrow-right-up-line text-base text-foreground-200 opacity-0 -translate-x-1 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300" />
        </div>
      </div>
      <div className="min-w-0">
        <h3 className="text-sm font-semibold text-foreground-900 truncate group-hover:text-accent-500 transition-colors duration-200">
          <HighlightText text={site.title} query={q} />
        </h3>
        <p className="text-[11px] text-foreground-300 truncate font-mono mt-0.5">
          <HighlightText text={domain} query={q} />
        </p>
        {desc ? (
          <p className="site-card-desc text-[11px] text-foreground-400 truncate mt-0.5 leading-snug">
            <HighlightText text={desc} query={q} />
          </p>
        ) : null}
      </div>
    </CardWrapper>
  );
}
