# nav.ax 主题规范 v1 设计

日期：2026-07-23
状态：已评审，待实现
范围：本文件详细设计**子项目 A（主题规范 v1 + 内置主题迁移）**，并界定 B（导入与分发）、C（声明式布局）的边界与依赖。

## 1. 目标与非目标

### 目标

- 把「主题」从前端硬编码的 TS 包，抽象成一份**可被第三方独立开发、校验、分发**的规范。
- 同一份规范同时服务两类宿主：nav.ax 主站（多租户，子域名启用）与开源自建实例。
- 主题内容进入实例数据库并**锁定版本**，公开页渲染不依赖任何外部网络。
- 不破坏现有安全不变量：访客 IP 不外泄、严格 CSP、无第三方代码执行、AGPL §13 源码链接始终可见。

### 非目标

- 不在主站运行任何第三方 JavaScript（规范为 tier 3 预留位置，但本次不实现，主站永不开启）。
- 不做主题市场、评分、付费、下载统计。
- 不做主题在线可视化编辑器。

## 2. 现状

- 主题是前端内置 TS 包 `web/src/themes/packages/*.ts`，形如 `{ id, meta, css }`；`themeRegistry.activate()` 把整段 CSS 注入 `<style>`，并在 `<html>` 上设 `data-theme`。
- 设计令牌是 OKLCH 三通道字符串（如 `--background-50: 0.990 0.003 12`）加字体、圆角、阴影。
- 数据库 `themes` 表只存目录元数据（`name/version/author/mode/enabled/is_default`），**不存 CSS**；`resolveThemeId`（`web/src/lib/themeResolve.ts`）在前端做兜底与别名映射，弥补目录与实际可用包之间的漂移。
- 布局（`layout.template/density/columns/categoryStyle`）属于页面设置，与主题无关。
- 主题 CSS 直接选择内部实现类名（如 `terminal` 主题选 `.material-card`、`.hairline`），无稳定契约。

## 3. 已确定的关键决策

| 决策 | 结论 |
|---|---|
| 能力边界 | 分级：tier 1（令牌 + 受限 CSS）、tier 2（声明式布局）、tier 3（JS，规范预留、本次不实现）。主站只开 1/2。 |
| 主站安装权限 | 官方目录（管理员审核，全站可见）+ 用户私有安装（导入到自己名下，仅自己可用，管理员可下架）。 |
| 版本策略 | 导入即入库锁版本，记录 commit sha；upstream 变更不影响已装主题；用户手动升级。 |
| 布局优先级 | 主题定结构并给旋钮默认值与允许范围（可锁定），用户在范围内调整，越界值一次性夹取。 |
| 资产 | 包内自带，导入时落地实例，统一同源供应；禁止运行时外链。 |
| CSS 校验 | 服务端真正解析 CSS，引入 `github.com/tdewolff/parse/v2`。 |

### 3.1 为什么不做 tier 3（JS 主题）

主题需要在 app 内预览，那一刻第三方 JS 与用户会话**同源**。会话 Cookie 是 HttpOnly，JS 读不到，但无需读取——直接 `fetch('/api/v1/...', { credentials: 'include' })` 即可，`Origin` 校验因同源而必然通过，等于把用户全部权限交给主题作者。公开子域一侧风险低一档但仍在：品牌域下的钓鱼登录框、访客数据外泄、挖矿、跳转，且会绕过「不存完整 IP、访客 ID 每日轮换 HMAC」这条不变量。

此外：CSP 需为第三方脚本放宽；JS 无法静态审核（混淆 + 运行时拼接请求）；GitHub 自动更新叠加 JS 等于供应链攻击面。唯一安全的做法是把渲染放进独立源的 sandbox iframe + postMessage RPC，但公开导航页整体进 iframe 会牺牲 SEO（快照正文不再作为页面内容被索引），并需重做缓存、快捷键、无障碍、背景媒体，属数周量级重构。

自建实例是单租户，实例主人给自己装 JS 主题等同装 WordPress 插件，风险自负——因此规范保留 tier 3 的声明位置，由宿主能力声明决定是否允许，但本次不实现，主站永不开启。

## 4. 架构总览

**主题是数据，不是代码。** 渲染始终由 nav.ax 自己的 React 组件完成，主题包只提供元信息、设计令牌、受限 CSS 与资产。

**信任边界在服务端。** 内置主题、GitHub 导入、zip 上传三条来源走同一条管线：

```
拉取 → 解析 manifest → 校验令牌 → 解析并校验 CSS → 编译（选择器加作用域、全局名命名空间化、
     资产 URL 重写） → 校验资产（magic bytes + 体积） → 计算 content_hash → 落库为不可变版本
```

浏览器只拿到已编译产物，前端不做任何安全决策。

