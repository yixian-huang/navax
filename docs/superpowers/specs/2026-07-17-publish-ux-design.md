# 发布体验优化（方案 A）设计说明

状态：`proposed`  
日期：2026-07-17  
范围：工作台发布状态可见性 + 发布页主操作清晰度（不改变后端草稿/快照模型）

## 1. 问题

用户在更新主题、链接、背景图、布局、首页组件等内容后：

1. **改完不知道有没有发布**——保存成功提示像「已上线」，实际上只写入草稿。
2. **到发布页不知道该点哪里**——发布开关、发布设置、域名/CNAME 混在一页；已发布有草稿时开关仍为 ON，没有「发布更新」主按钮。

后端「草稿 → 不可变快照」模型保持不变；本设计只优化前端状态表达与信息架构。

## 2. 目标与非目标

### 目标

- 任意编辑路径结束后，用户能立刻判断：草稿是否已上线。
- 发布页存在**唯一主操作**，文案与状态机一致。
- 概览/预览不再把「线上公开页」伪装成「当前草稿」。

### 非目标（明确不做）

- 保存即自动发布（方案 C）。
- 各编辑页处处独立实现发布业务逻辑副本（方案 B 的分散实现）；编辑页通过**共享 hook + 统一提示条**调用同一发布入口。
- 修改 OpenAPI 发布语义、snapshot 事务、revision 乐观锁。
- 改域名申请/审核/CNAME 业务逻辑。
- 让 `ReplacePublication`（可见性/slug/SEO）自动写新快照；仍通过文案要求「保存设置后如需上线请再点发布更新」。

## 3. 状态模型

所有 UI 从 `page.publication` 派生同一三态（`PublishUiState`）：

| 状态 ID | 条件 | 用户可见短文案 |
|---------|------|----------------|
| `never_published` | `!publication.published`（含从未发布与已取消发布） | 未发布 |
| `published_with_draft` | `publication.published && publication.hasUnpublishedChanges` | 有草稿未上线 |
| `published_current` | `publication.published && !publication.hasUnpublishedChanges` | 已是最新 |

派生字段（供组件使用）：

- `primaryAction`: `publish` | `publish_update` | `none`
- `primaryLabel`: 「发布」|「发布更新」|「已是最新」
- `primaryDisabled`: 仅 `published_current` 为 true
- `showUnpublish`: 仅当 `publication.published`（含有草稿时）
- `draftUpdatedAt` / `publishedAt`：用于副文案

实现建议：`web/src/lib/publish-state.ts`（纯函数 + 单测）+ 可选 `usePublishUiState()` 包装 `useMyPage`。

## 4. 全局顶栏（AppShell）

替换当前固定链接「发布 & 域名」为**状态感知控件**（sm 及以上完整展示；移动端可收成图标+点状指示）。

### 展示

| 状态 | 状态文案（次要色） | 主按钮 |
|------|-------------------|--------|
| `never_published` | 未发布 | 发布 |
| `published_with_draft` | 有草稿未上线 | 发布更新 |
| `published_current` | 已是最新 | 无主按钮；次要链接「发布设置」 |

主按钮旁保留次要链接「发布设置」→ `/app/publish?scope=…`。

### 行为

- 主按钮调用现有 `usePublish()`（`expectedRevision = draftRevision`）。
- 成功：toast「发布成功」或「更新已发布」；invalidate `navigation/page`。
- 失败：toast 错误信息；若为可见性 private 等业务错误，导航至发布页并带 query（如 `?highlight=visibility`）以便高亮可见性区块。
- 加载中禁用主按钮并显示 loading。

## 5. 发布页（`/app/publish`）

### 信息架构（自上而下）

1. **发布内容（主卡片）**
   - 大状态标题（三态之一）
   - 副文案：草稿更新时间 · 上次发布时间（若有）
   - `published_with_draft` 时强调：「当前访客仍看到线上版」
   - **唯一实心主色按钮**（规则同 §3）
   - 次要操作：草稿预览 · 打开线上版（已发布时）· 取消发布（已发布时，危险样式 + 确认）
2. **页面与访问（次要卡片）**
   - 可见性、slug、SEO 标题/描述、展示作者
   - 「保存设置」→ `useUpdatePublication`
   - 固定说明：以上设置保存后，若页面已发布，需再点「发布更新」才会进入公开快照（含 slug/SEO/分享相关字段）
3. **域名（次要卡片）**
   - 子域名申请/状态、CNAME：保持现有逻辑与 API

### 去掉

- 将「发布/取消发布」做成大开关的交互（易与「有草稿未上线」冲突）。
- 模糊小贴士「更改导航内容后记得重新发布」；改为状态驱动文案。

### 页面标题

- 主标题：发布 & 域名（可保留）
- 副标题：先确认内容上线，再管理访问方式与域名

## 6. 编辑页保存反馈与页内提示条（P0）

