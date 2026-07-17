// ============================================================
// Floating share chrome for public personal pages (/u/:slug).
// Portaled to body; does not alter main navigation layout.
// ============================================================

import { useCallback, useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/utils';

interface SharePageFabProps {
  title: string;
  url: string;
  ownerName: string;
  subdomain?: string;
}

export default function SharePageFab({ title, url, ownerName, subdomain }: SharePageFabProps) {
  const [mounted, setMounted] = useState(false);
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => { setMounted(true); }, []);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      const input = document.createElement('input');
      input.value = url;
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
    }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  }, [url]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open]);

  if (!mounted) return null;

  const displayUrl = url.replace(/^https?:\/\//, '');
  const encodedTitle = encodeURIComponent(title);
  const encodedUrl = encodeURIComponent(url);

  return createPortal(
    <div className="fixed bottom-6 left-6 z-40 flex flex-col items-start gap-2">
      {open && (
        <div
          className="browser-chrome-surface w-[min(calc(100vw-3rem),18rem)] rounded-xl border border-background-200 bg-background-50 p-3.5 shadow-overlay rise-in"
          role="dialog"
          aria-label="分享此导航"
        >
          <div className="flex items-start justify-between gap-2 mb-2.5">
            <div className="min-w-0">
              <p className="browser-chrome-title text-[13px] line-clamp-1">{title}</p>
              <p className="browser-chrome-muted text-[11px] mt-0.5 truncate">
                {ownerName}
                {subdomain ? ` · ${subdomain}` : ''}
              </p>
            </div>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="browser-chrome-muted w-7 h-7 rounded-md hover:bg-background-100 flex items-center justify-center flex-shrink-0"
              aria-label="关闭"
            >
              <i className="ri-close-line text-sm" aria-hidden />
            </button>
          </div>

          <p className="browser-chrome-muted text-[11px] font-mono truncate mb-3 px-0.5" title={displayUrl}>
            {displayUrl}
          </p>

          <div className="flex flex-col gap-1.5">
            <button
              type="button"
              onClick={() => void handleCopy()}
              className={cn(
                'h-9 w-full rounded-md text-[12px] font-medium flex items-center justify-center gap-1.5 transition-colors',
                copied
                  ? 'bg-green-50 text-green-700 border border-green-200'
                  : 'browser-chrome-primary hover:opacity-90',
              )}
            >
              <i className={copied ? 'ri-check-line' : 'ri-file-copy-line'} aria-hidden />
              {copied ? '已复制链接' : '复制链接'}
            </button>
            <div className="grid grid-cols-3 gap-1.5">
              <a
                href={`https://twitter.com/intent/tweet?text=${encodedTitle}&url=${encodedUrl}`}
                target="_blank"
                rel="nofollow noopener noreferrer"
                className="h-8 rounded-md bg-background-100 browser-chrome-body text-[11px] flex items-center justify-center gap-1 hover:bg-background-200"
              >
                <i className="ri-twitter-x-line" aria-hidden />
                X
              </a>
              <a
                href={`https://www.facebook.com/sharer/sharer.php?u=${encodedUrl}`}
                target="_blank"
                rel="nofollow noopener noreferrer"
                className="h-8 rounded-md bg-background-100 browser-chrome-body text-[11px] flex items-center justify-center gap-1 hover:bg-background-200"
              >
                <i className="ri-facebook-circle-line" aria-hidden />
                FB
              </a>
              <a
                href={`mailto:?subject=${encodedTitle}&body=${encodedUrl}`}
                className="h-8 rounded-md bg-background-100 browser-chrome-body text-[11px] flex items-center justify-center gap-1 hover:bg-background-200"
              >
                <i className="ri-mail-line" aria-hidden />
                邮件
              </a>
            </div>
          </div>
        </div>
      )}

      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className={cn(
          'browser-chrome-surface h-12 px-4 rounded-full border border-background-200 bg-background-50 shadow-overlay',
          'flex items-center gap-2 text-[13px] font-medium browser-chrome-body',
          'hover:bg-background-100 transition-colors duration-150',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50',
        )}
        aria-expanded={open}
        aria-haspopup="dialog"
        title="分享此导航"
      >
        <i className={open ? 'ri-close-line' : 'ri-share-forward-line'} aria-hidden />
        <span>{open ? '收起' : '分享'}</span>
      </button>
    </div>,
    document.body,
  );
}
