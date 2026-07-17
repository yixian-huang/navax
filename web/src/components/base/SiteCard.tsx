// ============================================================
// nav.ax SiteCard — Refined Neutral / Material
// Comfortable: icon + text side-by-side (not icon-alone row).
// Description: compact hidden; comfortable/list one line when present.
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
        className="site-card-list flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-background-100 transition-colors duration-200 focus-visible:outline-offset-[-2px]"
      >
        <span className="site-card-favicon flex h-9 w-9 flex-shrink-0 items-center justify-center">
          <Favicon url={site.url} className="w-5 h-5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="site-card-title block text-sm font-medium text-foreground-800 truncate group-hover:text-accent-500 transition-colors duration-200">
            <HighlightText text={site.title} query={q} />
          </span>
          <span
            className={cn(
              'block text-[11px] truncate',
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
      <CardWrapper {...shared} className="material-card flex flex-col items-center gap-2 p-3">
        <Favicon url={site.url} className="w-6 h-6" />
        <span className="site-card-title text-[11px] font-medium text-foreground-700 text-center truncate w-full leading-tight group-hover:text-accent-500 transition-colors duration-200">
          <HighlightText text={site.title} query={q} />
        </span>
      </CardWrapper>
    );
  }

  // Comfortable: icon left, title / domain / description stacked right.
  return (
    <CardWrapper
      {...shared}
      className="material-card site-card-comfortable flex items-start gap-3 p-3.5"
    >
      <span className="site-card-favicon flex h-10 w-10 flex-shrink-0 items-center justify-center">
        <Favicon url={site.url} className="w-6 h-6" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex items-start gap-1.5">
          <h3 className="site-card-title min-w-0 flex-1 text-sm font-semibold text-foreground-900 truncate group-hover:text-accent-500 transition-colors duration-200">
            <HighlightText text={site.title} query={q} />
          </h3>
          <i className="ri-arrow-right-up-line mt-0.5 text-sm text-foreground-300 opacity-0 -translate-x-0.5 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300 flex-shrink-0" />
        </div>
        <p className="site-card-domain text-[11px] text-foreground-500 truncate font-mono mt-0.5">
          <HighlightText text={domain} query={q} />
        </p>
        {desc ? (
          <p className="site-card-desc text-[11px] text-foreground-600 truncate mt-0.5 leading-snug">
            <HighlightText text={desc} query={q} />
          </p>
        ) : null}
      </div>
    </CardWrapper>
  );
}
