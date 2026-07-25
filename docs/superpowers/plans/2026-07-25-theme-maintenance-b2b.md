# 主题维护片(B2b)实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给主题分发补齐三块维护能力——历史版本视图 + 版本级 kill switch、GitHub 更新检查(手动)、墓碑回收(配额压力时自动)。

**Architecture:** 全部落在 B1/B2a 已就绪的地基上,不新建子系统、不新增迁移。版本级 kill switch 复用 B2a 的 status 语义,把入口从"当前版本"扩到"任意版本";更新检查从 B1 的 GitHubClient 导出一个"只解析 HEAD sha、不下载"的方法;墓碑回收把 B1 UninstallPrivate 的物理删逻辑抽成 `deleteThemeTx`,在 InstallPrivate 的配额检查里复用。

**Tech Stack:** Go 1.25 + chi + modernc.org/sqlite + 既有 `internal/netguard`/`internal/themeimport`;React 19 + Vite + TanStack Query;Playwright;libopenapi-validator 契约测试。**不新增任何依赖。**

**设计依据:** `docs/superpowers/specs/2026-07-25-theme-maintenance-b2b-design.md`

## Global Constraints

- 分支 `feat/theme-maintenance-b2b`(已存在,spec 已提交),最终单 PR。
- 每任务提交前该任务测试必须绿;推送前 `make check`、`go test -race ./...`、`make build` 全绿(CLAUDE.md)。
- Conventional Commit 英文主题行;用户可见文案与注释中文;gofmt 干净。
- `api/openapi.yaml` 是契约唯一来源;`internal/httpapi/` 只做路由/DTO/序列化。
- 迁移文件 append-only;**本计划不新增也不修改任何迁移**。
- 版本级 kill switch 唯一守卫:拒绝停用「`is_default=1` 主题的当前版本」(409 `DEFAULT_THEME_VERSION`);未知 versionId → 404;停用非当前版本一律放行(公开页 410 回落基线,§8.1.1)。
- 更新检查:netguard 全防护、主机白名单、token 作用域(仅官方主机)全部复用 B1,不放松;只查不升(升级仍走 B1 重新拉取);限流 20/h/IP。
- 墓碑回收:回收在 InstallPrivate 因配额满即将拒绝之前触发,只删「全部版本都无 published_snapshots 引用」的私有墓碑(enabled=0);owner 无感、不加端点/UI。
- 前端二空格缩进;`@/` 别名;`useQuery`/`useMutation` 手动 import;所有 HTTP 走 `web/src/api/`。

## 文件结构总览

| 文件 | 动作 | 任务 |
|---|---|---|
| `internal/themes/install.go` | 抽 `deleteThemeTx` + `reclaimTombstones` + InstallPrivate 配额段调用 | 1 |
| `internal/themes/install_test.go` | 回收测试 | 1 |
| `internal/admin/service.go` + `sqlstore.go` | `ThemeVersion` 类型 + `ListThemeVersions`/`SetVersionStatus` | 2 |
| `internal/admin/sqlstore_test.go` | 版本列表 + 版本级 kill switch 测试 | 2 |
| `internal/httpapi/admin.go` | 两个 handler + 路由 + 序列化 | 3 |
| `api/openapi.yaml` | 版本列表/版本 patch 端点 + schema | 3 |
| `internal/themeimport/github.go` | `ResolveHeadSHA` 导出方法 | 4 |
| `internal/themeimport/github_test.go` | ResolveHeadSHA 测试 | 4 |
| `internal/themeimport/service.go` + `service_test.go` | `CheckUpdate` + `UpdateStatus` | 4 |
| `internal/httpapi/themeimport.go` + `rate_limit.go` + `internal/app/run.go` | check-update 端点 + 限流 | 5 |
| `api/openapi.yaml` | check-update 端点 + schema | 5 |
| `tests/contract/api_contract_test.go` | 版本列表/patch/check-update 契约 | 6 |
| `web/src/api/mock-handlers.ts` + `web/tests/mock-contract.test.ts` | 三端点 mock | 6 |
| `web/src/api/*.ts` + `web/src/pages/admin/themes/page.tsx` + `web/src/pages/app/themes/page.tsx` | 版本视图 UI + 检查更新按钮 | 7 |
| `tests/e2e/specs/admin.spec.ts` | 版本停用 E2E | 8 |

---

### Task 1: 墓碑回收(配额压力时自动)

**Files:**
- Modify: `internal/themes/install.go`(抽 `deleteThemeTx`;`UninstallPrivate` 物理删段改调它;`InstallPrivate` 配额段加回收)
- Test: `internal/themes/install_test.go`(追加)

**Interfaces:**
- Consumes: 既有 `dbTime`、`database.WithinTx`、`published_snapshots.theme_version_id`(RESTRICT)。
- Produces:
  - `func deleteThemeTx(ctx context.Context, tx *sql.Tx, themeID string, now time.Time) error` — 物理删单个主题(置空 current 指针 → 删版本 → 删行)。
  - `func reclaimTombstones(ctx context.Context, tx *sql.Tx, ownerID string, now time.Time) (int, error)` — 扫该 owner 的墓碑(`scope=private AND enabled=0`),物理删除全部版本无快照引用者,返回回收数。

- [ ] **Step 1: 写失败测试**

`internal/themes/install_test.go` 追加(复用既有 `newTestDB`/`seedUser`/`installSample`;快照种子参照 B1 的 `TestUninstallPrivateTombstonesReferencedTheme` 用 `page_system_root`):

