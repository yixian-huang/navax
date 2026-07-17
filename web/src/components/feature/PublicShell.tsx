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
   * A light scrim is layered on top for text readability.
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

  // Image opacity: clamp so a weak photo still shows; scrim stays light for text.
  const imageAlpha = Math.min(1, Math.max(0.25, backgroundOpacity));
  const scrimAlpha = Math.min(0.55, Math.max(0.12, 1 - imageAlpha));

  return (
    <div
      className={cn(
        'min-h-screen flex flex-col relative',
        hasBackground ? 'bg-transparent' : 'bg-background-100',
      )}
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
          <div
            className="absolute inset-0"
            style={{ backgroundColor: `rgba(255, 255, 255, ${scrimAlpha})` }}
          />
        </div>
      )}

      {/* Navbar — transparent by default, glass on scroll */}
      <header
        className={cn(
          'fixed top-0 left-0 right-0 z-50 transition-all duration-300',
          scrolled
            ? 'bg-background-50/80 backdrop-blur-xl border-b border-background-200/50 shadow-raised'
            : 'bg-transparent',
        )}
      >
        <nav className="mx-auto max-w-4xl px-6 md:px-8 h-16 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2.5 group">
            <span className="text-base font-heading font-semibold text-foreground-800 tracking-tight">nav.ax</span>
          </Link>

          <div className="flex items-center gap-1.5">
            <Link
              to="/discover"
              className="h-9 px-3 flex items-center text-xs text-foreground-300 hover:text-primary-500 transition-colors duration-200 whitespace-nowrap"
            >
              发现
            </Link>
            <Link
              to="/login"
              className="h-9 px-3 flex items-center text-xs text-foreground-400 hover:text-foreground-600 transition-colors duration-200 whitespace-nowrap"
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
        </div>
      </footer>
    </div>
  );
}
