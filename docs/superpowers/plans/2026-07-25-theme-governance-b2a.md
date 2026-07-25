# 主题治理必需(B2a)实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给管理端补齐三项主题治理能力——版本级 kill switch(停用/启用主题当前版本)、`imported_by` 导入溯源、admin 主题列表加 owner 列并按 scope 分组。

**Architecture:** 全部落在 B1 已就绪的地基上,不新建子系统、不新增迁移(migration 0014 的列/CHECK/触发器已够用)。kill switch 只补"写入 `theme_versions.status='disabled'`"这一入口,读路径(公开页 410、eligibility 排除 disabled、回落默认)B1 已实现;溯源是 3 处小接线;owner 列是纯增量字段 + `LEFT JOIN users`。

**Tech Stack:** Go 1.25 + chi + modernc.org/sqlite;React 19 + Vite + TanStack Query;Playwright;libopenapi-validator 契约测试。**不新增任何依赖。**

**设计依据:** `docs/superpowers/specs/2026-07-25-theme-governance-b2a-design.md`

## Global Constraints

- 分支 `feat/theme-governance-b2a`(已存在,spec 已提交),最终单 PR。
- 每任务提交前该任务测试必须绿;推送前 `make check`、`go test -race ./...`、`make build` 全绿(CLAUDE.md)。
- Conventional Commit 英文主题行;用户可见文案与注释中文;gofmt 干净。
- `api/openapi.yaml` 是契约唯一来源;`internal/httpapi/` 只做路由/DTO/序列化。
- 迁移文件 append-only;**本计划不新增也不修改任何迁移**。
- `status` 值域 `{active, disabled}`;kill switch 作用于 `themes.current_version_id`,与 `enabled`(整包可见性)正交。
- 默认主题守卫:拒绝停用 `is_default=1` 主题的当前版本(409 `DEFAULT_THEME_VERSION`);无当前版本时 patch status 返回 409 `NO_CURRENT_VERSION`。
- 前端二空格缩进;`@/` 别名;`useQuery`/`useMutation` 手动 import(hooks 已封装在 useQueries.ts);所有 HTTP 走 `web/src/api/`。
- kill switch 不承诺"即时全局撤销"(§8.1.1):撤销靠页面回落,不依赖缓存清除。

## 文件结构总览

| 文件 | 动作 | 任务 |
|---|---|---|
| `internal/themes/store.go` | `upsertVersionTx`/`UpsertVersion` 加 importer 形参 | 1 |
| `internal/themes/install.go` | 两处调用传 ownerID | 1 |
| `internal/themes/store_test.go`、`internal/themeimport/service_test.go` | imported_by 断言 | 1 |
| `internal/admin/service.go` | `Theme` 加 owner/status 字段;`ThemePatch.Status`;新错误哨兵;`UpdateTheme` 校验 | 2, 3 |
| `internal/admin/sqlstore.go` | `themeSelect` JOIN users + status;`scanTheme`;`UpdateTheme` 处理 status | 2, 3 |
| `internal/admin/sqlstore_test.go` | owner/status 读断言 + kill switch 断言 | 2, 3 |
| `internal/httpapi/admin.go` | `themeData` 输出 owner/status;`updateTheme` 收 status + 错误映射 | 2, 3 |
| `api/openapi.yaml` | `Theme` schema 加 owner/status;PATCH body 加 status | 2, 3 |
| `tests/contract/api_contract_test.go` | kill switch 契约断言 | 4 |
| `web/src/api/mock-handlers.ts`、`web/tests/mock-contract.test.ts` | mock owner/status + patch status | 4 |
| `web/src/api/types.ts`、`web/src/themes/types.ts`、`web/src/api/admin.ts`、`web/src/hooks/useQueries.ts` | 前端类型与 hook | 5 |
| `web/src/pages/admin/themes/page.tsx` | 按 scope 分组 + owner + 停用版本按钮 | 5 |
| `tests/e2e/specs/admin.spec.ts` | 主题治理 E2E | 6 |

---

### Task 1: imported_by 导入溯源接线

**Files:**
- Modify: `internal/themes/store.go`(`UpsertVersion` ~55、`upsertVersionTx` ~78、INSERT ~105-112)
- Modify: `internal/themes/install.go`(两处 `upsertVersionTx` 调用,行 70、84)
- Test: `internal/themeimport/service_test.go`(追加)、`internal/themes/sync_test.go`(追加)