```go
func TestInstallPrivateReclaimsUnreferencedTombstonesUnderQuota(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_recl_0001")

	// 配额=2。装两个:one(将转墓碑,无引用)、two(将转墓碑,有快照引用)。
	one := installSample(t, store, "usr_recl_0001", "one", 2)
	two := installSample(t, store, "usr_recl_0001", "two", 2)

	// 给 two 造一条快照引用,使其卸载后成为"有引用墓碑"。
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO published_snapshots (id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at, theme_version_id)
		VALUES ('snp_recl_0001', 'page_system_root', 1, 'recl', 'public', '{}', 'W/"x"', ?, ?)`,
		stamp, two.VersionID); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	// 卸载两个:one 无引用→物理删(配额腾出);two 有引用→墓碑(占配额)。
	if removed, err := store.UninstallPrivate(t.Context(), "usr_recl_0001", one.ThemeID, time.Now().UTC()); err != nil || !removed {
		t.Fatalf("uninstall one: removed=%v err=%v", removed, err)
	}
	if removed, err := store.UninstallPrivate(t.Context(), "usr_recl_0001", two.ThemeID, time.Now().UTC()); err != nil || removed {
		t.Fatalf("uninstall two: want tombstone, removed=%v err=%v", removed, err)
	}
	// 现在 owner 名下:two(墓碑,占配额)。装 three + four 应无问题(配额 2,占 1)。
	installSample(t, store, "usr_recl_0001", "three", 2)
	// 此刻名下 two(墓碑)+ three = 2,已满。装 four 前若不回收会 ErrQuotaExceeded;
	// 但 two 仍有引用,不可回收 → 确实应满 → four 被拒。
	if _, err := store.InstallPrivate(t.Context(), "usr_recl_0001", "four", "upload", "", "d4", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC()); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("four should be rejected (two still referenced): %v", err)
	}

	// 移除对 two 的快照引用 → two 变为可回收墓碑。
	if _, err := db.Exec(`DELETE FROM published_snapshots WHERE id = 'snp_recl_0001'`); err != nil {
		t.Fatal(err)
	}
	// 再装 four:配额满 → 回收 two(现无引用)→ 腾出 → 安装成功。
	four, err := store.InstallPrivate(t.Context(), "usr_recl_0001", "four", "upload", "", "d4b", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if err != nil {
		t.Fatalf("four should install after reclaiming two: %v", err)
	}
	if four.ThemeID == "" {
		t.Fatal("four not installed")
	}
	// two 已被物理删。
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE id = ?`, two.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("reclaimable tombstone two should have been physically deleted")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/themes -run TestInstallPrivateReclaimsUnreferencedTombstones`
Expected: FAIL(装 four 时未回收 → `ErrQuotaExceeded`,测试在"应成功"处 Fatal)

- [ ] **Step 3: 实现**

`internal/themes/install.go`:抽出 `deleteThemeTx`(把 `UninstallPrivate` 物理删的三条语句移入):

```go
// deleteThemeTx 物理删除单个主题:先摘 current 指针(theme_versions_current_guard
// 才放行删版本),再删版本与行。调用方需已确认该主题无 published_snapshots 引用。
func deleteThemeTx(ctx context.Context, tx *sql.Tx, themeID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `UPDATE themes SET current_version_id = NULL, updated_at = ? WHERE id = ?`, dbTime(now), themeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM theme_versions WHERE theme_id = ?`, themeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, themeID); err != nil {
		return err
	}
	return nil
}

// reclaimTombstones 回收该 owner 的可回收墓碑:scope=private、enabled=0、且其全部
// 版本都不再被任何 published_snapshots 引用。返回回收数量。墓碑对 owner 不可见
// 却占配额,回收在配额压力时自动发生。
func reclaimTombstones(ctx context.Context, tx *sql.Tx, ownerID string, now time.Time) (int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM themes WHERE scope = 'private' AND owner_id = ? AND enabled = 0`, ownerID)
	if err != nil {
		return 0, err
	}
	var tombstones []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		tombstones = append(tombstones, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	reclaimed := 0
	for _, themeID := range tombstones {
		var refs int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM published_snapshots
			WHERE theme_version_id IN (SELECT id FROM theme_versions WHERE theme_id = ?)`,
			themeID).Scan(&refs); err != nil {
			return 0, err
		}
		if refs > 0 {
			continue
		}
		if err := deleteThemeTx(ctx, tx, themeID, now); err != nil {
			return 0, err
		}
		reclaimed++
	}
	return reclaimed, nil
}
```

`UninstallPrivate` 物理删分支改为调 `deleteThemeTx`:

```go
		// 物理删除:无快照引用,可安全回收。
		if err := deleteThemeTx(ctx, tx, themeID, now); err != nil {
			return err
		}
		removed = true
		return nil
