# Design: 文件夹分类视图 · 主题精简 · 卡片中度收敛

状态：`accepted`  
日期：2026-07-20  
范围：公开导航首页 / 预览、布局设置、主题包、SiteCard/SiteGrid 视觉密度  

## 1. 背景与目标

公开导航在「站点不多」时仍显得卡片臃肿；主题数量多但辨识度重叠；希望分类支持类似手机桌面文件夹的聚合缩略，悬停后展开站点图标再跳转。

本设计将一次落地三件事：

1. 新增 `categoryStyle: folders`（文件夹磁贴 + 悬停/点按浮层）。
2. 内置主题从 12 精简为 6，并做旧 id 映射。
3. 全主题共用组件层做「中度」卡片收敛（约 −30% 留白/边框）。

## 2. 决策摘要（已确认）

| 项 | 选择 |
| --- | --- |
| 聚合交互 | 文件夹磁贴 + 悬停气泡；触控为点按打开浮层 |
| 配置入口 | `PageSettings.layout.categoryStyle = 'folders'`，与 `tabs` / `sidebar` / `grid` 并列 |
| 主题保留 | Slate、Slate Dark、Sakura、Noir、Orbit、Terminal |
| 主题移除 | Kyoto、Terracotta、Mochi、Pastel Sky、Mono、Cyber |
| 卡片密度 | 中度：图标 + 标题为主，描述进 tooltip |

## 3. 文件夹视图

### 3.1 数据与契约

- OpenAPI `PageSettings.layout.categoryStyle` 枚举扩展为：`tabs | sidebar | grid | folders`。
- 前端 `web/src/api/types.ts`、mock handlers、Go DTO（若有枚举校验）同步。
- 工作台布局设置 UI 增加「文件夹」选项（现有 categoryStyle 控件扩展）。
- 发布快照沿用现有 settings 序列化；无迁移 SQL（JSON 内枚举扩展即可）。

### 3.2 呈现规则

**默认态（无搜索 query）**

- 主内容区渲染「分类文件夹墙」，不渲染 CategoryTabs + 扁平 SiteGrid。
- 每个分类一个文件夹磁贴：
  - 最多 4 个站点图标 2×2 缩略（取分类内排序前 4 个启用站点）。
  - 分类名称；可选数量角标。
  - 空分类：空文件夹样式，悬停/点按不展示站点或展示空态文案。
- 仅 `enabled` 且发布投影规则下会出现的站点进入缩略与浮层（与公开载荷一致）。

**悬停（pointer fine / 桌面）**

- 磁贴悬停打开浮层（popover），展示该分类全部站点图标网格。
- 点击站点图标：与现有 `onSiteOpen` 一致（新窗口/统计）。
- 浮层定位：优先磁贴下方或旁侧，视口避让；z-index 高于导航内容。
- 离开磁贴与浮层后关闭（小延迟防误关）；`Escape` 关闭。

**触控（无 hover）**

- 点按文件夹：打开/切换浮层。
- 点遮罩或再次点同一文件夹：关闭。
- 浮层内图标最小触控目标约 40–44px。

**搜索**

- 存在非空 `query` 时：不走文件夹墙，改扁平 `SiteGrid` 展示匹配站点（现有语义过滤逻辑复用）。
- 清空 query 后回到文件夹墙。

**与密度的关系**

- `categoryStyle === 'folders'` 时隐藏首页 `DensitySwitcher`（密度对文件夹墙无意义）。
- 切回 `tabs` / `sidebar` / `grid` 后恢复密度切换。
- 浮层内站点呈现固定为紧凑图标格，不读 `layout.density`。

### 3.3 组件边界

- 新增例如 `CategoryFolderWall` + `CategoryFolderTile` + `FolderSitesPopover`（命名以实现为准）。
- 接入点：`SitesSection` / 各 `Layout*` / `PublicNavigationView` 在渲染站点区时分支：
  - `folders && !query` → FolderWall
  - 否则 → 现有 CategoryTabs（按布局）+ SiteGrid
- 预览页（工作台 DnD 预览）至少只读展示文件夹墙；文件夹内拖拽排序 **不做**。

### 3.4 无障碍

