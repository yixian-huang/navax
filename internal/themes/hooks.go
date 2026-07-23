package themes

import "sort"

// ThemeRootHook 是主题作用域的根。data-theme 设在它上面，主题 CSS 只能
// 触达它的后代。
const ThemeRootHook = "page-root"

// FrameAttr 是宿主 wrapper 的标记属性。它承载 contain: paint，同时提供
// 包含块、层叠上下文与绘制裁剪——这是视觉隔离的唯一边界。
//
// 它刻意不在 allowedHooks 中：主题在语法上无法命中它，因此边界不可被
// 主题用 transform/contain/z-index 覆盖掉。
const FrameAttr = "data-nx-frame"

// ProtectedAttr 标记必须始终可见的元素（AGPL §13 源码链接等）。它位于
// 宿主 wrapper 之外，因此不在被裁剪的绘制范围内。
const ProtectedAttr = "data-nx-protected"

// allowedHooks 是主题可以选择的稳定钩子，初版从现有 6 个内置主题实际
// 用到的选择器反推（见 docs/theme-api.md 的迁移映射）。
// 新增前先更新 docs/theme-api.md。
var allowedHooks = []string{
	"category-tab",     // [role="tablist"] button
	"category-tablist", // [role="tablist"]
	"clock",
	"divider",          // .hairline
	"divider-gradient", // .hairline-gradient
	"greeting",
	"nav-brand",   // header nav a[href="/"]
	"nav-link",    // header nav 内的其余链接
	"nav-tagline", // header p[class*="tracking"]
	"navbar",
	"page-root",
	"search-box",   // form
	"search-input", // form input
	"section-title",
	"site-card",       // .material-card
	"site-card-desc",  // .site-card-desc
	"site-card-icon",  // span[class*="w-11"]
	"site-card-title", // .site-card-title
	"site-grid",       // .grid（错峰入场动画的父容器）
	"skeleton",        // .skeleton
}

func init() { sort.Strings(allowedHooks) }

// AllowedHooks 返回已登记的 data-nx 值，按字典序。
func AllowedHooks() []string {
	out := make([]string, len(allowedHooks))
	copy(out, allowedHooks)
	return out
}

// IsAllowedHook 判断 data-nx 值是否已登记。
func IsAllowedHook(name string) bool {
	index := sort.SearchStrings(allowedHooks, name)
	return index < len(allowedHooks) && allowedHooks[index] == name
}
