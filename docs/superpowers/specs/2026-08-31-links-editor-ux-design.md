# 导航编辑页：分组列表当主力

日期：2026-08-31
状态：已与用户确认方向，待实现
范围：`/app/links` 编辑页交互。布局设置让出列表高度；分组列表成为默认工作面；右键管动作，拖拽管顺序；预览站点可拖进左侧分组。
依据：现行实现 `web/src/pages/app/links/page.tsx`、`web/src/components/feature/DnDPreview.tsx`、`web/src/components/base/ContextMenu.tsx`。

## 1. 目标

链接管理是高频操作，布局旋钮是低频操作。分组列表要成为打开页面后的主力面：能看全部分组、用右键编辑/隐藏/删除、用拖拽改顺序，并能把右侧预览里的站点拖进左侧某个分组。布局设置改到对话框，不再钉在左侧底栏。

公开页渲染、主题夹取、composition API、表格批量能力保持原样。

## 2. 非目标

- 不改表格视图的行交互（不加右键、不当投放区）。
- 不在「文件夹」分类样式的预览里启用拖拽（预览继续只读；排序与跨组移动只在左侧做）。
- 不记住「上次用的是分组还是表格」（不写 localStorage）。
- 不做多选拖拽、撤销栈、右键里的上移/下移/移到某分类。
- 不新增 API、不改 `savePageComposition` 契约。
- 不把 `/app/links/page.tsx` 整页重写成新架构；只抽这次需要的对话框和分组列表。

## 3. 已确认决策

| 项 | 选择 |
|---|---|
| 主力视图 | 分组列表。默认 `viewMode = 'card'`。表格保留给批量勾选 / 上架 / 链接检测。 |
| 布局入口 | 工具栏「布局」按钮 → 模态对话框。不是气泡，不是侧栏。 |
| 职责拆分 | 右键管动作，拖拽管顺序。 |
| 右键对象 | 站点与分类都有。 |
| 悬停按钮 | 保留，作为发现性提示。 |
| 拖拽范围 | 左右共用一个 `DndContext`。组内拖站点、分类手柄排序、预览内拖、预览 → 左侧分组。 |
| 保存 | 布局与顺序仍走现有防抖 `persistLayout` / `savePageComposition`。 |

## 4. 交互

### 4.1 默认与工具栏

打开 `/app/links` 即为分组列表。视图切换控件保留。

左侧底栏「布局设置」整块删除，列表吃满剩余高度。

链接管理工具栏增加「布局」按钮，`aria-haspopup="dialog"`。按钮旁用当前值做摘要，格式 `密度标签 · N列`，例如 `舒适 · 4列`。密度文案沿用现有 `densityLabels`（列表 / 紧凑 / 舒适）。

搜索、新建分类、添加站点、链接检测位置不变。

### 4.2 布局对话框

视觉对齐 `AddCategoryDialog`：遮罩 `bg-black/30`、居中卡片、圆角、关闭按钮。

内容即今日底栏四个旋钮，行为不变：

- 导航页布局（`HOME_LAYOUTS`，主题 `template.allowed` / `locked`）
- 密度（`list` / `compact` / `comfortable`，主题 `density`）
- 分类样式（标签 / 侧栏 / 网格 / 文件夹，主题 `categoryStyle`）
- 列数滑杆（主题 `columns.min/max/locked`）

改动仍调用现有 `setHomeLayout` / `setDensity` / `setCategoryStyle` / `setColumns`，预览在对话框后方即时更新，自动写入草稿。关闭对话框不丢改动、不另提交。保存状态继续用预览顶栏芯片和「立即保存」；对话框内不复制一套状态文案。

Escape 关闭对话框。打开对话框期间不开始新的拖拽。

### 4.3 右键

只在站点行、分类行上拦截 `contextmenu`。空白处、工具栏、预览浏览器壳不拦截。

站点菜单（顺序固定）：

1. 编辑站点 → 现有 `PropertiesPanel`
2. 隐藏站点 / 上架站点（按 `enabled` 二选一）→ 现有 `handleToggleSiteEnabled`
3. 删除站点（destructive）→ 现有 `ConfirmDialog`

分类菜单（顺序固定）：

1. 编辑分类 → 现有 `PropertiesPanel`
2. 添加站点 → 现有 `AddSiteDialog`，`defaultCategoryId` 为该分类
3. 隐藏分类 / 显示分类 → 现有 `handleToggleCategoryEnabled`
4. 删除分类（destructive）→ 现有确认框，文案仍说明其下站点一并删除

实现复用 `useContextMenu`。站点可扩 `createSiteContextActions`（补隐藏/上架）；分类新增 `createCategoryContextActions`，不要另起菜单组件。

悬停露出的编辑 / 删除 / 隐藏按钮保留。右键与按钮走同一套 handler。

### 4.4 拖拽手感