**作用域封闭在主题根。** 主题 CSS 只能触达 `[data-nx="page-root"]` 的后代；宿主的应用外壳、管理界面与受许可证保护的元素都在这个根之外（§5.3）。这是整套安全模型的地基——令牌与规则都建立在"主题够不到宿主 DOM"之上。

**版本不可变，快照锁版本。** `theme_versions` 一行 = 一个来源版本的编译产物，永不改写。发布快照记录 `themeVersionId`，公开页据此从内容寻址 URL 取样式。

## 5. 包格式

### 5.1 仓库结构

GitHub 导入与 zip 上传共用同一布局：

```
theme.json      必需 —— 元信息 + 设计令牌 + 能力声明
theme.css       可选 —— 受限自由 CSS
assets/         可选 —— woff2 / png / jpg / webp
preview.png     可选 —— 目录卡片预览图
LICENSE  README.md
```

### 5.2 theme.json

`theme.json` 是唯一契约来源，其 schema 写入 `api/openapi.yaml`（组件名 `ThemeManifestV1`），前端类型与 Go DTO 均从中派生。

```json
{
  "specVersion": 1,
  "id": "sakura",
  "name": "Sakura",
  "version": "1.2.0",
  "author": "…",
  "license": "MIT",
  "homepage": "https://…",
  "mode": "light",
  "vibe": "cute",
  "swatches": ["#fef5f7", "#e88da5", "#8ecfba"],
  "tier": 1,
  "tokens": {
    "font":      { "heading": "…", "body": "…", "label": "…", "mono": "…" },
    "radius":    { "none": "0", "sm": "7px", "md": "14px", "lg": "22px", "xl": "28px", "2xl": "36px", "full": "9999px" },
    "elevation": { "surface": "…", "raised": "…", "float": "…", "overlay": "…" },
    "color": {
      "background": { "50": "0.990 0.003 12", "…": "…" },
      "foreground": { "…": "…" },
      "primary":    { "…": "…" },
      "accent":     { "…": "…" }
    }
  }
}
```

字段约束：

- `specVersion` 必须为 `1`；未来不兼容变更递增，旧版本仍可加载。
- `id`：`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`，作为 slug（见 §7.2）。
- `version`：语义化版本。
- `mode`：`light | dark | both`；`vibe`：`serious | cute`（沿用现有选择器分组）。
- `swatches`：三个 hex，供选择器预览。
- `tier`：`1 | 2 | 3`。字段在 v1 即冻结，但**子项目 A 的校验器只接受 `tier: 1`**，`2`/`3` 一律拒绝并提示"宿主暂不支持该能力级别"。tier 2 的语义随子项目 C 发布，tier 3 的入口、权限与隔离语义**尚未定义，不构成任何跨版本兼容承诺**。
- `tokens.color.*`：OKLCH 三通道。除形状正则外还须做数值范围校验：L ∈ [0, 1]、C ∈ [0, 0.5]、H ∈ [0, 360)。仅靠正则会放行 `9 9 999` 这类合法形状但不可用的值。
- **必填令牌组**：`color.background`、`color.foreground`、`color.primary`、`color.accent` 四组各至少一个档位，以及 `font` 四族。
- **可选令牌组**：`radius`、`elevation` 缺失时回落基线值。因此一个最小可用主题需要写的是**颜色四组 + 字体四族**，其余可省。
- `font.*` 只能是系统字体名，或包内 `@font-face` 声明的 family（校验器交叉检查）；字符白名单，防注入。

### 5.3 主题根与稳定选择器契约

**主题根不是 `<html>`。** 现有实现把 `data-theme` 设在 `document.documentElement` 上，且 `/app/themes` 预览页也走同一条路径——这意味着第三方 CSS 会作用于**整个已登录应用**，包括表单、按钮、管理界面。仅禁止直接命中 `[data-nx-protected]` 不足以补救：主题可以隐藏、裁剪或覆盖它的任意祖先。

因此规范规定：

- 公开页与预览页各自渲染一个**主题根**元素 `[data-nx="page-root"]`，`data-theme` 设在它上面，而不是 `<html>`。
- 所有主题 CSS 被编译到该根的后代作用域内，主题无法触达根之外的任何 DOM。
- **`[data-nx-protected]` 元素必须位于主题根之外**（AGPL §13 源码链接从 `PublicShell` 的内容区移到主题根外的外层容器）。这样"始终可见"由 DOM 结构保证，而不是靠规则拉黑。

**封闭需要两道机制，缺一不可——选择器前缀只解决其中一道。**

**(1) DOM 包含：选择器 subject 必须落在根内。** 仅仅"给选择器加前缀"不够：`[data-nx="page-root"] + footer` 会被改写成 `[data-theme="x"] + footer`，直接命中根外的兄弟元素——正是受保护页脚所在的位置。因此校验基于选择器 AST 强制：

