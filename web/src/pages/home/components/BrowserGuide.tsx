import { useState, useEffect } from 'react';
import { cn } from '@/lib/utils';
import { useToast } from '@/components/base/Toast';
import {
  bookmarkShortcutLabel,
  copyCurrentUrl,
  detectBrowserFamily,
  homepageSteps,
  tryAddBookmark,
} from '@/lib/browserHome';

const STORAGE_KEY = 'inav_browser_guide_dismissed';

export default function BrowserGuide() {
  const { toast } = useToast();
  const [visible, setVisible] = useState(false);
  const [fadingOut, setFadingOut] = useState(false);
  const [expanded, setExpanded] = useState<'homepage' | 'bookmark' | null>(null);

  useEffect(() => {
    const dismissed = localStorage.getItem(STORAGE_KEY);
    if (!dismissed) {
      // 首次访客延迟 3 秒后弹出，不要太突兀
      const timer = setTimeout(() => setVisible(true), 3000);
      return () => clearTimeout(timer);
    }
  }, []);

  const handleDismiss = (persist = true) => {
    setFadingOut(true);
    setTimeout(() => {
      setVisible(false);
      if (persist) localStorage.setItem(STORAGE_KEY, '1');
    }, 300);
  };

  const handleSetHomepage = async () => {
    const ok = await copyCurrentUrl();
    setExpanded('homepage');
    toast(ok ? 'info' : 'error', ok ? '地址已复制，按下方步骤设为主页' : '请从地址栏手动复制后设为主页');
  };

  const handleAddBookmark = async () => {
    const result = tryAddBookmark();
    if (result.usedNative) {
      toast('success', result.hint);
      return;
    }
    await copyCurrentUrl();
    setExpanded('bookmark');
    toast('info', `请按 ${bookmarkShortcutLabel()} 加入书签`);
  };

  if (!visible) return null;

  const isMac = typeof navigator !== 'undefined' && (
    navigator.platform?.toLowerCase().includes('mac')
    || /Mac OS X|iPhone|iPad/.test(navigator.userAgent)
  );
  const steps = homepageSteps(detectBrowserFamily());

  return (
    <div
      className={cn(
        'fixed bottom-6 left-1/2 -translate-x-1/2 z-40 w-[calc(100%-2rem)] max-w-lg transition-all duration-300',
        fadingOut ? 'opacity-0 translate-y-2 scale-95' : 'opacity-100 translate-y-0 scale-100',
      )}
    >
      <div className="material-card rounded-2xl p-5 relative overflow-hidden">
        <div className="absolute top-0 right-0 w-24 h-24 bg-accent-100/40 rounded-full -translate-y-1/2 translate-x-1/2 blur-2xl" />

        <div className="relative flex items-start gap-4">
          <div className="w-10 h-10 flex-shrink-0 flex items-center justify-center rounded-xl bg-accent-100 text-accent-600">
            <i className="ri-bookmark-3-line text-lg" />
          </div>

          <div className="flex-1 min-w-0">
            <h4 className="font-heading text-sm font-semibold text-foreground-900 mb-1.5">
              把本站设为浏览器首页
            </h4>
            <p className="text-[12px] text-foreground-500 leading-relaxed mb-3">
              每天打开浏览器就能看到你的导航。也可在页面空白处右键，快速「加入收藏」或「设为主页」。
            </p>

            {expanded === 'homepage' && (
              <ol className="mb-3 space-y-1.5 rounded-xl bg-background-100/70 px-3 py-2.5">
                {steps.map((step, i) => (
                  <li key={step} className="text-[11px] text-foreground-600 leading-relaxed flex gap-1.5">
                    <span className="text-foreground-400 font-mono flex-shrink-0">{i + 1}.</span>
                    {step}
                  </li>
                ))}
              </ol>
            )}

            {expanded === 'bookmark' && (
              <div className="mb-3 rounded-xl bg-background-100/70 px-3 py-2.5 text-[11px] text-foreground-600 leading-relaxed">
                按{' '}
                <kbd className="inline-flex items-center h-[18px] px-1.5 rounded bg-background-50 border border-background-200 text-[10px] font-mono text-foreground-500">
                  {isMac ? '⌘' : 'Ctrl'}
                </kbd>
                <span className="mx-0.5">+</span>
                <kbd className="inline-flex items-center h-[18px] px-1.5 rounded bg-background-50 border border-background-200 text-[10px] font-mono text-foreground-500">
                  D
                </kbd>
                {' '}，在弹出框中确认并勾选「显示书签栏」。
              </div>
            )}

            <div className="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onClick={() => void handleSetHomepage()}
                className="h-8 px-3.5 rounded-lg bg-primary-500 text-background-50 text-[12px] font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap cursor-pointer"
              >
                一键设为主页
              </button>
              <button
                type="button"
                onClick={() => void handleAddBookmark()}
                className="h-8 px-3.5 rounded-lg bg-background-100 text-foreground-700 text-[12px] font-medium hover:bg-background-200 transition-colors duration-150 whitespace-nowrap cursor-pointer"
              >
                加入收藏夹
              </button>
              <button
                type="button"
                onClick={() => handleDismiss(true)}
                className="text-[11px] text-foreground-400 hover:text-foreground-600 transition-colors duration-150 whitespace-nowrap cursor-pointer px-1"
              >
                不再提示
              </button>
            </div>
          </div>

          <button
            type="button"
            onClick={() => handleDismiss(true)}
            className="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded-lg text-foreground-300 hover:text-foreground-500 hover:bg-background-100 transition-colors duration-150 cursor-pointer"
            aria-label="关闭"
          >
            <i className="ri-close-line text-sm" />
          </button>
        </div>
      </div>
    </div>
  );
}
