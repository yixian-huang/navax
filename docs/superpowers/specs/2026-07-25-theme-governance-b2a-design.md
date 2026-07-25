# 主题治理必需(子项目 B2a)设计

日期:2026-07-25
状态:方向已与用户确认,spec 待审阅
范围:B1 落地后管理端治理的三项高价值小改动——版本级 kill switch 写入口、`imported_by` 溯源接线、admin 主题列表加 owner 列 / 按 scope 分组。子项目 B2 的其余部分(目录审核、更新检查、墓碑回收、starter 仓库)各自独立、不在本文。
设计依据:`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`(§8.1、§8.1.1、§10.B)、`docs/superpowers/specs/2026-07-24-theme-import-b1-design.md`(§9)

## 1. 背景与原则

B1 交付了第三方主题导入,但管理端治理留了三个缺口,读路径/判定地基都已就绪,只差写入口与增量字段:

1. **管理员无法停用一个坏版本**:`theme_versions.status` 的 410 读端点、eligibility 排除 disabled、回落默认全部就绪(B1),但**没有任何生产写路径**能把版本置 disabled——管理员目前只有整包 `enabled` 开关。
2. **导入溯源丢失**:`theme_versions.imported_by` 恒为 NULL,升级历史无操作者。
3. **admin 主题列表不可用**:全量视图混入所有用户的私有主题却不显示 owner、不按 scope 分组;更糟的是前端按 `vibe` 分组,私有主题若 manifest 无 `vibe` 字段会**既不落入任一组、根本不显示**。

**原则**:不新建子系统、不新增迁移(0014 的列/CHECK/触发器已够用);`internal/httpapi` 只做路由/DTO/序列化;沿用既有审计与错误码惯例。

## 2. imported_by 溯源接线

- `upsertVersionTx`(`internal/themes/store.go`)增加 `importerID string` 形参,写进 INSERT 的 `imported_by` 列(空串 → NULL,用 `sql.NullString` 或 `nullable()` 辅助)。
- `UpsertVersion` 公开包装函数透传空串——内置主题走 `SyncBuiltin`→`UpsertVersion`,无用户,`imported_by` 保持 NULL。
- `InstallPrivate` 的两处 `upsertVersionTx` 调用(新装、升级)传入 `ownerID` 作为 importer——私有主题的导入者即 owner。
- **测试**:导入(zip/github)后 `theme_versions.imported_by == 导入者 userID`;`SyncBuiltin` 后内置版本 `imported_by IS NULL`。

## 3. 版本级 kill switch

### 3.1 语义

- 扩 `PATCH /api/v1/admin/themes/{themeId}` 的请求体加可选 `status ∈ {active, disabled}`。
- 作用对象是**该主题的当前版本**(`themes.current_version_id`):`disabled` → `UPDATE theme_versions SET status='disabled' WHERE id = <current_version_id>`;`active` → 置回 `'active'`。
- 与 `enabled` **正交**:`enabled` 是整包在目录/选择器的可见性(也是私有卸载的墓碑机制,B1 已有);`status='disabled'` 是版本级撤销——公开页对锁定该版本的快照返回 410,eligibility 对该主题回落默认(§8.1.1)。停用版本不改 `enabled`,下架整包不改 `status`。
- **写入不被触发器阻碍**:`themes_current_version_valid` 只监听 `UPDATE OF current_version_id`,不监听 `theme_versions.status`。停用当前版本后,eligibility 因 status≠active 自动回落,kill switch 语义天然成立(B1 调研已核实)。
- kill switch **不依赖缓存清除**(§8.1.1):内容寻址 URL 的 immutable 缓存只对字节成立,撤销靠"页面不再引用它";文档不承诺"即时全局撤销"。

### 3.2 守卫

- **默认主题守卫**:拒绝停用 `is_default=1` 主题的当前版本——否则下次启动 `AssertDefaultThemeUsable` 失败、发布/预览解析 503/500。返回 409 `DEFAULT_THEME_VERSION`,与既有 `ErrDefaultTheme`(默认主题必须启用)同款。
- **无当前版本**:`current_version_id IS NULL`(如 culled 行)时 patch status → 409 `NO_CURRENT_VERSION`「无当前版本可停用」。
- **归属**:status patch 对 catalog 与 private 都放行(管理员对两者都有全站下架权,§8.1);默认主题守卫只对 default 生效(default 必是 catalog)。
- **幂等**:置为当前已是的状态不报错(乐观 `RowsAffected` 判定用于并发,不用于幂等拒绝)。

### 3.3 实现落点