- 根选择器之后只允许后代（空格）与子代（`>`）组合符；出现 `+` 或 `~` 一律拒绝；
- 每个选择器的 **subject**（最右侧被样式化的复合选择器）必须是根自身或其后代；
- `:is()` / `:where()` / `:has()` / `:not()` 内部递归应用同一规则，不得借嵌套绕过。
- 隔离测试须覆盖：根后代、根自身、根后兄弟（拒绝）、逗号分隔的选择器列表中混入越界项（整条拒绝）、`:is()` 嵌套越界。

**(2) 视觉包含：根必须建立包含块与独立层叠上下文。** DOM 封闭不等于视觉封闭。`position: relative` **不会**为 `position: fixed` 的后代建立包含块（只有 `transform` / `filter` / `perspective` / `will-change` / `contain` 会），所以根内的 fixed 覆盖层仍会铺满整个视口——在 `/app/themes` 预览时就会盖住整个已登录应用。绝对定位、超大 `box-shadow`、`filter` 的越界绘制同理。因此：

- 主题根设 `transform: translateZ(0)`，使其成为后代 fixed 元素的包含块，并建立独立层叠上下文——根内任何 `z-index`（上限 50）都被压在根这一层内，无法与根外元素比较层级；
- 受保护区域（页脚）在 DOM 上位于根之后，且置于更高层（`position: relative; z-index: 100`），因此永远绘制在主题之上；
- 浏览器测试用一个"恶意主题"夹具验证：`position: fixed` 全屏、绝对定位溢出、巨型 `box-shadow`、`filter` 四种手段都无法遮挡根外 UI 与源码链接。
- 主题 CSS **禁止选择 `html`、`body`**。全屏装饰效果改用 `[data-nx="page-root"]::before/::after`（主题根设为 `position: relative`，其内的 `position: fixed` 覆盖层仍须 `pointer-events: none`）。
- `/app/themes` 预览把主题根限定在预览容器内，管理与编辑 UI 不受主题影响。

在此之上，冻结一层公共钩子（现状问题：`terminal` 主题选 `.material-card`、`.hairline` 等内部实现类名，任何组件重构都会打碎第三方主题）：

- 渲染组件挂 `data-nx="page-root" | "site-card" | "category-tab" | "search-box" | "clock" | …`，清单写入 `docs/theme-api.md`，每个钩子标注 `stable` 或 `experimental`。
- 主题 CSS 只允许选择：`data-nx` 钩子、主题根内的标准 HTML 元素与伪元素、主题自己的私有 CSS 变量。命中未登记的内部类名（含 `[class*="w-11"]` 这类对 Tailwind 原子类的属性匹配）→ **校验失败并给出明确报错**，不静默忽略。
- 命中 `[data-nx-protected]` 的规则仍然拒绝，运行时再加一条 `!important` 兜底——在"位于主题根之外"之上的第三道保险。

`data-nx` 钩子清单的初版从现有 6 个主题实际用到的选择器反推。**但迁移不是无损的**，差异见 §6.4。

## 6. CSS 校验与编译

服务端实现，位于新包 `internal/themes`。使用 `github.com/tdewolff/parse/v2` 解析——正则不可行（注释、字符串转义、嵌套均可绕过）。

### 6.1 编译

- 作者不写作用域，编译器统一添加：所有选择器改写为 `[data-theme="<packageId>"] …`；`:root` 与 `[data-nx="page-root"]` 改写为 `[data-theme="<packageId>"]` 自身。
- 迁移内置主题时删除手写的 `[data-theme="terminal"]` 前缀。
- **全局名一律命名空间化**（选择器前缀不隔离这些名字）：`@keyframes` 名、`@font-face` 的 `font-family` 名都加 `<packageId>-` 前缀。重写必须覆盖**全部引用面**，否则会出现"通过校验但字体找不到"：
  - `@keyframes` 名 → 同步重写 `animation-name` 与 `animation` 简写中的引用；
  - `@font-face` 的 `font-family` descriptor → 同步重写 `font-family` 声明中对该族名的引用；
  - **令牌生成同步**：`tokens.font.*` 里引用包内字体时，生成的 `--font-*` 变量必须写重命名后的族名（否则 `theme.json` 与 `theme.css` 对不上）；
  - **v1 禁止在 `font` 简写中引用包内字体**（简写解析的歧义不值得在 v1 承担），校验阶段直接拒绝并提示改用 `font-family`；系统字体名不受影响。
  - 测试须覆盖：同名字体跨主题隔离、多词 family、带引号与不带引号的写法、`font` 简写引用包内字体被拒。