**Interfaces:**
- Consumes: 无。
- Produces: `upsertVersionTx(ctx, tx, packageID string, compiled Compiled, sourceType, sourceRef, importerID string, now time.Time) (string, error)`——`importerID` 空串写 NULL。`UpsertVersion` 公开签名**不变**(内部传空串)。

- [ ] **Step 1: 写失败测试**

`internal/themeimport/service_test.go` 追加(该文件已有 `newServiceDB`/`sampleZip`/`makeSampleTarGz` 等辅助与 `context`/`testing` import):

```go
func TestImportZipRecordsImporter(t *testing.T) {
	service, store := newServiceDB(t)
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", sampleZip(t))
	if err != nil {
		t.Fatalf("ImportZip() error = %v", err)
	}
	var importer sql.NullString
	if err := store.DB().QueryRow(`SELECT imported_by FROM theme_versions WHERE id = ?`, installed.VersionID).Scan(&importer); err != nil {
		t.Fatal(err)
	}
	if !importer.Valid || importer.String != "usr_svc_0001" {
		t.Fatalf("imported_by = %v, want usr_svc_0001", importer)
	}
}
```

注意:该测试用 `store.DB()` 取 `*sql.DB`。若 `themes.Store` 没有导出取 db 的方法,以 `newServiceDB` 返回的 `*sql.DB`(它已返回 store 与 db,见该文件既有签名)为准改写——**动手前先读 `newServiceDB` 的真实返回**,用它返回的 db 直接查询,不要臆造 `store.DB()`。补 import `"database/sql"`。

`internal/themes/sync_test.go` 追加(验证内置主题 importer 为 NULL):

