// ============================================================
// nav.ax Public Shell — transparent, borderless, with scroll-aware
// glassmorphism navbar. Theme controlled by admin panel.
// ============================================================

import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { themeRegistry } from '@/themes/registry';
import { useKeyboardShortcuts } from '@/hooks/useKeyboardShortcuts';
import { cn } from '@/lib/utils';

import '@/themes/packages';

const DEFAULT_THEME = 'slate';

interface PublicShellProps {
  children: React.ReactNode;
  showSearch?: boolean;
  themeId?: string;
  /** Full-bleed background image URL (page settings appearance.background). */
  backgroundUrl?: string;
  /**
   * Image strength 0–1 (higher = more of the photo shows through).
   * Readability uses local glass surfaces on content, not a full-page wash.
   */
  backgroundOpacity?: number;
}

export default function PublicShell({
  children,
  showSearch = true,
  themeId = DEFAULT_THEME,
  backgroundUrl,
  backgroundOpacity = 1,
}: PublicShellProps) {
  const [scrolled, setScrolled] = useState(false);
  const hasBackground = Boolean(backgroundUrl);

  // 公开页主题来自服务端发布快照，不在浏览器持久化服务端状态。
  useEffect(() => {
    themeRegistry.activate(themeRegistry.has(themeId) ? themeId : DEFAULT_THEME);
  }, [themeId]);

  // Scroll-aware navbar
  useEffect(() => {
    let ticking = false;
    const onScroll = () => {
      if (!ticking) {
        requestAnimationFrame(() => {
          setScrolled(window.scrollY > 20);
          ticking = false;
        });
        ticking = true;
      }
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  useKeyboardShortcuts({});

  // Keep the photo vivid; only a soft edge vignette (not a flat white wash).
  const imageAlpha = Math.min(1, Math.max(0.25, backgroundOpacity));

  return (
    <div
      className={cn(
        'min-h-screen flex flex-col relative',
        hasBackground ? 'bg-transparent' : 'bg-background-100',
      )}
      data-wallpaper={hasBackground ? 'true' : undefined}
    >
      {hasBackground && (
        <div className="fixed inset-0 z-0 pointer-events-none" aria-hidden>
          <img
            src={backgroundUrl}
            alt=""
            // External preset hosts often reject requests that include a site Referer.
            referrerPolicy="no-referrer"
            decoding="async"
            className="absolute inset-0 w-full h-full object-cover"
            style={{ opacity: imageAlpha }}
          />
          {/* Soft vignette only — center stays photographic; edges ease chrome contrast */}
          <div
            className="absolute inset-0"
            style={{
              background: [
                'radial-gradient(ellipse 90% 75% at 50% 35%, transparent 35%, rgba(15, 23, 42, 0.22) 100%)',
                'linear-gradient(to bottom, rgba(255,255,255,0.10) 0%, transparent 18%, transparent 72%, rgba(15,23,42,0.16) 100%)',
              ].join(', '),
            }}
          />
        </div>
      )}

      {/*
        Navbar transparency strategy:
        - No wallpaper: transparent → solid glass on scroll (unchanged).
        - Wallpaper: stay open over the photo; use type contrast, not a frosted bar.
          Only after scroll, a *light* frost appears so content doesn't collide.
      */}
      <header
        className={cn(
          'fixed top-0 left-0 right-0 z-50 transition-all duration-300',
          hasBackground
            ? (scrolled
              ? 'bg-background-50/40 backdrop-blur-md border-b border-background-200/25'
              : 'bg-transparent')
            : (scrolled
              ? 'bg-background-50/75 backdrop-blur-xl border-b border-background-200/40 shadow-raised'
              : 'bg-transparent'),
        )}
      >
        <nav className={cn(
          'mx-auto max-w-4xl px-6 md:px-8 h-16 flex items-center justify-between',
          // Readable logo/links without a permanent glass slab over the photo.
          hasBackground && !scrolled && 'wallpaper-type',
        )}>
          <Link to="/" className="flex items-center gap-2.5 group">
            <span className={cn(
              'text-base font-heading font-semibold tracking-tight',
              hasBackground ? 'text-foreground-900' : 'text-foreground-800',
            )}>
              nav.ax
            </span>
          </Link>

          <div className="flex items-center gap-1.5">
            <Link
              to="/discover"
              className={cn(
                'h-9 px-3 flex items-center text-xs transition-colors duration-200 whitespace-nowrap',
                hasBackground
                  ? 'text-foreground-700 hover:text-primary-500'
                  : 'text-foreground-500 hover:text-primary-500',
              )}
            >
              发现
            </Link>
            <Link
              to="/login"
              className={cn(
                'h-9 px-3 flex items-center text-xs transition-colors duration-200 whitespace-nowrap',
                hasBackground
                  ? 'text-foreground-800 hover:text-foreground-950'
                  : 'text-foreground-600 hover:text-foreground-800',
              )}
            >
              登录
            </Link>
          </div>
        </nav>
      </header>

      {/* Spacer to compensate for fixed navbar */}
      <div className="h-16 flex-shrink-0 relative z-10" />

      <main className="flex-1 relative z-10">{children}</main>

      <footer className="mt-auto relative z-10">
        <div className="mx-auto max-w-4xl px-6 md:px-8 py-8">
          {/*
            Wallpaper footer: no frost, no glowing text-shadow.
            Quiet type only — legal chrome should not compete with the photo.
          */}
          {hasBackground ? (
            <div className="flex items-center justify-between gap-4 flex-wrap border-t border-background-50/20 pt-6">
              <span className="text-[11px] text-foreground-600/90 tracking-wide">
                nav.ax
              </span>
              <nav className="flex items-center gap-3">
                <Link to="/privacy" className="text-[11px] text-foreground-600/80 hover:text-primary-500 transition-colors duration-200">隐私</Link>
                <Link to="/terms" className="text-[11px] text-foreground-600/80 hover:text-primary-500 transition-colors duration-200">条款</Link>
                <Link to="/cookies" className="text-[11px] text-foreground-600/80 hover:text-primary-500 transition-colors duration-200">Cookie</Link>
              </nav>
            </div>
          ) : (
            <>
              <div className="hairline mb-6" />
              <div className="flex items-center justify-between gap-4 flex-wrap">
                <span className="text-[11px] text-foreground-300 tracking-wide">
                  nav.ax · 开源导航站
                </span>
                <nav className="flex items-center gap-4">
                  <Link to="/privacy" className="text-[11px] text-foreground-300 hover:text-primary-500 transition-colors duration-200">隐私政策</Link>
                  <Link to="/terms" className="text-[11px] text-foreground-300 hover:text-primary-500 transition-colors duration-200">服务条款</Link>
                  <Link to="/cookies" className="text-[11px] text-foreground-300 hover:text-primary-500 transition-colors duration-200">Cookie 说明</Link>
                  <a
                    href="https://github.com"
                    target="_blank"
                    rel="nofollow noopener noreferrer"
                    className="text-[11px] text-foreground-300 hover:text-primary-500 transition-colors duration-200"
                  >
                    GitHub
                  </a>
                </nav>
              </div>
            </>
          )}
        </div>
      </footer>
    </div>
  );
}