```

`InstallPrivate` 新装分支的配额检查改为回收后重算:

```go
			var owned int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE owner_id = ?`, ownerID).Scan(&owned); err != nil {
				return err
			}
			if owned >= quota {
				// 配额满:先回收无快照引用的墓碑(它们对 owner 不可见却占位),再重算。
				if _, err := reclaimTombstones(ctx, tx, ownerID, now); err != nil {
					return err
				}
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE owner_id = ?`, ownerID).Scan(&owned); err != nil {
					return err
				}
				if owned >= quota {
					return ErrQuotaExceeded
				}
			}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/themes -race`
Expected: PASS(含既有 `TestUninstallPrivate*` 全部——`deleteThemeTx` 抽取行为等价)

- [ ] **Step 5: Commit**

```bash
git add internal/themes/
git commit -m "feat: reclaim unreferenced private tombstones under quota pressure"
```

---

### Task 2: 版本视图 + 版本级 kill switch(admin store/service)

**Files:**
- Modify: `internal/admin/service.go`(`ThemeVersion` 类型 + 错误已有 `ErrDefaultThemeVersion`/`ErrNotFound`;`ListThemeVersions`/`SetVersionStatus` service 包装)
- Modify: `internal/admin/sqlstore.go`(store 实现)
- Test: `internal/admin/sqlstore_test.go`(追加)

**Interfaces:**
- Consumes: `AuditRecord`、`database.WithinTx`、`dbTime`。
- Produces:
  - `type ThemeVersion struct { VersionID, Version, SourceRef, Status, CreatedAt, ImportedBy string; IsCurrent bool; SnapshotRefs int }`
  - `func (s *SQLStore) ListThemeVersions(ctx, themeID string) ([]ThemeVersion, error)`(themeID 不存在 → `ErrNotFound`)
  - `func (s *SQLStore) SetVersionStatus(ctx, versionID, status string, now time.Time, audit AuditRecord) (ThemeVersion, error)`
  - service:`(s *Service) ThemeVersions(ctx, actor, themeID)`、`(s *Service) SetThemeVersionStatus(ctx, actor, versionID, status string, requestID)`(authorize admin + status 值校验 + 审计)

- [ ] **Step 1: 写失败测试**

`internal/admin/sqlstore_test.go` 追加:

```go
func TestListAndDisableThemeVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := themes.SyncBuiltin(ctx, themes.NewStore(db), time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	insertTestUser(t, db, "usr_admin_ver", "admin", "admin@example.com", "admin", time.Now().UTC())
	store := NewSQLStore(db)

	// sakura 单版本、当前、非默认。
	versions, err := store.ListThemeVersions(ctx, "sakura")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || !versions[0].IsCurrent || versions[0].Status != "active" {
		t.Fatalf("unexpected versions: %+v", versions)
	}
	sakuraVersion := versions[0].VersionID

	// 停用 sakura 当前版本(非默认,允许)。
	audit := AuditRecord{AuditEntry: AuditEntry{ID: "aud_ver_1", ActorID: "usr_admin_ver", ActorName: "admin", Action: "theme.version.disable", TargetType: "theme_version", TargetID: sakuraVersion, CreatedAt: time.Now().UTC()}}
	updated, err := store.SetVersionStatus(ctx, sakuraVersion, "disabled", time.Now().UTC(), audit)
	if err != nil || updated.Status != "disabled" {
		t.Fatalf("disable sakura version: %+v err=%v", updated, err)
	}

	// 停用 slate(默认主题)当前版本 → 拒绝。
	sv, err := store.ListThemeVersions(ctx, "slate")
	if err != nil {
		t.Fatal(err)
	}
	slateVersion := sv[0].VersionID
	audit2 := AuditRecord{AuditEntry: AuditEntry{ID: "aud_ver_2", ActorID: "usr_admin_ver", ActorName: "admin", Action: "theme.version.disable", TargetType: "theme_version", TargetID: slateVersion, CreatedAt: time.Now().UTC()}}
	if _, err := store.SetVersionStatus(ctx, slateVersion, "disabled", time.Now().UTC(), audit2); !errors.Is(err, ErrDefaultThemeVersion) {
		t.Fatalf("disabling default current version must be rejected, got %v", err)
	}

	// 未知版本 → ErrNotFound。
	audit3 := AuditRecord{AuditEntry: AuditEntry{ID: "aud_ver_3", ActorID: "usr_admin_ver", ActorName: "admin", Action: "theme.version.disable", TargetType: "theme_version", TargetID: "vzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", CreatedAt: time.Now().UTC()}}
	if _, err := store.SetVersionStatus(ctx, "vzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", "disabled", time.Now().UTC(), audit3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown version want ErrNotFound, got %v", err)
	}

	// themeID 不存在 → ErrNotFound。
	if _, err := store.ListThemeVersions(ctx, "no-such-theme"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown theme want ErrNotFound, got %v", err)
	}
}
```

(`insertTestUser`、`AuditEntry` 字段以 admin 包既有定义为准——动手前 grep `func insertTestUser`、`type AuditRecord`/`AuditEntry` 确认字段名,按真实结构构造 audit。)

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/admin -run TestListAndDisableThemeVersion`
Expected: FAIL(`ListThemeVersions` 未定义,编译错误)

- [ ] **Step 3: 实现**

`internal/admin/service.go` 加类型(错误哨兵 `ErrDefaultThemeVersion`/`ErrNotFound` B2a 已有,复用):

```go
type ThemeVersion struct {
	VersionID    string
	Version      string
	SourceRef    string
	Status       string
	CreatedAt    string
	ImportedBy   string
	IsCurrent    bool
	SnapshotRefs int
}
```

service 包装:

```go
func (s *Service) ThemeVersions(ctx context.Context, actor Actor, themeID string) ([]ThemeVersion, error) {
	if err := authorize(actor); err != nil {
		return nil, err
	}
	return s.store.ListThemeVersions(ctx, themeID)
}

func (s *Service) SetThemeVersionStatus(ctx context.Context, actor Actor, versionID, status, requestID string) (ThemeVersion, error) {
	if err := authorize(actor); err != nil {
		return ThemeVersion{}, err
	}
	if status != "active" && status != "disabled" {
		return ThemeVersion{}, ErrInvalidInput
	}
	action := "theme.version.enable"
	if status == "disabled" {
		action = "theme.version.disable"
	}
	audit, err := s.audit(actor, action, "theme_version", versionID, map[string]any{"status": status}, requestID)
	if err != nil {
		return ThemeVersion{}, err
	}
	return s.store.SetVersionStatus(ctx, versionID, status, s.now().UTC(), audit)
}
```

`internal/admin/sqlstore.go` store 实现:

```go
func (s *SQLStore) ListThemeVersions(ctx context.Context, themeID string) ([]ThemeVersion, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM themes WHERE id = ?`, themeID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tv.id, tv.version, tv.source_ref, tv.status, tv.created_at,
		       COALESCE(users.username, ''),
		       (tv.id = themes.current_version_id) AS is_current,
		       (SELECT COUNT(*) FROM published_snapshots ps WHERE ps.theme_version_id = tv.id) AS snapshot_refs
		FROM theme_versions tv
		JOIN themes ON themes.id = tv.theme_id
		LEFT JOIN users ON users.id = tv.imported_by
		WHERE tv.theme_id = ?
		ORDER BY tv.created_at DESC, tv.id`, themeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ThemeVersion, 0)
	for rows.Next() {
		var v ThemeVersion
		var isCurrent int
		if err := rows.Scan(&v.VersionID, &v.Version, &v.SourceRef, &v.Status, &v.CreatedAt, &v.ImportedBy, &isCurrent, &v.SnapshotRefs); err != nil {
			return nil, err
		}
		v.IsCurrent = isCurrent == 1
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *SQLStore) SetVersionStatus(ctx context.Context, versionID, status string, now time.Time, audit AuditRecord) (ThemeVersion, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// 读该版本所属主题的 is_default 与 current_version_id,判守卫。
		var themeID string
		var isDefault bool
		var currentVersionID sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT themes.id, themes.is_default, themes.current_version_id
			FROM theme_versions tv JOIN themes ON themes.id = tv.theme_id
			WHERE tv.id = ?`, versionID).Scan(&themeID, &isDefault, &currentVersionID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if status == "disabled" && isDefault && currentVersionID.Valid && currentVersionID.String == versionID {
			return ErrDefaultThemeVersion
		}
		if _, err := tx.ExecContext(ctx, `UPDATE theme_versions SET status = ? WHERE id = ?`, status, versionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE themes SET updated_at = ? WHERE id = ?`, dbTime(now), themeID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return ThemeVersion{}, err
	}
	// 回读该版本(带 is_current/snapshot_refs)。
	versions, err := s.versionByID(ctx, versionID)
	if err != nil {
		return ThemeVersion{}, err
	}
	return versions, nil
}

// versionByID 回读单个版本行(供 SetVersionStatus 返回)。
func (s *SQLStore) versionByID(ctx context.Context, versionID string) (ThemeVersion, error) {
	var v ThemeVersion
	var isCurrent int
	err := s.db.QueryRowContext(ctx, `
		SELECT tv.id, tv.version, tv.source_ref, tv.status, tv.created_at,
		       COALESCE(users.username, ''),
		       (tv.id = themes.current_version_id),
		       (SELECT COUNT(*) FROM published_snapshots ps WHERE ps.theme_version_id = tv.id)
		FROM theme_versions tv JOIN themes ON themes.id = tv.theme_id
		LEFT JOIN users ON users.id = tv.imported_by
		WHERE tv.id = ?`, versionID).Scan(&v.VersionID, &v.Version, &v.SourceRef, &v.Status, &v.CreatedAt, &v.ImportedBy, &isCurrent, &v.SnapshotRefs)
	if errors.Is(err, sql.ErrNoRows) {
		return ThemeVersion{}, ErrNotFound
	}
	if err != nil {
		return ThemeVersion{}, err
	}
	v.IsCurrent = isCurrent == 1
	return v, nil
}
```

(SQLite 的布尔比较 `tv.id = themes.current_version_id` 返回 0/1 整数,用 `int` 扫描;若 driver 返回 bool 需相应调整——以 `go test` 报错为准微调扫描类型。)

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/admin -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/admin/
git commit -m "feat: list theme versions and disable any version via the admin store"
```

---

### Task 3: 版本视图 httpapi + openapi

**Files:**
- Modify: `internal/httpapi/admin.go`(两个 handler + 路由 + `themeVersionData` 序列化)
- Modify: `api/openapi.yaml`

**Interfaces:**
- Consumes: Task 2 的 `Service.ThemeVersions`/`SetThemeVersionStatus`、`adminpkg.ThemeVersion`。
- Produces: `GET /api/v1/admin/themes/{themeId}/versions`、`PATCH /api/v1/admin/theme-versions/{versionId}`。

- [ ] **Step 1: 实现 handler + 路由**

`internal/httpapi/admin.go`:路由(在 `management.Patch("/themes/{themeId}", ...)` 之后):

```go
	management.Get("/themes/{themeId}/versions", h.themeVersions)
	management.Patch("/theme-versions/{versionId}", h.updateThemeVersion)
```

序列化 + handler:

```go
func themeVersionData(v adminpkg.ThemeVersion) map[string]any {
	data := map[string]any{
		"versionId": v.VersionID, "version": v.Version, "sourceRef": v.SourceRef,
		"status": v.Status, "createdAt": v.CreatedAt, "isCurrent": v.IsCurrent,
		"snapshotRefs": v.SnapshotRefs,
	}
	if v.ImportedBy != "" {
		data["importedBy"] = v.ImportedBy
	}
	return data
}

func (h *AdminHandler) themeVersions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ThemeVersions(r.Context(), actorFromRequest(r), chi.URLParam(r, "themeId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, v := range items {
		out = append(out, themeVersionData(v))
	}
	WriteJSON(w, r, http.StatusOK, out)
}

func (h *AdminHandler) updateThemeVersion(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.SetThemeVersionStatus(r.Context(), actorFromRequest(r), chi.URLParam(r, "versionId"), request.Status, middleware.GetReqID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, themeVersionData(item))
}
```

(`writeError` 已映射 `ErrDefaultThemeVersion`→409、`ErrNotFound`→404、`ErrInvalidInput`→422,复用。)

- [ ] **Step 2: openapi**

`api/openapi.yaml` 新增两端点(admin themes patch 之后):

```yaml
  /api/v1/admin/themes/{themeId}/versions:
    get:
      tags: [Admin]
      operationId: listThemeVersions
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeId'
      responses:
        '200': { $ref: '#/components/responses/ThemeVersionsResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '403': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
  /api/v1/admin/theme-versions/{versionId}:
    patch:
      tags: [Admin]
      operationId: updateThemeVersionStatus
      security: [{ sessionCookie: [] }]
      parameters:
        - in: path
          name: versionId
          required: true
          schema: { $ref: '#/components/schemas/ThemeVersionId' }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [status]
              properties:
                status: { type: string, enum: [active, disabled] }
      responses:
        '200': { $ref: '#/components/responses/ThemeVersionResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '403': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
```

schemas / responses(信封模式照抄既有惯例,先 grep 确认 `ThemeVersion`/`ThemeVersionEnvelope` 是否已存在):

```yaml
    ThemeVersion:
      type: object
      required: [versionId, version, sourceRef, status, createdAt, isCurrent, snapshotRefs]
      properties:
        versionId: { $ref: '#/components/schemas/ThemeVersionId' }
        version: { type: string }
        sourceRef: { type: string }
        status: { type: string, enum: [active, disabled] }
        createdAt: { $ref: '#/components/schemas/Timestamp' }
        importedBy: { type: string }
        isCurrent: { type: boolean }
        snapshotRefs: { type: integer }
```

`ThemeVersionsEnvelope`(data 为 array of ThemeVersion)、`ThemeVersionEnvelope`(data 为单个 ThemeVersion),`ThemeVersionsResponse`/`ThemeVersionResponse` 按既有 `<X>Response`→`<X>Envelope` 惯例新增。

- [ ] **Step 3: 编译与既有测试回归**

Run: `go build ./... && go test ./internal/httpapi ./internal/admin -race && make test-contract`
Expected: PASS(契约测试此前未打这两个新端点,不受影响;若 route 快照类断言存在需按新路由更新)

- [ ] **Step 4: Commit**

```bash
git add internal/httpapi/admin.go api/openapi.yaml
git commit -m "feat: expose theme version listing and version-level kill switch endpoints"
```

---

### Task 4: GitHub 更新检查(themeimport)

**Files:**
- Modify: `internal/themeimport/github.go`(`ResolveHeadSHA` 导出)
- Modify: `internal/themeimport/service.go`(`CheckUpdate` + `UpdateStatus`)
- Test: `internal/themeimport/github_test.go`、`internal/themeimport/service_test.go`(追加)

**Interfaces:**
- Consumes: 既有 `parseRepoURL`、`resolveSHA`、`shaPattern`、`ErrUpstream`/`ErrHostNotAllowed`;`themes.Store`。
- Produces:
  - `func (c *GitHubClient) ResolveHeadSHA(ctx context.Context, rawURL, ref string) (string, error)`
  - `type UpdateStatus struct { SourceType string; HasUpdate bool; CurrentSha, LatestSha string }`
  - `func (s *Service) CheckUpdate(ctx context.Context, ownerID, themeID string) (UpdateStatus, error)`

- [ ] **Step 1: 写失败测试**

`internal/themeimport/github_test.go` 追加(复用既有 `publicResolver`/`roundTripFunc`/`respond`):

```go
func TestResolveHeadSHAResolvesWithoutDownload(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" {
			t.Fatalf("must only hit api.github.com, got %s", r.URL.Host)
		}
		if r.URL.Path != "/repos/alice/lilac/commits/HEAD" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		return respond(200, `{"sha":"`+strings.Repeat("a", 40)+`"}`), nil
	})
	client := NewGitHubClient(publicResolver("api.github.com"), transport, nil, "")
	sha, err := client.ResolveHeadSHA(context.Background(), "https://github.com/alice/lilac", "")
	if err != nil || sha != strings.Repeat("a", 40) {
		t.Fatalf("sha = %q, err = %v", sha, err)
	}
}

func TestResolveHeadSHARejectsDisallowedHost(t *testing.T) {
	client := NewGitHubClient(publicResolver(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("must not reach network")
		return nil, nil
	}), nil, "")
	if _, err := client.ResolveHeadSHA(context.Background(), "https://evil.example.com/a/b", ""); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err = %v, want ErrHostNotAllowed", err)
	}
}
```

`internal/themeimport/service_test.go` 追加(复用既有 `newServiceDB`、`sampleZip`;github 主题种子:先 ImportGitHub 一个假 tarball 装一个 github 主题,再改 status):

```go
func TestCheckUpdateDetectsNewCommit(t *testing.T) {
	// 用假 transport 装一个 github 主题(source_ref = 旧 sha),再让 check 返回新 sha。
	service, _ := newServiceDB(t)
	tarball := makeSampleTarGz(t)
	oldSha := strings.Repeat("a", 40)
	newSha := strings.Repeat("b", 40)
	service.github = NewGitHubClient(publicResolver("api.github.com", "codeload.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.github.com":
			return respond(200, `{"sha":"`+oldSha+`"}`), nil
		case "codeload.github.com":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(tarball)), Header: http.Header{}}, nil
		}
		t.Fatalf("host %s", r.URL.Host)
		return nil, nil
	}), nil, "")
	installed, err := service.ImportGitHub(context.Background(), "usr_svc_0001", "https://github.com/e2e/lilac", "main")
	if err != nil {
		t.Fatalf("ImportGitHub: %v", err)
	}
	// 现在让 api 返回新 sha。
	service.github = NewGitHubClient(publicResolver("api.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return respond(200, `{"sha":"`+newSha+`"}`), nil
	}), nil, "")
	status, err := service.CheckUpdate(context.Background(), "usr_svc_0001", installed.ThemeID)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.SourceType != "github" || !status.HasUpdate || status.CurrentSha != oldSha || status.LatestSha != newSha {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestCheckUpdateUploadHasNoUpstream(t *testing.T) {
	service, _ := newServiceDB(t)
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", sampleZip(t))
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.CheckUpdate(context.Background(), "usr_svc_0001", installed.ThemeID)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.SourceType != "upload" || status.HasUpdate {
		t.Fatalf("upload theme must have no update: %+v", status)
	}
}

func TestCheckUpdateForeignThemeNotFound(t *testing.T) {
	service, _ := newServiceDB(t)
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", sampleZip(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CheckUpdate(context.Background(), "usr_other", installed.ThemeID); !errors.Is(err, themes.ErrNotFound) {
		t.Fatalf("foreign check want ErrNotFound, got %v", err)
	}
}
```

(`io`/`bytes` import 若缺补上;`makeSampleTarGz` 是该文件既有辅助。)

- [ ] **Step 2: 确认失败后实现**

`internal/themeimport/github.go` 追加(复用 `parseRepoURL`/`resolveSHA`;github.com 走 API 解析 HEAD,extraHosts 无 commits API 语义 → 直接返回 ref 或报错):

```go
// ResolveHeadSHA 解析仓库某 ref 的 commit sha,但不下载 tarball——供更新检查用。
// github.com 走 api.github.com/commits;缺省 ref 用默认分支 HEAD。追加白名单主机
// 没有等价的 commits API,直接把 ref 原样返回(调用方据此判断是否要求显式 ref)。
func (c *GitHubClient) ResolveHeadSHA(ctx context.Context, rawURL, ref string) (string, error) {
	owner, repo, host, err := parseRepoURL(rawURL, c.extraHosts)
	if err != nil {
		return "", err
	}
	if host != "github.com" {
		if strings.TrimSpace(ref) == "" {
			return "", fmt.Errorf("%w: 该主机需要显式 ref", ErrHostNotAllowed)
		}
		return ref, nil
	}
	refPath := "HEAD"
	if strings.TrimSpace(ref) != "" {
		refPath = ref
	}
	return c.resolveSHA(ctx, owner, repo, refPath)
}
```

`internal/themeimport/service.go` 追加:

```go
// UpdateStatus 是更新检查结果。upload 来源无 upstream,HasUpdate 恒 false。
type UpdateStatus struct {
	SourceType string `json:"sourceType"`
	HasUpdate  bool   `json:"hasUpdate"`
	CurrentSha string `json:"currentSha"`
	LatestSha  string `json:"latestSha"`
}