```go
func TestSyncBuiltinLeavesImporterNull(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	var nonNull int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE imported_by IS NOT NULL`).Scan(&nonNull); err != nil {
		t.Fatal(err)
	}
	if nonNull != 0 {
		t.Fatalf("builtin versions must have NULL imported_by, found %d non-null", nonNull)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/themeimport -run TestImportZipRecordsImporter`
Expected: FAIL(`imported_by` 恒 NULL → `importer.Valid` 为 false)

- [ ] **Step 3: 实现**

`internal/themes/store.go`:`upsertVersionTx` 签名在 `sourceRef` 后、`now` 前插入 `importerID string`:

```go
func upsertVersionTx(ctx context.Context, tx *sql.Tx, packageID string, compiled Compiled, sourceType, sourceRef, importerID string, now time.Time) (string, error) {
```

INSERT 语句把 `'active', NULL, ?` 改为 `'active', ?, ?`,参数列表在 `compiled.ContentHash` 之后、`dbTime(now)` 之前插入 importer:

```go
	result, err := tx.ExecContext(ctx, `
		INSERT INTO theme_versions(
			id, theme_id, version, source_ref, manifest_json,
			compiled_css, content_hash, status, imported_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
		ON CONFLICT (theme_id, content_hash) DO NOTHING`,
		compiled.VersionID, packageID, compiled.Manifest.Version, sourceRef, string(manifestJSON),
		compiled.CSS, compiled.ContentHash, sql.NullString{String: importerID, Valid: importerID != ""}, dbTime(now))
```

`UpsertVersion` 包装内的调用传空串:

```go
		id, err := upsertVersionTx(ctx, tx, packageID, compiled, sourceType, sourceRef, "", now)
```

`internal/themes/install.go` 两处(新装、升级)把 `ownerID` 作为 importer 传入:

```go
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, ownerID, now)
```

(两处调用点在 `InstallPrivate` 内,`ownerID` 是该函数的入参,直接可用。)

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/themes ./internal/themeimport -race`
Expected: PASS(含既有全部用例——签名变更后 grep 确认无其他 `upsertVersionTx` 调用点遗漏)

- [ ] **Step 5: Commit**

```bash
git add internal/themes/ internal/themeimport/
git commit -m "feat: record the importer on third-party theme versions"
```

---

### Task 2: admin 主题读取加 owner / status 字段

**Files:**
- Modify: `internal/admin/service.go`(`Theme` 结构体 ~98-119)
- Modify: `internal/admin/sqlstore.go`(`themeSelect` ~213-219、`scanTheme` ~438-468)
- Modify: `internal/httpapi/admin.go`(`themeData` ~513-549)
- Modify: `api/openapi.yaml`(`Theme` schema ~2379-2411)
- Test: `internal/admin/sqlstore_test.go`(追加)

**Interfaces:**
- Consumes: Task 1 之后 `theme_versions.imported_by` 有数据(用于将来,本任务只读 owner 与 status)。
- Produces: `adminpkg.Theme` 增 `OwnerID string`、`OwnerName string`、`Status string`(当前版本状态,无版本时空串)。`themeData` 非空才输出 `ownerId`/`ownerName`/`status`。

- [ ] **Step 1: 写失败测试**

`internal/admin/sqlstore_test.go` 追加(该文件已有 `database.OpenAndMigrate`、`themes.SyncBuiltin`、`strings`、`time` 等;私有主题种子参照既有 `TestUpdateThemeRejectsPrivateDefault` 的 INSERT 写法):

```go
func TestThemeListingIncludesOwnerAndStatus(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	// 造一个属于 alice 的私有主题 + 一个 active 当前版本。
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_alice_b2a', 'alice', 'alice@example.com', 'x', 'user', 'active', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES ('thm_alice_b2a', 'Alice Theme', '1.0.0', 'alice', '', 'light', '', 1, 0, ?, ?, 'alice-theme', 'private', 'usr_alice_b2a', 'upload')`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES ('vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'thm_alice_b2a', '1.0.0', 'digest', '{}', 'x', 'hashalicetheme0001', 'active', ?)`, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = 'vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' WHERE id = 'thm_alice_b2a'`); err != nil {
		t.Fatal(err)
	}

	store := NewSQLStore(db)
	items, err := store.ListThemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Theme{}
	for _, item := range items {
		byID[item.ID] = item
	}
	alice, ok := byID["thm_alice_b2a"]
	if !ok {
		t.Fatal("列表缺少私有主题")
	}
	if alice.OwnerID != "usr_alice_b2a" || alice.OwnerName != "alice" {
		t.Fatalf("owner 未接线: %+v", alice)
	}
	if alice.Status != "active" {
		t.Fatalf("status = %q, want active", alice.Status)
	}
	// catalog 内置主题无 owner。
	if slate := byID["slate"]; slate.OwnerName != "" || slate.OwnerID != "" {
		t.Fatalf("catalog 主题不应有 owner: %+v", slate)
	}
	if slate := byID["slate"]; slate.Status != "active" {
		t.Fatalf("slate status = %q, want active", slate.Status)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/admin -run TestThemeListingIncludesOwnerAndStatus`
Expected: FAIL(编译错误,`Theme` 无 `OwnerID` 字段)

- [ ] **Step 3: 实现**

`internal/admin/service.go` 的 `Theme` 结构体在 `SourceURL` 后追加:

```go
	SourceType       string
	SourceURL        string
	OwnerID          string
	OwnerName        string
	// Status 是当前版本的状态(active/disabled)。无当前版本时空串。
	Status           string
```

`internal/admin/sqlstore.go` 的 `themeSelect` 增列与 JOIN:

```go
const themeSelect = `
	SELECT themes.id, themes.name, themes.version, themes.author, themes.description,
	       themes.mode, themes.preview, themes.enabled, themes.is_default,
	       themes.current_version_id, themes.scope, themes.source_type, themes.source_url,
	       themes.owner_id, users.username, theme_versions.status,
	       theme_versions.manifest_json
	FROM themes
	LEFT JOIN theme_versions ON theme_versions.id = themes.current_version_id
	LEFT JOIN users ON users.id = themes.owner_id`
```

`scanTheme` 增加对应扫描(在既有 `sourceType, sourceURL` 之后、`manifestJSON` 之前插入三列;注意 SELECT 顺序:owner_id、username、status 在 manifest_json 之前):

```go
func scanTheme(row rowScanner) (Theme, error) {
	var item Theme
	var currentVersionID, manifestJSON, sourceType, sourceURL sql.NullString
	var ownerID, ownerName, status sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.Version, &item.Author, &item.Description,
		&item.Mode, &item.Preview, &item.Enabled, &item.Default,
		&currentVersionID, &item.Scope, &sourceType, &sourceURL,
		&ownerID, &ownerName, &status, &manifestJSON); err != nil {
		return Theme{}, err
	}
	if currentVersionID.Valid && currentVersionID.String != "" {
		item.CurrentVersionID = currentVersionID.String
		item.CSSHref = "/api/v1/public/themes/" + currentVersionID.String + ".css"
	}
	if sourceType.Valid && sourceType.String != "" {
		item.SourceType = sourceType.String
	}
	if sourceURL.Valid && sourceURL.String != "" {
		item.SourceURL = sourceURL.String
	}
	if ownerID.Valid {
		item.OwnerID = ownerID.String
	}
	if ownerName.Valid {
		item.OwnerName = ownerName.String
	}
	if status.Valid {
		item.Status = status.String
	}
	if manifestJSON.Valid && manifestJSON.String != "" {
		var manifest themes.Manifest
		if err := json.Unmarshal([]byte(manifestJSON.String), &manifest); err != nil {
			return Theme{}, err
		}
		item.Subtitle = manifest.Subtitle
		item.Tier = manifest.Tier
		item.Vibe = manifest.Vibe
		item.Swatches = manifest.Swatches
	}
	return item, nil
}
```

`internal/httpapi/admin.go` 的 `themeData` 在 `sourceUrl` 之后追加:

```go
	if item.OwnerID != "" {
		data["ownerId"] = item.OwnerID
	}
	if item.OwnerName != "" {
		data["ownerName"] = item.OwnerName
	}
	if item.Status != "" {
		data["status"] = item.Status
	}
```

`api/openapi.yaml` 的 `Theme` schema 在 `sourceUrl` 之后追加(可选字段,不进 required):

```yaml
        sourceUrl: { type: string, maxLength: 300 }
        ownerId: { $ref: '#/components/schemas/Id' }
        ownerName: { type: string }
        status: { type: string, enum: [active, disabled] }
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/admin -race && make test-contract`
Expected: PASS(既有 `TestThemeListingIncludesSpecV1Fields` 不受影响——它不断言 owner/status;契约测试的 `GET /admin/themes` 现在多几个可选字段,过 schema)

- [ ] **Step 5: Commit**

```bash
git add internal/admin/ internal/httpapi/admin.go api/openapi.yaml
git commit -m "feat: surface theme owner and current-version status in the admin listing"
```

---

### Task 3: 版本级 kill switch(status patch)

**Files:**
- Modify: `internal/admin/service.go`(错误哨兵 ~22-30、`ThemePatch` ~121-125、`UpdateTheme` 校验 ~398-420)
- Modify: `internal/admin/sqlstore.go`(`UpdateTheme` 事务 ~247-285)
- Modify: `internal/httpapi/admin.go`(`updateTheme` handler ~302-318、`writeError` 映射 ~最后)
- Modify: `api/openapi.yaml`(PATCH body ~1338-1352)
- Test: `internal/admin/sqlstore_test.go`(追加)

**Interfaces:**
- Consumes: Task 2 的 `Theme.Status`(响应回读);`themes.ResolveEligibleVersion`(验证回落)。
- Produces: `ThemePatch.Status *string`;`ErrDefaultThemeVersion`、`ErrNoCurrentVersion` 哨兵;`UpdateTheme` 对 `status='disabled'` 写 `theme_versions.status`。

- [ ] **Step 1: 写失败测试**

`internal/admin/sqlstore_test.go` 追加:

```go
func TestUpdateThemeDisablesCurrentVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	store := NewSQLStore(db)
	service := NewService(store)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	actor := Actor{ID: "usr_admin_b2a", Username: "admin", Role: "admin", Status: "active"}

	// sakura 非默认,可停用。
	disabled := "disabled"
	updated, err := service.UpdateTheme(ctx, actor, "sakura", ThemePatch{Status: &disabled, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("disable sakura version error = %v", err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("status = %q, want disabled", updated.Status)
	}
	// 停用后 sakura 不再可解析,回落默认(slate)。
	var slateVersion string
	if err := db.QueryRow(`SELECT current_version_id FROM themes WHERE id = 'slate'`).Scan(&slateVersion); err != nil {
		t.Fatal(err)
	}
	got, err := themes.ResolveEligibleVersion(ctx, db, "sakura", "")
	if err != nil {
		t.Fatalf("resolve after disable error = %v", err)
	}
	if got != slateVersion {
		t.Fatalf("disabled theme should fall back to default %q, got %q", slateVersion, got)
	}
	// 重新启用恢复。
	active := "active"
	if _, err := service.UpdateTheme(ctx, actor, "sakura", ThemePatch{Status: &active, RequestID: "req-2"}); err != nil {
		t.Fatalf("re-enable error = %v", err)
	}
	reGot, err := themes.ResolveEligibleVersion(ctx, db, "sakura", "")
	if err != nil || reGot == slateVersion {
		t.Fatalf("re-enabled sakura should resolve to its own version, got %q (err %v)", reGot, err)
	}
}

func TestUpdateThemeRejectsDisablingDefaultVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC) }
	actor := Actor{ID: "usr_admin_b2a", Username: "admin", Role: "admin", Status: "active"}
	disabled := "disabled"
	// slate 是默认主题,拒绝停用其当前版本。
	if _, err := service.UpdateTheme(ctx, actor, "slate", ThemePatch{Status: &disabled, RequestID: "req"}); !errors.Is(err, ErrDefaultThemeVersion) {
		t.Fatalf("err = %v, want ErrDefaultThemeVersion", err)
	}
}
```

(`service.now` 字段与 `NewService` 的用法参照既有 `TestAdminManagementLifecycle`;`errors` import 已在该文件。)

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/admin -run 'TestUpdateThemeDisables|TestUpdateThemeRejectsDisabling'`
Expected: FAIL(`ThemePatch` 无 `Status`,编译错误)