- 文件夹磁贴：`button` 或等效可聚焦控件，`aria-expanded`、`aria-haspopup`。
- 浮层：`role="dialog"` 或 `role="menu"`（实现选一种并保持一致），焦点可进图标链接。
- 键盘：Enter/Space 开合；Escape 关闭；Tab 在浮层内循环或按常规对话框模式。

## 4. 主题精简

### 4.1 保留与移除

**保留（前端 `themeRegistry` 注册）**

- `slate`、`slate-dark`、`sakura`、`noir`、`orbit`、`terminal`

**移除包文件与注册**

- `kyoto`、`terracotta`、`mochi`、`pastelsky`、`mono`、`cyber`

### 4.2 旧 themeId 映射

读取 `appearance.themeId`（或公开快照 themeId）时，若 id 不在注册表，按表解析后再 `activate`：

| 旧 id | 映射到 |
| --- | --- |
| `kyoto` | `slate` |
| `terracotta` | `slate` |
| `mono` | `slate` |
| `mochi` | `sakura` |
| `pastelsky` | `sakura` |
| `cyber` | `orbit` |

- 映射在前端激活路径与（如有）后端默认主题校验处一致。
- 不强制写回用户草稿；用户下次保存主题时可自然落到新 id。
- 管理端主题库 / 种子数据：停用或删除已移除 id，避免管理员再启用幽灵主题。

### 4.3 默认主题

- 实例默认仍为 `slate`（或现有系统默认）；若默认曾是被删 id，改为映射目标。

## 5. 站点卡片中度收敛

在 **组件层**（`SiteCard` / `SiteGrid` / 相关 CSS）统一收紧，主题包只保留色板与少量装饰，不各自复制卡片尺寸。

### 5.1 各 density

| density | 行为 |
| --- | --- |
| `comfortable` | 横排 icon ~28px + 标题；域名与描述默认不展示，写入 `title` tooltip |
| `compact` | 更小图标 + 标题单行截断；网格列数可略增、gap 减小 |
| `list` | 保留一行域名；描述仍 tooltip；行高与水平 padding 约 −20–30% |

### 5.2 视觉 token（目标）

- padding / gap 约相对现状 −30%。
- 圆角略减（例如 16→10–12）。
- 边框更淡或依赖 surface 对比，避免「厚框空内容」。
- 延续 wallpaper 模式：无整块大毛玻璃板包住网格。

### 5.3 文件夹浮层

- 图标格 + 下方单行标题；风格与中度紧凑一致。

## 6. 明确不做

- 文件夹原地放大展开、作为第 4 种 density。
- 文件夹内拖拽排序、跨文件夹拖拽。
- 第三方主题上传/执行。
- 支付/白标等无关范围。

## 7. 实现顺序建议

1. OpenAPI + 类型 + mock：`folders` 枚举。
2. `CategoryFolderWall` 与悬停/触控浮层；接入公开导航与预览。
3. 布局设置 UI 选项 + i18n/中文文案。
4. SiteCard/SiteGrid 中度收敛。
5. 主题包删除、注册表、映射、管理端/种子同步。
6. 测试：单元/契约（枚举）、E2E 或组件级交互；浏览器冒烟（加载/空/移动/键盘/暗色）。

## 8. 验收

- [ ] 布局选「文件夹」后，公开页与预览显示文件夹墙；桌面悬停展开、移动点按展开；点击图标跳转。
- [ ] 搜索时扁平结果；清空后回文件夹墙。
- [ ] `folders` 下无 DensitySwitcher；其它 categoryStyle 正常。
- [ ] 仅 6 主题可选；旧 themeId 不白屏。
- [ ] 列表/紧凑/舒适视觉明显更紧，信息以 tooltip 补全。
- [ ] `make check`、相关 Go/前端测试、UI 冒烟通过。

## 9. 相关代码锚点

- `web/src/api/types.ts` — `categoryStyle`、`Density`、`PageSettings`
- `api/openapi.yaml` — `PageSettings.layout.categoryStyle`
- `web/src/pages/home/components/SharedSections.tsx` — `SitesSection`
- `web/src/components/feature/PublicNavigationView.tsx`
- `web/src/components/base/SiteCard.tsx`、`SiteGrid.tsx`、`DensitySwitcher.tsx`
- `web/src/themes/packages/*`、`registry.ts`、`packages/index.ts`
- `web/src/pages/app/links/page.tsx` — 布局/密度设置