- 分类行：只有手柄可拖；点击行仍展开/收起。勾选框点击不触发拖拽。
- 站点行：整行可拖，但 `button` / `a` / `input` / `[data-no-dnd]` 上按下不起拖。右键、悬停按钮、勾选加 `data-no-dnd`。
- PointerSensor `activationConstraint.distance` 保持 6。
- 拖拽中用 `DragOverlay` 跟手，避免左右各留一个半透明幽灵。Overlay 按 `type` 分别画紧凑站点行或分类行，不复用预览大卡片。

## 5. 组件与文件

`page.tsx` 继续做编排：查询、`localPage`、保存、对话框开关、拖拽 handler。新增文件只服务本页，不上升为通用设计系统。

| 文件 | 职责 |
|---|---|
| `web/src/pages/app/links/components/LayoutSettingsDialog.tsx` | 布局对话框。纯 UI + 回调，不自己打 API。 |
| `web/src/pages/app/links/components/GroupedCategoryList.tsx` | 左侧分组列表：展开、勾选、右键、分类手柄、站点行、投放高亮。 |
| `web/src/pages/app/links/dndIds.ts` | 管理面 ID 前缀的编码/解码，以及 `applySiteDrop`。供 handler 与列表共用，单测覆盖。 |
| `web/src/components/base/ContextMenu.tsx` | 补隐藏/上架与分类动作工厂。 |
| `web/src/pages/app/links/page.tsx` | 默认视图、拆底栏、`DndContext` 提到左右外层、handler 改读 `data.current` + 前缀。 |
| `web/src/components/feature/DnDPreview.tsx` | 预览 sortable 的 `data.current` 补 `surface: 'preview'`。 |

`LayoutSettingsDialog` 的 props 只收当前 `layout`、可选 `themeLayout`、以及四个 setter 和 `open`/`onClose`。不要把整个 `NavigationPage` 灌进去。

`GroupedCategoryList` 不直接调 mutation。删除/编辑/隐藏/添加通过回调回到 `page.tsx` 已有函数。

## 6. 拖拽数据流

### 6.1 为什么要分面 ID

同一站点同时渲染在左侧列表和右侧预览。一个 `DndContext` 里两个节点不能共用 `site.id`。预览保持现有 `site.id` / `category.id`，少改现有预览逻辑。左侧全部加前缀。

### 6.2 ID 约定

前缀：

- 管理面站点：`manage-site-{siteId}`
- 管理面分类（sortable 手柄）：`manage-cat-{categoryId}`
- 管理面投放点（分类标题，含收起态）：`manage-drop-{categoryId}`

`dndIds.ts` 导出 `manageSiteId` / `manageCatId` / `manageDropId` / `parseDndId`。`parseDndId` 对无前缀 ID 视为预览面（`surface: 'preview'`），站点与分类靠 `data.current.type` 区分；没有 `data` 时，用页面数据 `findCategoryId` 回退，与今日预览行为一致。

每个 sortable / droppable 的 `data.current` 必须带：

```
{ type: 'site' | 'category' | 'drop', surface: 'manage' | 'preview', siteId?: string, categoryId: string }
```

`siteId` / `categoryId` 永远是裸实体 ID，不含前缀。handler **禁止**把带前缀的 `active.id` 直接丢给今日的 `findCategoryId(page, active.id)`。

### 6.3 移动语义（搬家，不是复制）

`handleDragOver` / `handleDragEnd` 在剥前缀后，继续今日规则：

- `type === 'category'`：只在 `dragEnd` 用 `arrayMove` 调整分类顺序。管理面 `manage-cat-*` 与预览 `category.id` 映射到同一组 `page.categories`。
- `type === 'site'`：跨组在 `dragOver` 中搬进目标列表；同组顺序在 `dragEnd` 用 `arrayMove` 落定。
- `setLocalPage` + `markLayoutDirty` + 防抖 `persistLayout` 不变。保存中再拖，沿用 `layoutSavePendingRef` 排队，不丢最后一次顺序。

从预览拖到左侧、左侧拖到预览、预览组间、左侧组间，都是同一套搬家。

### 6.4 投放规则

解析 `over` 得到目标分类 `overCatId` 与可选插入锚点（另一站点）：

| over | 结果 |
|---|---|
| `manage-drop-{catId}` 且该组收起 | 站点追加到该组末尾，并 `setExpandedCat(catId)` |
| `manage-drop-{catId}` 且该组已展开、指针未落在站点行上 | 追加到末尾 |
| `manage-site-{id}` 或预览站点 id | 插入到该站点所在位置（其所属分类即目标组） |
| 预览分类块 / 分类 id | 与今日预览一致：进入该组，插入位置按指针下的站点或末尾 |
| 空白、工具栏、对话框、无 `over` | 不改数据、不保存 |
| 站点拖到**自己当前所在分组**的标题 | 当作没换组；不改 `categoryId` |