- [ ] **Step 3: 实现**

`internal/admin/service.go` 错误哨兵区(`ErrPrivateDefault` 之后)追加:

```go
	// ErrDefaultThemeVersion:不能停用默认主题的当前版本——否则默认主题失去可用版本。
	ErrDefaultThemeVersion = errors.New("cannot disable the default theme's current version")
	// ErrNoCurrentVersion:主题没有当前版本可供停用/启用。
	ErrNoCurrentVersion = errors.New("theme has no current version")
```

`ThemePatch` 加字段:

```go
type ThemePatch struct {
	Enabled   *bool
	Default   *bool
	Status    *string
	RequestID string
}
```

`service.UpdateTheme` 的空 patch 校验与 status 值校验(替换现有 `if patch.Enabled == nil && patch.Default == nil` 那段,并把 status 纳入审计 detail):

```go
	if patch.Enabled == nil && patch.Default == nil && patch.Status == nil {
		return Theme{}, ErrInvalidInput
	}
	if patch.Status != nil && *patch.Status != "active" && *patch.Status != "disabled" {
		return Theme{}, ErrInvalidInput
	}
```

并把 audit 那行的 detail 加上 status:

```go
	audit, err := s.audit(actor, "theme.update", "theme", themeID, map[string]any{"enabled": patch.Enabled, "default": patch.Default, "status": patch.Status}, patch.RequestID)
```

