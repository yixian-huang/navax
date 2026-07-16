// ============================================================
// nav.ax ShareButton — copy link, share to social
// ============================================================

import { useState, useCallback } from 'react';
import { cn } from '@/lib/utils';

interface ShareButtonProps {
  url: string;
  title: string;
}

export default function ShareButton({ url, title }: ShareButtonProps) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      setOpen(false);
    } catch {
      // fallback — select and copy
      const input = document.createElement('input');
      input.value = url;
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      setOpen(false);
    }
  }, [url]);

  const encodedTitle = encodeURIComponent(title);
  const encodedUrl = encodeURIComponent(url);

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={cn(
          'flex items-center gap-1.5 h-9 px-3 rounded-lg text-sm font-medium transition-colors duration-150 whitespace-nowrap focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-500',
          copied
            ? 'bg-green-50 text-green-700 border border-green-200'
            : 'bg-background-100 text-foreground-600 border border-background-200/70 hover:bg-background-200'
        )}
        aria-expanded={open}
        aria-haspopup="true"
      >
        {copied ? (
          <>
            <i className="ri-check-line text-sm" />
            <span>已复制</span>
          </>
        ) : (
          <>
            <i className="ri-share-line text-sm" />
            <span>分享</span>
          </>
        )}
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 top-full mt-1.5 z-20 w-44 bg-white rounded-xl border border-background-200/70 shadow-overlay py-1.5">
            <button
              onClick={handleCopy}
              className="w-full flex items-center gap-2.5 px-3 py-2.5 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-100 whitespace-nowrap"
            >
              <i className="ri-link text-base text-foreground-400" />
              复制链接
            </button>
            <a
              href={`https://x.com/intent/tweet?text=${encodedTitle}&url=${encodedUrl}`}
              target="_blank"
              rel="nofollow noopener noreferrer"
              className="flex items-center gap-2.5 px-3 py-2.5 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-100 whitespace-nowrap"
              onClick={() => setOpen(false)}
            >
              <i className="ri-twitter-x-line text-base text-foreground-400" />
              分享到 X
            </a>
            <a
              href={`https://t.me/share/url?url=${encodedUrl}&text=${encodedTitle}`}
              target="_blank"
              rel="nofollow noopener noreferrer"
              className="flex items-center gap-2.5 px-3 py-2.5 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-100 whitespace-nowrap"
              onClick={() => setOpen(false)}
            >
              <i className="ri-telegram-line text-base text-foreground-400" />
              分享到 Telegram
            </a>
          </div>
        </>
      )}
    </div>
  );
}