- `internal/admin/service.go`:`ThemePatch` 加 `Status *string`;新增 `ErrDefaultThemeVersion`、`ErrNoCurrentVersion` 哨兵。
- `internal/admin/sqlstore.go` 的 `UpdateTheme` 事务内:读 `is_default`/`scope` 的同一查询扩读 `current_version_id`;`patch.Status != nil` 时按 §3.2 校验后 `UPDATE theme_versions`。审计沿用既有 `AuditRecord`(action 如 `theme.version.disable`/`theme.version.enable`)。
- `internal/httpapi/admin.go` 的 `updateTheme`:解 `status` 字段;映射 `ErrDefaultThemeVersion`→409、`ErrNoCurrentVersion`→409(错误码如上)。
- **测试**:停用当前版本 → `theme_versions.status='disabled'` 且 `ResolveEligibleVersion` 对该主题回落默认;重新启用恢复解析;停用默认主题当前版本被拒(`ErrDefaultThemeVersion`);无版本主题被拒;审计落库。

## 4. admin 列表加 owner 列 / 按 scope 分组

### 4.1 后端

- `admin.Theme`(`internal/admin/service.go`)加 `OwnerID string`、`OwnerName string`、`Status string`(当前版本状态,`active`/`disabled`,无版本时空串)。
- `themeSelect`(`internal/admin/sqlstore.go`)增 `themes.owner_id`、`theme_versions.status`,并 `LEFT JOIN users ON users.id = themes.owner_id` 取 `users.username`。保持 LEFT JOIN theme_versions 不变(管理端全量视图,不复用 eligibility)。
- `scanTheme` 对应扫描(owner_id/username/status 用 `sql.NullString`,无版本/无 owner 时零值)。
- `themeData`(`internal/httpapi/admin.go`)非空才输出 `ownerId`/`ownerName`/`status`。
- **测试**:私有主题行返回 `OwnerName==导入者用户名`、`Status==当前版本状态`;catalog 主题 `OwnerName` 空;停用版本后 `Status=='disabled'`。

### 4.2 前端 admin 主题页

- `web/src/pages/admin/themes/page.tsx` 主题卡分组从"按 `vibe`(Classic/Kawaii)"改为"**按 scope**":`官方目录`(scope=catalog)与 `用户主题`(scope=private,卡片显示 owner 名 + 当前版本状态徽章)。
- **修隐形 bug**:现按 `vibe` filter 会漏掉无 `vibe` 的主题;改为按 scope 分组后所有主题都归入某组;私有主题即便 manifest 无 `vibe`/`swatches` 也用占位色板正常渲染。
- 每张卡:启用开关(既有)、设默认(既有,私有已被后端拒)、**停用/启用当前版本**按钮(新,调 PATCH status;停用默认主题版本时按钮禁用并提示原因)。
- `Theme` 前端类型(`web/src/api/types.ts`)加可选 `ownerId`/`ownerName`/`status`;admin API 模块(`web/src/api/admin.ts`)的 theme patch 支持 status。
- 走 `web/src/api/`,不绕过契约;浏览器六态冒烟(含暗色、移动、键盘)。

## 5. 契约

- `api/openapi.yaml`:
  - `PATCH /api/v1/admin/themes/{themeId}` 请求体 properties 加 `status: { type: string, enum: [active, disabled] }`(全字段可选,至少一个——沿用既有惯例);responses 加 `409`(已存在 ErrorResponse 引用则复用)。
  - `Theme` schema 加可选 `ownerId: { $ref Id }`、`ownerName: { type: string }`、`status: { type: string, enum: [active, disabled] }`。
- `tests/contract/`:PATCH status→200;停用后该主题从 user `GET /api/v1/themes` eligible 列表消失(410 分支按既定不进契约,由 `internal/httpapi/themes_test.go` 单测覆盖)。
- mock(`web/src/api/mock-handlers.ts`)+ `web/tests/mock-contract.test.ts`:admin themes / patch 响应补 owner/status,保持 `make test-mock` 通过。

## 6. 测试策略

- **Go 单测**:§3.3、§4.1 列出的 store/service 断言;kill switch 的回落一致性复用 `eligibility_test.go` 范式。
- **契约**:§5。
- **E2E**(`tests/e2e/specs/admin.spec.ts`,照 `subdomain.spec.ts` 管理员审批范式):管理员进主题页 → 用户主题分组显示 owner → 停用某主题当前版本 → 状态徽章变化。admin.spec 目前无任何主题测试,这是首个。
- 门槛:`make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、`make e2e`;UI 六态冒烟。

## 7. 交付与不在范围

- 分支 `feat/theme-governance-b2a`(自最新 main),单 PR。
- **不在范围(各自独立子项目)**:目录审核(私有→catalog 晋升申请-审批流)、更新检查(GitHub upstream 新 commit)、墓碑回收 + 账号删除前置清理(后者卡在"删用户功能不存在")、starter 仓库、非 GitHub 布局适配。
- 按 CLAUDE.md shipping path:PR → CI 绿 → auto-merge → 生产 CD(注意:CD 需服务器有 rsync,已在 2026-07-25 修复)。