`internal/admin/sqlstore.go` 的 `UpdateTheme` 事务:读 `is_default, scope` 的查询扩读 `current_version_id`,并在既有 enabled 处理之后、`insertAudit` 之前加 status 处理:

```go
		var currentDefault bool
		var scope string
		var currentVersionID sql.NullString
		if err := tx.QueryRowContext(ctx, "SELECT is_default, scope, current_version_id FROM themes WHERE id = ?", themeID).Scan(&currentDefault, &scope, &currentVersionID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
```

status 处理块(放在 `if patch.Enabled != nil { ... }` 之后、`return insertAudit(...)` 之前):

```go
		if patch.Status != nil {
			if currentDefault && *patch.Status == "disabled" {
				return ErrDefaultThemeVersion
			}
			if !currentVersionID.Valid || currentVersionID.String == "" {
				return ErrNoCurrentVersion
			}
			if _, err := tx.ExecContext(ctx, "UPDATE theme_versions SET status = ? WHERE id = ?", *patch.Status, currentVersionID.String); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE themes SET updated_at = ? WHERE id = ?", dbTime(now), themeID); err != nil {
				return err
			}
		}
```

`internal/httpapi/admin.go` 的 `updateTheme` handler 收 status:

```go
	var request struct {
		Enabled *bool   `json:"enabled"`
		Default *bool   `json:"default"`
		Status  *string `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.UpdateTheme(r.Context(), actorFromRequest(r), chi.URLParam(r, "themeId"), adminpkg.ThemePatch{
		Enabled: request.Enabled, Default: request.Default, Status: request.Status, RequestID: middleware.GetReqID(r.Context()),
	})
```

`writeError` 映射(在 `ErrPrivateDefault` 分支旁追加):

```go
	case errors.Is(err, adminpkg.ErrDefaultThemeVersion):
		WriteError(w, r, http.StatusConflict, "DEFAULT_THEME_VERSION", "不能停用默认主题的当前版本", nil)
	case errors.Is(err, adminpkg.ErrNoCurrentVersion):
		WriteError(w, r, http.StatusConflict, "NO_CURRENT_VERSION", "该主题没有可停用的当前版本", nil)
