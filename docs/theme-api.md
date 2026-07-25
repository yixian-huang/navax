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
| `nav-link` | 导航栏普通链接（发现等） | stable |
| `nav-cta` | 导航栏主行动按钮（登录） | stable |
| `nav-tagline` | 导航栏标语文字 | experimental ⚠️ |
| `search-box` | 搜索表单容器 | stable |
| `search-input` | 搜索输入框 | stable |
| `category-tablist` | 分类标签栏容器 | stable |
| `category-tab` | 单个分类标签 | stable |
| `category-indicator` | 标签栏下方的动画下划线指示器 | stable |
| `site-grid` | 站点卡片网格容器（错峰入场动画的父级） | stable |
| `site-card` | 站点卡片 | stable |
| `site-card-title` | 卡片标题 | stable |
| `site-card-desc` | 卡片描述 | experimental ⚠️ |
| `site-card-icon` | 卡片图标容器 | stable |
| `section-title` | 区块标题 | stable |
| `divider` | 细分隔线 | experimental ⚠️ |
| `divider-gradient` | 渐变分隔线 | experimental |
| `clock` | 时钟 | stable |
| `greeting` | 问候语 | stable |
| `skeleton` | 骨架屏占位 | experimental |

标 `experimental` 的钩子可能在小版本中变更或移除，变更会记录在本文件。

**⚠️ 标记的钩子当前在公开页没有挂载点**（`nav-tagline`、`site-card-desc`、`divider`）。
它们是已登记的契约，校验器会放行，但页面上暂时不存在对应元素，因此针对它们的
规则不会有任何视觉效果。这不是 bug，是如实告知——不写明的话你会得到一条编译
通过却永远不生效的规则，而且没有任何报错。

### 2.1 主题根上的状态属性

主题根除了 `data-theme`，还会按页面状态带上：

| 属性 | 含义 |
|---|---|
| `data-wallpaper="true"` | 当前页面设置了壁纸背景 |
| `data-wallpaper-tone="light\|dark"` | 壁纸的明暗取样结果 |

它们在**主题根自身**上，不是祖先。所以要写成复合选择器：

```css
/* 对：命中根自身 */
[data-nx="page-root"][data-wallpaper] [data-nx="site-card"] { … }

/* 错：后代形式永远不匹配，且不会报错 */
[data-wallpaper] [data-nx="site-card"] { … }
```

## 3. 资产

- 包内资产放 `assets/`，在 CSS 中用 `url("asset:fonts/x.woff2")` 引用，编译器会重写成同源路径。
- 允许的**资产文件**类型：`woff2`、`png`、`jpeg`、`webp`。**`.svg` 文件一律拒绝**——它可被直接导航，那是文档上下文，脚本会执行。
- **CSS 里的 `data:image/svg+xml` 允许**（≤ 8 KB）。`url()` 中的 SVG 处于图片上下文，浏览器以 secure static mode 加载：脚本不执行、外部资源不加载。仍会额外净化，含 `<script>`、`<foreignObject>`、`<use>`、`<image>`、事件处理器或 `javascript:` 的一律拒绝。噪点、纹理这类需求用它。
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
| `header nav a[href="/login"]` | `[data-nx="nav-cta"]` |
| `header p[class*="tracking"]` | `[data-nx="nav-tagline"]` |
| `.skeleton` | `[data-nx="skeleton"]` |
| `body::before` / `body::after` | `[data-nx="page-root"]::before` / `::after` |
| `[data-theme="<id>"] …` 手写前缀 | 删除，编译器自动添加 |

### 4.1 实现方式变更与视觉降级

迁移**不是无损的**。下表是 6 个主题迁移后的实际结果（不是预估），差异都是规则与
安全模型的必然结果：

| 主题 | 变更 | 类型 | 说明 |
|---|---|---|---|
| `sakura` | `footer a:hover` | 移除 | 页脚是受保护区域，位于主题根之外，主题不再能触达 |
| `sakura` | `.rise-in` 入场动画 | 移除 | 宿主通用动画，不属主题契约 |
| `sakura` | `@keyframes gentleFloat` 原本复用宿主全局定义 | 实现方式变更 | 编译器会把引用重写为 `sakura-gentleFloat`，故包内自带同名 keyframes，行为等价 |
| `slate` / `slate-dark` | 壁纸态下 `.site-card-domain` 与 `.wallpaper-ink-scope` 的规则 | 移除 | 内部类名，无对应钩子 |
| 全部 | `body` / `html` 选择器 | 实现方式变更 | 改为 `[data-nx="page-root"]`，配合宿主 wrapper 的 `contain: paint` |

