// ============================================================
// nav.ax Public Shell — transparent, borderless, with scroll-aware
// glassmorphism navbar. Theme controlled by admin panel.
// ============================================================

import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { themeRegistry } from '@/themes/registry';
import { useKeyboardShortcuts } from '@/hooks/useKeyboardShortcuts';
import { useWallpaperTone } from '@/hooks/useWallpaperTone';
import NavaxLogo from '@/components/base/NavaxLogo';
import { resolveThemeId } from '@/lib/themeResolve';
import { cn } from '@/lib/utils';

import '@/themes/packages';

const DEFAULT_THEME = 'slate';

// AGPL §13: every public page must offer users a way to reach the source.
const SOURCE_REPO_URL = 'https://github.com/yixian-huang/navax';

interface PublicShellProps {
  children: React.ReactNode;
  showSearch?: boolean;
  themeId?: string;
  /** Full-bleed background image or video URL (page settings appearance.background). */
  backgroundUrl?: string;
  /**
   * Image strength 0–1 (higher = more of the photo shows through).
   * Readability uses local glass surfaces on content, not a full-page wash.
   */
  backgroundOpacity?: number;
  /** image (default) or video loop background. */
  backgroundMediaType?: 'image' | 'video';
  /** Poster frame for video backgrounds (tone sampling + first paint). */
  backgroundPoster?: string;
}

export default function PublicShell({
  children,
  showSearch = true,
  themeId = DEFAULT_THEME,
  backgroundUrl,
  backgroundOpacity = 1,
  backgroundMediaType = 'image',
  backgroundPoster,
}: PublicShellProps) {
  const [scrolled, setScrolled] = useState(false);
  const hasBackground = Boolean(backgroundUrl);
  // Sample wallpaper + opacity → light|dark ink (low opacity ⇒ base shows ⇒ dark ink).
  // Prefer poster for video so canvas sampling works without decoding video frames.
  const wallpaperTone = useWallpaperTone(
    backgroundMediaType === 'video' ? (backgroundPoster || backgroundUrl) : backgroundUrl,
    backgroundOpacity,
  );

  // 公开页主题来自服务端发布快照，不在浏览器持久化服务端状态。
  // Culled package ids map to retained themes so activate never hits a missing package.
  useEffect(() => {
    themeRegistry.activate(resolveThemeId(themeId));
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
  const tone = wallpaperTone ?? 'light';
  // Dark photos: slightly lighter edge veil; light photos: soft dark rim.
  const vignette = tone === 'dark'
    ? [
        'radial-gradient(ellipse 90% 75% at 50% 35%, transparent 40%, rgba(0,0,0,0.28) 100%)',
        'linear-gradient(to bottom, rgba(0,0,0,0.22) 0%, transparent 22%, transparent 70%, rgba(0,0,0,0.28) 100%)',
      ].join(', ')
    : [
        'radial-gradient(ellipse 90% 75% at 50% 35%, transparent 35%, rgba(15, 23, 42, 0.18) 100%)',
        'linear-gradient(to bottom, rgba(255,255,255,0.12) 0%, transparent 18%, transparent 72%, rgba(15,23,42,0.14) 100%)',
      ].join(', ');

  return (
    <div
      className={cn(
        'min-h-screen flex flex-col relative',
        hasBackground ? 'bg-transparent' : 'bg-background-100',
      )}
      data-wallpaper={hasBackground ? 'true' : undefined}
      data-wallpaper-tone={hasBackground ? tone : undefined}
    >
      {hasBackground && (
        <div className="fixed inset-0 z-0 pointer-events-none" aria-hidden>
          {backgroundMediaType === 'video' ? (
            <video
              src={backgroundUrl}
              poster={backgroundPoster}
              autoPlay
              muted
              loop
              playsInline
              // External hosts may reject Referer; keep consistent with images.
              // @ts-expect-error referrerPolicy supported on modern video elements
              referrerPolicy="no-referrer"
              className="absolute inset-0 w-full h-full object-cover"
              style={{ opacity: imageAlpha }}
            />
          ) : (
            <img
              src={backgroundUrl}
              alt=""
              // CORS-friendly load so luminance sampling can read pixels when allowed.
              crossOrigin="anonymous"
              // External preset hosts often reject requests that include a site Referer.
              referrerPolicy="no-referrer"
              decoding="async"
              className="absolute inset-0 w-full h-full object-cover"
              style={{ opacity: imageAlpha }}
            />
          )}
          {/* Soft vignette tuned by wallpaper tone for edge chrome */}
          <div className="absolute inset-0" style={{ background: vignette }} />
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
          hasBackground && !scrolled && 'wallpaper-type wallpaper-ink-scope',
        )}>
          <Link
            to="/"
            className="group flex items-center rounded-lg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50"
            aria-label="nav.ax 首页"
          >
            <NavaxLogo size="md" />
          </Link>

          <div className="flex items-center gap-1.5">
            <Link
              to="/discover"
              className={cn(
                'h-9 px-3 flex items-center text-xs transition-colors duration-200 whitespace-nowrap',
                hasBackground
                  ? 'wallpaper-ink-muted hover:opacity-90'
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
                  ? 'wallpaper-ink hover:opacity-90'
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

      <main className={cn('flex-1 relative z-10', hasBackground && 'wallpaper-ink-scope')}>
        {children}
      </main>

      <footer className="mt-auto relative z-10">
        <div className="mx-auto max-w-4xl px-6 md:px-8 py-8">
          {/*
            Wallpaper footer: no frost, no glowing text-shadow.
            Quiet type only — legal chrome should not compete with the photo.
          */}
          {hasBackground ? (
            <div className="flex items-center justify-between gap-4 flex-wrap border-t border-[color:var(--wp-edge)] pt-6">
              <span className="text-[11px] tracking-wide wallpaper-ink-muted">
                nav.ax
              </span>
              <nav className="flex items-center gap-3">
                <Link to="/privacy" className="text-[11px] wallpaper-ink-soft hover:opacity-100 opacity-90 transition-opacity duration-200">隐私</Link>
                <Link to="/terms" className="text-[11px] wallpaper-ink-soft hover:opacity-100 opacity-90 transition-opacity duration-200">条款</Link>
                <Link to="/cookies" className="text-[11px] wallpaper-ink-soft hover:opacity-100 opacity-90 transition-opacity duration-200">Cookie</Link>
                <a
                  href={SOURCE_REPO_URL}
                  target="_blank"
                  rel="nofollow noopener noreferrer"
                  className="text-[11px] wallpaper-ink-soft hover:opacity-100 opacity-90 transition-opacity duration-200"
                >
                  源码
                </a>
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
                    href={SOURCE_REPO_URL}
                    target="_blank"
                    rel="nofollow noopener noreferrer"
                    className="text-[11px] text-foreground-300 hover:text-primary-500 transition-colors duration-200"
                  >
                    源码
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