```

`api/openapi.yaml` 的 PATCH `/admin/themes/{themeId}` 请求体 properties 加 status:

```yaml
              properties:
                enabled: { type: boolean }
                default: { type: boolean }
                status: { type: string, enum: [active, disabled] }
              minProperties: 1
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/admin ./internal/httpapi -race && make test-contract`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/admin/ internal/httpapi/admin.go api/openapi.yaml
git commit -m "feat: add a version-level kill switch to the admin theme patch"
```

---

### Task 4: 契约测试与 mock

**Files:**
- Modify: `tests/contract/api_contract_test.go`(「管理端读取」t.Run 之后追加,或在既有 admin 段内)
- Modify: `web/src/api/mock-handlers.ts`、`web/tests/mock-contract.test.ts`

**Interfaces:**
- Consumes: Task 3 的 PATCH status 行为;既有 `admin`/`user` 契约客户端。
- Produces: 无。

- [ ] **Step 1: 契约断言**

`tests/contract/api_contract_test.go` 新增 t.Run(放在「管理端读取」之后):

```go
	t.Run("版本级 kill switch", func(t *testing.T) {
		// 先确认 sakura 在 user 的 eligible 列表里(非默认、active)。
		before := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, before, http.StatusOK, "停用前主题列表")
		if !strings.Contains(string(before.body), "\"sakura\"") {
			t.Fatal("停用前列表应含 sakura")
		}

		// 管理员停用 sakura 当前版本。
		disabled := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/sakura",
			map[string]any{"status": "disabled"})
		mustStatus(t, disabled, http.StatusOK, "停用 sakura 版本")
		if got := stringField(t, disabled.data(), "status", "sakura 状态"); got != "disabled" {
			t.Fatalf("status = %q, want disabled", got)
		}

		// 停用后 sakura 从 user 的 eligible 列表消失(410 分支由 httpapi 单测覆盖,这里断言列表回落)。
		after := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, after, http.StatusOK, "停用后主题列表")
		if strings.Contains(string(after.body), "\"sakura\"") {
			t.Fatal("停用后列表不应再含 sakura")
		}

		// 停用默认主题 slate 的当前版本 → 409。
		rejected := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/slate",
			map[string]any{"status": "disabled"})
		mustStatus(t, rejected, http.StatusConflict, "停用默认主题版本被拒")

		// 恢复 sakura,避免污染后续断言。
		restored := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/sakura",
			map[string]any{"status": "active"})
		mustStatus(t, restored, http.StatusOK, "恢复 sakura")
	})
```

注意:该 t.Run 依赖 sakura 是内置非默认主题(SyncBuiltin 后成立)、slate 是默认。若既有契约流程此前对 sakura 有别的断言,本段结尾已恢复 active,不留副作用。`strings` import 已在该文件。

- [ ] **Step 2: mock 补 status/owner**

`web/src/api/mock-handlers.ts`:
1. `web/src/mocks/data.ts` 的 `mockThemes` 三条各补 `status: 'active'`(owner 留空,catalog 无 owner);
2. `mockPrivateThemes` 的构造(import handler 里 push 的对象)补 `status: 'active'`、`ownerId: 'usr_001'`、`ownerName: 'lucaspeng'`(mock 当前用户);
3. admin themes PATCH handler(找处理 `/admin/themes/{id}` PATCH 的分支;若不存在则在个人 `/me` 无关、admin 段新增):收到 `{status}` 时,在合并数组里把对应主题的 `status` 改为该值并返回该主题对象(200)。**动手前先 grep `admin/themes` 的 PATCH 处理,按既有 enabled/default patch 的处理方式扩展 status。**

`web/tests/mock-contract.test.ts`:既有已有 `/api/v1/admin/themes` GET case;补一条 PATCH:

```ts
  { name: '管理主题状态', path: '/api/v1/admin/themes/{themeId}', method: 'patch', status: '200', url: '/api/v1/admin/themes/slate', init: { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status: 'active' }) } },
```

(路径 key 必须与 openapi 一致 `/api/v1/admin/themes/{themeId}`;mock 对 slate patch active 是幂等安全操作。)

- [ ] **Step 3: 运行**