搜索过滤时：只允许投放到**当前可见**的分组；被滤掉的站点不能当插入锚点。`over` 若指向不可见实体，视为无效，不改数据。

表格模式（`viewMode === 'table'`）：左侧不是投放区，不挂 manage sortable。预览内拖拽照旧。`DndContext` 仍包住两栏，避免按模式反复挂载传感器。

`categoryStyle === 'folders'`：右侧预览保持只读（不进 `SortableContext`），因此没有「从预览拖进左侧」。左侧组内拖站点与分类手柄排序仍可用。

### 6.5 碰撞与高亮

碰撞检测保持 `pointerWithin` 优先，否则 `closestCenter`。

`overCategoryId` 继续用来高亮目标分组。管理面分组标题在 `isOver` 时使用与预览分类块相同的主色 ring，避免两套视觉语言。

### 6.6 传感器

`PointerSensor` 增加过滤：事件目标（或祖先）匹配 `button, a, input, textarea, [data-no-dnd]` 时不激活。`KeyboardSensor` 保留，给无指针用户排序。

布局对话框打开（`open === true`）时，不开始新拖拽：可对 `DndContext` 传空 sensors，或在 `onDragStart` 里立即 `cancel`。已在进行的拖拽在打开对话框前应先结束（对话框由工具栏按钮打开，正常不会与拖拽重叠）。

## 7. 失败与边界

- 保存失败：预览顶栏红字 + 「立即保存」；`localPage` 保留用户刚排出的顺序，不回滚。
- 删除仍经确认框。分类删除文案不改。
- 主题锁定的旋钮在对话框里 `disabled`，不能从对话框绕过夹取。
- Escape：先关右键菜单，再关对话框；不离开 `/app/links`。
- 触控：6px 位移才起拖，避免与点按冲突。Mac 触控板双指点按走右键（浏览器 `contextmenu`）。
- 左侧未展开过时，仍按今日逻辑默认展开第一个分类。

## 8. 测试

不新增契约测试（无 API 变化）。`make test-mock` 若未改 mock 形状则保持通过。

### 8.1 单测 `web/src/pages/app/links/dndIds.ts`

- 编码/解码 `manage-site` / `manage-cat` / `manage-drop` 往返。
- 无前缀 ID 解析为 `preview`。
- 畸形字符串不抛异常，返回无法识别。

投放逻辑抽成 `applySiteDrop(page, active, over) → nextPage`（与 `dndIds.ts` 同目录），`handleDragOver` / `handleDragEnd` 只调用它。单测覆盖：

- 跨组 + 收起投放点 → 追加到目标末尾。
- 跨组 + 锚在目标组某站点 → 插到该索引。
- 同组两站点 → `arrayMove`。
- 拖到自己所在组的 drop 标题 → 引用相等或深比较相等，不改 `categoryId`。
- `over` 为空或不可见实体 → 原样返回。

### 8.2 组件测

- `LayoutSettingsDialog`：改密度会调用传入的 `onDensityChange`；`locked` 时按钮 `disabled`。
- `GroupedCategoryList`：站点右键「编辑站点」触发 `onEditSite`；分类右键「删除分类」触发 `onDeleteCategory`。不必在 jsdom 里完整模拟 dnd-kit 拖拽。

### 8.3 E2E

若现有 `tests/e2e` 已覆盖 `/app/links` 登录态，补一条尽力而为：分组视图下把预览站点拖到另一分组，随后草稿 composition 的 `siteIds` 变化。Playwright 对 dnd-kit 指针拖拽不稳定时，允许改测「打开布局对话框并改密度后草稿 layout.density 变化」+ 右键删除确认框出现。不为此单独引入拖拽测试库。

## 9. 实现顺序

1. `dndIds.ts` + 投放纯函数与单测。
2. `LayoutSettingsDialog` 接入，删除底栏，默认 `card`。
3. `GroupedCategoryList` + 右键动作（此时拖拽仍仅预览，页面已能用）。
4. 提升 `DndContext`、左侧 sortable/droppable、`DragOverlay`、handler 读前缀。
5. 补组件测；能跑的 e2e 补上。浏览器验收：分组默认、布局对话框、右键、组内拖、预览拖进左侧分组、主题锁定旋钮、表格模式预览仍可拖。

## 10. 验收

- 打开编辑页，左侧是分组列表，底栏没有布局旋钮，列表高度明显高于改前。
- 「布局」打开对话框，改密度/列数，预览立刻变，草稿自动保存。
- 站点/分类右键动作可用；悬停按钮仍可用。
- 左侧组内拖、分类手柄拖、预览拖进另一分组，保存后刷新顺序仍在。
- 表格模式仍能批量选择；文件夹预览仍只读。
- 大目录（单组 >24 站点）拖拽不明显卡顿：列表项 `memo`，`animateLayoutChanges` 继续返回 `false`。
