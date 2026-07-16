import { useState, useEffect } from 'react';
import { cn } from '@/lib/utils';

const STORAGE_KEY = 'inav_browser_guide_dismissed';

export default function BrowserGuide() {
  const [visible, setVisible] = useState(false);
  const [fadingOut, setFadingOut] = useState(false);

  useEffect(() => {
    const dismissed = localStorage.getItem(STORAGE_KEY);
    if (!dismissed) {
      // 首次访客延迟 3 秒后弹出，不要太突兀
      const timer = setTimeout(() => setVisible(true), 3000);
      return () => clearTimeout(timer);
    }
  }, []);

  const handleDismiss = () => {
    setFadingOut(true);
    setTimeout(() => {
      setVisible(false);
      localStorage.setItem(STORAGE_KEY, '1');
    }, 300);
  };

  if (!visible) return null;

  // 检测操作系统
  const isMac = typeof navigator !== 'undefined' && navigator.platform?.toLowerCase().includes('mac');

  return (
    <div
      className={cn(
        'fixed bottom-6 left-1/2 -translate-x-1/2 z-40 w-[calc(100%-2rem)] max-w-lg transition-all duration-300',
        fadingOut ? 'opacity-0 translate-y-2 scale-95' : 'opacity-100 translate-y-0 scale-100'
      )}
    >
      <div className="material-card rounded-2xl p-5 relative overflow-hidden">
        {/* 装饰背景 */}
        <div className="absolute top-0 right-0 w-24 h-24 bg-accent-100/40 rounded-full -translate-y-1/2 translate-x-1/2 blur-2xl" />

        <div className="relative flex items-start gap-4">
          {/* 图标 */}
          <div className="w-10 h-10 flex-shrink-0 flex items-center justify-center rounded-xl bg-accent-100 text-accent-600">
            <i className="ri-bookmark-3-line text-lg" />
          </div>

          {/* 内容 */}
          <div className="flex-1 min-w-0">
            <h4 className="font-heading text-sm font-semibold text-foreground-900 mb-1.5">
              把 nav.ax 设为浏览器首页
            </h4>
            <p className="text-[12px] text-foreground-500 leading-relaxed mb-3">
              每天打开浏览器就能看到你的导航，试试这两个方法：
            </p>

            <div className="space-y-2.5 mb-4">
              {/* 方法 1：设为主页 */}
              <div className="flex items-start gap-2.5">
                <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-md bg-secondary-100 text-secondary-700 text-[10px] font-bold mt-0.5">
                  1
                </span>
                <div>
                  <p className="text-[12px] font-medium text-foreground-700 leading-snug mb-0.5">
                    拖拽到主页按钮
                  </p>
                  <p className="text-[11px] text-foreground-400 leading-relaxed">
                    把地址栏左侧的锁形图标 <i className="ri-lock-line text-[10px] mx-0.5 align-middle" /> 拖到浏览器工具栏的「主页」图标上即可
                  </p>
                </div>
              </div>

              {/* 方法 2：加入书签 */}
              <div className="flex items-start gap-2.5">
                <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-md bg-secondary-100 text-secondary-700 text-[10px] font-bold mt-0.5">
                  2
                </span>
                <div>
                  <p className="text-[12px] font-medium text-foreground-700 leading-snug mb-0.5">
                    加入书签栏
                  </p>
                  <p className="text-[11px] text-foreground-400 leading-relaxed">
                    按 <kbd className="inline-flex items-center h-[18px] px-1.5 rounded bg-background-100 border border-background-200 text-[10px] font-mono text-foreground-500 mx-0.5 align-middle">
                      {isMac ? '⌘' : 'Ctrl'}
                    </kbd>
                    <span className="mx-0.5">+</span>
                    <kbd className="inline-flex items-center h-[18px] px-1.5 rounded bg-background-100 border border-background-200 text-[10px] font-mono text-foreground-500 mx-0.5 align-middle">D</kbd>
                    ，然后勾选「显示书签栏」就能随时点击访问
                  </p>
                </div>
              </div>
            </div>

            {/* 操作按钮 */}
            <div className="flex items-center gap-3">
              <button
                onClick={() => {
                  // 尝试触发浏览器默认的主页设置对话框（部分浏览器支持）
                  try {
                    if (typeof (window as any).external?.AddSearchProvider === 'function') {
                      // 旧版 IE 行为，不做实际调用
                    }
                  } catch { /* 忽略 */ }
                  // 通用做法：提示用户手动操作
                  handleDismiss();
                }}
                className="h-8 px-4 rounded-lg bg-primary-500 text-background-50 text-[12px] font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap cursor-pointer"
              >
                我知道了
              </button>
              <button
                onClick={handleDismiss}
                className="text-[11px] text-foreground-400 hover:text-foreground-600 transition-colors duration-150 whitespace-nowrap cursor-pointer"
              >
                不再提示
              </button>
            </div>
          </div>

          {/* 关闭按钮 */}
          <button
            onClick={handleDismiss}
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