初版迁移时还有三处降级，后来通过放宽规则或补钩子**已全部恢复**，保留在此作为规则演进的记录：

| 主题 | 初版降级 | 恢复方式 |
|---|---|---|
| `sakura` | `content: '✿'` 悬停花瓣被删 | `content` 规则从「只允许空」放宽为「符号/标点类目且 ≤ 2 字符」——限制的目标是不能成词，而不是不能有内容 |
| `slate` / `slate-dark` | SVG `feTurbulence` 噪点退化为 CSS 渐变 | 区分图片上下文与文档上下文：CSS 里的 `data:image/svg+xml` 放行并净化，`.svg` 资产文件仍拒绝 |
| `sakura` | 隐藏分类栏下划线指示器的规则被删 | 补 `category-indicator` 钩子——这是正当的主题需求（胶囊标签不需要下划线），缺的只是一个钩子 |

## 5. 被拒绝的写法

校验发生在服务端，浏览器只拿到已编译产物。下列写法一律拒绝并给出定位信息：

- `@import`、`@layer`；白名单外的 at-rule
- 白名单外的函数（`src()`、`image()`、`image-set()`、`cross-fade()`、`element()`、`paint()` 等一切可能触发外部请求的函数）
- 外部 URL、形如 URL 的字符串字面量;`data:` 图片仅限 png/jpeg/webp/svg+xml 且单条 ≤ 8 KB(SVG 需通过净化检查;`.svg` 资产**文件**仍一律拒绝——图片上下文与文档上下文的风险不同)
- 选择 `html` / `body`、命中 `[data-nx-frame]` 或 `[data-nx-protected]`
- 主题根之后使用 `+` / `~`，或选择器 subject 落在根外
- 类选择器与 `[class*="…"]` 属性匹配
- CSS nesting（`&`、`@nest`）与命名空间选择器（`ns|el`）
- 伪元素 `content` 含字母或文字，或超过 2 个字符（装饰性符号与标点可用，如 `✿ · → ★`）
- `behavior`、`-moz-binding`、`expression()`

标识符在比较前会先解码 CSS 转义并做 ASCII 小写规范化——`h\74ml` 等同于 `html`，不能借此绕过。

## 6. 导入与安装

本节面向想把自己的主题包装到 nav.ax 实例上的作者。实现见 `internal/themes`
（解包/校验/编译）与 `internal/themeimport`（GitHub 拉取与导入编排）。

### 6.1 包布局

一个主题包是一个 zip（或 GitHub 仓库的 tarball），解压后按白名单提取：

| 路径 | 必需 | 说明 |
|---|---|---|
| `theme.json` | 是 | manifest，字段与 `api/openapi.yaml` 的 `ThemeManifestV1` 一一对应 |
| `theme.css` | 否 | 缺失按空 CSS 处理，仅令牌生成的基础样式生效 |
| `assets/**` | 否 | 字体与图片，包内路径去掉 `assets/` 前缀后即为资产路径 |
| `preview.png` | 否 | 主题预览图，落库后成为 `themes.preview` 的实际取值 |

其余文件（`README`、`LICENSE`、`.github/` 等）会被忽略，不参与校验也不占资产配额。

如果解压出的全部条目共享同一个顶层目录（GitHub tarball 固有的
`{repo}-{sha}/` 前缀，或作者打 zip 时习惯带的外层文件夹），该目录会被自动
剥离一层，因此仓库根目录放 `theme.json` 或套一层文件夹再放，效果相同。

### 6.2 zip 上传与 GitHub 导入

`POST /api/v1/me/themes/import`（需登录）按 `Content-Type` 分两种请求形态：

- `multipart/form-data`，字段名 `file`：zip 上传。
- `application/json`：`{"githubUrl": "https://github.com/{owner}/{repo}", "ref": "<可选>"}`，从 GitHub 拉取 tarball 后走同一条解包/校验/编译管线。

  `ref` 请直接传分支名、标签名或 40 位 commit sha（如 `main`、`v1.2.0`）——
  不要传 `refs/heads/main` 这种带斜杠的完整引用形式。服务端会对整个 `ref`
  做 `url.PathEscape` 再拼进 `commits/{ref}` 这一段路径，斜杠会被转义成
  `%2F`，导致 GitHub API 找不到这个 ref 而拉取失败。

