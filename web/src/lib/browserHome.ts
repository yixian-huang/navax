// Browser homepage / bookmark helpers.
// Modern browsers do not allow JS to set the homepage or add bookmarks silently;
// we provide best-effort UX (shortcut hints + per-browser steps).

export type BrowserFamily = 'chrome' | 'edge' | 'firefox' | 'safari' | 'other';

export function detectBrowserFamily(): BrowserFamily {
  if (typeof navigator === 'undefined') return 'other';
  const ua = navigator.userAgent;
  if (/Edg\//.test(ua)) return 'edge';
  if (/Firefox\//.test(ua)) return 'firefox';
  if (/Safari\//.test(ua) && !/Chrome\//.test(ua) && !/Chromium\//.test(ua)) return 'safari';
  if (/Chrome\//.test(ua) || /Chromium\//.test(ua)) return 'chrome';
  return 'other';
}

export function isApplePlatform(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /Mac|iPhone|iPad|iPod/i.test(navigator.platform || '')
    || /Mac OS X|iPhone|iPad/.test(navigator.userAgent);
}

export function bookmarkShortcutLabel(): string {
  return isApplePlatform() ? '⌘ + D' : 'Ctrl + D';
}

/** Try to surface the browser bookmark UI; returns whether a native path ran. */
export function tryAddBookmark(): { usedNative: boolean; hint: string } {
  const hint = `请按 ${bookmarkShortcutLabel()} 将本页加入书签，并勾选「显示书签栏」。`;
  try {
    // Legacy IE / old Trident only — kept for completeness; almost never true today.
    const external = (window as Window & { external?: { AddFavorite?: (url: string, title: string) => void } }).external;
    if (typeof external?.AddFavorite === 'function') {
      external.AddFavorite(window.location.href, document.title || 'nav.ax');
      return { usedNative: true, hint: '已尝试打开收藏夹对话框。' };
    }
  } catch {
    /* ignore */
  }
  return { usedNative: false, hint };
}

export function homepageSteps(browser: BrowserFamily = detectBrowserFamily()): string[] {
  switch (browser) {
    case 'chrome':
      return [
        '打开 Chrome 设置（地址栏输入 chrome://settings/onStartup）',
        '在「启动时」选择「打开特定网页或一组网页」',
        '点「添加新网页」，粘贴本站地址并保存',
      ];
    case 'edge':
      return [
        '打开 Edge 设置 → 启动时 / 或访问 edge://settings/startHomeNTP',
        '选择「打开这些页面」并添加本站地址',
        '也可在「外观 → 主页按钮」里把主页设为本站',
      ];
    case 'firefox':
      return [
        '打开 Firefox 设置 → 主页',
        '将「主页和新窗口」设为「自定义网址」',
        '粘贴本站地址并保存',
      ];
    case 'safari':
      return [
        '菜单栏 Safari → 设置（或偏好设置）→ 通用',
        '将「主页」改为本站地址',
        '新窗口 / 新标签页可同样指向该地址',
      ];
    default:
      return [
        '打开浏览器设置中的「主页 / 启动页」',
        '将主页设为当前站点地址',
        '保存后新开浏览器窗口验证是否生效',
      ];
  }
}

export async function copyCurrentUrl(): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(window.location.href);
    return true;
  } catch {
    try {
      const input = document.createElement('input');
      input.value = window.location.href;
      document.body.appendChild(input);
      input.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(input);
      return ok;
    } catch {
      return false;
    }
  }
}