Run: `make test-contract && make test-mock && make check`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tests/contract/ web/src/ web/tests/
git commit -m "test: cover the theme kill switch in contract and mock"
```

---

### Task 5: 前端 admin 主题页按 scope 分组 + owner + 停用版本按钮

**Files:**
- Modify: `web/src/api/types.ts`(`Theme` 接口)、`web/src/themes/types.ts`(`ThemeMeta` + `themeDisplayFromApi`)
- Modify: `web/src/api/admin.ts`(`updateThemeState` 支持 status)、`web/src/hooks/useQueries.ts`(`useUpdateAdminThemeState` 类型)
- Modify: `web/src/pages/admin/themes/page.tsx`(分组 + owner + 按钮)

**Interfaces:**
- Consumes: Task 2/3 的 `Theme.ownerName`/`status` 与 PATCH status。
- Produces: 无。

- [ ] **Step 1: 前端类型**

`web/src/api/types.ts` 的 `Theme` 接口追加(在 `sourceUrl` 附近):

```ts
  ownerId?: string;
  ownerName?: string;
  status?: 'active' | 'disabled';
```

`web/src/themes/types.ts` 的 `ThemeMeta` 追加:

```ts
  ownerName?: string;
  status?: 'active' | 'disabled';
```

`themeDisplayFromApi` 透传:

```ts
      scope: theme.scope,
      sourceType: theme.sourceType,
      sourceUrl: theme.sourceUrl,
      ownerName: theme.ownerName,
      status: theme.status,
```

`web/src/api/admin.ts` 的 `updateThemeState` 放宽 data 类型:

```ts
  updateThemeState: (themeId: string, data: { enabled?: boolean; default?: boolean; status?: 'active' | 'disabled' }) =>
    request<ApiResponse<Theme>>(`/admin/themes/${themeId}`, { method: 'PATCH', body: data }),
```

`web/src/hooks/useQueries.ts` 的 `useUpdateAdminThemeState` 的 mutationFn 入参类型同步放宽:

```ts
    mutationFn: ({ themeId, data }: { themeId: string; data: { enabled?: boolean; default?: boolean; status?: 'active' | 'disabled' } }) =>
      adminApi.updateThemeState(themeId, data),
```

- [ ] **Step 2: 按 scope 分组 + owner + 停用按钮**

`web/src/pages/admin/themes/page.tsx`:

1. 分组从 vibe 改为 scope(替换 `seriousThemes`/`cuteThemes`):

```ts
  const catalogThemes = useMemo(() => themes.filter(t => t.meta.scope !== 'private'), [themes]);
  const privateThemes = useMemo(() => themes.filter(t => t.meta.scope === 'private'), [themes]);
```

(scope 缺失的内置主题落入 catalog——它们本就是 catalog。这样所有主题都归入某组,修掉私有主题按 vibe 分组时可能落空的问题。)

2. 分组渲染(替换 Classic/Kawaii 两段)为「官方目录」与「用户主题」;用户主题卡额外显示 owner 名与状态徽章。`renderThemeCard` 保持原签名,新增一层 `renderGovernedCard(pkg)` 叠加 owner/状态/停用按钮(参照 B1 前端 `renderMyThemeCard` 的叠加模式,不侵入 `renderThemeCard`):

```tsx
  const disableVersion = useCallback((pkg: ThemePackage, next: 'active' | 'disabled') => {
    updateTheme.mutate({ themeId: pkg.id, data: { status: next } }, {
      onError: (cause) => toast('error', cause instanceof ApiError ? (cause.detail || cause.message) : '操作失败'),
    });
  }, [updateTheme, toast]);

  const renderGovernedCard = (pkg: ThemePackage, showOwner: boolean) => (
    <div key={pkg.id} className="relative">
      {renderThemeCard(pkg)}
      <div className="mt-1 flex items-center gap-2 text-xs text-foreground-400">
        {showOwner && pkg.meta.ownerName && <span>作者：{pkg.meta.ownerName}</span>}
        {pkg.meta.status === 'disabled' && <span className="text-red-500">已停用版本</span>}
        {pkg.id !== activeId && (
          <button
            type="button"
            onClick={() => disableVersion(pkg, pkg.meta.status === 'disabled' ? 'active' : 'disabled')}
            className="underline hover:text-foreground-600"
          >
            {pkg.meta.status === 'disabled' ? '启用版本' : '停用版本'}
          </button>
        )}
      </div>
    </div>
  );