两条路径落库时都会：

- **锁定来源**：GitHub 导入把 `ref` 解析为具体的 40 位 commit sha 后记入
  `theme_versions.source_ref`（即便你传的是分支名，落库的也是那一刻解析出
  的 sha，不是活动引用）；zip 上传则记入上传内容的 `sha256:` 摘要
  （`ContentDigest`）。两者都保证同一版本行的内容可追溯、不随上游变化。
- **私有安装仅自己可用**：新装的主题以 `scope = 'private'`、
  `owner_id = 当前用户` 落库，只有 owner 能在选择器、我的主题列表和发布时
  看到/用到它（与目录主题共用同一条可用性谓词，见 `internal/themes/store.go`
  的 `EligibilityWhere`）。
- **同 slug 重复导入即升级**：`theme.json` 里的 `id` 字段同时是主题标识与
  私有安装的 slug。同一用户对同一 `id` 再次导入（无论换成新版本还是原样
  重传）不会新建一行、不占新配额，而是复用既有 `themes` 行、写入新版本并
  切换 `current_version_id`；已发布快照引用的历史版本不受影响（版本内容
  寻址、不可变）。如果这个 slug 此前被卸载成了「墓碑」（见 §6.8），重新
  导入会把它唤醒（`enabled` 改回 `1`），而不是被拒绝或产生一个用户看不到
  的新行。

成功返回 `201` 与该主题的完整 `Theme` 对象（形状同列表接口）。

### 6.3 体积与数量硬限

解包、组包、编译三层各自设限，任一超限都直接失败，不做截断或降级：

| 限制 | 数值 | 作用层 |
|---|---|---|
| 压缩包大小 / 解压总量 | 16 MiB | 解包（`MaxArchiveBytes`）——上传的 zip 体积、GitHub tarball 响应体、解压后全部文件累计字节数，三者共用同一个上限；解压过程中一旦累计超限立即失败，不等读完 |
| 包内文件数 | 200 | 解包（`MaxArchiveFiles`），忽略目录条目 |
| 整包体积（CSS + 全部资产） | 4 MiB | 编译（`MaxPackageBytes`） |
| 单个 `theme.css` | 256 KiB | CSS 校验（`MaxCSSBytes`） |
| 单个资产文件 | 512 KiB | 资产校验（`MaxAssetBytes`）——中文字体请自行子集化 |
| CSS 内单条 `data:` URI | 8 KiB | CSS 校验（`MaxDataURIBytes`，见 §3） |

解包层另有三项不可绕过的防护：拒绝 zip-slip（绝对路径、`..` 逃逸、反斜杠）；
GitHub tarball 中的软链接/硬链接/设备文件一律拒绝，只接受普通文件；zip 内
目录条目被跳过，不计入文件数。

### 6.4 私有主题配额

每个用户可持有的私有主题数量有实例级配额，默认 **10**，运维可通过环境变量
`NAVAX_THEME_PRIVATE_QUOTA` 调整。计数按**行数**而非「当前可见」数：被
已发布快照引用因而只能转为墓碑（`enabled = 0`，见 §6.8）的已卸载主题仍占
一个名额。墓碑不会出现在「我的主题」列表里（列表复用可用性谓词，要求
`enabled = 1`），因此无法从界面回收这个名额；配额只有在所有引用它的发布
快照都被替换掉之后、且再次对同一 `themeId` 发起卸载请求时才会被物理删除
并释放。超出配额导入会收到 `409 QUOTA_EXCEEDED`。

### 6.5 dry-run 校验：`POST /api/v1/themes/validate`

在真正导入前，可以用同一个 zip 走一遍**只读**的完整管线（解包 → 组包 →
编译），不写入数据库、不占配额、不消耗导入限流：

```
POST /api/v1/themes/validate
Content-Type: multipart/form-data; boundary=...

file: <你的主题包 zip>
```

响应：

```json
{ "valid": true, "errors": [] }
```

失败时 `valid` 为 `false`，`errors` 是结构化问题列表，`stage` 取以下四类之一：

| stage | 含义 | `path` 是否填充 |
|---|---|---|
| `archive` | zip 本身不可接受（路径逃逸、超限、格式损坏） | 否，定位信息在 `message` 文本里 |
| `manifest` | `theme.json` 解析或校验失败 | 是，固定为 `theme.json` |
| `css` | `theme.css` 校验失败 | 是，固定为 `theme.css` |
| `asset` | 某个资产文件校验失败（类型、体积、路径） | 否，具体资产名在 `message` 文本里 |