// CheckUpdate 检查某 owner 的私有主题有无 upstream 新版(只查不升)。
// 仅 github 来源做网络解析;upload 来源直接返回无更新。
func (s *Service) CheckUpdate(ctx context.Context, ownerID, themeID string) (UpdateStatus, error) {
	sourceType, sourceURL, currentRef, err := s.store.PrivateThemeSource(ctx, ownerID, themeID)
	if err != nil {
		return UpdateStatus{}, err
	}
	if sourceType != "github" {
		return UpdateStatus{SourceType: sourceType}, nil
	}
	latest, err := s.github.ResolveHeadSHA(ctx, sourceURL, "")
	if err != nil {
		return UpdateStatus{}, err
	}
	return UpdateStatus{SourceType: "github", HasUpdate: latest != currentRef, CurrentSha: currentRef, LatestSha: latest}, nil
}
```

`internal/themes/store.go` 加一个只读查询(供 service 拿来源三元组;私有归属校验在此,非本人/不存在 → `ErrNotFound`):

```go
// PrivateThemeSource 返回某 owner 私有主题的来源类型、仓库 URL 与当前版本 source_ref。
// 非本人或不存在统一 ErrNotFound(防枚举)。
func (s *Store) PrivateThemeSource(ctx context.Context, ownerID, themeID string) (sourceType, sourceURL, currentRef string, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT themes.source_type, themes.source_url, COALESCE(tv.source_ref, '')
		FROM themes
		LEFT JOIN theme_versions tv ON tv.id = themes.current_version_id
		WHERE themes.id = ? AND themes.scope = 'private' AND themes.owner_id = ?`,
		themeID, ownerID).Scan(&sourceType, &sourceURL, &currentRef)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrNotFound
	}
	return sourceType, sourceURL, currentRef, err
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test ./internal/themeimport ./internal/themes -race`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/themeimport/ internal/themes/store.go
git commit -m "feat: check github themes for upstream updates without downloading"
```

---

### Task 5: check-update 端点 + 限流 + openapi

**Files:**
- Modify: `internal/httpapi/themeimport.go`(handler + 路由)、`internal/httpapi/rate_limit.go`
- Modify: `api/openapi.yaml`

**Interfaces:**
- Consumes: Task 4 的 `Service.CheckUpdate`/`UpdateStatus`;`SessionFromContext`。
- Produces: `POST /api/v1/me/themes/{themeId}/check-update`。

- [ ] **Step 1: handler + 路由**

`internal/httpapi/themeimport.go` 的 `MountProtected` 追加:

```go
	router.Post("/me/themes/{themeId}/check-update", h.checkUpdate)
```

handler:

```go
func (h *ThemeImportHandler) checkUpdate(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	status, err := h.service.CheckUpdate(r.Context(), session.User.ID, chi.URLParam(r, "themeId"))
	if err != nil {
		switch {
		case errors.Is(err, themes.ErrNotFound):
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "主题不存在", nil)
		case errors.Is(err, themeimport.ErrHostNotAllowed):
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "仓库地址不被允许", err)
		case errors.Is(err, themeimport.ErrUpstream):
			WriteError(w, r, http.StatusBadGateway, "UPSTREAM_ERROR", "上游仓库检查失败", err)
		default:
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "检查更新失败", nil)
		}
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"sourceType": status.SourceType, "hasUpdate": status.HasUpdate,
		"currentSha": status.CurrentSha, "latestSha": status.LatestSha,
	})
}
```

(import 补 `errors`、`themes`、`themeimport` 若缺。)

`internal/httpapi/rate_limit.go` 规则表(validate 那条之后)追加:

```go
		{http.MethodPost, func(path string) bool {
			return strings.HasPrefix(path, "/api/v1/me/themes/") && strings.HasSuffix(path, "/check-update")
		}, 20, time.Hour},
