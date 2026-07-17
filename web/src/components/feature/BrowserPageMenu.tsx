// Right-click menu on public navigation pages: bookmark + set as homepage.
// Portaled to document.body so wallpaper ink tokens do not recolor the chrome.

import { useCallback, useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/utils';
import { useToast } from '@/components/base/Toast';
import {
  bookmarkShortcutLabel,
  copyCurrentUrl,
  detectBrowserFamily,
  homepageSteps,
  tryAddBookmark,
} from '@/lib/browserHome';

interface MenuState {
  x: number;
  y: number;
}

interface GuideMode {
  kind: 'bookmark' | 'homepage';
}

export default function BrowserPageMenu() {
  const { toast } = useToast();
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [guide, setGuide] = useState<GuideMode | null>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => { setMounted(true); }, []);

  const closeMenu = useCallback(() => setMenu(null), []);

  useEffect(() => {
    const onContextMenu = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      // Keep native menus for form fields / media / interactive controls.
      // Allow on plain anchors that are site cards? No — keep native "open in new tab".
      if (target.closest('input, textarea, select, [contenteditable="true"], a, button, video, audio, img, canvas, [role="menu"], [role="dialog"]')) {
        return;
      }
      event.preventDefault();
      const pad = 8;
      const menuW = 228;
      const menuH = 104;
      const x = Math.min(event.clientX, window.innerWidth - menuW - pad);
      const y = Math.min(event.clientY, window.innerHeight - menuH - pad);
      setMenu({ x: Math.max(pad, x), y: Math.max(pad, y) });
    };

    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        closeMenu();
        setGuide(null);
      }
    };

    document.addEventListener('contextmenu', onContextMenu);
    document.addEventListener('keydown', onKey);
    document.addEventListener('click', closeMenu);
    window.addEventListener('blur', closeMenu);
    window.addEventListener('scroll', closeMenu, true);
    return () => {
      document.removeEventListener('contextmenu', onContextMenu);
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('click', closeMenu);
      window.removeEventListener('blur', closeMenu);
      window.removeEventListener('scroll', closeMenu, true);
    };
  }, [closeMenu]);

  const handleBookmark = async () => {
    closeMenu();
    const result = tryAddBookmark();
    const copied = await copyCurrentUrl();
    if (result.usedNative) {
      toast('success', result.hint);
      return;
    }
    setGuide({ kind: 'bookmark' });
    if (copied) {
      toast('info', `地址已复制 · ${bookmarkShortcutLabel()} 加入书签`);
    } else {
      toast('info', result.hint);
    }
  };

  const handleHomepage = async () => {
    closeMenu();
    await copyCurrentUrl();
    setGuide({ kind: 'homepage' });
    toast('info', '地址已复制，按指引设为浏览器主页');
  };

  const browser = detectBrowserFamily();
  const steps = homepageSteps(browser);

  if (!mounted) return null;

  return createPortal(
    <>
      {menu && (
        <div
          role="menu"
          className="browser-chrome-surface fixed z-[80] min-w-[212px] rounded-lg border border-background-200 bg-background-50 py-1 shadow-overlay"
          style={{ left: menu.x, top: menu.y }}
          onClick={e => e.stopPropagation()}
          onContextMenu={e => e.preventDefault()}
        >
          <MenuItem
            icon="ri-star-line"
            label="加入收藏夹 / 书签"
            hint={bookmarkShortcutLabel()}
            onClick={() => void handleBookmark()}
          />
          <MenuItem
            icon="ri-home-heart-line"
            label="设为浏览器主页"
            onClick={() => void handleHomepage()}
          />
        </div>
      )}

      {guide && (
        <div
          className="fixed inset-0 z-[90] flex items-end sm:items-center justify-center p-4 bg-black/35"
          onClick={() => setGuide(null)}
        >
          <div
            className="browser-chrome-surface w-full max-w-md rounded-xl border border-background-200 bg-background-50 p-5 shadow-overlay rise-in"
            onClick={e => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-labelledby="browser-home-guide-title"
          >
            <div className="flex items-start gap-3 mb-3">
              <div className="w-10 h-10 rounded-lg bg-primary-100 flex items-center justify-center flex-shrink-0">
                <i
                  className={cn(
                    'text-lg text-primary-600',
                    guide.kind === 'bookmark' ? 'ri-bookmark-3-line' : 'ri-home-heart-line',
                  )}
                  aria-hidden
                />
              </div>
              <div className="min-w-0 flex-1">
                <h3
                  id="browser-home-guide-title"
                  className="browser-chrome-title text-sm font-semibold tracking-tight"
                >
                  {guide.kind === 'bookmark' ? '加入收藏夹' : '设为浏览器主页'}
                </h3>
                <p className="browser-chrome-muted text-[12px] mt-1 leading-relaxed">
                  {guide.kind === 'bookmark'
                    ? `浏览器出于安全限制不允许网页静默加书签。请按 ${bookmarkShortcutLabel()}，或按下列步骤操作。`
                    : '现代浏览器不允许网页直接修改主页。已复制当前地址，按下列步骤完成设置。'}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setGuide(null)}
                className="browser-chrome-muted w-7 h-7 rounded-md hover:bg-background-100 flex items-center justify-center"
                aria-label="关闭"
              >
                <i className="ri-close-line text-sm" aria-hidden />
              </button>
            </div>

            {guide.kind === 'bookmark' ? (
              <ol className="space-y-2 mb-4">
                <li className="browser-chrome-body text-[12px] leading-relaxed flex gap-2">
                  <span className="browser-chrome-muted tabular-nums flex-shrink-0">1.</span>
                  <span>
                    按键盘{' '}
                    <kbd className="inline-flex items-center h-[18px] px-1.5 rounded bg-background-100 border border-background-200 text-[11px] tabular-nums">
                      {bookmarkShortcutLabel()}
                    </kbd>
                  </span>
                </li>
                <li className="browser-chrome-body text-[12px] leading-relaxed flex gap-2">
                  <span className="browser-chrome-muted tabular-nums flex-shrink-0">2.</span>
                  <span>在弹出的对话框中确认，并勾选「显示在书签栏」</span>
                </li>
              </ol>
            ) : (
              <ol className="space-y-2 mb-4">
                {steps.map((step, i) => (
                  <li key={step} className="browser-chrome-body text-[12px] leading-relaxed flex gap-2">
                    <span className="browser-chrome-muted tabular-nums flex-shrink-0">{i + 1}.</span>
                    <span>{step}</span>
                  </li>
                ))}
              </ol>
            )}

            <div className="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onClick={async () => {
                  const ok = await copyCurrentUrl();
                  toast(ok ? 'success' : 'error', ok ? '地址已复制' : '复制失败，请手动从地址栏复制');
                }}
                className="browser-chrome-primary h-8 px-3.5 rounded-md text-[12px] font-medium hover:opacity-90"
              >
                复制本页地址
              </button>
              <button
                type="button"
                onClick={() => setGuide(null)}
                className="browser-chrome-muted h-8 px-3 rounded-md text-[12px] hover:bg-background-100"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </>,
    document.body,
  );
}

function MenuItem({
  icon,
  label,
  hint,
  onClick,
}: {
  icon: string;
  label: string;
  hint?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onClick}
      className="w-full flex items-center gap-2.5 px-3 py-2 text-left text-[13px] browser-chrome-body hover:bg-background-100 transition-colors"
    >
      <i className={cn(icon, 'text-base browser-chrome-muted')} aria-hidden />
      <span className="flex-1 min-w-0">{label}</span>
      {hint && (
        <span className="text-[10px] tabular-nums browser-chrome-muted flex-shrink-0">{hint}</span>
      )}
    </button>
  );
}