当前粒度是**首个错误**：管线在第一处失败就停止，不会把同一份包里的全部
问题一次性列出——改完第一条错误再重新校验，才会看到下一条。

`/themes/validate` 只接受 `multipart/form-data` 的 zip，不支持对 GitHub
仓库做 dry-run；要校验一个仓库，先把它打成 zip。

导入与校验各自独立限流、按来源 IP 计数：`POST /me/themes/import` 每小时 10
次，`POST /themes/validate` 每小时 20 次，互不共享配额，超出返回
`429 RATE_LIMITED` 并带 `Retry-After`。

### 6.6 GitHub 匿名 API 限额

把 `ref`（分支/标签）解析为 commit sha 这一步会调用
`api.github.com/repos/{owner}/{repo}/commits/{ref}`。这是 GitHub 自己对
未认证请求的限制（不是 nav.ax 施加的），默认约 60 次/小时/来源 IP，多用户
共享同一实例出口 IP 时容易触顶。三种规避方式：

- **直接传 40 位十六进制 commit sha 作为 `ref`**：服务端识别出这已经是
  sha 后跳过 `api.github.com` 解析，直接向 `codeload.github.com` 请求
  tarball——下载 tarball 本身不计入 REST API 限额。
- **配置 `NAVAX_GITHUB_TOKEN`**：设置后，请求 `github.com` /
  `api.github.com` / `codeload.github.com` 这三个官方主机时会带上
  `Authorization: Bearer <token>`，把限额从匿名档提升到认证档（具体数值由
  GitHub 决定）。这个 token **只**对上述三个官方主机生效，不会被转发给
  `NAVAX_THEME_IMPORT_HOSTS` 白名单里的自建 Gitea 等第三方主机。
- **zip 兜底**：GitHub 拉取因限额、网络或仓库不可达失败时，直接把仓库打成
  zip 走 §6.2 的上传路径——两条路径共用同一套解包/校验/编译管线，效果等价。

### 6.7 `NAVAX_THEME_IMPORT_HOSTS`：追加导入主机

默认只允许从 `github.com` 导入。运维可通过 `NAVAX_THEME_IMPORT_HOSTS`
（逗号分隔的主机名列表，大小写不敏感）追加允许的仓库主机，典型场景是
自建的 GitHub Enterprise 或 Gitea 镜像。追加主机与 `github.com` 有两点
关键差异：

- **按 Gitea 兼容的 archive 布局取包**：请求
  `https://{host}/{owner}/{repo}/archive/{ref}.tar.gz`，不经过任何
  `.../commits/{ref}` 式的 API 解析——因为非官方 GitHub 主机不保证有这个
  API。
- **必须显式传 `ref`**：这些主机没有「默认分支」这一步解析，留空 `ref`
  会直接返回 422，不会像 `github.com` 那样缺省取 `HEAD`。

请求同样经过 SSRF 防护（`internal/netguard`）：每次 DNS 解析与重定向都会
拒绝回环、私网、链路本地、保留地址与云元数据地址；仓库地址必须是
`https://` 且不含用户名/密码。`NAVAX_GITHUB_TOKEN` 不会下发给这些主机
（见 §6.6）。

### 6.8 卸载

`DELETE /api/v1/me/themes/{themeId}`（仅本人）根据是否还有历史发布引用，
走两条分支之一：

1. **没有任何已发布快照引用过它的任一版本**：物理删除——`theme_versions`
   与 `themes` 行一并清除，配额立即释放。
2. **至少一个已发布快照引用过某个历史版本**：转为「墓碑」——只把
   `themes.enabled` 置为 `0`，版本与资产原样保留。已经发布出去的页面
   继续正常渲染这个主题；它只是从你的选择器、我的主题列表和后续可选目标
   里消失（沿用 §6.2 提到的可用性谓词）。这一行仍计入配额，直到不再被
   任何快照引用、被再次卸载时才会物理删除。

不存在的 `themeId` 与「存在但不是你的」统一返回 `404 NOT_FOUND`，不做区分
——避免被用来探测其他用户持有哪些私有主题。同 slug 重新导入可以唤醒一个
墓碑（见 §6.2），这是它存在的主要原因：卸载不是不可逆操作。