- `@layer` 在 v1 直接禁用——它的层序是全局的，无法靠改名隔离。
- 资产引用语法**唯一**：`url("asset:<包内路径>")`，编译器重写为同源路径 `/api/v1/public/themes/{versionId}/assets/<路径>`。校验不是"扫描 URL token"，而是**对声明值做语义级白名单**（见 §6.2）——CSS 里能触发资源加载的写法并不都产生 URL token。
- 输出规范化 CSS，与 manifest、资产一并计算 SHA-256 作为 `content_hash`；同一输入编译结果必须逐字节一致。

### 6.2 拒绝规则

| 规则 | 理由 |
|---|---|
| `@import` 一律拒绝 | 任意外部加载 |
| URL token 只允许 `url("asset:…")` 与 `data:image/png\|jpeg\|webp`（≤ 8 KB，按解码后 magic bytes 复核） | 访客 IP 不外泄；`data:image/svg+xml` **一并拒绝**，与资产层拒绝 SVG 保持一致 |
| `image-set()` / `-webkit-image-set()` 一律拒绝 | 它接受**字符串**形式的地址（`image-set("https://…" 1x)`），不产生 URL token，扫描 token 挡不住。v1 直接禁用，需要多倍图的主题用 `@media` 分辨率查询替代 |
| 声明值中出现形如 URL 的字符串字面量（含 `://`、以 `//` 开头、或 `\` 转义拼接）一律拒绝 | 堵住字符串形式的资源地址 |
| 自定义属性（`--*`）的值走**与普通声明相同**的完整校验 | `--x: url(https://…)` 再 `background: var(--x)` 是等价的外链通道 |
| `@font-face` 的 `src` 必须是 `url("asset:…")`，且其 `font-family` 需被 `tokens.font` 引用 | 同上 + 防悬空字体 |
| 选择器命中 `html` / `body` 一律拒绝 | 主题不得越出主题根（§5.3） |
| 根选择器之后出现 `+` / `~`，或 subject 不在根内 | 选择器逃逸（§5.3 (1)）|
| `font` 简写中引用包内字体 | 简写解析歧义，v1 不承担（§6.1）|
| 伪元素 `content` 只允许 `""` 与 `none` | 挡掉文本注入与钓鱼文案 |
| `position: fixed` 必须同时声明 `pointer-events: none` | 防全屏覆盖层劫持点击 |
| `z-index` ≤ 50 | 宿主导航栏、弹窗、Toast 永远在上层 |
| at-rule 白名单：`@media` `@supports` `@keyframes` `@font-face`，其余拒绝（含 `@layer`） | 未知语义面；`@layer` 层序全局不可隔离 |
| 禁 `behavior`、`-moz-binding`、`expression()` | 老式脚本注入面 |
| 命中 `[data-nx-protected]` 的规则拒绝 | 许可证义务 |
| CSS ≤ 256 KB；单个资产 ≤ 512 KB；整包 ≤ 4 MB | 体积预算 |

### 6.3 资产校验

按 magic bytes 判定真实类型（`wOF2`、PNG、JPEG、WebP），拒绝声明与内容不符者；沿用现有策略**拒绝 SVG**。体积上限见上表。中文字体通常超出 512 KB 上限，规范中明确要求作者自行子集化。

### 6.4 内置主题迁移差异（不是无损迁移）

现有 6 个主题并非都能原样通过上述规则。逐条列出并给出处置，**实现时先产出符合最终规则的 `theme.css` 黄金文件，再用它们证明规则可实施**：

| 主题 | 现状 | 冲突规则 | 处置 |
|---|---|---|---|
| `terminal` | `body::after` 全屏扫描线，`content: ''`、`position: fixed`、`pointer-events: none`、`z-index: 1` | 禁选 `body` | 选择器改为 `[data-nx="page-root"]::after`，其余原样通过 |
| `sakura` | `body::before` 全屏柔光 | 禁选 `body` | 同上，改 `[data-nx="page-root"]::before` |
| `sakura` | `content: '✿'` 装饰字符 | `content` 只允许空 | 改用 `background-image: url("asset:deco/blossom.png")` 或 mask 实现，视觉等价；**记录为实现方式变更** |
| `sakura` | `[class*="relative"]`、`[class*="w-11"]` 等对 Tailwind 原子类的属性匹配 | 只允许登记钩子 | 为受影响元素补 `data-nx` 钩子并改写选择器；补不出对应语义的，删除该条规则并记录视觉降级 |
| `slate` / `slate-dark` | `url("data:image/svg+xml,…")` 噪点纹理 | 拒绝 SVG（含 data URL） | 改为 `assets/noise.png`（或纯 CSS 渐变）；**记录为实现方式变更** |
| 全部 | 手写 `[data-theme="<id>"]` 前缀 | 编译器统一加作用域 | 迁移时删除前缀 |

上表就是 `docs/theme-api.md`「迁移映射」小节的初始内容。

## 7. 存储与加载

### 7.1 数据模型

新增迁移 `0014_theme_packages.sql`（append-only）：

```sql
-- themes 表扩列，成为「包」表
ALTER TABLE themes ADD COLUMN slug TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN scope TEXT NOT NULL DEFAULT 'catalog'
  CHECK (scope IN ('catalog','private'));
ALTER TABLE themes ADD COLUMN owner_id TEXT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE themes ADD COLUMN source_type TEXT NOT NULL DEFAULT 'builtin'
  CHECK (source_type IN ('builtin','github','upload'));
ALTER TABLE themes ADD COLUMN source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN current_version_id TEXT;
ALTER TABLE themes ADD COLUMN spec_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE theme_versions (
  id            TEXT PRIMARY KEY,
  -- RESTRICT 而非 CASCADE：已发布快照按 version_id 引用编译产物，
  -- 删包不得静默毁掉线上公开页的样式。删除路径必须先处理引用（见 §7.5）。
  theme_id      TEXT NOT NULL REFERENCES themes(id) ON DELETE RESTRICT,
  version       TEXT NOT NULL,
  source_ref    TEXT NOT NULL DEFAULT '',   -- commit sha / 上传摘要 / 'builtin'
  manifest_json TEXT NOT NULL,
  compiled_css  BLOB NOT NULL,
  content_hash  TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  imported_by   TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at    TEXT NOT NULL,
  UNIQUE (theme_id, content_hash)
);

CREATE TABLE theme_assets (
  id               TEXT PRIMARY KEY,
  theme_version_id TEXT NOT NULL REFERENCES theme_versions(id) ON DELETE CASCADE,
  path             TEXT NOT NULL,
  mime             TEXT NOT NULL,
  bytes            INTEGER NOT NULL,
  sha256           TEXT NOT NULL,
  data             BLOB NOT NULL,
  UNIQUE (theme_version_id, path)
);

-- 快照对主题版本的引用必须是可查询的外键列，不能只藏在 payload_json 里：
-- 否则 DELETE FROM theme_versions 能直接抽掉线上公开页的样式，
-- themes 上的 RESTRICT 只挡得住删包，挡不住删版本。
-- 可空：NULL = 本次迁移之前发布的旧快照，读取时回落默认主题。
ALTER TABLE published_snapshots ADD COLUMN theme_version_id TEXT
  REFERENCES theme_versions(id) ON DELETE RESTRICT;
CREATE INDEX idx_published_snapshots_theme_version
  ON published_snapshots(theme_version_id) WHERE theme_version_id IS NOT NULL;

-- 既有行（含 0013 已停用的 culled 主题）回填 slug，否则下面的唯一索引会因空串冲突
UPDATE themes SET slug = id WHERE slug = '';

-- 目录主题 slug 全站唯一；私有主题按 owner 唯一
CREATE UNIQUE INDEX idx_themes_catalog_slug ON themes(slug) WHERE scope = 'catalog';
CREATE UNIQUE INDEX idx_themes_private_slug ON themes(owner_id, slug) WHERE scope = 'private';
```

**归属约束。** SQLite 无法对已有表补 `CHECK`，而 `scope` 与 `owner_id` 必须成对成立（否则 `owner_id IS NULL` 的私有主题会因 NULL 在唯一索引中互不相等而绕过 slug 唯一性）。用一对触发器强制该不变量，插入与更新各一个：

```sql
CREATE TRIGGER themes_scope_owner_insert BEFORE INSERT ON themes
BEGIN
  SELECT RAISE(ABORT, 'catalog theme must have null owner_id; private theme must have owner_id')
  WHERE NOT ((NEW.scope = 'catalog' AND NEW.owner_id IS NULL)
          OR (NEW.scope = 'private' AND NEW.owner_id IS NOT NULL));
END;

CREATE TRIGGER themes_scope_owner_update BEFORE UPDATE ON themes
BEGIN
  SELECT RAISE(ABORT, 'catalog theme must have null owner_id; private theme must have owner_id')
  WHERE NOT ((NEW.scope = 'catalog' AND NEW.owner_id IS NULL)
          OR (NEW.scope = 'private' AND NEW.owner_id IS NOT NULL));
END;
```

`current_version_id` 同样无法补外键。因此**写入路径必须在同一事务内校验**：目标版本存在、`theme_id` 等于本行、`status = 'active'`。该校验有专门的回归测试（写入一个属于别的主题的 version_id 必须失败）。

### 7.2 包 ID 与 slug 分离

`themes.id` 保持不变（内置仍为 `slate`、`sakura` 等），因此 `appearance.themeId` 与所有现存页面设置**无需迁移**。第三方包的 `id` 是不透明 ULID，manifest 中的 `id` 仅作为 slug。两个用户各自导入名为 `sakura` 的不同主题不会冲突。CSS 作用域直接使用包 ID，天然无碰撞。

### 7.3 资产存储：SQLite BLOB

主题资产存 `theme_assets.data`，不复用 `internal/assets`。理由：主题版本必须是原子、不可变、自包含的单元；混入用户资产的生命周期（S3、归属、清理）会为 ≤4 MB 的数据引入不必要的耦合，且 `internal/assets` 的校验面向图片（`inspectImage`），不适用于 woff2。代价是中文字体基本无法在 512 KB 上限内直接使用，规范中已要求子集化。

### 7.4 内置主题迁移

- 6 个内置主题改写为 `theme.json` + `theme.css`（+ 必要资产），以 `//go:embed` 打进二进制。
- **启动时经真实校验器编译，并按 `content_hash` 幂等 upsert** 到 `theme_versions`。好处：规范每次启动都被自家主题验证，CI 中同样生效，杜绝目录与实现漂移；代价：启动增加数十毫秒。
- 删除前端 `resolveThemeId` 兜底与 `THEME_ID_ALIASES`；未知或已下架的 `themeId` 由服务端一处回落到默认主题，别名表（`kyoto → slate` 等）搬到 `internal/themes`，对外行为不变。
- `web/src/themes/packages/*.ts` 整体删除。

## 8. API 与前端运行时

### 8.1 发布锁版本与可见性谓词

草稿只存 `themeId`。`Publish` 在同一事务内解析出当时的 `themeVersionId`，**同时写入 `published_snapshots.theme_version_id` 列与快照 payload**，`PublishedPage` 增加该字段。已发布页面自此与主题更新完全解耦，且版本行受外键保护无法被删除。

**单一 eligibility 谓词。** 列表、选择、预览、发布必须复用同一个判定，否则会出现"选择器里能选、发布时静默回落"这种不一致（例如私有主题被卸载、或当前版本被撤销）：

```
eligible(themeId, actor) :=
      themes.enabled = 1
  AND themes.current_version_id IS NOT NULL
  AND theme_versions.theme_id = themes.id
  AND theme_versions.status = 'active'
  AND ( themes.scope = 'catalog'
        OR (themes.scope = 'private' AND themes.owner_id = actor) )
```

注意 `enabled = 1` **同时作用于目录主题与私有主题**——卸载私有主题正是通过 `enabled = 0` 实现的，私有分支若漏掉这条，已卸载的主题会继续出现在选择器里。

| 调用方 | 集合 |
|---|---|
| 匿名（公开页） | 不查询主题列表；只按快照的 `theme_version_id` 取 CSS |
| 登录用户（列表/选择/预览/发布） | `eligible(·, 当前用户)` |
| 管理员（后台目录） | 单独的全量只读谓词，含 `enabled=0` 与他人私有；**不复用 eligible**，且不可代为启用到自己页面 |

`Publish` 时 `eligible` 不成立 → 回落默认主题（不是报错，避免用户被他人的下架操作卡住发布）。

必需的一致性测试：私有主题卸载后（`enabled=0`）在列表、预览、发布三处行为一致；当前版本被撤销（`status='disabled'`）同理；**跨租户回归**——用户 A 的页面设置里塞进用户 B 的私有主题 ID，发布后快照必须落到默认主题。

### 8.1.1 生命周期与撤销语义

- **被快照引用的版本不得物理删除**，由两条外键共同保证：`theme_versions.theme_id → themes` 用 `ON DELETE RESTRICT`（挡删包），`published_snapshots.theme_version_id → theme_versions` 也用 `ON DELETE RESTRICT`（挡删版本）。只有后者才真正兑现这条承诺——把 versionId 藏在 `payload_json` 里数据库管不着。卸载主题走软删除（`themes.enabled = 0`），版本行与资产保留，已发布页面继续可用。物理清理只能针对无任何快照引用的版本，属于后续维护任务，不在 A 范围。
- **`status='disabled'` 的版本**：公开 CSS/资产端点对它返回 `410 Gone`（不是 404，语义上是"曾存在、已撤销"）；解析路径把引用它的页面回落到默认主题版本。
- **撤销与长缓存的关系**：CSS URL 是内容寻址的，`immutable` 一年缓存只对"这个 versionId 的字节"成立，而撤销的生效路径是**让页面不再引用它**——公开页的快照响应走 ETag/短缓存，回落后浏览器根本不会再请求被撤销的 URL。因此 kill switch 不依赖缓存清除。代价是：已在某个访客浏览器缓存中的旧 CSS，若该访客在回落生效前打开过页面，那一次访问无法追回——这是可接受的，也须在文档中写明，不要宣称"即时全局撤销"。

### 8.2 端点

新增两个公开端点（内容寻址，`Cache-Control: public, max-age=31536000, immutable` + ETag）：

```
GET /api/v1/public/themes/{versionId}.css
GET /api/v1/public/themes/{versionId}/assets/{path}
```

`{path}` 需做路径规范化校验，仅允许命中 `theme_assets` 中登记的确切 path。

扩展 `GET /api/v1/themes` 返回 `currentVersionId`、`tier`、`scope`、`swatches`、`mode`，选择器 UI 自此数据驱动。

`api/openapi.yaml` 与 `tests/contract/` 同步更新。

### 8.3 前端运行时

- `ThemePackage` 由 `{ id, meta, css }` 改为 `{ id, meta, cssHref }`。
- `data-theme` 设在主题根元素 `[data-nx="page-root"]` 上，**不再设在 `<html>`**（§5.3）。`/app/themes` 的预览把主题根限定在预览容器内。
- `themeRegistry.activate` 是一个**原子切换状态机**，不是简单的 onload 回调：
  - 每次切换递增一个序号；`load`/`error` 回调先比对序号，过期回调直接丢弃，避免慢请求覆盖用户较新的选择；
  - 新 link 成功加载前**保留旧 link**（先移除会闪烁），成功后同帧移除旧 link 并更新 `data-theme`；
  - 加载失败或超时（默认 5 秒）→ 移除失败的 link，保持当前主题不变，并向用户提示；若当前无任何主题（首次加载失败），显式回落到基线令牌；
  - 已撤销版本返回 `410` 时走同一条失败路径。
  - 这四条各有单测，E2E 覆盖"快速连点切换后最终状态与最后一次选择一致"。
- 主题列表来自 `GET /api/v1/themes`；`ThemePicker`、`/app/themes`、`/admin/themes` 三处改为数据驱动。
- 首屏不会无样式：基线令牌（slate）保留在主 CSS 中作为默认值，主题样式只做覆盖，观感与现状一致。

三个副作用均为收益：约 900 行 CSS 移出 JS bundle；主题样式成为可长缓存的独立资源；`style-src 'unsafe-inline'` 不再因主题而必需。

### 8.4 CSP 收紧（待核实）

`internal/httpapi/security_headers.go` 当前放行 `fonts.googleapis.com`、`fonts.gstatic.com`、`cdnjs.cloudflare.com`。字体自托管后这些应可移除，但实现时须先核实是否另有用途（`sakura`/`orbit` 只声明 `font-family`、未写 `@import`，疑似依赖系统已装字体）。核实后再改，本设计不硬性承诺。

## 9. 测试策略

- **校验器表驱动单测**：每条拒绝规则至少一个正例与一个反例，含转义、注释、嵌套、字符串形式 URL、`var()` 间接等绕过尝试；6 个迁移后的内置主题作为「必须通过」的黄金语料。
- **选择器隔离测试**：根后代（放行）、根自身（放行）、根后兄弟 `+`/`~`（拒绝）、逗号列表中混入越界项（整条拒绝）、`:is()`/`:has()` 嵌套越界（拒绝）。
- **视觉隔离浏览器测试**：一个"恶意主题"夹具用 `position: fixed` 全屏、绝对定位溢出、巨型 `box-shadow`、`filter` 四种手段尝试遮挡根外 UI 与源码链接，全部必须失败。
- **命名空间测试**：两个主题声明同名 `@keyframes` 与同名 `font-family` 互不干扰；多词 family、带引号/不带引号写法、令牌与 `@font-face` 的族名一致。
- **编译确定性**：同一输入两次编译 `content_hash` 一致。
- **SQLite 集成测试**：内置主题 upsert 幂等（连续两次启动不产生新版本行）；归属触发器拒绝 `scope`/`owner_id` 不匹配的行；`current_version_id` 指向他主题的版本必须写入失败；删除被快照引用的 `theme_versions` 行必须被 `RESTRICT` 拒绝；公开 CSS 与资产端点的 200/404/410/ETag/304。
- **eligibility 一致性**：私有主题 `enabled=0`、当前版本 `status='disabled'` 两种情形下，列表、预览、发布三处行为一致。
- **跨租户回归**：用户 A 的页面引用用户 B 的私有主题 → 发布后快照落到默认主题。
- **契约测试**：`tests/contract/` 覆盖两个新公开端点与 `GET /api/v1/themes` 扩展字段。
- **快照锁版本回归**：发布 → 变更主题当前版本 → 已发布快照仍指向旧 `themeVersionId`；删除主题包必须被 `RESTRICT` 拒绝。
- **E2E**：公开页加载后 `<link>` 存在、主题根上 `data-theme` 正确、切换主题后旧 link 被移除、快速连点后最终状态正确、页脚源码链接在任何主题下都可见。
- **`make test-mock`**：mock handlers 补齐新端点，保持契约守卫通过。

## 10. 后续子项目

### B. 导入与分发（依赖 A 的校验器与存储模型）

GitHub 一键导入：服务端解析 ref → commit sha，从 `codeload.github.com` 拉取 tarball，主机白名单 + 现有 `internal/netguard` SSRF 防护，防 zip-slip 与解压炸弹（限总解压体积、文件数、路径校验），自建实例可通过环境变量追加主机（GitLab 等）。另提供 zip 上传作为离线后备。用户私有安装、管理员目录审核、版本级 kill switch（禁用某版本后公开页回落默认主题）、更新检查。同时提供 `POST /api/v1/themes/validate` 供作者 dry-run 校验，以及官方 starter 仓库。

### C. tier 2 声明式布局（独立于 B）

主题声明区块顺序、卡片形态、分类展现方式，以及 `density`/`columns` 等旋钮的默认值、允许范围与锁定标记；公开页渲染改为 slot 驱动；用户既有设置越界时一次性夹取到合法值。此项改动现有公开页组件，风险最高，排在最后。

## 11. 风险与权衡

| 风险 | 缓解 |
|---|---|
| 稳定选择器契约冻结后限制内部重构 | 钩子标注 stable/experimental；experimental 可变更并在 `docs/theme-api.md` 记录 |
| 512 KB 资产上限对中文字体不友好 | 规范明确要求子集化；上限为宿主可配置项 |
| 启动时编译内置主题增加启动耗时 | 幂等 upsert，仅 6 个包，实测应在数十毫秒量级；若超预期改为构建期预编译 |
| 新增 CSS 解析依赖 | `tdewolff/parse/v2` 纯 Go、无 CGO、无传递依赖，符合项目「不引入重型框架」的边界 |
| 受限 CSS 表达力不足以吸引作者 | tier 2 布局能力（子项目 C）补足；先以内置主题验证令牌覆盖面 |
| 内置主题迁移存在视觉实现变更（§6.4） | 逐条列明并在 `docs/theme-api.md` 存档；实现时先产出黄金 `theme.css` 再收紧规则，避免"规则与现实互相迁就"|
| 主题根改造触及公开页 DOM 结构 | 与钩子契约在同一任务内完成并做浏览器冒烟；页脚源码链接的位置变更有专门 E2E 断言 |
| 撤销无法追回已缓存的一次访问 | 内容寻址 + 快照回落使新访问立即生效；文档明确不承诺"即时全局撤销"（§8.1.1）|

## 12. 评审记录

2026-07-23 由 codex 独立评审（`--focus` 架构一致性 / 模块边界 / YAGNI / 安全模型），返回 `request_changes`，7 major + 3 minor。经逐条核实：

- **已采纳并改写本文**：CSS 作用域越界到整个已登录应用（§5.3、§4）、多租户归属约束与可见性谓词（§7.1、§8.1）、级联删除会毁掉已发布快照（§7.1、§8.1.1）、内置主题迁移与规则冲突（§6.4）、资产引用语法歧义与 `data:image/svg+xml` 自相矛盾（§6.1、§6.2）、`@keyframes`/`font-family`/`@layer` 全局名冲突（§6.1、§6.2）、令牌必填表述矛盾与 OKLCH 数值范围（§5.2）、样式切换缺状态机（§8.3）。
- **部分采纳**：撤销与 `immutable` 缓存的冲突——实际撤销路径是快照回落而非缓存清除，已在 §8.1.1 写清边界，不改缓存策略。
- **未采纳**：从 v1 schema 删除 `tier` 字段（YAGNI 意见）。保留字段是明确的产品决策，但 A 阶段校验器只接受 `tier: 1`，且已在 §5.2 声明 tier 3 语义未定、不构成兼容承诺。

**第二轮**（同日，针对修订版）：`request_changes`，6 major，均为新问题、非重复。全部核实成立并已改写：

1. **选择器逃逸**——`[data-nx="page-root"] + footer` 加前缀后仍命中根外兄弟。已加基于 AST 的 subject 包含规则（§5.3 (1)）。
2. **视觉逃逸**——`position: relative` 不为 fixed 后代建立包含块，覆盖层仍能铺满视口。已要求主题根 `transform: translateZ(0)` 建立包含块与独立层叠上下文，受保护区域置于更高层（§5.3 (2)）。
3. **快照引用无外键**——`published_snapshots` 只有 `payload_json`，`RESTRICT` 挡删包不挡删版本。已增列 `theme_version_id` 外键（§7.1、§8.1.1）。
4. **可见性谓词自相矛盾**——私有分支漏了 `enabled=1`，导致列表可选、发布静默回落。已统一为单一 `eligible` 谓词（§8.1）。
5. **`image-set()` 字符串形式与 `var()` 间接绕过 URL token 扫描**。已改为语义级白名单：禁 `image-set()`、禁 URL 形态字符串字面量、自定义属性值走同等校验（§6.2）。
6. **字体重命名未覆盖 `font` 简写与令牌生成**。已明确三处引用面并在 v1 禁止 `font` 简写引用包内字体（§6.1）。
