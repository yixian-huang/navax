// ============================================================
// Theme Package Type — the frontend view of a server-side theme.
//
// A package is no longer a self-contained CSS blob: the compiled
// stylesheet lives on the server behind a content-addressed URL,
// so the browser can cache it forever and the JS bundle stays free
// of theme CSS.
// ============================================================

import type { Theme } from '@/api/types';

export interface ThemeMeta {
  name: string;
  subtitle: string;
  description: string;
  swatches: [string, string, string];
  vibe: 'serious' | 'cute';
}

export interface ThemePackage {
  /** Unique theme ID, used as the data-theme attribute value on the theme root */
  id: string;
  /** Display metadata for the picker UI */
  meta: ThemeMeta;
  /** Content-addressed stylesheet URL of the theme's current version.
   *  Content-addressed means the bytes behind it never change, so it is
   *  safe to hand straight to a <link rel="stylesheet">. */
  cssHref: string;
}

/** 缺色板时的中性占位色，只为让卡片可渲染，不冒充任何主题的真实配色。 */
const PLACEHOLDER_SWATCHES: [string, string, string] = ['#f6f7f9', '#8a8f98', '#232833'];

/**
 * API 的扁平 `Theme` → UI 的 `ThemePackage`。
 *
 * 保留这层映射而不是让 UI 直接吃 `Theme`：`Theme` 还带着 enabled/default/tier
 * 等只有管理面关心的字段，而渲染主题卡片只需要展示元数据与样式地址。
 *
 * 管理端的全量列表会包含尚无编译版本的主题，服务端对这类主题会省略 cssHref /
 * swatches / vibe。它们不可被选用，但仍要能在后台列出来，所以这里对缺失字段兜底
 * 到占位值——留空会让渲染直接崩在 `swatches.map`。
 */
/**
 * 把 API 的扁平 Theme 映射成 UI 模型。
 *
 * 没有编译版本的主题返回 null：它取不到样式，因此不可被选用。管理后台的
 * 全量列表会包含这类主题，用空 href 冒充可用只会让用户选中一个永远加载
 * 失败的主题——过滤掉才是诚实的表达。
 */
export function themePackageFromApi(theme: Theme): ThemePackage | null {
  if (!theme.cssHref || !theme.currentVersionId) return null;
  return {
    id: theme.id,
    cssHref: theme.cssHref,
    meta: {
      name: theme.name,
      subtitle: theme.subtitle ?? '',
      description: theme.description ?? '',
      swatches: theme.swatches ?? PLACEHOLDER_SWATCHES,
      vibe: theme.vibe ?? 'serious',
    },
  };
}

/** 批量转换并剔除不可用主题。 */
export function themePackagesFromApi(list: Theme[] | undefined): ThemePackage[] {
  return (list ?? []).map(themePackageFromApi).filter((pkg): pkg is ThemePackage => pkg !== null);
}

/**
 * 管理后台的展示映射：保留没有编译版本的主题。
 *
 * 与 themePackagesFromApi 的取舍不同是有意的——管理员必须能看到并启停这类
 * 主题，而普通用户不该在选择器里看到取不到样式的选项。cssHref 可能为空串，
 * 因此这个结果只能用于展示，绝不能拿去 activate。
 */
export function themeDisplayFromApi(theme: Theme): ThemePackage {
  return {
    id: theme.id,
    cssHref: theme.cssHref ?? '',
    meta: {
      name: theme.name,
      subtitle: theme.subtitle ?? '',
      description: theme.description ?? '',
      swatches: theme.swatches ?? PLACEHOLDER_SWATCHES,
      vibe: theme.vibe ?? 'serious',
    },
  };
}
