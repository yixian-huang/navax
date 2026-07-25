# 主题机制收尾(缺口 1-3)设计

日期:2026-07-24
状态:已与用户确认方向,待实现
范围:主题规范 v1(子项目 A)落地后遗留的三个缺口,加两处文档矛盾与死代码清理。子项目 B(第三方导入)切为 B1(导入与私有安装)→ B2(分发治理)两期串行,各有独立 spec,不在本文。
设计依据:`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`

## 1. 背景

子项目 A 已交付服务端校验/编译/版本化管线、隔离边界、发布锁版本与默认主题自愈,但有三处收尾没有跟上:

1. **管理后台主题列表不读 manifest**:`internal/admin/sqlstore.go` 的 `ListThemes`/`Theme` 仍只查 `themes` 表旧列,`adminpkg.Theme` 的 v1 字段(`CurrentVersionID`/`CSSHref`/`Tier`/`Scope`/`Vibe`/`Swatches`)恒为零值,`httpapi` 序列化时全部省略。后果:后台主题卡色板全是占位灰、`vibe` 缺省导致「Kawaii 可爱」分组永远为空(sakura 被错分到 Classic)、mock(`mock-handlers.ts` 返回完整字段)与真实响应已经漂移。
2. **草稿预览没有主题**:`internal/navigation/sqlstore.go` 的 `Preview` 不解析 `themeVersionId`,`/app/preview` 渲染的是一个无主题的简化投影——用户预览看到的与发布后不一致。
3. **契约漏登记**:`PublishedPage.themeVersionId` 前后端都在用(Go 侧 `omitempty` 序列化、前端消费),但 `api/openapi.yaml` 的 `PublishedPage` schema 没有该字段,违反「openapi 是契约唯一来源」;公开主题端点(`GET /api/v1/themes`、CSS/资产供应)没有任何契约测试。

## 2. 缺口 1:管理后台字段接线

**方案**:`ListThemes`/`Theme` 的 SQL 改为

```sql
SELECT themes.…, themes.current_version_id, themes.scope, theme_versions.manifest_json
FROM themes
LEFT JOIN theme_versions ON theme_versions.id = themes.current_version_id
```

- 用 **LEFT JOIN** 而不是 eligibility 谓词:管理后台的职责是全量只读视图,必须保留停用的、没有编译版本的行(culled 主题等)——这正是设计文档 §8.1 规定「管理员目录不复用 eligible」的原因。
- `manifest_json` 可空扫描;有版本时解析出 `Subtitle`/`Tier`/`Vibe`/`Swatches` 并拼 `CSSHref`(`/api/v1/public/themes/{versionId}.css`),无版本时这些字段保持零值,`httpapi` 继续省略——`Theme` schema 中它们本就是可选字段。
- 解析逻辑与 `internal/catalog/service.go:114`(`Themes`)完全同构;`current_version_id` 指向本主题 active 版本由数据库触发器保证,无需在 SQL 里重复防御。
- `Theme()` 单行查询(PATCH 响应用)同样处理。

**测试**:admin 包 SQLite 集成测试,`SyncBuiltin` 后断言:sakura 返回 `vibe = cute` 且三个 swatch 为真实 hex(非零值);slate 的 `CSSHref` 前缀正确;一个被停用且无版本的主题行仍出现在列表且 v1 字段为零值。修完后真实响应与 mock 形状对齐,`make test-mock` 维持通过。

## 3. 缺口 2:草稿预览所见即所得

### 3.1 服务端

`SQLStore.Preview` 在构建 `PublishedPage` 后,用已注入的 `s.resolveThemeVersion` 在 `s.db` 上(非事务,`Queryer` 接口本就支持)解析 `page.Settings.Appearance.ThemeID` → `ThemeVersionID`,写入返回值。语义与 `Publish` 完全一致:

- resolver 未接线 → 响亮报错(不静默返回无主题预览);
- 主题不可用 → 回落默认主题版本;
- 默认主题自己不可用 → `ErrDefaultThemeUnavailable`,HTTP 层与发布同样处理。

预览是只读操作,不写快照,因此不需要事务内解析(发布必须在事务内是为了防「解析与写快照之间版本被撤销」,预览没有这个窗口问题)。

### 3.2 前端(用户已确认:复用公开页渲染)

`/app/preview` 弃用现有简化投影,改为渲染 `PublicNavigationView`(它接受的 `page` 类型正是预览端点返回的 `PublishedPage` 形状),主题、壁纸、布局模板全部所见即所得:

- 顶部保留现有预览工具栏(返回发布页、主/次发布按钮、打开线上版),工具栏下方全宽渲染公开页组件;
- `PublicNavigationView` 新增可选 prop 关闭点击事件追踪(预览态不得产生公开页统计事件,`snapshotId` 为 `preview_*` 的请求也不该发出);
- `share` 传 `null`;空草稿沿用组件的空态;
- 现有 `PreviewCategorySection`/`PreviewSiteCard` 随之删除;
- 预览容器结构与公开页一致(`[data-nx-frame]` wrapper + 主题根),隔离边界语义不变;
- 若 `/app` 布局约束了内容宽度,预览路由做全宽处理(实现时按现状选择最小改动)。

**验收**:浏览器冒烟覆盖加载/空/错误/移动/键盘/暗色主题六态(CLAUDE.md 要求);切换主题保存后进预览,能看到与公开页一致的主题样式。

## 4. 缺口 3:契约补登记

- `api/openapi.yaml` 的 `PublishedPage` 增加**可选**字段 `themeVersionId: { $ref: '#/components/schemas/ThemeVersionId' }`,描述注明:发布(或预览)时锁定的主题版本;迁移前的旧快照没有该字段,与 Go 侧 `omitempty` 一致——因此**不加入 required**。
- `tests/contract/` 扩展既有流程:
  - 发布后的公开读步骤断言 `themeVersionId` 存在且匹配 `^v[0-9a-f]{32}$`;
  - 新增步骤:`GET /api/v1/public/themes/{versionId}.css` → 200(`text/css` + `ETag` + immutable `Cache-Control`);携带 `If-None-Match` 重取 → 304;未知版本 → 404;未知资产路径 → 404;
  - `GET /api/v1/themes` → 200 且响应过 schema 校验;
  - 预览端点响应含 `themeVersionId`(若流程尚无预览步骤则新增)。
  - 410 分支不在契约测试造(需要手工撤销版本,httpapi 单测已覆盖)。
- mock:`mock-handlers.ts` 的发布/预览响应补 `themeVersionId`(固定假哈希,满足模式),`make test-mock` 通过。
- E2E 顺带收紧:`guest.spec.ts` 中内容寻址 `<link>` 断言目前包在 `if (await link.count())` 里会静默空过;发布路径自此必然有 `themeVersionId`,改为无条件断言。

## 5. 顺带清理(用户已确认)

**文档/注释矛盾**:

- `docs/theme-api.md` §5 拒绝列表仍写「`data:image/svg+xml` 拒绝」,与 §3/§4.1 的放宽规则(CSS 内 ≤8KB 经净化允许;`.svg` 资产文件仍拒绝)直接矛盾——修正 §5;
- `internal/themes/builtin/sakura/theme.css` 头部注释仍称 `content: "✿"` 被规则拒绝,与同文件实际使用矛盾——修正;
- `internal/themes/store.go`、`internal/themes/tokens.go` 中指向已删除前端文件(`themeResolve.ts`、`packages/slate.ts`)的过期注释——修正;`migrations/0008` 的过期注释**不动**(迁移 append-only)。

**兑现文档声明**:`docs/theme-api.md` 第 6 行声称钩子清单与 `hooks.go` 的一致性「有测试保证」,该测试不存在。补一个 Go 测试:解析 `docs/theme-api.md` 的钩子表格,与 `AllowedHooks()` 精确比对。这同时回答了 `AllowedHooks()` 疑似死代码的问题——它从此有生产级消费者。

**死代码删除**:

- `web/src/components/base/ThemePicker.tsx`(全仓无引用);
- `web/src/api/types.ts` 的 `ThemeManifest` 接口(manifest.ts 删除后的残留);
- `internal/themes/store.go` 的 `ResolvePackageVersion` 及其专属测试(生产路径已全部走 `ResolveEligibleVersion`,两套回落逻辑并存是漂移隐患);
- `internal/themes/tokens.go` 的 `var _ = sort.Strings` 残留;
- `internal/themes/csscompile.go` 疑似重言条件(实现时核实确属恒真再删,存疑则保留)。

## 6. 不在范围

- B1/B2(独立 spec);
- 对抗性 fixture 的 negative control(报告缺口 4);
- `themes` 表行元数据与 manifest 的漂移回写(缺口 6,B1 导入管线落地时一并设计);
- `web/src/index.css` 暗色主题硬编码列表(缺口 7,正确解法是宿主按 manifest `mode` 下发属性,牵涉数据流,单独做);
- culled 主题孤儿行清理(缺口 8)、被撤销版本快照的自愈(缺口 11)。

## 7. 交付与验收

- 分支 `fix/theme-followups`(自最新 `main`),单 PR,含本 spec 与实施计划文档;
- 门槛:`make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、E2E 相关 spec 通过;UI 变更浏览器六态冒烟;
- 按 CLAUDE.md shipping path:PR → CI 绿 → auto-merge。
