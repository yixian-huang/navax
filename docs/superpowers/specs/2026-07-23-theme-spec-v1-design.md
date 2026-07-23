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
拉取 → 解析 manifest → 校验令牌 → 解析并校验 CSS → 编译（选择器加作用域、asset() 重写）
     → 校验资产（magic bytes + 体积） → 计算 content_hash → 落库为不可变版本
```

浏览器只拿到已编译产物，前端不做任何安全决策。

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
- `tier`：`1 | 2 | 3`。宿主拒绝加载超出自身允许级别的包。
- `tokens.color.*`：OKLCH 三通道，正则 `^\d(\.\d+)? \d(\.\d+)? \d{1,3}(\.\d+)?$`，沿用现有格式，现有 6 个主题可一比一映射。
- **必填令牌组**：`color.background`、`color.foreground`、`color.primary`、`color.accent` 的完整档位，以及 `font` 四族。
- **可选令牌组**：`radius`、`elevation` 缺失时回落基线值，因此最小主题只写颜色即可运行。
- `font.*` 只能是系统字体名，或包内 `@font-face` 声明的 family（校验器交叉检查）；字符白名单，防注入。

### 5.3 稳定选择器契约

现状问题：`terminal` 主题选 `.material-card`、`.hairline` 等内部实现类名，任何组件重构都会打碎第三方主题。规范必须冻结一层公共钩子。

- 渲染组件挂 `data-nx="site-card" | "category-tab" | "search-box" | "clock" | …`，清单写入 `docs/theme-api.md`，每个钩子标注 `stable` 或 `experimental`。
- 主题 CSS 只允许选择：`data-nx` 钩子、标准 HTML 元素与伪元素、主题自己的私有 CSS 变量。命中未登记的内部类名 → **校验失败并给出明确报错**，不静默忽略。
- `[data-nx-protected]` 标记 AGPL §13 源码链接等必须可见的元素。命中它的规则一律拒绝；运行时再以一条 `!important` 规则兜底强制其可见。这是许可证义务，双保险。

`data-nx` 钩子清单的初版从现有 6 个主题实际用到的选择器反推，确保迁移无损。

## 6. CSS 校验与编译

服务端实现，位于新包 `internal/themes`。使用 `github.com/tdewolff/parse/v2` 解析——正则不可行（注释、字符串转义、嵌套均可绕过）。

### 6.1 编译

- 作者不写作用域，编译器统一添加：所有选择器改写为 `[data-theme="<packageId>"] …`，`:root` 改写为 `[data-theme="<packageId>"]`。
- 迁移内置主题时删除手写的 `[data-theme="terminal"]` 前缀。
- `asset("fonts/x.woff2")` 重写为同源路径 `/api/v1/public/themes/{versionId}/assets/fonts/x.woff2`。
- 输出规范化 CSS，与 manifest、资产一并计算 SHA-256 作为 `content_hash`；同一输入编译结果必须逐字节一致。

### 6.2 拒绝规则

| 规则 | 理由 |
|---|---|
| `@import` 一律拒绝 | 任意外部加载 |
| `url()` 只允许 `asset("…")` 与小体积 `data:image/*` | 访客 IP 不外泄 |
| `@font-face` 的 `src` 必须是 `asset()`，且其 `font-family` 需被 `tokens.font` 引用 | 同上 + 防悬空字体 |
| 伪元素 `content` 只允许 `""` 与 `none` | 挡掉文本注入与钓鱼文案 |
| `position: fixed` 必须同时声明 `pointer-events: none` | 防全屏覆盖层劫持点击 |
| `z-index` ≤ 50 | 宿主导航栏、弹窗、Toast 永远在上层 |
| at-rule 白名单：`@media` `@supports` `@keyframes` `@font-face` `@layer`，其余拒绝 | 未知语义面 |
| 禁 `behavior`、`-moz-binding`、`expression()` | 老式脚本注入面 |
| 命中 `[data-nx-protected]` 的规则拒绝 | 许可证义务 |
| CSS ≤ 256 KB；单个资产 ≤ 512 KB；整包 ≤ 4 MB | 体积预算 |

规则从真实主题反推：`terminal` 的扫描线正是 `body::after` 全屏 `position: fixed`，规范允许该效果，仅强制 `pointer-events: none` 与空 `content`，迁移时原样通过。

### 6.3 资产校验

按 magic bytes 判定真实类型（`wOF2`、PNG、JPEG、WebP），拒绝声明与内容不符者；沿用现有策略**拒绝 SVG**。体积上限见上表。中文字体通常超出 512 KB 上限，规范中明确要求作者自行子集化。

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
  theme_id      TEXT NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
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

-- 既有行（含 0013 已停用的 culled 主题）回填 slug，否则下面的唯一索引会因空串冲突
UPDATE themes SET slug = id WHERE slug = '';

-- 目录主题 slug 全站唯一；私有主题按 owner 唯一
CREATE UNIQUE INDEX idx_themes_catalog_slug ON themes(slug) WHERE scope = 'catalog';
CREATE UNIQUE INDEX idx_themes_private_slug ON themes(owner_id, slug) WHERE scope = 'private';
```

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

### 8.1 发布锁版本

草稿只存 `themeId`。`Publish` 在同一事务内解析出当时的 `themeVersionId` 写入快照，`PublishedPage` 增加该字段。已发布页面自此与主题更新完全解耦。

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
- `themeRegistry.activate` 改为切换 `<link rel="stylesheet">`，`onload` 后再设置 `data-theme`，避免闪烁；切换时移除旧 link。
- 主题列表来自 `GET /api/v1/themes`；`ThemePicker`、`/app/themes`、`/admin/themes` 三处改为数据驱动。
- 首屏不会无样式：基线令牌（slate）保留在主 CSS 中作为默认值，主题样式只做覆盖，观感与现状一致。

三个副作用均为收益：约 900 行 CSS 移出 JS bundle；主题样式成为可长缓存的独立资源；`style-src 'unsafe-inline'` 不再因主题而必需。

### 8.4 CSP 收紧（待核实）

`internal/httpapi/security_headers.go` 当前放行 `fonts.googleapis.com`、`fonts.gstatic.com`、`cdnjs.cloudflare.com`。字体自托管后这些应可移除，但实现时须先核实是否另有用途（`sakura`/`orbit` 只声明 `font-family`、未写 `@import`，疑似依赖系统已装字体）。核实后再改，本设计不硬性承诺。

## 9. 测试策略

- **校验器表驱动单测**：每条拒绝规则至少一个正例与一个反例，含转义、注释、嵌套等绕过尝试；6 个内置主题作为「必须通过」的黄金语料。
- **编译确定性**：同一输入两次编译 `content_hash` 一致。
- **SQLite 集成测试**：内置主题 upsert 幂等（连续两次启动不产生新版本行）；公开 CSS 与资产端点的 200/404/ETag/304。
- **契约测试**：`tests/contract/` 覆盖两个新公开端点与 `GET /api/v1/themes` 扩展字段。
- **快照锁版本回归**：发布 → 变更主题当前版本 → 已发布快照仍指向旧 `themeVersionId`。
- **E2E**：公开页加载后 `<link>` 存在、`data-theme` 正确、切换主题后旧 link 被移除。
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
