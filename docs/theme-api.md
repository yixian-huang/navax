# nav.ax 主题 API

本文件是**主题作者面向的契约**：主题 CSS 能选什么、不能选什么，以及这些钩子在页面上对应哪块内容。

设计依据：`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`。
钩子清单的权威实现在 `internal/themes/hooks.go`，两者必须一致（有测试保证）。

## 1. 三层结构与作用域

公开页与预览页都渲染成三层：

```
[data-nx-frame]                         宿主 wrapper —— 主题选不到
  └─ [data-nx="page-root"][data-theme]  主题根 —— 主题 CSS 的作用域上界
[data-nx-protected]                     受保护区域 —— wrapper 之外的兄弟
```

- 你写的每条选择器都会被编译器自动加上 `[data-theme="<你的包 ID>"]` 前缀，**不要自己写作用域前缀**。
- `:root` 与 `[data-nx="page-root"]` 会被改写为主题根自身，用它们定义变量或做全屏装饰。
- 宿主 wrapper 承载 `contain: paint`，它同时是包含块、层叠上下文和绘制裁剪边界。这意味着：你的 `position: fixed` 覆盖层铺满的是**页面区域**，不是浏览器视口；超出 wrapper 的阴影与滤镜会被裁掉。这是有意为之——它保证主题无法遮挡宿主 UI 与许可证要求的源码链接。
- `[data-nx-protected]` 标记的元素（AGPL §13 源码链接等）位于 wrapper 之外，主题既选不到也盖不住。

## 2. 钩子清单

主题只能选择下列 `data-nx` 值、主题根内的标准 HTML 元素与伪元素，以及自己的私有 CSS 变量。**不能选类名**（`.material-card` 这类内部实现类会被拒绝），也不能用 `[class*="…"]` 绕过。

| 钩子 | 对应内容 | 稳定性 |
|---|---|---|
| `page-root` | 主题根，全屏装饰用它 | stable |
| `navbar` | 顶部导航栏 | stable |
| `nav-brand` | 导航栏品牌链接 | stable |
| `nav-link` | 导航栏其余链接 | stable |
| `nav-tagline` | 导航栏标语文字 | experimental |
| `search-box` | 搜索表单容器 | stable |
| `search-input` | 搜索输入框 | stable |
| `category-tablist` | 分类标签栏容器 | stable |
| `category-tab` | 单个分类标签 | stable |
| `site-grid` | 站点卡片网格容器（错峰入场动画的父级） | stable |
| `site-card` | 站点卡片 | stable |
| `site-card-title` | 卡片标题 | stable |
| `site-card-desc` | 卡片描述 | stable |
| `site-card-icon` | 卡片图标容器 | stable |
| `section-title` | 区块标题 | stable |
| `divider` | 细分隔线 | experimental |
| `divider-gradient` | 渐变分隔线 | experimental |
| `clock` | 时钟 | stable |
| `greeting` | 问候语 | stable |
| `skeleton` | 骨架屏占位 | experimental |

标 `experimental` 的钩子可能在小版本中变更或移除，变更会记录在本文件。

## 3. 资产

- 包内资产放 `assets/`，在 CSS 中用 `url("asset:fonts/x.woff2")` 引用，编译器会重写成同源路径。
- 允许的类型：`woff2`、`png`、`jpeg`、`webp`。**SVG 一律拒绝**，包括 `data:image/svg+xml`。
- 单个资产 ≤ 512 KB，整包 ≤ 4 MB，CSS ≤ 256 KB。中文字体请自行子集化。

## 4. 迁移映射（内置主题）

首批 6 个内置主题从旧的"前端硬编码 CSS 包"迁移到本规范时的对应关系。第三方主题作者可以照此理解钩子语义。

| 旧写法 | 新写法 |
|---|---|
| `.material-card` | `[data-nx="site-card"]` |
| `.material-card .site-card-title` | `[data-nx="site-card-title"]` |
| `.material-card .site-card-desc` | `[data-nx="site-card-desc"]` |
| `.material-card span[class*="w-11"]` | `[data-nx="site-card-icon"]` |
| `.grid > .material-card:nth-child(n)` | `[data-nx="site-grid"] > [data-nx="site-card"]:nth-child(n)` |
| `.hairline` / `.hairline-gradient` | `[data-nx="divider"]` / `[data-nx="divider-gradient"]` |
| `[role="tablist"]` / `[role="tablist"] button` | `[data-nx="category-tablist"]` / `[data-nx="category-tab"]` |
| `form` / `form input` | `[data-nx="search-box"]` / `[data-nx="search-input"]` |
| `header nav a[href="/"]` | `[data-nx="nav-brand"]` |
| `header nav a[href="/login"]` | `[data-nx="nav-link"]` |
| `header p[class*="tracking"]` | `[data-nx="nav-tagline"]` |
| `.skeleton` | `[data-nx="skeleton"]` |
| `body::before` / `body::after` | `[data-nx="page-root"]::before` / `::after` |
| `[data-theme="<id>"] …` 手写前缀 | 删除，编译器自动添加 |

### 4.1 实现方式变更与视觉降级

迁移**不是无损的**。以下差异是规则与安全模型的必然结果，不是疏漏：

| 主题 | 旧实现 | 新实现 | 原因 |
|---|---|---|---|
| `sakura` | `content: '✿'` 装饰字符 | 改用 `background-image: url("asset:…")` 或 mask | 伪元素 `content` 只允许空字符串——非空 content 是文本注入与钓鱼文案的通道 |
| `slate` / `slate-dark` | `url("data:image/svg+xml,…")` 噪点纹理 | 改用 `assets/noise.png` 或纯 CSS 渐变 | SVG 一律拒绝，`data:` 形式也不例外 |
| `sakura` | `footer a:hover` 页脚链接样式 | **移除** | 页脚是受保护区域，位于主题根之外，主题不再能触达 |
| 全部 | `body` / `html` 选择器 | `[data-nx="page-root"]` | 主题不得越出主题根 |

## 5. 被拒绝的写法

校验发生在服务端，浏览器只拿到已编译产物。下列写法一律拒绝并给出定位信息：

- `@import`、`@layer`；白名单外的 at-rule
- 白名单外的函数（`src()`、`image()`、`image-set()`、`cross-fade()`、`element()`、`paint()` 等一切可能触发外部请求的函数）
- 外部 URL、形如 URL 的字符串字面量、`data:image/svg+xml`
- 选择 `html` / `body`、命中 `[data-nx-frame]` 或 `[data-nx-protected]`
- 主题根之后使用 `+` / `~`，或选择器 subject 落在根外
- 类选择器与 `[class*="…"]` 属性匹配
- CSS nesting（`&`、`@nest`）与命名空间选择器（`ns|el`）
- 非空伪元素 `content`
- `behavior`、`-moz-binding`、`expression()`

标识符在比较前会先解码 CSS 转义并做 ASCII 小写规范化——`h\74ml` 等同于 `html`，不能借此绕过。
