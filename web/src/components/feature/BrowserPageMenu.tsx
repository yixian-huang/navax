// Right-click menu on public navigation pages: bookmark + set as homepage.
// Native set-homepage is blocked by browsers; we guide with clear steps.

import { useCallback, useEffect, useState } from 'react';
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

  const closeMenu = useCallback(() => setMenu(null), []);

  useEffect(() => {
    const onContextMenu = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      // Keep native menus for inputs / editable / links / media.
      if (target.closest('input, textarea, select, [contenteditable="true"], a, button, video, audio, img, canvas')) {
        return;
      }
      event.preventDefault();
      const pad = 8;
      const menuW = 220;
      const menuH = 100;
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

  return (
    <>
      {menu && (
        <div
          role="menu"
          className="fixed z-[80] min-w-[200px] rounded-xl border border-background-200/80 bg-background-50 py-1.5 shadow-float"
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
          className="fixed inset-0 z-[90] flex items-end sm:items-center justify-center p-4 bg-black/30 backdrop-blur-[2px]"
          onClick={() => setGuide(null)}
        >
          <div
            className="w-full max-w-md rounded-2xl bg-background-50 border border-background-200/70 p-5 shadow-overlay rise-in"
            onClick={e => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-labelledby="browser-home-guide-title"
          >
            <div className="flex items-start gap-3 mb-3">
              <div className="w-10 h-10 rounded-xl bg-primary-100 text-primary-600 flex items-center justify-center flex-shrink-0">
                <i className={cn(
                  'text-lg',
                  guide.kind === 'bookmark' ? 'ri-bookmark-3-line' : 'ri-home-heart-line',
                )} />
              </div>
              <div className="min-w-0 flex-1">
                <h3 id="browser-home-guide-title" className="font-heading text-sm font-semibold text-foreground-900">
                  {guide.kind === 'bookmark' ? '加入收藏夹' : '设为浏览器主页'}
                </h3>
                <p className="text-[12px] text-foreground-500 mt-0.5 leading-relaxed">
                  {guide.kind === 'bookmark'
                    ? `浏览器出于安全限制不允许网页静默加书签。请按 ${bookmarkShortcutLabel()}，或按下列步骤操作。`
                    : '现代浏览器不允许网页直接修改主页。已复制当前地址，按下列步骤完成设置。'}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setGuide(null)}
                className="w-7 h-7 rounded-lg text-foreground-300 hover:text-foreground-600 hover:bg-background-100 flex items-center justify-center"
                aria-label="关闭"
              >
                <i className="ri-close-line" />
              </button>
            </div>

            {guide.kind === 'bookmark' ? (
              <ol className="space-y-2 mb-4">
                <li className="text-[12px] text-foreground-600 leading-relaxed flex gap-2">
                  <span className="text-foreground-400 font-mono">1.</span>
                  按键盘 <kbd className="px-1.5 py-0.5 rounded bg-background-100 border border-background-200 text-[11px] font-mono">{bookmarkShortcutLabel()}</kbd>
                </li>
                <li className="text-[12px] text-foreground-600 leading-relaxed flex gap-2">
                  <span className="text-foreground-400 font-mono">2.</span>
                  在弹出的对话框中确认，并勾选「显示在书签栏」
                </li>
              </ol>
            ) : (
              <ol className="space-y-2 mb-4">
                {steps.map((step, i) => (
                  <li key={step} className="text-[12px] text-foreground-600 leading-relaxed flex gap-2">
                    <span className="text-foreground-400 font-mono">{i + 1}.</span>
                    {step}
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
                className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 text-[12px] font-medium hover:bg-primary-600"
              >
                复制本页地址
              </button>
              <button
                type="button"
                onClick={() => setGuide(null)}
                className="h-8 px-3 rounded-lg text-[12px] text-foreground-500 hover:text-foreground-700 hover:bg-background-100"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </>
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
      className="w-full flex items-center gap-2.5 px-3 py-2 text-left text-[13px] text-foreground-700 hover:bg-background-100 transition-colors"
    >
      <i className={cn(icon, 'text-base text-foreground-400')} />
      <span className="flex-1 min-w-0">{label}</span>
      {hint && (
        <span className="text-[10px] font-mono text-foreground-300 flex-shrink-0">{hint}</span>
      )}
    </button>
  );
}