```

- [ ] **Step 2: openapi**

```yaml
  /api/v1/me/themes/{themeId}/check-update:
    post:
      tags: [Themes]
      operationId: checkThemeUpdate
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeId'
      responses:
        '200': { $ref: '#/components/responses/ThemeUpdateStatusResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
        '502': { $ref: '#/components/responses/ErrorResponse' }
```

schema + 信封:

```yaml
    ThemeUpdateStatus:
      type: object
      required: [sourceType, hasUpdate, currentSha, latestSha]
      properties:
        sourceType: { type: string, enum: [builtin, github, upload] }
        hasUpdate: { type: boolean }
        currentSha: { type: string }
        latestSha: { type: string }
```

`ThemeUpdateStatusEnvelope`/`ThemeUpdateStatusResponse` 按既有惯例新增。

- [ ] **Step 3: 编译与回归**

Run: `go build ./... && go test ./internal/httpapi -race && make test-contract`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/httpapi/ api/openapi.yaml
git commit -m "feat: expose the owner theme update-check endpoint with rate limiting"
```

---

### Task 6: 契约测试与 mock

**Files:**
- Modify: `tests/contract/api_contract_test.go`
- Modify: `web/src/api/mock-handlers.ts`、`web/tests/mock-contract.test.ts`

**Interfaces:**
- Consumes: Task 3/5 的端点;既有 `admin`/`user` 契约客户端与 B1 导入的私有主题。
- Produces: 无。

- [ ] **Step 1: 契约断言**

`tests/contract/api_contract_test.go` 新 t.Run(放「管理端读取」之后):

```go
	t.Run("主题版本视图与版本级 kill switch", func(t *testing.T) {
		// slate(默认)版本列表:一条、当前、active。
		list := admin.call(t, http.MethodGet, "/api/v1/admin/themes/slate/versions", nil)
		mustStatus(t, list, http.StatusOK, "slate 版本列表")

		// sakura 版本列表 → 取 versionId。
		sakuraList := admin.call(t, http.MethodGet, "/api/v1/admin/themes/sakura/versions", nil)
		mustStatus(t, sakuraList, http.StatusOK, "sakura 版本列表")
		items, _ := sakuraList.json["data"].([]any)
		if len(items) == 0 {
			t.Fatal("sakura 应至少一个版本")
		}
		first, _ := items[0].(map[string]any)
		versionID, _ := first["versionId"].(string)
		if versionID == "" {
			t.Fatal("缺 versionId")
		}

		// 停用 sakura 版本 → 200 disabled。
		disabled := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "disabled"})
		mustStatus(t, disabled, http.StatusOK, "停用 sakura 版本")
		if got := stringField(t, disabled.data(), "status", "版本状态"); got != "disabled" {
			t.Fatalf("status = %q", got)
		}

		// 停用 slate(默认)当前版本 → 409。
		slateItems, _ := list.json["data"].([]any)
		slateFirst, _ := slateItems[0].(map[string]any)
		slateVersion, _ := slateFirst["versionId"].(string)
		rejected := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+slateVersion,
			map[string]any{"status": "disabled"})
		mustStatus(t, rejected, http.StatusConflict, "停用默认版本被拒")

		// 恢复 sakura。
		restored := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "active"})
		mustStatus(t, restored, http.StatusOK, "恢复 sakura 版本")

		// 未知版本 → 404。
		missing := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/v00000000000000000000000000000000",
			map[string]any{"status": "disabled"})
		mustStatus(t, missing, http.StatusNotFound, "未知版本 404")
	})
```

注意:`sakuraList.json["data"]` 是数组(handler 直接 `WriteJSON` 数组);`stringField`/`data()` 已有。若既有契约流程此前对 sakura 版本状态有断言,本段结尾已恢复 active。

check-update 契约(在既有「主题导入与私有安装」t.Run 内,导入私有主题之后追加——用 zip 导入的 upload 主题走无网络分支):

```go
		// upload 主题检查更新:无 upstream,hasUpdate=false(无网络)。
		checkUpd := user.call(t, http.MethodPost, "/api/v1/me/themes/"+importedID+"/check-update", nil)
		mustStatus(t, checkUpd, http.StatusOK, "检查更新(upload)")
		if hasUpdate, _ := checkUpd.data()["hasUpdate"].(bool); hasUpdate {
			t.Fatal("upload 主题不应有更新")
		}
```

(`importedID` 是该 t.Run 里 zip 导入返回的私有主题 id。)

- [ ] **Step 2: mock**

`web/src/api/mock-handlers.ts`:
1. admin 版本列表 `GET /admin/themes/{id}/versions` → 返回一个固定版本数组(一条,isCurrent true,status active,snapshotRefs 0);
2. `PATCH /admin/theme-versions/{versionId}` → 回显 `{versionId, status, ...}` 200;
3. `POST /me/themes/{id}/check-update` → 返回 `{sourceType:'github', hasUpdate:true, currentSha:'aaa...', latestSha:'bbb...'}`(mock 固定有更新,便于前端演示)。
先 grep 确认这些路径不被既有 mock 正则误吞(`/me/themes/import`、`/admin/themes` 的处理),按需在正确区段加分支。

`web/tests/mock-contract.test.ts` cases 追加(path key 与 openapi 一致):

```ts
  { name: '主题版本列表', path: '/api/v1/admin/themes/{themeId}/versions', method: 'get', status: '200', url: '/api/v1/admin/themes/slate/versions' },
  { name: '版本级状态', path: '/api/v1/admin/theme-versions/{versionId}', method: 'patch', status: '200', url: '/api/v1/admin/theme-versions/v00000000000000000000000000000001', init: { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status: 'active' }) } },
  { name: '检查更新', path: '/api/v1/me/themes/{themeId}/check-update', method: 'post', status: '200', url: '/api/v1/me/themes/thm_mock_priv_1/check-update', init: { method: 'POST' } },
```

(check-update 的 mock url 用一个 mock 私有主题 id;若 mock 私有主题数组为空,mock handler 对该路径直接返回固定 UpdateStatus 即可,不依赖具体主题存在。)

- [ ] **Step 3: 运行**

Run: `make test-contract && make test-mock && make check`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tests/contract/ web/src/ web/tests/
git commit -m "test: cover theme version view and update check in contract and mock"
```

---

### Task 7: 前端——版本视图 UI + 检查更新按钮

**Files:**
- Modify: `web/src/api/admin.ts`、`web/src/api/themes.ts`、`web/src/api/types.ts`、`web/src/hooks/useQueries.ts`
- Modify: `web/src/pages/admin/themes/page.tsx`(版本展开)
- Modify: `web/src/pages/app/themes/page.tsx`(检查更新按钮)

**Interfaces:**
- Consumes: Task 3/5 端点。
- Produces: 无。

- [ ] **Step 1: API 层**

`web/src/api/types.ts` 加类型:

```ts
export interface ThemeVersionRow {
  versionId: string;
  version: string;
  sourceRef: string;
  status: 'active' | 'disabled';
  createdAt: string;
  importedBy?: string;
  isCurrent: boolean;
  snapshotRefs: number;
}

export interface ThemeUpdateStatus {
  sourceType: 'builtin' | 'github' | 'upload';
  hasUpdate: boolean;
  currentSha: string;
  latestSha: string;
}
```

`web/src/api/admin.ts` 加:

```ts
  getThemeVersions: (themeId: string) =>
    request<ApiResponse<ThemeVersionRow[]>>(`/admin/themes/${encodeURIComponent(themeId)}/versions`),

  setThemeVersionStatus: (versionId: string, status: 'active' | 'disabled') =>
    request<ApiResponse<ThemeVersionRow>>(`/admin/theme-versions/${encodeURIComponent(versionId)}`, { method: 'PATCH', body: { status } }),
```

`web/src/api/themes.ts` 加:

```ts
  checkUpdate: (themeId: string) =>
    request<ApiResponse<ThemeUpdateStatus>>(`/me/themes/${encodeURIComponent(themeId)}/check-update`, { method: 'POST' }),
```

(import ThemeVersionRow/ThemeUpdateStatus/ApiResponse。)

- [ ] **Step 2: admin 版本视图**

`web/src/pages/admin/themes/page.tsx`:主题卡加「查看版本」展开(点击 → `adminApi.getThemeVersions(pkg.id)`,渲染版本表:sha 短号 `sourceRef.slice(0,7)`、status 徽章、`snapshotRefs` 引用数、`isCurrent` 标记;每个 `!isCurrent` 版本带停用/启用按钮 → `adminApi.setThemeVersionStatus` + invalidate)。当前版本沿用卡片既有 B2a 停用控件,不重复。用局部 state 存展开的 themeId 与其版本列表;错误用 toast + ApiError.detail。

- [ ] **Step 3: app 检查更新**

`web/src/pages/app/themes/page.tsx`:「我的主题」卡片 `meta.sourceType === 'github'` 时加「检查更新」按钮 → `themesApi.checkUpdate(pkg.id)` → `hasUpdate` 则 toast「有新版本,可点重新拉取更新」并高亮既有升级入口;否则 toast「已是最新」。upload 主题不显示该按钮。

- [ ] **Step 4: 类型检查与冒烟**

Run: `make check && make test-mock`
然后 `cd web && VITE_ENABLE_API_MOCKS=true npm run dev`:`/admin/themes` 展开版本列表、停用历史版本;`/app/themes` 私有 github 主题点检查更新(mock 返回有更新)。六态(加载/空/错误/移动/键盘/暗色)记录进报告,关 dev server。
Expected: 全绿。

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat: admin theme version panel and owner update-check button"
```

---

### Task 8: E2E + 全量验证 + 交付

**Files:**
- Modify: `tests/e2e/specs/admin.spec.ts`

- [ ] **Step 1: E2E 用例**

`tests/e2e/specs/admin.spec.ts` 追加(照既有 admin 用例范式;E2E 走真实二进制,内置主题有版本):

```ts
  test('管理员查看并停用主题历史版本', async ({ page }) => {
    await page.goto('/admin/themes');
    // 展开 sakura 的版本列表(定位以 Task 7 实际渲染文案为准,用 role+name)。
    const sakuraCard = page.getByRole('button', { name: '设为默认主题 Sakura' }).locator('xpath=../..');
    await sakuraCard.getByRole('button', { name: /查看版本/ }).click();
    // 版本行出现(当前版本标记)。
    await expect(sakuraCard.getByText(/当前/)).toBeVisible({ timeout: 10000 });
  });
```

注意:定位器以 Task 7 实际渲染为准(用 role+name);sakura 单版本且为当前版本,当前版本沿用卡片停用控件,因此该用例只断言"版本面板可展开且显示当前版本",不点历史版本停用(种子无历史版本)。若要断言停用,需先制造第二个版本——超出 E2E 便利范围,留给契约/单测。

- [ ] **Step 2: 全量门槛**

Run:
```bash
make check && go test -race ./... && make build && make test-contract && make test-mock && make e2e
```
Expected: 全部 PASS。任一失败回对应任务修复,不带失败推送。

- [ ] **Step 3: 推送与 PR**

```bash
git push -u origin feat/theme-maintenance-b2b
gh pr create --title "feat: theme maintenance — version view, update check, tombstone reclaim (B2b)" --body "$(cat <<'EOF'
## Summary
- 历史版本视图 + 版本级 kill switch:GET /admin/themes/{id}/versions(含 sha/状态/快照引用数/当前标记)+ PATCH /admin/theme-versions/{versionId};唯一守卫拒绝停用默认主题当前版本;停用非当前版本→公开页 410 回落基线
- GitHub 更新检查(手动):导出 ResolveHeadSHA(只解析不下载),POST /me/themes/{id}/check-update 让 owner 查自己私有 github 主题有无新版(只查不升);netguard/token 作用域全复用;限流 20/h
- 墓碑回收:InstallPrivate 因配额满要拒绝前,自动回收无快照引用的私有墓碑(抽 deleteThemeTx 与 UninstallPrivate 共享);owner 无感
- 契约/mock/E2E 覆盖

设计文档:docs/superpowers/specs/2026-07-25-theme-maintenance-b2b-design.md

## 不在范围(B2c / 独立)
目录审核(私有→catalog 晋升)、starter 仓库、后台定时更新轮询、账号删除前置清理。

## Test plan
- [x] make check / go test -race ./... / make build
- [x] make test-contract / make test-mock / make e2e
- [x] /admin/themes 与 /app/themes 浏览器六态冒烟

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr merge --auto --rebase
```

- [ ] **Step 4: 确认 CI**

Run: `gh pr checks --watch`(或后台轮询)
Expected: `verify`、`e2e`、`container` 全绿自动合并;`deploy-production` 随 main 自动执行(rsync 已修,CD 正常)。