```

(停用按钮对默认主题 `pkg.id === activeId` 隐藏——默认主题不能停用版本,与后端守卫一致。样式类沿用该文件既有风格,实现时对齐现有 className 习惯。)

3. 分组区块:

```tsx
      {catalogThemes.length > 0 && (
        <section>
          <h3 className="...">官方目录</h3>
          <div className="...">{catalogThemes.map(pkg => renderGovernedCard(pkg, false))}</div>
        </section>
      )}
      {privateThemes.length > 0 && (
        <section>
          <h3 className="...">用户主题</h3>
          <div className="...">{privateThemes.map(pkg => renderGovernedCard(pkg, true))}</div>
        </section>
      )}
```

(标题/容器 className 复用被替换的 Classic/Kawaii 两段的原有类名。)

- [ ] **Step 3: 类型检查与冒烟**

Run: `make check && make test-mock`
然后 `cd web && VITE_ENABLE_API_MOCKS=true npm run dev` 浏览 `/admin/themes`(mock 管理员态):官方目录/用户主题两组渲染、私有主题显示作者、停用/启用按钮切换状态徽章、移动端 375px、暗色主题、键盘可达。六态记录进报告,完事关 dev server。
Expected: 全绿。

- [ ] **Step 4: Commit**

```bash
git add web/src/
git commit -m "feat: group admin themes by scope with owner and kill-switch controls"
```

---

### Task 6: E2E + 全量验证 + 交付

**Files:**
- Modify: `tests/e2e/specs/admin.spec.ts`(新增主题治理用例)

- [ ] **Step 1: E2E 用例**

`tests/e2e/specs/admin.spec.ts` 追加(照该文件既有 admin 用例的 storageState 与导航方式;E2E 走真实二进制,种子里无私有主题,因此断言聚焦"内置主题可停用/恢复"):

```ts
  test('管理员停用并恢复主题版本', async ({ page }) => {
    await page.goto('/admin/themes');
    // 官方目录分组存在。
    await expect(page.getByRole('heading', { name: '官方目录' })).toBeVisible();
    // sakura 非默认,可停用。找到其卡片区域的停用按钮。
    const sakuraCard = page.locator('text=Sakura').locator('xpath=ancestor::div[1]');
    // 用页面级停用按钮(sakura 行);按渲染实际结构调整定位,优先 role+name。
    const disableBtn = page.getByRole('button', { name: '停用版本' }).first();
    await disableBtn.click();
    await expect(page.getByText('已停用版本').first()).toBeVisible({ timeout: 10000 });
    // 恢复。
    await page.getByRole('button', { name: '启用版本' }).first().click();
    await expect(page.getByText('已停用版本')).toHaveCount(0, { timeout: 10000 });
  });
```

注意:定位器以 Task 5 实际渲染文本为准(用 role+name,不用 CSS 类);若"停用版本"按钮在多个卡片出现,`.first()` 命中第一个非默认主题。实现时先跑单测调对选择器,最终必须完整 `make e2e` 全绿。

- [ ] **Step 2: 全量门槛**

Run:
```bash
make check && go test -race ./... && make build && make test-contract && make test-mock && make e2e
```
Expected: 全部 PASS。任一失败回对应任务修复,不带失败推送。

- [ ] **Step 3: 推送与 PR**

```bash
git push -u origin feat/theme-governance-b2a
gh pr create --title "feat: theme governance essentials (B2a)" --body "$(cat <<'EOF'
## Summary
- 版本级 kill switch:PATCH /admin/themes/{id} 加 status(active|disabled),作用于当前版本;守卫拒绝停用默认主题版本(409)与无版本主题(409);读路径(公开页 410、eligibility 回落默认)B1 已就绪,与 enabled 正交
- imported_by 导入溯源:第三方主题版本记录导入者(内置保持 NULL)
- admin 主题列表加 owner 列 + 按 scope 分组(官方目录/用户主题),修掉私有主题按 vibe 分组时的错分,加停用/启用版本控件
- 契约/mock/E2E 覆盖

设计文档:docs/superpowers/specs/2026-07-25-theme-governance-b2a-design.md

## 不在范围(各自独立子项目)
目录审核(私有→catalog 晋升)、更新检查、墓碑回收、starter 仓库。

## Test plan
- [x] make check / go test -race ./... / make build
- [x] make test-contract / make test-mock / make e2e
- [x] /admin/themes 浏览器六态冒烟

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr merge --auto --rebase
```

- [ ] **Step 4: 确认 CI**

Run: `gh pr checks --watch`(或后台轮询)
Expected: `verify`、`e2e`、`container` 全绿自动合并;`deploy-production` 随 main 自动执行(服务器已装 rsync,CD 应正常)。