覆盖路径：导航编辑（链接/布局）、主题/背景、首页组件（widgets）、导入成功写草稿后。

### Toast

| 场景 | 文案 |
|------|------|
| 尚未发布时保存成功 | 已保存到草稿 |
| 已发布且产生草稿差 | 已保存到草稿 · 访客仍看线上版 |
| 失败 | 保持现有错误文案 |

主题切换等可带名称：「主题已写入草稿：「{name}」」。

### 页内提示条（P0）

条件：`published_with_draft`。

位置：各相关编辑页标题下方（及草稿预览页顶部条，见 §7）。

内容：

> 你有未上线的草稿 · [发布更新] [草稿预览]

- [发布更新]：同一 `usePublish`
- [草稿预览]：`/app/preview?scope=…`
- 可关闭：sessionStorage 键建议 `navax:publish-banner-dismissed:{scope}:{pageId}`；关闭后同会话同页不再显示；成功发布后自动消失（状态不再满足条件）

共享组件建议：`PublishDraftBanner`（读 `usePublishUiState` + `usePublish`）。

## 7. 概览与预览

### 概览（`/app`）

| 项 | 行为 |
|----|------|
| 原「预览」 | 改为「草稿预览」→ `/app/preview?scope=…` |
| 线上入口 | 仅 `publication.published` 时显示「线上版」→ `/u/{slug}` 或规范公开 URL |
| 发布状态统计 | 三态文案，不用简单「已发布/未发布」掩盖草稿差 |
| 未发布更改区 | 主 CTA「发布更新」（直接 publish），旁链「发布设置」 |
| 快捷流程 step0 | 文案「发布与访问」；`published_with_draft` 时卡片高亮 |

### 草稿预览（`/app/preview`）

顶部固定条：

- 文案：草稿预览 · 非公开
- 主按钮：发布 / 发布更新（同状态机）
- 次要：打开线上版（若已发布）

## 8. 组件与代码落点

| 项 | 路径（建议） |
|----|----------------|
| 状态派生 | `web/src/lib/publish-state.ts` + 单测 |
| Hook | `web/src/hooks/usePublishUiState.ts` 或并入 `useQueries.ts` |
| 顶栏控件 | `web/src/components/feature/AppShell.tsx` 或抽出 `PublishStatusControl` |
| 草稿提示条 | `web/src/components/base/PublishDraftBanner.tsx`（或 `feature/`） |
| 发布页 | `web/src/pages/app/publish/page.tsx` 重写主卡片结构 |
| 概览 | `web/src/pages/app/overview/page.tsx` |
| 预览 | `web/src/pages/app/preview/page.tsx` |
| 保存 toast | `themes` / `widgets` / `links` / `import-export` 等 mutation 成功回调 |
| 已有徽章 | `PublishStatusBadge` 与三态对齐（文案统一） |

后端与契约：**无必须变更**。若 E2E 断言旧文案/开关，则更新 `tests/e2e`。

## 9. 错误与边界

- `visibility === private` 时后端拒绝发布。统一前端行为：
  - **发布页**：主按钮 disabled，其下展示说明「请先将可见性改为「知道链接即可访问」或「公开展示」并保存设置」。
  - **顶栏 / 提示条 / 概览**：主按钮仍可点；若当前已知为 private，则导航至 `/app/publish?scope=…&highlight=visibility` 且不调用 publish；若状态过期导致仍发起请求失败，则 toast 错误并同样导航高亮。
- `expectedRevision` 冲突（409/precondition）：toast「内容已变更，请刷新后重试」，并 `refetch` page。
- 取消发布：确认对话框「取消后公开链接将不可访问；草稿保留」；成功后状态回到 `never_published`。
- system / personal scope：所有链接与 queryKey 继续带 `scope`。

## 10. 测试计划

- 单元：`publish-state` 三态表驱动。
- 前端：`make check`；相关 mock 若暴露 `hasUnpublishedChanges` 保持一致。
- 手动/冒烟：未发布→编辑→顶栏显示未发布→发布；已发布→改主题→顶栏与页内条→发布更新→条消失；概览草稿预览 vs 线上版；取消发布确认。
- 若有 E2E 覆盖发布流：更新选择器与文案断言。

## 11. 分阶段交付（仍属本设计同一 PR 范围时可一次做完）

P0（本设计默认全部包含）：

1. `PublishUiState` + 单测  
2. 顶栏状态控件  
3. 发布页主卡片重做  
4. 编辑页 toast 文案  
5. `PublishDraftBanner` 接入编辑页  
6. 概览 + 草稿预览条  

无单独 P1 阻塞项（页内条已定为 P0）。

## 12. 成功标准

- 改主题/链接后：用户在**当前页或顶栏**能判断草稿是否已上线。
- 进入发布页：唯一主按钮文案与状态一致，无需猜开关。
- 概览「预览」不再误导向线上公开版。
- 取消发布为显式次要危险操作，不再伪装成开关。
