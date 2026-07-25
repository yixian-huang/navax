# 目录审核(子项目 B2c)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the private-theme → official-catalog request/approve workflow (mirroring `internal/subdomains`'s apply-approve pattern), plus two bundled fixes named in the roadmap: persisting the imported git ref (fixes a false "has update" for non-default-branch/tag imports) and an `openapi.yaml` 422 gap on `PATCH /admin/theme-versions/{versionId}`.

**Architecture:** New domain package `internal/themecatalog` owns a `theme_catalog_requests` table and a `pending→approved|rejected` state machine (CAS updates, no explicit version column — same pattern as `internal/subdomains`). Approval is a single transaction that also flips `themes.scope='catalog', owner_id=NULL` (the trigger-enforced pairing). `internal/themes.InstallPrivate` gains a lock: while a theme has a pending catalog request, re-importing (upgrading) it is rejected — this guarantees the content an admin approves is the content that gets promoted. The import-ref bugfix threads a new `themes.source_git_ref` column through `internal/themes` and `internal/themeimport` so `CheckUpdate` re-resolves the *actual* imported ref instead of always comparing against the default branch. Frontend: a "提交官方目录" affordance on the owner's private-theme cards (`web/src/pages/app/themes/page.tsx`) and a new admin review queue section on `web/src/pages/admin/themes/page.tsx`.

**Tech Stack:** Go 1.25 (`net/http` + chi, `database/sql` + `modernc.org/sqlite`), React 19 + TypeScript + TanStack Query, `api/openapi.yaml` as contract source of truth.

## Global Constraints

- Every task must leave `go build ./...` and the relevant `go test` green — this is a shared modular monolith, not independently deployable packages.
- `internal/httpapi/` stays routing/DTO/serialization only; business logic and transaction boundaries live in domain packages (`internal/themecatalog`, `internal/themes`, `internal/themeimport`).
- `api/openapi.yaml` is the source of truth — any request/response shape change lands there in the same task as the Go change, never a follow-up.
- Chinese for all user-facing strings (toasts, labels, error messages, audit-adjacent text); Go identifiers, JSON field names, and commit subjects stay English.
- Conventional Commit subjects (`feat:`, `fix:`, `test:`, `docs:`).
- No new dependencies, no ORM, no ID/ref-count based revocation of `approved` requests — promotion is one-directional (down-grading a catalog theme back to private is out of scope; use the existing kill switch / `enabled` toggle instead).
- `gofmt` and `go vet ./...` must be clean (`make check` covers this alongside the frontend type-check/ESLint/mock-contract guard).

---

## Task 1: `openapi.yaml` 422 gap on theme-version status PATCH

**Files:**
- Modify: `api/openapi.yaml:1467-1488` (`/api/v1/admin/theme-versions/{versionId}` patch responses)
- Modify: `tests/contract/api_contract_test.go` (extend the existing "主题版本视图与版本级 kill switch" subtest, after the "恢复 sakura 版本" block, before "未知版本 → 404")

**Interfaces:**
- Consumes: nothing new — `internal/admin.Service.SetThemeVersionStatus` (`internal/admin/service.go:459-465`) already returns `ErrInvalidInput` for a `status` value outside `active`/`disabled`, and `internal/httpapi/admin.go:473-474` already maps `ErrInvalidInput` to 422 `VALIDATION_FAILED`. This task only documents the already-reachable response.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Add the missing 422 to the endpoint's contract**

In `api/openapi.yaml`, find the `/api/v1/admin/theme-versions/{versionId}` `patch` block:

```yaml
  /api/v1/admin/theme-versions/{versionId}:
    patch:
      tags: [Admin]
      operationId: updateThemeVersionStatus
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeVersionId'
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

Add a `422` line after `409`:

```yaml
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
```

- [ ] **Step 2: Write the contract test assertion (will fail against the *old* spec if run through the validator, but here it's asserting server behavior — write it now, it should already pass since the server-side 422 already works; this step exists to lock the behavior in before the spec catches up)**

In `tests/contract/api_contract_test.go`, inside `t.Run("主题版本视图与版本级 kill switch", ...)`, insert right after the `restored := ...; mustStatus(t, restored, http.StatusOK, "恢复 sakura 版本")` lines and before the `// 未知版本 → 404.` comment:

```go
		// 非法 status 值 → 422。
		invalidStatus := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "bogus"})
		mustStatus(t, invalidStatus, http.StatusUnprocessableEntity, "非法版本状态 422")
```

- [ ] **Step 3: Run the contract test**

Run: `go test ./tests/contract/... -run TestAPIContract -v 2>&1 | grep -A3 "主题版本视图"`
Expected: PASS (the server-side 422 already worked; this proves it, and the openapi validator inside the contract harness — `libopenapi-validator` — will now find `422` documented for this response since Step 1 added it, so the harness doesn't flag it as an undocumented status code).

- [ ] **Step 4: Commit**

```bash
git add api/openapi.yaml tests/contract/api_contract_test.go
git commit -m "$(cat <<'EOF'
docs: document the 422 response on theme-version status PATCH

SetThemeVersionStatus already rejects an invalid status with 422, but
the contract never declared it — a reachable response the spec didn't
know about.
EOF
)"
```

---

## Task 2: Fix import-ref persistence (CheckUpdate always compared against the default branch)

**Files:**
- Create: `migrations/0015_theme_source_ref.sql`
- Modify: `internal/themes/install.go:32` (`InstallPrivate` signature + both write paths)
- Modify: `internal/themes/store.go:326-337` (`PrivateThemeSource` return shape)
- Modify: `internal/themeimport/service.go:66-97,113-129` (`install`, `ImportZip`, `ImportGitHub`, `CheckUpdate`)
- Modify: `internal/themes/install_test.go` (9 call sites — see Step 3)
- Modify: `internal/themeimport/service_test.go` (new regression test)

**Interfaces:**
- Produces: `Store.InstallPrivate(ctx, ownerID, slug, sourceType, sourceURL, sourceRef, sourceGitRef string, quota int, compile func(string) (Compiled, error), now time.Time) (InstalledTheme, error)` — one new `sourceGitRef` parameter, inserted right after `sourceRef`.
- Produces: `Store.PrivateThemeSource(ctx, ownerID, themeID string) (sourceType, sourceURL, currentRef, gitRef string, err error)` — one new return value, `gitRef`, inserted after `currentRef`.
- Consumes (Task 5, the upgrade-lock task): the same `InstallPrivate` signature — Task 5 only adds a check inside the existing `default:` branch, it does not touch the signature again.

- [ ] **Step 1: Write the failing regression test**

In `internal/themeimport/service_test.go`, add this test after `TestCheckUpdateDetectsNewCommit` (which ends around line 308):

```go
// TestCheckUpdateUsesImportedRefNotDefaultBranch 是本次 bugfix 的回归测试:
// 从非默认分支导入后,CheckUpdate 必须复用该分支名重新解析 HEAD,而不是恒定
// 对比默认分支——否则任何非默认分支/tag 导入的主题会永远显示"有更新"。假
// transport 只认 .../commits/release%2F2.0 这一条路径,修复前 CheckUpdate
// 恒传空 ref 会打到 .../commits/HEAD,命中 t.Fatalf。
func TestCheckUpdateUsesImportedRefNotDefaultBranch(t *testing.T) {
	service, _, _ := newServiceDB(t)
	tarball := makeSampleTarGz(t)
	branchSha := strings.Repeat("c", 40)
	service.github = NewGitHubClient(publicResolver("api.github.com", "codeload.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "codeload.github.com":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(tarball)), Header: http.Header{}}, nil
		case strings.HasSuffix(r.URL.Path, "/commits/release%2F2.0"):
			return respond(200, `{"sha":"`+branchSha+`"}`), nil
		}
		t.Fatalf("unexpected resolve path %s", r.URL.Path)
		return nil, nil
	}), nil, "")
	installed, err := service.ImportGitHub(context.Background(), "usr_svc_0001", "https://github.com/e2e/lilac", "release/2.0")
	if err != nil {
		t.Fatalf("ImportGitHub: %v", err)
	}

	service.github = NewGitHubClient(publicResolver("api.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/commits/release%2F2.0") {
			t.Fatalf("CheckUpdate resolved wrong ref, path = %s (want .../commits/release%%2F2.0)", r.URL.Path)
		}
		return respond(200, `{"sha":"`+branchSha+`"}`), nil
	}), nil, "")
	status, err := service.CheckUpdate(context.Background(), "usr_svc_0001", installed.ThemeID)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.HasUpdate {
		t.Fatalf("same branch sha compared against itself must report no update: %+v", status)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/themeimport/... -run TestCheckUpdateUsesImportedRefNotDefaultBranch -v`
Expected: FAIL — the fake transport's `t.Fatalf("unexpected resolve path %s", ...)` fires with a path of `/repos/e2e/lilac/commits/HEAD` (proving `CheckUpdate` ignores the imported ref today).

- [ ] **Step 3: Migration — add `themes.source_git_ref`**

Create `migrations/0015_theme_source_ref.sql`:

```sql
-- Persist the exact git ref (branch/tag name; empty string = default branch)
-- used at import time, separate from theme_versions.source_ref (the resolved
-- commit sha). Without this, CheckUpdate has no way to know which ref to
-- re-resolve and always compares against the default branch HEAD, producing
-- a false "has update" for every theme imported from a non-default branch
-- or a tag.
ALTER TABLE themes ADD COLUMN source_git_ref TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 4: Thread `sourceGitRef` through `internal/themes/install.go`**

In `internal/themes/install.go`, change the `InstallPrivate` signature (line 32):

```go
func (s *Store) InstallPrivate(ctx context.Context, ownerID, slug, sourceType, sourceURL, sourceRef, sourceGitRef string, quota int, compile func(themeID string) (Compiled, error), now time.Time) (InstalledTheme, error) {
```

In the new-row INSERT (lines 71-79), add the column and value:

```go
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
				                    created_at, updated_at, slug, scope, owner_id, source_type, source_url, source_git_ref, spec_version)
				VALUES (?, ?, ?, ?, ?, ?, '', 1, 0, ?, ?, ?, 'private', ?, ?, ?, ?, ?)`,
				themeID, compiled.Manifest.Name, compiled.Manifest.Version, compiled.Manifest.Author,
				compiled.Manifest.Description, compiled.Manifest.Mode, dbTime(now), dbTime(now),
				slug, ownerID, sourceType, sourceURL, sourceGitRef, compiled.Manifest.SpecVersion); err != nil {
				return fmt.Errorf("insert private theme: %w", err)
			}
```

In the upgrade branch's UPDATE (lines 101-102), add the column:

```go
			if _, err := tx.ExecContext(ctx, `UPDATE themes SET source_url = ?, source_git_ref = ?, enabled = 1, updated_at = ? WHERE id = ?`,
				sourceURL, sourceGitRef, dbTime(now), themeID); err != nil {
				return err
			}
```

- [ ] **Step 5: Update `PrivateThemeSource` in `internal/themes/store.go`**

Replace the function at lines 326-337:

```go
// PrivateThemeSource 返回某 owner 私有主题的来源类型、仓库 URL、当前版本
// source_ref(已解析 commit sha)与导入时使用的 git ref(分支/tag 名,空串
// 代表默认分支)。非本人或不存在统一 ErrNotFound(防枚举)。
func (s *Store) PrivateThemeSource(ctx context.Context, ownerID, themeID string) (sourceType, sourceURL, currentRef, gitRef string, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT themes.source_type, themes.source_url, COALESCE(tv.source_ref, ''), themes.source_git_ref
		FROM themes
		LEFT JOIN theme_versions tv ON tv.id = themes.current_version_id
		WHERE themes.id = ? AND themes.scope = 'private' AND themes.owner_id = ?`,
		themeID, ownerID).Scan(&sourceType, &sourceURL, &currentRef, &gitRef)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", "", ErrNotFound
	}
	return sourceType, sourceURL, currentRef, gitRef, err
}
```

- [ ] **Step 6: Update the 9 `InstallPrivate` call sites in `internal/themes/install_test.go`**

Every call currently ends its positional args with `..., sourceRef, quota, func...`. Insert a new `""` argument between `sourceRef` and `quota` at each of these lines: `install_test.go:23` (inside `installSample`), `67`, `95`, `101`, `123`, `256`, `308`, `318`, `367`.

Example for line 23 (`installSample` helper) — before:
```go
	installed, err := store.InstallPrivate(t.Context(), ownerID, slug, "upload", "", "digest-"+slug, quota,
```
after:
```go
	installed, err := store.InstallPrivate(t.Context(), ownerID, slug, "upload", "", "digest-"+slug, "", quota,
```

Apply the identical `, ""` insertion (right after the `sourceRef` string literal, right before `quota` / `2` / `10`) at the other 8 call sites. None of them pass a non-empty git ref today — this bugfix only changes what gets *persisted and re-used later*, not any existing test's assertions.

- [ ] **Step 7: Thread the ref through `internal/themeimport/service.go`**

Change `install` (lines 66-69):

```go
func (s *Service) install(ctx context.Context, ownerID string, pkg themes.Package, sourceType, sourceURL, sourceRef, sourceGitRef string) (themes.InstalledTheme, error) {
	return s.store.InstallPrivate(ctx, ownerID, pkg.Manifest.ID, sourceType, sourceURL, sourceRef, sourceGitRef, s.quota,
		func(themeID string) (themes.Compiled, error) { return themes.Compile(pkg, themeID) }, s.now().UTC())
}
```

Change `ImportZip`'s call (line 77) — upload has no git ref concept:

```go
	return s.install(ctx, ownerID, pkg, "upload", "", themes.ContentDigest(zipData), "")
```

Change `ImportGitHub`'s call (line 96) — forward the caller's `ref`:

```go
	return s.install(ctx, ownerID, pkg, "github", fetched.CanonicalURL, fetched.SHA, ref)
```

Change `CheckUpdate` (lines 113-129) to use the persisted ref instead of `""`:

```go
func (s *Service) CheckUpdate(ctx context.Context, ownerID, themeID string) (UpdateStatus, error) {
	sourceType, sourceURL, currentRef, gitRef, err := s.store.PrivateThemeSource(ctx, ownerID, themeID)
	if err != nil {
		return UpdateStatus{}, err
	}
	if sourceType != "github" {
		return UpdateStatus{SourceType: sourceType}, nil
	}
	latest, err := s.github.ResolveHeadSHA(ctx, sourceURL, gitRef)
	if err != nil {
		if errors.Is(err, ErrUpdateCheckUnsupported) {
			return UpdateStatus{SourceType: "github", HasUpdate: false, CurrentSha: currentRef}, nil
		}
		return UpdateStatus{}, err
	}
	return UpdateStatus{SourceType: "github", HasUpdate: latest != currentRef, CurrentSha: currentRef, LatestSha: latest}, nil
}
```

- [ ] **Step 8: Run the full themes + themeimport suites**

Run: `go test ./internal/themes/... ./internal/themeimport/... -v 2>&1 | tail -60`
Expected: PASS, including `TestCheckUpdateUsesImportedRefNotDefaultBranch` and all 9 updated `install_test.go` tests.

- [ ] **Step 9: `go build ./...` sanity check (no other call sites missed)**

Run: `go build ./...`
Expected: no errors. (There are exactly two production call sites of `InstallPrivate` — `internal/themes/install.go` itself and `internal/themeimport/service.go`'s `install` helper — both updated above.)

- [ ] **Step 10: Commit**

```bash
git add migrations/0015_theme_source_ref.sql internal/themes/install.go internal/themes/store.go \
  internal/themes/install_test.go internal/themeimport/service.go internal/themeimport/service_test.go
git commit -m "$(cat <<'EOF'
fix: persist the imported git ref so update checks compare the right branch

CheckUpdate always resolved the default branch HEAD regardless of which
ref a theme was actually imported from, so anything imported off a
non-default branch or a tag showed a false "has update" forever.
EOF
)"
```

---

## Task 3: Migration — `theme_catalog_requests` table

**Files:**
- Create: `migrations/0016_theme_catalog_requests.sql`

**Interfaces:**
- Produces: table `theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, reviewer_id, applied_at, reviewed_at)` — consumed by Task 4 (`internal/themecatalog`) and Task 5 (the upgrade lock in `internal/themes/install.go`).

- [ ] **Step 1: Write the migration**

Create `migrations/0016_theme_catalog_requests.sql`:

```sql
-- Private-theme → official-catalog promotion requests. Mirrors the
-- subdomain_requests apply-approve pattern (0002_short_subdomains.sql):
-- pending/approved/rejected/revoked state machine, CAS updates instead of
-- an explicit version column, one active (pending) request per theme.
--
-- owner_id is a snapshot of who applied: once approved, themes.owner_id is
-- set to NULL by the promotion itself (scope flips to 'catalog'), so this
-- column is the only place "who submitted this" survives.
--
-- version_id snapshots themes.current_version_id at submission time; the
-- approval transaction re-checks it still matches before promoting, so an
-- admin can never approve content different from what they reviewed.
CREATE TABLE theme_catalog_requests (
    id TEXT PRIMARY KEY,
    theme_id TEXT NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'revoked')),
    reason TEXT NOT NULL DEFAULT '' CHECK (length(reason) <= 300),
    version_id TEXT NOT NULL REFERENCES theme_versions(id),
    reviewer_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    applied_at TEXT NOT NULL,
    reviewed_at TEXT
);

CREATE UNIQUE INDEX idx_theme_catalog_active ON theme_catalog_requests(theme_id) WHERE status = 'pending';
CREATE INDEX idx_theme_catalog_status_time ON theme_catalog_requests(status, applied_at DESC);
```

- [ ] **Step 2: Verify the migration applies cleanly**

Run: `go test ./internal/database/... -run TestOpenAndMigrate -v`
Expected: PASS. If there's no such exact test name, instead run: `go run ./cmd/navax --help >/dev/null 2>&1; go test ./internal/themes/... -run TestNothing -v` — any package test that calls `database.OpenAndMigrate` against `:memory:` will fail loudly if the SQL is malformed. The simplest direct check:

Run: `cat <<'EOF' | go run -
package main
EOF`

(skip the inline script — instead just run the full test suite, which touches `OpenAndMigrate` in dozens of `:memory:` setups):

Run: `go test ./internal/... 2>&1 | tail -30`
Expected: PASS across the board — a broken migration fails every single package test that opens a database.

- [ ] **Step 3: Commit**

```bash
git add migrations/0016_theme_catalog_requests.sql
git commit -m "$(cat <<'EOF'
feat: add theme_catalog_requests table

Schema for the private-to-catalog promotion request/approve workflow,
mirroring subdomain_requests' apply-approve pattern.
EOF
)"
```

---

## Task 4: `internal/themecatalog` package (types, service, SQL store)

**Files:**
- Create: `internal/themecatalog/service.go`
- Create: `internal/themecatalog/sqlstore.go`
- Create: `internal/themecatalog/sqlstore_test.go`

**Interfaces:**
- Consumes: `internal/database.WithinTx(ctx, db, nil, func(tx *sql.Tx) error) error`; `internal/identity.New(prefix string) (string, error)`; table `theme_catalog_requests` and `themes` (both already exist by this point).
- Produces (for Task 6, the HTTP handler): `themecatalog.Actor{ID, Username, Role, Status string}`; `themecatalog.Request{ID, ThemeID, ThemeName, Slug, OwnerID, OwnerName, Status, Reason, VersionID, ReviewerID string; AppliedAt time.Time; ReviewedAt *time.Time}`; `themecatalog.Page{Items []Request, Page, PageSize, Total int}`; `*themecatalog.Service` with methods `Request(ctx, actor Actor, themeID, httpRequestID string) (Request, error)`, `Cancel(ctx, actor Actor, themeID, httpRequestID string) error`, `Requests(ctx, actor Actor, status string, page, pageSize int) (Page, error)`, `Review(ctx, actor Actor, requestID, decision, reason, httpRequestID string) (Request, error)`; `themecatalog.NewService(store Store) *Service`; `themecatalog.NewSQLStore(db *sql.DB) *SQLStore`; sentinel errors `ErrForbidden, ErrNotFound, ErrConflict, ErrInvalidTransition, ErrInvalidInput, ErrSlugConflict, ErrThemeNotEligible`.

- [ ] **Step 1: Write `internal/themecatalog/service.go`**

```go
// Package themecatalog owns catalog-promotion requests for private themes:
// an owner asks to publish a private theme into the official catalog, an
// admin approves or rejects. This is unrelated to internal/catalog (the
// site navigation directory/discover feed) — the shared "catalog" word
// here refers to themes.scope = 'catalog'.
package themecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrForbidden         = errors.New("admin permission required")
	ErrNotFound          = errors.New("theme catalog request not found")
	ErrConflict          = errors.New("theme catalog request conflicts with an active request")
	ErrInvalidTransition = errors.New("invalid theme catalog request status transition")
	ErrInvalidInput      = errors.New("invalid input")
	ErrSlugConflict      = errors.New("theme slug already exists in the catalog")
	ErrThemeNotEligible  = errors.New("theme is not eligible for catalog submission")
)

type Actor struct {
	ID       string
	Username string
	Role     string
	Status   string
}

type Request struct {
	ID         string
	ThemeID    string
	ThemeName  string
	Slug       string
	OwnerID    string
	OwnerName  string
	Status     string
	Reason     string
	VersionID  string
	ReviewerID string
	AppliedAt  time.Time
	ReviewedAt *time.Time
}

type Page struct {
	Items    []Request
	Page     int
	PageSize int
	Total    int
}

type CreateParams struct {
	Request
	Audit AuditRecord
}

type ReviewParams struct {
	RequestID  string
	ReviewerID string
	Decision   string
	Reason     string
	ReviewedAt time.Time
	Audit      AuditRecord
}

type AuditRecord struct {
	ID         string
	ActorID    string
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	DetailJSON string
	RequestID  string
	CreatedAt  time.Time
}

type Store interface {
	Create(context.Context, CreateParams) (Request, error)
	CancelPending(context.Context, string, string, time.Time, AuditRecord) error
	List(context.Context, string, int, int) (Page, error)
	Review(context.Context, ReviewParams) (Request, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

// Request submits themeID (must be a private theme owned by actor) for
// catalog review. Store.Create does the DB-dependent eligibility checks
// (ownership, enabled, has a current version, slug not already in the
// catalog) because only a single transactional read can answer them
// consistently.
func (s *Service) Request(ctx context.Context, actor Actor, themeID, httpRequestID string) (Request, error) {
	if actor.ID == "" || actor.Username == "" {
		return Request{}, ErrInvalidInput
	}
	themeID = strings.TrimSpace(themeID)
	if themeID == "" {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	id, err := identity.New("tcr")
	if err != nil {
		return Request{}, err
	}
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog.apply", id, map[string]any{"themeId": themeID}, httpRequestID, now)
	if err != nil {
		return Request{}, err
	}
	return s.store.Create(ctx, CreateParams{
		Request: Request{ID: id, ThemeID: themeID, OwnerID: actor.ID, Status: "pending", AppliedAt: now},
		Audit:   audit,
	})
}

// Cancel lets the owner revoke their own pending request.
func (s *Service) Cancel(ctx context.Context, actor Actor, themeID, httpRequestID string) error {
	if actor.ID == "" || actor.Username == "" {
		return ErrInvalidInput
	}
	themeID = strings.TrimSpace(themeID)
	if themeID == "" {
		return ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog.revoke", themeID, nil, httpRequestID, now)
	if err != nil {
		return err
	}
	return s.store.CancelPending(ctx, actor.ID, themeID, now, audit)
}

func (s *Service) Requests(ctx context.Context, actor Actor, status string, page, pageSize int) (Page, error) {
	if err := authorize(actor); err != nil {
		return Page{}, err
	}
	if status != "" && status != "pending" && status != "approved" && status != "rejected" && status != "revoked" {
		return Page{}, ErrInvalidInput
	}
	page, pageSize = pagination(page, pageSize)
	return s.store.List(ctx, status, page, pageSize)
}

// Review is admin-only. decision is "approve" or "reject" — there is no
// "revoke" here: once a request is approved the theme is catalog-scoped,
// and taking it back down is the existing kill switch's job, not this
// state machine's.
func (s *Service) Review(ctx context.Context, actor Actor, requestID, decision, reason, httpRequestID string) (Request, error) {
	if err := authorize(actor); err != nil {
		return Request{}, err
	}
	reason = strings.TrimSpace(reason)
	if requestID == "" || (decision != "approve" && decision != "reject") || len([]rune(reason)) > 300 {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	audit, err := newAudit(actor.ID, actor.Username, "theme_catalog."+decision, requestID, map[string]any{"reason": reason}, httpRequestID, now)
	if err != nil {
		return Request{}, err
	}
	return s.store.Review(ctx, ReviewParams{
		RequestID: requestID, ReviewerID: actor.ID, Decision: decision,
		Reason: reason, ReviewedAt: now, Audit: audit,
	})
}

func authorize(actor Actor) error {
	if actor.ID == "" || actor.Username == "" || actor.Role != "admin" || actor.Status != "active" {
		return ErrForbidden
	}
	return nil
}

func pagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func newAudit(actorID, actorName, action, targetID string, detail any, requestID string, now time.Time) (AuditRecord, error) {
	if len(requestID) > 128 {
		return AuditRecord{}, ErrInvalidInput
	}
	id, err := identity.New("aud")
	if err != nil {
		return AuditRecord{}, err
	}
	encoded := []byte("{}")
	if detail != nil {
		encoded, err = json.Marshal(detail)
		if err != nil {
			return AuditRecord{}, err
		}
	}
	return AuditRecord{
		ID: id, ActorID: actorID, ActorName: actorName, Action: action,
		TargetType: "theme_catalog_request", TargetID: targetID, DetailJSON: string(encoded),
		RequestID: requestID, CreatedAt: now,
	}, nil
}
```

- [ ] **Step 2: Write `internal/themecatalog/sqlstore.go`**

```go
package themecatalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

type SQLStore struct{ db *sql.DB }

var _ Store = (*SQLStore)(nil)

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

const requestSelect = `
	SELECT r.id, r.theme_id, t.name, t.slug, r.owner_id, u.username, r.status, r.reason,
	       r.version_id, r.reviewer_id, r.applied_at, r.reviewed_at
	FROM theme_catalog_requests r
	JOIN themes t ON t.id = r.theme_id
	JOIN users u ON u.id = r.owner_id`

func (s *SQLStore) Create(ctx context.Context, params CreateParams) (Request, error) {
	var requestID string
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var slug, scope, ownerID string
		var enabled int
		var currentVersionID sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT slug, scope, owner_id, enabled, current_version_id
			FROM themes WHERE id = ?`, params.ThemeID).Scan(&slug, &scope, &ownerID, &enabled, &currentVersionID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// 非本人/非私有统一 404,不暴露主题是否存在或归属于谁。
		if scope != "private" || ownerID != params.OwnerID {
			return ErrNotFound
		}
		if enabled == 0 || !currentVersionID.Valid || currentVersionID.String == "" {
			return ErrThemeNotEligible
		}
		var conflict int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM themes WHERE scope = 'catalog' AND slug = ?`, slug).Scan(&conflict); err == nil {
			return ErrSlugConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, applied_at)
			VALUES (?, ?, ?, 'pending', '', ?, ?)`,
			params.ID, params.ThemeID, params.OwnerID, currentVersionID.String, dbTime(params.AppliedAt)); err != nil {
			return mapSQLError(err)
		}
		if err := insertAudit(ctx, tx, params.Audit); err != nil {
			return err
		}
		requestID = params.ID
		return nil
	})
	if err != nil {
		return Request{}, err
	}
	return s.byID(ctx, requestID)
}

func (s *SQLStore) CancelPending(ctx context.Context, ownerID, themeID string, now time.Time, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var requestID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM theme_catalog_requests
			WHERE theme_id = ? AND owner_id = ? AND status = 'pending'`, themeID, ownerID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE theme_catalog_requests SET status = 'revoked', reason = '用户撤回申请', reviewed_at = ?
			WHERE id = ? AND status = 'pending'`, dbTime(now), requestID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrInvalidTransition
		}
		audit.TargetID = requestID
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) List(ctx context.Context, status string, page, pageSize int) (Page, error) {
	where := ""
	args := make([]any, 0, 3)
	if status != "" {
		where = " WHERE r.status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM theme_catalog_requests r"+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, requestSelect+where+` ORDER BY r.applied_at DESC, r.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Request, 0, pageSize)
	for rows.Next() {
		item, err := scanRequest(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// Review is the approval/rejection transaction. On approve, it re-checks
// the request's version_id still matches themes.current_version_id (the
// upgrade lock in internal/themes should make a mismatch impossible, this
// is a defensive backstop) and that the slug still has no catalog conflict
// (another request could have been approved with the same slug in the
// meantime), then flips themes.scope/owner_id in the same statement the
// scope/owner trigger requires.
func (s *SQLStore) Review(ctx context.Context, params ReviewParams) (Request, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var status, themeID, versionID string
		if err := tx.QueryRowContext(ctx, `
			SELECT status, theme_id, version_id FROM theme_catalog_requests WHERE id = ?`,
			params.RequestID).Scan(&status, &themeID, &versionID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if status != "pending" {
			return ErrInvalidTransition
		}
		targetStatus := "rejected"
		switch params.Decision {
		case "approve":
			targetStatus = "approved"
			var currentVersionID, slug sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT current_version_id, slug FROM themes WHERE id = ?`, themeID).
				Scan(&currentVersionID, &slug); err != nil {
				return err
			}
			if !currentVersionID.Valid || currentVersionID.String != versionID {
				return ErrInvalidTransition
			}
			var conflict int
			if err := tx.QueryRowContext(ctx, `
				SELECT 1 FROM themes WHERE scope = 'catalog' AND slug = ? AND id <> ?`,
				slug.String, themeID).Scan(&conflict); err == nil {
				return ErrSlugConflict
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE themes SET scope = 'catalog', owner_id = NULL, updated_at = ? WHERE id = ?`,
				dbTime(params.ReviewedAt), themeID); err != nil {
				return mapSQLError(err)
			}
		case "reject":
			// targetStatus 已是 "rejected",无需改 themes 表。
		default:
			return ErrInvalidInput
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE theme_catalog_requests
			SET status = ?, reason = ?, reviewer_id = ?, reviewed_at = ?
			WHERE id = ? AND status = 'pending'`,
			targetStatus, params.Reason, params.ReviewerID, dbTime(params.ReviewedAt), params.RequestID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrInvalidTransition
		}
		return insertAudit(ctx, tx, params.Audit)
	})
	if err != nil {
		return Request{}, err
	}
	return s.byID(ctx, params.RequestID)
}

func (s *SQLStore) byID(ctx context.Context, id string) (Request, error) {
	item, err := scanRequest(s.db.QueryRowContext(ctx, requestSelect+" WHERE r.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return item, err
}

type rowScanner interface{ Scan(...any) error }

func scanRequest(row rowScanner) (Request, error) {
	var item Request
	var appliedAt string
	var reviewedAt, reviewerID sql.NullString
	if err := row.Scan(
		&item.ID, &item.ThemeID, &item.ThemeName, &item.Slug, &item.OwnerID, &item.OwnerName,
		&item.Status, &item.Reason, &item.VersionID, &reviewerID, &appliedAt, &reviewedAt,
	); err != nil {
		return Request{}, err
	}
	var err error
	if item.AppliedAt, err = parseDBTime(appliedAt); err != nil {
		return Request{}, err
	}
	if reviewedAt.Valid {
		value, err := parseDBTime(reviewedAt.String)
		if err != nil {
			return Request{}, err
		}
		item.ReviewedAt = &value
	}
	item.ReviewerID = reviewerID.String
	return item, nil
}

type auditExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertAudit(ctx context.Context, execer auditExecer, record AuditRecord) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_id, actor_name, action, target_type, target_id, detail_json, request_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.ActorID, record.ActorName,
		record.Action, record.TargetType, record.TargetID, record.DetailJSON, record.RequestID, dbTime(record.CreatedAt))
	return err
}

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed") {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseDBTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time %q: %w", value, err)
	}
	return parsed, nil
}
```

- [ ] **Step 3: Write `internal/themecatalog/sqlstore_test.go`**

```go
package themecatalog

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
)

func insertUser(t *testing.T, db *sql.DB, id, username, email, role string, now time.Time) {
	t.Helper()
	hash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`, id, username, email, hash, role, dbTime(now), dbTime(now)); err != nil {
		t.Fatal(err)
	}
}

// insertPrivateTheme seeds a minimal private theme + one active current
// version, matching the fixture pattern in internal/admin/sqlstore_test.go.
func insertPrivateTheme(t *testing.T, db *sql.DB, themeID, ownerID, slug, versionID string, now time.Time) {
	t.Helper()
	stamp := dbTime(now)
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
			created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES (?, ?, '1.0.0', 'owner', '', 'light', '', 1, 0, ?, ?, ?, 'private', ?, 'upload')`,
		themeID, themeID, stamp, stamp, slug, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES (?, ?, '1.0.0', 'digest', '{}', 'x', ?, 'active', ?)`,
		versionID, themeID, "hash-"+themeID, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = ? WHERE id = ?`, versionID, themeID); err != nil {
		t.Fatal(err)
	}
}

func TestThemeCatalogRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insertUser(t, db, "usr_admin_tcr", "owner", "admin@example.com", "admin", now)
	insertUser(t, db, "usr_alice_tcr", "alice", "alice@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_alice_tcr", "usr_alice_tcr", "aurora", "vaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now)

	service := NewService(NewSQLStore(db))
	service.now = func() time.Time { return now }
	admin := Actor{ID: "usr_admin_tcr", Username: "owner", Role: "admin", Status: "active"}
	alice := Actor{ID: "usr_alice_tcr", Username: "alice", Role: "user", Status: "active"}

	// 非本人提交 → ErrNotFound(防枚举)。
	stranger := Actor{ID: "usr_stranger_tcr", Username: "stranger"}
	if _, err := service.Request(ctx, stranger, "thm_alice_tcr", "req-stranger"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger request error = %v", err)
	}

	first, err := service.Request(ctx, alice, "thm_alice_tcr", "req-apply")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "pending" || first.Slug != "aurora" || first.ThemeID != "thm_alice_tcr" {
		t.Fatalf("unexpected request: %+v", first)
	}

	// 重复提交(仍 pending)→ ErrConflict。
	if _, err := service.Request(ctx, alice, "thm_alice_tcr", "req-dup"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate request error = %v", err)
	}

	// 非管理员看队列 → ErrForbidden。
	if _, err := service.Requests(ctx, alice, "", 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin list error = %v", err)
	}
	page, err := service.Requests(ctx, admin, "pending", 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("pending page = %+v, %v", page, err)
	}

	approved, err := service.Review(ctx, admin, first.ID, "approve", "", "req-approve")
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	var scope string
	var ownerID sql.NullString
	if err := db.QueryRow(`SELECT scope, owner_id FROM themes WHERE id = ?`, "thm_alice_tcr").Scan(&scope, &ownerID); err != nil {
		t.Fatal(err)
	}
	if scope != "catalog" || ownerID.Valid {
		t.Fatalf("promotion did not flip scope/owner: scope=%q owner=%v", scope, ownerID)
	}

	// 已批准的请求不能再次审批 → ErrInvalidTransition。
	if _, err := service.Review(ctx, admin, first.ID, "approve", "", "req-again"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeat approval error = %v", err)
	}

	// slug 冲突:第二个私有主题、同 slug,提交 → ErrSlugConflict。
	insertPrivateTheme(t, db, "thm_bob_tcr", "usr_alice_tcr", "aurora", "vbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now)
	if _, err := service.Request(ctx, alice, "thm_bob_tcr", "req-slug-conflict"); !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("slug conflict at submission error = %v", err)
	}

	// 拒绝流程:另建一个不同 slug 的主题,拒绝后 owner 仍是 private/自己。
	insertUser(t, db, "usr_carol_tcr", "carol", "carol@example.com", "user", now)
	insertPrivateTheme(t, db, "thm_carol_tcr", "usr_carol_tcr", "borealis", "vcccccccccccccccccccccccccccccccc", now)
	carol := Actor{ID: "usr_carol_tcr", Username: "carol", Role: "user", Status: "active"}
	pending, err := service.Request(ctx, carol, "thm_carol_tcr", "req-carol")
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := service.Review(ctx, admin, pending.ID, "reject", "内容不符合规范", "req-reject")
	if err != nil || rejected.Status != "rejected" || rejected.Reason != "内容不符合规范" {
		t.Fatalf("reject = %+v, %v", rejected, err)
	}
	var carolScope string
	if err := db.QueryRow(`SELECT scope FROM themes WHERE id = ?`, "thm_carol_tcr").Scan(&carolScope); err != nil {
		t.Fatal(err)
	}
	if carolScope != "private" {
		t.Fatalf("rejected theme scope changed: %q", carolScope)
	}
	// 被拒绝后可重新提交。
	resubmitted, err := service.Request(ctx, carol, "thm_carol_tcr", "req-carol-again")
	if err != nil || resubmitted.Status != "pending" {
		t.Fatalf("resubmit after rejection = %+v, %v", resubmitted, err)
	}
	// owner 可撤回自己的 pending 请求。
	if err := service.Cancel(ctx, carol, "thm_carol_tcr", "req-carol-cancel"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Review(ctx, admin, resubmitted.ID, "approve", "", "req-carol-approve-after-cancel"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve after cancel error = %v", err)
	}
}
```

- [ ] **Step 4: Run the new test**

Run: `go test ./internal/themecatalog/... -v`
Expected: PASS.

- [ ] **Step 5: `gofmt` and `go vet`**

Run: `gofmt -l internal/themecatalog && go vet ./internal/themecatalog/...`
Expected: no output from `gofmt -l` (nothing to reformat), no errors from `go vet`.

- [ ] **Step 6: Commit**

```bash
git add internal/themecatalog
git commit -m "$(cat <<'EOF'
feat: add internal/themecatalog package

Catalog-promotion request/approve state machine for private themes,
mirroring internal/subdomains' apply-approve pattern. Approval flips
themes.scope/owner_id in the same transaction the scope/owner trigger
requires, after re-checking the version snapshot and slug uniqueness.
EOF
)"
```

---

## Task 5: Lock theme upgrades while a catalog request is pending

**Files:**
- Modify: `internal/themes/install.go` (add `ErrCatalogRequestPending`, check inside the upgrade branch)
- Modify: `internal/httpapi/themeimport.go:123-137` (`writeImportError` — new case)
- Create/Modify: `internal/themes/install_test.go` (new test)

**Interfaces:**
- Consumes: table `theme_catalog_requests` (Task 3).
- Produces: `themes.ErrCatalogRequestPending` — consumed by `internal/httpapi/themeimport.go`'s error mapping (this task) and asserted directly in the Task 7 contract test.

- [ ] **Step 1: Write the failing test**

In `internal/themes/install_test.go`, add after `TestInstallPrivateSameSlugUpgrades` (ends around line 86):

```go
// TestInstallPrivateUpgradeBlockedByPendingCatalogRequest 确认审核期间
// (theme_catalog_requests 有一条 pending 记录)重新导入同 slug 会被拒绝——
// 保证管理员审核的内容就是最终晋升的内容,不会被中途替换。
func TestInstallPrivateUpgradeBlockedByPendingCatalogRequest(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_pending")

	first := installSample(t, store, "usr_inst_pending", "lilac", 10)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, applied_at)
		VALUES ('tcr_pending_0001', ?, 'usr_inst_pending', 'pending', '', ?, ?)`,
		first.ThemeID, first.VersionID, stamp); err != nil {
		t.Fatal(err)
	}

	_, err := store.InstallPrivate(t.Context(), "usr_inst_pending", "lilac", "upload", "", "digest-2", "", 10,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.7; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC())
	if !errors.Is(err, ErrCatalogRequestPending) {
		t.Fatalf("upgrade during pending review error = %v, want ErrCatalogRequestPending", err)
	}

	// 请求被拒绝后,升级恢复可用。
	if _, err := db.Exec(`UPDATE theme_catalog_requests SET status = 'rejected' WHERE id = 'tcr_pending_0001'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallPrivate(t.Context(), "usr_inst_pending", "lilac", "upload", "", "digest-3", "", 10,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.6; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC()); err != nil {
		t.Fatalf("upgrade after rejection error = %v", err)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/themes/... -run TestInstallPrivateUpgradeBlockedByPendingCatalogRequest -v`
Expected: FAIL — `ErrCatalogRequestPending` doesn't exist yet (compile error), or once you stub it in as an unused var, the upgrade silently succeeds instead of being blocked.

- [ ] **Step 3: Add the guard in `internal/themes/install.go`**

Add the sentinel error near `ErrQuotaExceeded` (line 17):

```go
// ErrQuotaExceeded 表示该用户的私有主题数已达实例配额。
var ErrQuotaExceeded = errors.New("private theme quota exceeded")

// ErrCatalogRequestPending 表示该主题有一条待审核的目录晋升申请,审核结果
// 出来前不允许升级——保证管理员审核的内容就是最终晋升的内容。
var ErrCatalogRequestPending = errors.New("theme has a pending catalog request")
```

In the `default:` (upgrade) branch of `InstallPrivate`'s transaction body (right after `case err != nil: return err` and before the `// 升级:同一行换版本` comment), add:

```go
		default:
			var pending int
			if err := tx.QueryRowContext(ctx, `
				SELECT 1 FROM theme_catalog_requests WHERE theme_id = ? AND status = 'pending'`, themeID).Scan(&pending); err == nil {
				return ErrCatalogRequestPending
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			// 升级:同一行换版本。source_url 跟随本次来源(github 重新拉取会刷新)。
			compiled, err := compile(themeID)
```

(the rest of the `default:` branch is unchanged — this only adds the check before `compile(themeID)` is called, so a blocked upgrade doesn't waste a compile pass.)

- [ ] **Step 4: Map the new error in the HTTP layer**

In `internal/httpapi/themeimport.go`, `writeImportError` (lines 123-137), add a case before the `default:`:

```go
func (h *ThemeImportHandler) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, themes.ErrQuotaExceeded):
		WriteError(w, r, http.StatusConflict, "QUOTA_EXCEEDED", "私有主题数量已达上限(含已卸载但仍被历史发布引用的主题)", nil)
	case errors.Is(err, themes.ErrCatalogRequestPending):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "该主题正在目录审核中，暂不可升级", nil)
	case errors.Is(err, themes.ErrInvalidArchive), errors.Is(err, themes.ErrInvalidManifest),
		errors.Is(err, themes.ErrInvalidCSS), errors.Is(err, themes.ErrInvalidAsset):
		WriteError(w, r, http.StatusUnprocessableEntity, "THEME_INVALID", "主题包未通过校验", err)
	case errors.Is(err, themeimport.ErrHostNotAllowed):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "仓库地址不被允许", err)
	case errors.Is(err, themeimport.ErrUpstream):
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_ERROR", "上游仓库拉取失败", err)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "导入失败", nil)
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/themes/... ./internal/httpapi/... -v 2>&1 | tail -60`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/themes/install.go internal/themes/install_test.go internal/httpapi/themeimport.go
git commit -m "$(cat <<'EOF'
feat: block private-theme upgrades while a catalog request is pending

Prevents an owner from swapping a theme's content out from under an
admin mid-review — the version an admin approves is the version that
gets promoted.
EOF
)"
```

---

## Task 6: `internal/httpapi/themecatalog.go` handlers + wiring

**Files:**
- Create: `internal/httpapi/themecatalog.go`
- Create: `internal/httpapi/themecatalog_test.go`
- Modify: `internal/app/run.go` (construct + mount the new service/handler)

**Interfaces:**
- Consumes: `internal/themecatalog.Service` and its `Actor`/`Request`/errors (Task 4).
- Produces: routes `POST /me/themes/{themeId}/catalog-request`, `DELETE /me/themes/{themeId}/catalog-request`, `GET /admin/theme-catalog-requests`, `PATCH /admin/theme-catalog-requests/{requestId}` — consumed by Task 7 (openapi contract test) and Task 9 (frontend API client).

- [ ] **Step 1: Write `internal/httpapi/themecatalog.go`**

```go
package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/themecatalog"
)

// ThemeCatalogHandler exposes the private→catalog promotion request/approve
// workflow. Owner-facing routes live under /me, admin routes under /admin.
type ThemeCatalogHandler struct {
	auth    *auth.Service
	service *themecatalog.Service
}

func NewThemeCatalogHandler(authService *auth.Service, service *themecatalog.Service) *ThemeCatalogHandler {
	return &ThemeCatalogHandler{auth: authService, service: service}
}

func (h *ThemeCatalogHandler) MountUserRoutes(router chi.Router) {
	router.Post("/me/themes/{themeId}/catalog-request", h.request)
	router.Delete("/me/themes/{themeId}/catalog-request", h.cancel)
}

func (h *ThemeCatalogHandler) MountAdminRoutes(router chi.Router) {
	router.Get("/theme-catalog-requests", h.list)
	router.Patch("/theme-catalog-requests/{requestId}", h.review)
}

func (h *ThemeCatalogHandler) request(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Request(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "themeId"), middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, themeCatalogRequestData(item))
}

func (h *ThemeCatalogHandler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Cancel(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "themeId"), middleware.GetReqID(r.Context()),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ThemeCatalogHandler) list(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Requests(r.Context(), themeCatalogActor(r), r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, themeCatalogRequestData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *ThemeCatalogHandler) review(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.Review(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "requestId"),
		request.Decision, request.Reason, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, themeCatalogRequestData(item))
}

func (h *ThemeCatalogHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, themecatalog.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
	case errors.Is(err, themecatalog.ErrSlugConflict):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "该主题 slug 已在官方目录中被占用", nil)
	case errors.Is(err, themecatalog.ErrThemeNotEligible):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "主题必须已启用且有可用版本才能提交审核", nil)
	case errors.Is(err, themecatalog.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请求参数无效", nil)
	case errors.Is(err, themecatalog.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "该主题已有生效的目录申请", nil)
	case errors.Is(err, themecatalog.ErrInvalidTransition):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "当前申请状态不允许此操作", nil)
	case errors.Is(err, themecatalog.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "目录申请不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "目录审核操作失败", nil)
	}
}

func themeCatalogActor(r *http.Request) themecatalog.Actor {
	session, _ := SessionFromContext(r.Context())
	return themecatalog.Actor{
		ID: session.User.ID, Username: session.User.Username,
		Role: session.User.Role, Status: session.User.Status,
	}
}

func themeCatalogRequestData(item themecatalog.Request) map[string]any {
	return map[string]any{
		"id": item.ID, "themeId": item.ThemeID, "themeName": item.ThemeName, "slug": item.Slug,
		"ownerId": item.OwnerID, "ownerName": item.OwnerName, "status": item.Status,
		"reason": item.Reason, "appliedAt": item.AppliedAt, "reviewedAt": item.ReviewedAt,
	}
}
```

- [ ] **Step 2: Write `internal/httpapi/themecatalog_test.go`**

```go
package httpapi

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/security"
	"github.com/yixian-huang/navax/internal/themecatalog"
	"github.com/yixian-huang/navax/internal/themeimport"
	"github.com/yixian-huang/navax/internal/themes"
)

// auroraManifest/auroraZip build a minimal legal theme package the same way
// internal/themeimport/service_test.go's sampleManifest/sampleZip do (theme
// package Go structs use plain map[string]string tokens, not their own named
// types — going through ImportZip's real zip→manifest→compile pipeline
// avoids hand-constructing themes.Manifest/themes.Compiled by field name).
const auroraManifest = `{
  "specVersion": 1, "id": "aurora", "name": "Aurora", "version": "1.0.0",
  "author": "t", "license": "MIT", "mode": "light", "vibe": "serious",
  "swatches": ["#f5f3ff", "#8b5cf6", "#1e1b4b"], "tier": 1,
  "tokens": {
    "font": { "heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace" },
    "color": {
      "background": { "50": "0.985 0.010 300" },
      "foreground": { "900": "0.210 0.040 300" },
      "primary":    { "500": "0.585 0.200 300" },
      "accent":     { "500": "0.700 0.150 160" }
    }
  }
}`

func auroraZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{
		"theme.json": []byte(auroraManifest),
		"theme.css":  []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
	} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestThemeCatalogHandlerLifecycle(t *testing.T) {
	db, authService, _, _, token := setupHandlerServices(t)
	themeStore := themes.NewStore(db)
	stamp := time.Now().UTC()
	passwordHash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_tcr_owner', 'requester', 'requester@example.com', ?, 'user', 'active', ?, ?)`,
		passwordHash, stamp.Format(time.RFC3339Nano), stamp.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	importService := themeimport.NewService(themeStore, nil, 10)
	installed, err := importService.ImportZip(t.Context(), "usr_tcr_owner", auroraZip(t))
	if err != nil {
		t.Fatal(err)
	}

	// requester 会话:用真实密码登录换 token,不走 bootstrap(那是 setupHandlerServices
	// 已经用过的管理员账号)。auth.Service.Login 按 username 或 email 匹配。
	_, requesterToken, err := authService.Login(t.Context(), "requester", "integration-password", "e2e-test")
	if err != nil {
		t.Fatal(err)
	}

	service := themecatalog.NewService(themecatalog.NewSQLStore(db))
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	handler := NewThemeCatalogHandler(authService, service)
	router.Group(func(protected chi.Router) {
		protected.Use(RequireSession(authService))
		handler.MountUserRoutes(protected)
		protected.Route("/admin", func(admin chi.Router) {
			admin.Use(RequireAdmin)
			handler.MountAdminRoutes(admin)
		})
	})

	response := performRequest(router, http.MethodPost, "/me/themes/"+installed.ThemeID+"/catalog-request", nil, requesterToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("submit status = %d: %s", response.Code, response.Body.String())
	}
	created := decodeEnvelope(t, response)["data"].(map[string]any)
	requestID := created["id"].(string)
	if created["status"] != "pending" || created["slug"] != "aurora" {
		t.Fatalf("created request = %+v", created)
	}

	response = performRequest(router, http.MethodGet, "/admin/theme-catalog-requests?status=pending", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("admin list status = %d: %s", response.Code, response.Body.String())
	}
	listed := decodeEnvelope(t, response)
	if listed["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("admin list = %+v", listed)
	}

	response = performRequest(router, http.MethodPatch, "/admin/theme-catalog-requests/"+requestID,
		map[string]any{"decision": "approve"}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("approve status = %d: %s", response.Code, response.Body.String())
	}
	approved := decodeEnvelope(t, response)["data"].(map[string]any)
	if approved["status"] != "approved" {
		t.Fatalf("approved request = %+v", approved)
	}
	var scope string
	if err := db.QueryRow(`SELECT scope FROM themes WHERE id = ?`, installed.ThemeID).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if scope != "catalog" {
		t.Fatalf("scope = %q, want catalog", scope)
	}
}
```

- [ ] **Step 3: Run it to confirm it fails, then wire the app and re-run**

Run: `go test ./internal/httpapi/... -run TestThemeCatalogHandlerLifecycle -v`
Expected: FAIL — `NewThemeCatalogHandler`, `themecatalog.NewService`/`NewSQLStore` already exist from Task 4/this task's Step 1, so the likely failure at this point is the handler not yet being reachable through a real router the same way `internal/app/run.go` wires it; this test builds its own local `chi.NewRouter()` so it should actually compile and pass once Step 1's handler file exists — treat any failure here as a real bug to fix, not an expected gap.

- [ ] **Step 4: Wire the new service/handler into `internal/app/run.go`**

Add the import (alongside the existing `"github.com/yixian-huang/navax/internal/subdomains"` import):

```go
	"github.com/yixian-huang/navax/internal/themecatalog"
```

After `subdomainHandler := httpapi.NewSubdomainHandler(authService, subdomainService)` (line 212), add:

```go
	themeCatalogService := themecatalog.NewService(themecatalog.NewSQLStore(db))
	themeCatalogHandler := httpapi.NewThemeCatalogHandler(authService, themeCatalogService)
```

In the `MountAPI` closure, after `themeImportHandler.MountProtected(protected)` (line 281), add:

```go
				themeCatalogHandler.MountUserRoutes(protected)
```

And inside the `protected.Route("/admin", ...)` block, after `subdomainHandler.MountAdminRoutes(admin)` (line 289), add:

```go
					themeCatalogHandler.MountAdminRoutes(admin)
```

- [ ] **Step 5: Run the test again and the full httpapi suite**

Run: `go test ./internal/httpapi/... -v 2>&1 | tail -60`
Expected: PASS.

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/httpapi/themecatalog.go internal/httpapi/themecatalog_test.go internal/app/run.go
git commit -m "$(cat <<'EOF'
feat: wire the theme catalog review HTTP endpoints

POST/DELETE /me/themes/{themeId}/catalog-request for owners,
GET /admin/theme-catalog-requests and PATCH .../{requestId} for admins.
EOF
)"
```

---

## Task 7: `openapi.yaml` contract + contract test

**Files:**
- Modify: `api/openapi.yaml` (new paths, new schemas, `Theme` schema additions)
- Modify: `tests/contract/api_contract_test.go` (new subtest)

**Interfaces:**
- Consumes: the live endpoints from Task 6.
- Produces: `Theme.catalogRequestStatus` / `Theme.catalogRequestReason` fields — consumed by Task 8 (Go DTO wiring) and Task 9/10 (frontend types).

- [ ] **Step 1: Add the new paths**

In `api/openapi.yaml`, right after the `/api/v1/me/themes/{themeId}:` block (after its `delete:` responses, before `/api/v1/themes/validate:`, around line 1175), insert:

```yaml
  /api/v1/me/themes/{themeId}/catalog-request:
    post:
      tags: [Themes]
      operationId: submitThemeCatalogRequest
      description: 私有主题所有者提交官方目录审核申请。
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeId'
      responses:
        '201': { $ref: '#/components/responses/ThemeCatalogRequestResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
    delete:
      tags: [Themes]
      operationId: cancelThemeCatalogRequest
      description: 撤回本人待审核的目录申请。
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeId'
      responses:
        '204': { description: 已撤回 }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
```

Right after the `/api/v1/admin/subdomains/{requestId}:` block (before `/api/v1/admin/discover:`, around line 1546), insert:

```yaml
  /api/v1/admin/theme-catalog-requests:
    get:
      tags: [Admin]
      operationId: listThemeCatalogRequests
      security: [{ sessionCookie: [] }]
      parameters:
        - in: query
          name: status
          schema: { $ref: '#/components/schemas/ThemeCatalogRequestStatus' }
        - $ref: '#/components/parameters/Page'
        - $ref: '#/components/parameters/PageSize'
      responses:
        '200': { $ref: '#/components/responses/ThemeCatalogRequestsResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '403': { $ref: '#/components/responses/ErrorResponse' }
  /api/v1/admin/theme-catalog-requests/{requestId}:
    patch:
      tags: [Admin]
      operationId: reviewThemeCatalogRequest
      security: [{ sessionCookie: [] }]
      parameters:
        - in: path
          name: requestId
          required: true
          schema: { $ref: '#/components/schemas/Id' }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/ThemeCatalogReviewRequest' }
      responses:
        '200': { $ref: '#/components/responses/ThemeCatalogRequestResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '403': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
```

- [ ] **Step 2: Add the response entries**

In the `responses:` block, right after `ThemeVersionResponse` (around line 1893), insert:

```yaml
    ThemeCatalogRequestResponse: { description: 目录审核申请, content: { application/json: { schema: { $ref: '#/components/schemas/ThemeCatalogRequestEnvelope' } } } }
    ThemeCatalogRequestsResponse: { description: 目录审核申请列表, content: { application/json: { schema: { $ref: '#/components/schemas/ThemeCatalogRequestsEnvelope' } } } }
```

- [ ] **Step 3: Add the schemas**

Right after the `ThemeVersion:` schema (around line 2483, before `ThemeManifestV1:`), insert:

```yaml
    ThemeCatalogRequestStatus: { type: string, enum: [pending, approved, rejected, revoked] }
    ThemeCatalogRequest:
      type: object
      description: 私有主题提交官方目录的审核申请。
      required: [id, themeId, themeName, slug, ownerId, status, appliedAt]
      properties:
        id: { $ref: '#/components/schemas/Id' }
        themeId: { $ref: '#/components/schemas/ThemeId' }
        themeName: { type: string }
        slug: { type: string }
        ownerId: { $ref: '#/components/schemas/Id' }
        ownerName: { type: string }
        status: { $ref: '#/components/schemas/ThemeCatalogRequestStatus' }
        reason: { type: string, maxLength: 300 }
        appliedAt: { $ref: '#/components/schemas/Timestamp' }
        reviewedAt: { oneOf: [{ $ref: '#/components/schemas/Timestamp' }, { type: 'null' }] }
    ThemeCatalogReviewRequest:
      type: object
      required: [decision]
      properties:
        decision: { type: string, enum: [approve, reject] }
        reason: { type: string, maxLength: 300 }
```

Add the two `catalogRequestStatus`/`catalogRequestReason` fields to the `Theme` schema, right after the existing `status:` property (line 2465):

```yaml
        status: { type: string, enum: [active, disabled] }
        catalogRequestStatus: { type: string, enum: [pending, rejected], description: 私有主题当前是否有未终结的目录审核申请;已批准的直接体现为 scope=catalog,不在此字段保留。 }
        catalogRequestReason: { type: string, maxLength: 300 }
```

- [ ] **Step 4: Add the envelopes**

Right after `ThemeVersionsEnvelope` (around line 3083), insert:

```yaml
    ThemeCatalogRequestEnvelope: { allOf: [{ $ref: '#/components/schemas/EnvelopeBase' }, { type: object, properties: { data: { $ref: '#/components/schemas/ThemeCatalogRequest' } }, required: [data] }] }
    ThemeCatalogRequestsEnvelope: { allOf: [{ $ref: '#/components/schemas/PaginatedEnvelopeBase' }, { type: object, properties: { data: { type: array, items: { $ref: '#/components/schemas/ThemeCatalogRequest' } } }, required: [data] }] }
```

- [ ] **Step 5: Write the contract test**

In `tests/contract/api_contract_test.go`, add a new `t.Run("主题目录审核", ...)` right after the `t.Run("主题导入与私有安装", ...)` block closes (so it can reuse the `buildThemeZip` helper and runs after quota state from that block has settled):

```go
	t.Run("主题目录审核", func(t *testing.T) {
		imported := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, imported, http.StatusCreated, "导入待审核主题")
		themeID := stringField(t, imported.data(), "id", "待审核主题 ID")

		submitted := user.call(t, http.MethodPost, "/api/v1/me/themes/"+themeID+"/catalog-request", nil)
		mustStatus(t, submitted, http.StatusCreated, "提交目录审核")
		requestID := stringField(t, submitted.data(), "id", "审核申请 ID")
		if got := stringField(t, submitted.data(), "status", "审核申请状态"); got != "pending" {
			t.Fatalf("submitted status = %q", got)
		}

		// 审核期间禁止升级。
		locked := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand2.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, locked, http.StatusConflict, "审核期间升级被拒")

		// 重复提交同一主题 → 409。
		duplicate := user.call(t, http.MethodPost, "/api/v1/me/themes/"+themeID+"/catalog-request", nil)
		mustStatus(t, duplicate, http.StatusConflict, "重复提交 409")

		queue := admin.call(t, http.MethodGet, "/api/v1/admin/theme-catalog-requests?status=pending", nil)
		mustStatus(t, queue, http.StatusOK, "目录审核队列")
		if !strings.Contains(string(queue.body), requestID) {
			t.Fatalf("审核队列缺少申请 %s", requestID)
		}

		approved := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-catalog-requests/"+requestID,
			map[string]any{"decision": "approve"})
		mustStatus(t, approved, http.StatusOK, "批准目录申请")
		if got := stringField(t, approved.data(), "status", "批准后状态"); got != "approved" {
			t.Fatalf("approved status = %q", got)
		}

		publicList := guest.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, publicList, http.StatusOK, "公开主题列表")
		if !strings.Contains(string(publicList.body), themeID) {
			t.Fatalf("公开列表应含已批准主题 %s", themeID)
		}

		// 批准后 slug 释放给同 owner 的新私有导入;拿它验证 slug 冲突路径。
		afterApproval := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand3.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, afterApproval, http.StatusCreated, "批准后重新导入同 slug 建私有副本")
		conflictThemeID := stringField(t, afterApproval.data(), "id", "冲突主题 ID")
		conflict := user.call(t, http.MethodPost, "/api/v1/me/themes/"+conflictThemeID+"/catalog-request", nil)
		mustStatus(t, conflict, http.StatusUnprocessableEntity, "slug 冲突 422")
	})
```

- [ ] **Step 6: Run the contract suite**

Run: `go test ./tests/contract/... -v 2>&1 | tail -80`
Expected: PASS, including "主题目录审核" and all prior subtests still green (this is the same binary that boots and validates every request/response against `api/openapi.yaml`, so a schema mistake in Step 1-4 fails loudly here).

- [ ] **Step 7: Commit**

```bash
git add api/openapi.yaml tests/contract/api_contract_test.go
git commit -m "$(cat <<'EOF'
feat: add the theme catalog review contract

New ThemeCatalogRequest paths/schemas and Theme.catalogRequestStatus/
catalogRequestReason, validated end-to-end (submit → queue → approve →
public visibility, slug conflict, pending-blocks-upgrade) in the API
contract suite.
EOF
)"
```

---

## Task 8: Wire `catalogRequestStatus`/`catalogRequestReason` into the shared Theme DTO

**Files:**
- Modify: `internal/admin/service.go:102-127` (`Theme` struct)
- Modify: `internal/admin/sqlstore.go:212-247,575-616` (`themeSelect`, `scanTheme`)
- Modify: `internal/catalog/service.go:114-150` (`Themes` query)
- Modify: `internal/httpapi/admin.go:548-593` (`themeData` serializer)
- Modify: `internal/admin/sqlstore_test.go` (new assertion)

**Interfaces:**
- Consumes: `theme_catalog_requests` table (Task 3).
- Produces: `Theme.CatalogRequestStatus`/`Theme.CatalogRequestReason` (Go), `catalogRequestStatus`/`catalogRequestReason` (JSON) — consumed by Task 10 (frontend types already declared the shape in Task 7's openapi change; this task makes the Go server actually emit it).

- [ ] **Step 1: Add the fields to `internal/admin.Theme`**

In `internal/admin/service.go`, add two fields to the `Theme` struct (after `Status`, line 126):

```go
	// Status 是当前版本的状态(active/disabled)。无当前版本时空串。
	Status string
	// CatalogRequestStatus 是该私有主题最近一条未终结(pending/rejected)的
	// 目录审核申请状态;已批准的不在这里体现(scope 已经是 catalog)。
	CatalogRequestStatus string
	CatalogRequestReason string
```

- [ ] **Step 2: Extend the admin listing query**

In `internal/admin/sqlstore.go`, change `themeSelect` (lines 212-220) to LEFT JOIN the most recent unresolved request per theme:

```go
const themeSelect = `
	SELECT themes.id, themes.name, themes.version, themes.author, themes.description,
	       themes.mode, themes.preview, themes.enabled, themes.is_default,
	       themes.current_version_id, themes.scope, themes.source_type, themes.source_url,
	       themes.owner_id, users.username, theme_versions.status,
	       theme_versions.manifest_json, tcr.status, tcr.reason
	FROM themes
	LEFT JOIN theme_versions ON theme_versions.id = themes.current_version_id
	LEFT JOIN users ON users.id = themes.owner_id
	LEFT JOIN theme_catalog_requests tcr ON tcr.id = (
		SELECT id FROM theme_catalog_requests
		WHERE theme_id = themes.id AND status IN ('pending', 'rejected')
		ORDER BY applied_at DESC LIMIT 1
	)`
```

Update `scanTheme` (lines 575-616) to scan the two new columns:

```go
func scanTheme(row rowScanner) (Theme, error) {
	var item Theme
	var currentVersionID, manifestJSON, sourceType, sourceURL sql.NullString
	var ownerID, ownerName, status sql.NullString
	var catalogRequestStatus, catalogRequestReason sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.Version, &item.Author, &item.Description,
		&item.Mode, &item.Preview, &item.Enabled, &item.Default,
		&currentVersionID, &item.Scope, &sourceType, &sourceURL,
		&ownerID, &ownerName, &status, &manifestJSON, &catalogRequestStatus, &catalogRequestReason); err != nil {
		return Theme{}, err
	}
	// 无编译版本的主题保持零值,序列化层据此省略字段——缺省即「不可选用」。
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
	if catalogRequestStatus.Valid {
		item.CatalogRequestStatus = catalogRequestStatus.String
	}
	if catalogRequestReason.Valid {
		item.CatalogRequestReason = catalogRequestReason.String
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

- [ ] **Step 3: Extend the owner-facing catalog query**

In `internal/catalog/service.go`, change `Themes` (lines 114-150) to join the request status **scoped to the caller** (so no other owner's request state leaks):

```go
func (s *Service) Themes(ctx context.Context, actorID string) ([]adminpkg.Theme, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT themes.id, themes.name, themes.version, themes.author, themes.description,
		       themes.mode, themes.preview, themes.enabled, themes.is_default,
		       themes.current_version_id, themes.scope, themes.source_type, themes.source_url,
		       theme_versions.manifest_json, tcr.status, tcr.reason
		FROM themes `+themes.EligibilityJoin+`
		LEFT JOIN theme_catalog_requests tcr ON tcr.id = (
			SELECT id FROM theme_catalog_requests
			WHERE theme_id = themes.id AND owner_id = ? AND status IN ('pending', 'rejected')
			ORDER BY applied_at DESC LIMIT 1
		)
		WHERE `+themes.EligibilityWhere+`
		ORDER BY themes.is_default DESC, themes.name, themes.id`, actorID, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]adminpkg.Theme, 0)
	for rows.Next() {
		var (
			theme                                    adminpkg.Theme
			manifestJSON                              string
			catalogRequestStatus, catalogRequestReason sql.NullString
		)
		if err := rows.Scan(&theme.ID, &theme.Name, &theme.Version, &theme.Author, &theme.Description,
			&theme.Mode, &theme.Preview, &theme.Enabled, &theme.Default,
			&theme.CurrentVersionID, &theme.Scope, &theme.SourceType, &theme.SourceURL, &manifestJSON,
			&catalogRequestStatus, &catalogRequestReason); err != nil {
			return nil, err
		}
		theme.CSSHref = "/api/v1/public/themes/" + theme.CurrentVersionID + ".css"
		if catalogRequestStatus.Valid {
			theme.CatalogRequestStatus = catalogRequestStatus.String
		}
		if catalogRequestReason.Valid {
			theme.CatalogRequestReason = catalogRequestReason.String
		}
		var manifest themes.Manifest
		if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
			return nil, err
		}
		theme.Subtitle = manifest.Subtitle
		theme.Tier = manifest.Tier
		theme.Vibe = manifest.Vibe
		theme.Swatches = manifest.Swatches
		list = append(list, theme)
	}
	return list, rows.Err()
}
```

Note the query now binds `actorID` twice (`EligibilityWhere` already consumes it once for the private-theme-ownership check; the new subquery needs its own copy) — `s.db.QueryContext(ctx, query, actorID, actorID)`.

- [ ] **Step 4: Serialize the new fields**

In `internal/httpapi/admin.go`, `themeData` (lines 548-593), add after the `status` block:

```go
	if item.Status != "" {
		data["status"] = item.Status
	}
	if item.CatalogRequestStatus != "" {
		data["catalogRequestStatus"] = item.CatalogRequestStatus
	}
	if item.CatalogRequestReason != "" {
		data["catalogRequestReason"] = item.CatalogRequestReason
	}
	return data
}
```

- [ ] **Step 5: Write a regression test**

In `internal/admin/sqlstore_test.go`, add after `TestThemeListingIncludesOwnerAndStatus`:

```go
// TestThemeListingIncludesCatalogRequestStatus 确认待审核/被拒绝的目录申请
// 状态随主题列表一起返回,已批准的不残留任何 catalogRequestStatus(scope
// 已经是 catalog,字段本身就该是空)。
func TestThemeListingIncludesCatalogRequestStatus(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_bob_tcr_dto', 'bob', 'bob@example.com', 'x', 'user', 'active', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
			created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES ('thm_bob_tcr_dto', 'Bob Theme', '1.0.0', 'bob', '', 'light', '', 1, 0, ?, ?, 'bob-theme', 'private', 'usr_bob_tcr_dto', 'upload')`,
		stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_versions (id, theme_id, version, source_ref, manifest_json, compiled_css, content_hash, status, created_at)
		VALUES ('vbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'thm_bob_tcr_dto', '1.0.0', 'digest', '{}', 'x', 'hashbobtheme0001', 'active', ?)`, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE themes SET current_version_id = 'vbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' WHERE id = 'thm_bob_tcr_dto'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, applied_at)
		VALUES ('tcr_bob_dto', 'thm_bob_tcr_dto', 'usr_bob_tcr_dto', 'rejected', '内容不合规', 'vbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', ?)`, stamp); err != nil {
		t.Fatal(err)
	}

	store := NewSQLStore(db)
	items, err := store.ListThemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got Theme
	for _, item := range items {
		if item.ID == "thm_bob_tcr_dto" {
			got = item
		}
	}
	if got.CatalogRequestStatus != "rejected" || got.CatalogRequestReason != "内容不合规" {
		t.Fatalf("theme = %+v", got)
	}
}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/admin/... ./internal/catalog/... -v 2>&1 | tail -60`
Expected: PASS.

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/admin/service.go internal/admin/sqlstore.go internal/admin/sqlstore_test.go \
  internal/catalog/service.go internal/httpapi/admin.go
git commit -m "$(cat <<'EOF'
feat: surface catalogRequestStatus/catalogRequestReason on Theme

Admin's full theme listing and the owner-scoped /themes read both now
report a private theme's pending or rejected catalog request inline,
so the frontend doesn't need a second round-trip to show the badge.
EOF
)"
```

---

## Task 9: Frontend types, API client, hooks, dev-mock

**Files:**
- Modify: `web/src/api/types.ts` (`Theme` fields, `AdminThemeCatalogRequest`, `ThemeCatalogReviewRequest`, `ContractThemeCatalogRequestStatus`)
- Modify: `web/src/api/themes.ts` (`submitCatalogRequest`, `cancelCatalogRequest`)
- Modify: `web/src/api/admin.ts` (`getThemeCatalogRequests`, `reviewThemeCatalogRequest`)
- Modify: `web/src/hooks/useQueries.ts` (mutation/query hooks)
- Modify: `web/src/api/mock-handlers.ts` (owner-facing mock routes)

**Interfaces:**
- Consumes: the endpoints from Task 6/7.
- Produces: `themesApi.submitCatalogRequest(themeId)`, `themesApi.cancelCatalogRequest(themeId)`, `adminApi.getThemeCatalogRequests(params)`, `adminApi.reviewThemeCatalogRequest(requestId, data)`, hooks `useSubmitCatalogRequest()`, `useCancelCatalogRequest()` — consumed by Task 10 (owner UI) and Task 11 (admin UI).

- [ ] **Step 1: Extend `Theme` and add the new types in `web/src/api/types.ts`**

Add two fields to `Theme` (after `status?: 'active' | 'disabled';`, line 317):

```ts
  /** 当前版本的状态;停用后该版本从用户可选列表消失。无当前版本时缺省。 */
  status?: 'active' | 'disabled';
  /** 未终结(pending/rejected)的目录审核申请状态;已批准的不体现在这里——scope 已经是 catalog。 */
  catalogRequestStatus?: 'pending' | 'rejected';
  catalogRequestReason?: string;
```

Add new types near `AdminSubdomainRequest`/`SubdomainReviewRequest` (after line 582):

```ts
export type ContractThemeCatalogRequestStatus = 'pending' | 'approved' | 'rejected' | 'revoked';

export interface AdminThemeCatalogRequest {
  id: string;
  themeId: string;
  themeName: string;
  slug: string;
  ownerId: string;
  ownerName?: string;
  status: ContractThemeCatalogRequestStatus;
  reason: string;
  appliedAt: string;
  reviewedAt: string | null;
}

export interface ThemeCatalogReviewRequest {
  decision: 'approve' | 'reject';
  reason?: string;
}
```

- [ ] **Step 2: Add owner-facing API calls to `web/src/api/themes.ts`**

```ts
import { request } from './client';
import type { ApiResponse, Theme, ThemeUpdateStatus } from './types';

export const themesApi = {
  importZip: (file: File) => {
    const body = new FormData();
    body.set('file', file);
    return request<ApiResponse<Theme>>('/me/themes/import', { method: 'POST', body });
  },
  importGitHub: (githubUrl: string, ref?: string) =>
    request<ApiResponse<Theme>>('/me/themes/import', {
      method: 'POST',
      body: ref ? { githubUrl, ref } : { githubUrl },
    }),
  uninstall: (themeId: string) =>
    request<ApiResponse<null>>(`/me/themes/${encodeURIComponent(themeId)}`, { method: 'DELETE' }),
  checkUpdate: (themeId: string) =>
    request<ApiResponse<ThemeUpdateStatus>>(`/me/themes/${encodeURIComponent(themeId)}/check-update`, { method: 'POST' }),
  submitCatalogRequest: (themeId: string) =>
    request<ApiResponse<unknown>>(`/me/themes/${encodeURIComponent(themeId)}/catalog-request`, { method: 'POST' }),
  cancelCatalogRequest: (themeId: string) =>
    request<ApiResponse<null>>(`/me/themes/${encodeURIComponent(themeId)}/catalog-request`, { method: 'DELETE' }),
};
```

- [ ] **Step 3: Add admin API calls to `web/src/api/admin.ts`**

Right after `reviewSubdomainRequest` (line 216-217), add (and add `AdminThemeCatalogRequest`, `ThemeCatalogReviewRequest`, `ContractThemeCatalogRequestStatus` to the file's type import list at the top):

```ts
  // Operations: theme catalog review
  getThemeCatalogRequests: (params?: { status?: ContractThemeCatalogRequestStatus; page?: number; pageSize?: number }) =>
    request<ApiResponse<AdminThemeCatalogRequest[] | PaginatedResponse<AdminThemeCatalogRequest>>>('/admin/theme-catalog-requests', { params }).then(asPaginated),

  reviewThemeCatalogRequest: (requestId: string, data: ThemeCatalogReviewRequest) =>
    request<ApiResponse<AdminThemeCatalogRequest>>(`/admin/theme-catalog-requests/${requestId}`, { method: 'PATCH', body: data }),
};
```

- [ ] **Step 4: Add TanStack Query hooks to `web/src/hooks/useQueries.ts`**

Add after `useUpdateAdminThemeState` (ends around line 592):

```ts
export function useSubmitCatalogRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (themeId: string) => themesApi.submitCatalogRequest(themeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'themes'] }),
  });
}

export function useCancelCatalogRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (themeId: string) => themesApi.cancelCatalogRequest(themeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'themes'] }),
  });
}
```

(`themesApi` needs to already be imported in this file — check the top-level imports; if it isn't, add `import { themesApi } from '@/api/themes';` alongside the other `*Api` imports.)

Add an admin queue hook near `useAdminThemeVersions`/`useSetThemeVersionStatus` (ends around line 618):

```ts
export function useThemeCatalogRequests(status: ContractThemeCatalogRequestStatus | '', page: number, pageSize: number) {
  return useQuery({
    queryKey: ['admin', 'themes', 'catalog-requests', status, page, pageSize],
    queryFn: async () => (await adminApi.getThemeCatalogRequests({ status: status || undefined, page, pageSize })).data,
  });
}

export function useReviewThemeCatalogRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ requestId, data }: { requestId: string; data: ThemeCatalogReviewRequest }) =>
      adminApi.reviewThemeCatalogRequest(requestId, data),
    // 失效 ['admin','themes'] 前缀连带失效 ['admin','themes','catalog-requests',...]
    // (本文件 useAdminThemeVersions 上方注释已解释这个前缀匹配惯例)——批准后
    // 主题网格(官方目录/用户主题分组)和审核队列同时刷新,不需要手动 reload。
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'themes'] }),
  });
}
```

(Add `ContractThemeCatalogRequestStatus`, `ThemeCatalogReviewRequest` to this file's type imports from `@/api/types`.)

- [ ] **Step 5: Add dev-mock routes**

In `web/src/api/mock-handlers.ts`, the two new routes must be checked **before** the existing generic `if (url.startsWith(`${API_BASE}/me/themes/`) && method === 'DELETE')` handler (line 615) — otherwise that broader prefix match swallows `DELETE .../catalog-request` first. Insert immediately before that block (i.e., right after the `POST /me/themes/import` handler closes at line 614):

```ts
  if (url.endsWith('/catalog-request') && url.startsWith(`${API_BASE}/me/themes/`) && method === 'POST') {
    const id = decodeURIComponent(url.slice(`${API_BASE}/me/themes/`.length, -'/catalog-request'.length));
    const theme = mockPrivateThemes.find(item => item.id === id);
    if (!theme) {
      return jsonResponse({ code: 'NOT_FOUND', data: null, meta: { message: '主题不存在', detail: '' } }, 404);
    }
    theme.catalogRequestStatus = 'pending';
    theme.catalogRequestReason = undefined;
    return jsonResponse({
      code: 'OK',
      data: { id: `tcr_mock_${id}`, themeId: id, themeName: theme.name, slug: theme.id, ownerId: theme.ownerId, status: 'pending', reason: '', appliedAt: new Date().toISOString(), reviewedAt: null },
      meta: { message: '已提交目录审核', detail: '' },
    }, 201);
  }
  if (url.endsWith('/catalog-request') && url.startsWith(`${API_BASE}/me/themes/`) && method === 'DELETE') {
    const id = decodeURIComponent(url.slice(`${API_BASE}/me/themes/`.length, -'/catalog-request'.length));
    const theme = mockPrivateThemes.find(item => item.id === id);
    if (theme) theme.catalogRequestStatus = undefined;
    return new Response(null, { status: 204 });
  }
```

- [ ] **Step 6: Type-check and run the mock contract guard**

Run: `cd web && npm run typecheck 2>&1 | tail -40` (use whatever the project's actual typecheck script is named — check `web/package.json` `scripts` if `typecheck` doesn't exist; `make check` runs it too and is the authoritative gate)
Expected: no errors.

Run: `make test-mock`
Expected: PASS — the new mock responses must still satisfy `api/openapi.yaml`'s `Theme`/`ThemeCatalogRequest` shapes for the endpoints that are mocked.

- [ ] **Step 7: Commit**

```bash
git add web/src/api/types.ts web/src/api/themes.ts web/src/api/admin.ts \
  web/src/hooks/useQueries.ts web/src/api/mock-handlers.ts
git commit -m "$(cat <<'EOF'
feat: add theme catalog review API client, hooks, and dev-mock routes
EOF
)"
```

---

## Task 10: Owner UI — submit / pending / rejected states

**Files:**
- Modify: `web/src/pages/app/themes/page.tsx`

**Interfaces:**
- Consumes: `useSubmitCatalogRequest`, `useCancelCatalogRequest` (Task 9); `pkg.meta.catalogRequestStatus`/`catalogRequestReason` (via `themePackagesFromApi`/`ThemePackage`, see `web/src/themes/types.ts` — confirm it already forwards unknown-to-it `Theme` fields through `meta`; if it doesn't, add `catalogRequestStatus`/`catalogRequestReason` to `ThemePackage['meta']` and to `themePackagesFromApi`'s mapping in that file as part of this task).

- [ ] **Step 1: Confirm `ThemePackage.meta` carries the new fields**

Run: `grep -n "catalogRequestStatus\|interface.*meta\|ownerName" web/src/themes/types.ts`

If `catalogRequestStatus`/`catalogRequestReason` are not already mapped from `Theme` onto `ThemePackage.meta` in `themePackagesFromApi`, add them there first (mirroring how `ownerName`/`sourceType` are already forwarded) before touching `page.tsx`.

- [ ] **Step 2: Add the mutation hooks and handlers**

In `web/src/pages/app/themes/page.tsx`, add to the imports (alongside `useMyPage, useThemes, useUpdatePageSettings`):

```ts
import { useMyPage, useThemes, useUpdatePageSettings, useSubmitCatalogRequest, useCancelCatalogRequest } from '@/hooks/useQueries';
```

Near the other mutation/state declarations (close to `const [checkingUpdateId, setCheckingUpdateId] = useState...` — grep the exact neighboring line first with `grep -n "checkingUpdateId" web/src/pages/app/themes/page.tsx`), add:

```ts
  const submitCatalogRequest = useSubmitCatalogRequest();
  const cancelCatalogRequest = useCancelCatalogRequest();
  const [catalogActionId, setCatalogActionId] = useState<string | null>(null);

  const handleSubmitCatalogRequest = useCallback(async (pkg: ThemePackage) => {
    setCatalogActionId(pkg.id);
    try {
      await submitCatalogRequest.mutateAsync(pkg.id);
      toast('success', '已提交官方目录审核');
    } catch (cause) {
      toast('error', cause instanceof ApiError ? (cause.detail || cause.message) : '提交失败');
    } finally {
      setCatalogActionId(null);
    }
  }, [submitCatalogRequest, toast]);

  const handleCancelCatalogRequest = useCallback(async (pkg: ThemePackage) => {
    setCatalogActionId(pkg.id);
    try {
      await cancelCatalogRequest.mutateAsync(pkg.id);
      toast('info', '已撤回目录审核申请');
    } catch (cause) {
      toast('error', cause instanceof ApiError ? (cause.detail || cause.message) : '撤回失败');
    } finally {
      setCatalogActionId(null);
    }
  }, [cancelCatalogRequest, toast]);
```

- [ ] **Step 3: Extend `renderMyThemeCard`**

In `renderMyThemeCard` (starts around line 530), add near the other per-card derived flags (`isUpgrading`, `isCheckingUpdate`, ...):

```ts
    const catalogStatus = pkg.meta.catalogRequestStatus;
    const isCatalogBusy = catalogActionId === pkg.id;
    const canSubmitToCatalog = pkg.meta.status === 'active' && !catalogStatus;
```

Disable the existing upgrade/check-update buttons while pending — change the `canUpgradeGithub`/`canUpgradeUpload`/`canCheckUpdate` gate to also require `catalogStatus !== 'pending'`:

```ts
    const canUpgradeGithub = pkg.meta.sourceType === 'github' && Boolean(pkg.meta.sourceUrl) && catalogStatus !== 'pending';
    const canUpgradeUpload = pkg.meta.sourceType === 'upload' && catalogStatus !== 'pending';
    const canCheckUpdate = pkg.meta.sourceType === 'github' && catalogStatus !== 'pending';
```

Add a status row + action button below the existing action-button `<div>` (right after its closing `</div>`, still inside the `<div key={pkg.id} className="relative group">` wrapper, before that wrapper's own closing `</div>`):

```tsx
        {catalogStatus === 'pending' && (
          <div className="mt-1 flex items-center gap-1.5 text-[10px]">
            <span className="px-1.5 py-0.5 rounded-full bg-primary-50 text-primary-700">审核中</span>
            <button
              type="button"
              disabled={isCatalogBusy}
              onClick={() => void handleCancelCatalogRequest(pkg)}
              className="underline text-foreground-400 hover:text-foreground-600 disabled:opacity-40"
            >
              撤回
            </button>
          </div>
        )}
        {catalogStatus === 'rejected' && (
          <div className="mt-1 flex items-center gap-1.5 text-[10px]">
            <span className="px-1.5 py-0.5 rounded-full bg-red-50 text-red-600" title={pkg.meta.catalogRequestReason || ''}>
              已拒绝{pkg.meta.catalogRequestReason ? `：${pkg.meta.catalogRequestReason}` : ''}
            </span>
            <button
              type="button"
              disabled={isCatalogBusy}
              onClick={() => void handleSubmitCatalogRequest(pkg)}
              className="underline text-foreground-400 hover:text-foreground-600 disabled:opacity-40"
            >
              重新提交
            </button>
          </div>
        )}
        {canSubmitToCatalog && (
          <button
            type="button"
            disabled={isCatalogBusy}
            onClick={() => void handleSubmitCatalogRequest(pkg)}
            className="mt-1 text-[10px] underline text-foreground-400 hover:text-foreground-600 disabled:opacity-40"
          >
            提交官方目录
          </button>
        )}
```

- [ ] **Step 4: Type-check**

Run: `cd web && npx tsc --noEmit 2>&1 | tail -40`
Expected: no errors.

- [ ] **Step 5: Manual smoke test**

Run: `cd web && VITE_ENABLE_API_MOCKS=true npm run dev` (background it or run in a separate terminal), then use the `claude-in-chrome` tools (load them first if deferred) to navigate to `/app/themes`, confirm: a private theme card shows "提交官方目录"; clicking it flips to "审核中" + "撤回"; clicking "撤回" flips it back. Check dark theme too (toggle via the app's theme switcher if present, or `prefers-color-scheme`).

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/app/themes/page.tsx web/src/themes/types.ts
git commit -m "$(cat <<'EOF'
feat: owner-facing submit/cancel/rejected UI for catalog review

Private theme cards on /app/themes gain a "提交官方目录" action, a
pending badge with cancel, and a rejected badge with the admin's
reason and a resubmit action. Upgrade/check-update are disabled while
a request is pending, matching the server-side lock.
EOF
)"
```

---

## Task 11: Admin UI — catalog review queue

**Files:**
- Create: `web/src/pages/admin/themes/components/CatalogRequestsSection.tsx`
- Modify: `web/src/pages/admin/themes/page.tsx` (mount the new section)

**Interfaces:**
- Consumes: `useThemeCatalogRequests`, `useReviewThemeCatalogRequest` (Task 9).

- [ ] **Step 1: Write `CatalogRequestsSection.tsx`**

Structure this exactly like `web/src/pages/admin/operations/components/SubdomainsSection.tsx` (status filter, paginated table, a `ReviewDialog` for approve/reject with an optional reason textarea), swapping subdomain fields for theme-catalog fields and dropping the "revoke" decision (out of scope — see plan header):

```tsx
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check, Palette, RotateCw, ShieldX, X } from 'lucide-react';
import { adminApi } from '@/api/admin';
import type { AdminThemeCatalogRequest, ContractThemeCatalogRequestStatus, ThemeCatalogReviewRequest } from '@/api/types';
import { EmptyState, ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { FormField, FormSelect, FormTextarea } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';
import { useThemeCatalogRequests } from '@/hooks/useQueries';

const statusLabels: Record<ContractThemeCatalogRequestStatus, string> = { pending: '待审核', approved: '已批准', rejected: '已拒绝', revoked: '已撤回' };
const statusStyles: Record<ContractThemeCatalogRequestStatus, string> = { pending: 'bg-primary-50 text-primary-700', approved: 'bg-accent-50 text-accent-700', rejected: 'bg-red-50 text-red-600', revoked: 'bg-background-100 text-foreground-500' };

function ReviewDialog({ request, decision, onClose }: { request: AdminThemeCatalogRequest; decision: ThemeCatalogReviewRequest['decision']; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [reason, setReason] = useState('');
  const labels = { approve: '批准', reject: '拒绝' } as const;
  const mutation = useMutation({
    mutationFn: () => adminApi.reviewThemeCatalogRequest(request.id, { decision, ...(reason.trim() ? { reason: reason.trim() } : {}) }),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['admin', 'themes'] }); toast('success', `目录申请已${labels[decision]}`); onClose(); },
    onError: (error: Error) => toast('error', error.message || '审核目录申请失败'),
  });
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
      <button aria-label="关闭审核窗口" className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative bg-background-50 rounded-xl shadow-overlay border border-background-200/70 p-5 w-full max-w-md">
        <div className="flex items-start">
          <div className="flex-1">
            <h3 className="text-base font-semibold text-foreground-900">{labels[decision]}目录申请</h3>
            <p className="text-xs text-foreground-400 mt-1">{request.ownerName ?? request.ownerId} · {request.themeName}（{request.slug}）</p>
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-background-100"><X className="w-4 h-4" /></button>
        </div>
        <FormField label="审核说明（可选）" className="mt-4">
          <FormTextarea rows={4} maxLength={300} value={reason} onChange={event => setReason(event.target.value)} placeholder="记录审核原因，最多 300 字" />
        </FormField>
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="h-8 px-3 rounded-lg text-xs text-foreground-500 hover:bg-background-100">取消</button>
          <button onClick={() => mutation.mutate()} disabled={mutation.isPending} className={`h-8 px-3 rounded-lg text-xs font-medium text-white flex items-center gap-1.5 disabled:opacity-50 ${decision === 'approve' ? 'bg-primary-500' : 'bg-red-600'}`}>
            {mutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : decision === 'approve' ? <Check className="w-3.5 h-3.5" /> : <ShieldX className="w-3.5 h-3.5" />}
            {labels[decision]}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function CatalogRequestsSection() {
  const [status, setStatus] = useState<ContractThemeCatalogRequestStatus | ''>('pending');
  const [page, setPage] = useState(1);
  const [review, setReview] = useState<{ request: AdminThemeCatalogRequest; decision: ThemeCatalogReviewRequest['decision'] } | null>(null);
  const pageSize = 20;
  const query = useThemeCatalogRequests(status, page, pageSize);
  if (query.isLoading) return <LoadingSkeleton count={4} />;
  if (query.error || !query.data) return <ErrorState message={(query.error as Error)?.message || '加载目录审核申请失败'} onRetry={() => query.refetch()} />;
  const totalPages = query.data.totalPages;

  return (
    <>
      <section className="bg-white rounded-xl border border-background-200/70 overflow-hidden mt-8">
        <div className="p-4 border-b border-background-200/70 flex items-center justify-between gap-3">
          <div><h3 className="text-sm font-semibold text-foreground-800">目录审核申请</h3><p className="text-xs text-foreground-400 mt-0.5">共 {query.data.total} 条记录</p></div>
          <FormSelect value={status} onChange={event => { setStatus(event.target.value as ContractThemeCatalogRequestStatus | ''); setPage(1); }} className="w-32 h-8 text-xs">
            <option value="">全部状态</option>
            <option value="pending">待审核</option>
            <option value="approved">已批准</option>
            <option value="rejected">已拒绝</option>
            <option value="revoked">已撤回</option>
          </FormSelect>
        </div>
        {query.data.items.length === 0 ? (
          <EmptyState icon={Palette} title="没有匹配的申请" description="调整状态筛选后再试。" />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="bg-background-50 border-b border-background-200/70 text-[10px] text-foreground-400">
                  <th className="px-4 py-2.5 font-medium">主题</th>
                  <th className="px-4 py-2.5 font-medium">作者</th>
                  <th className="px-4 py-2.5 font-medium">状态</th>
                  <th className="px-4 py-2.5 font-medium">申请时间</th>
                  <th className="px-4 py-2.5 font-medium">说明</th>
                  <th className="px-4 py-2.5 font-medium text-right">审核</th>
                </tr>
              </thead>
              <tbody>
                {query.data.items.map(item => (
                  <tr key={item.id} className="border-b border-background-100 last:border-0">
                    <td className="px-4 py-3"><p className="text-xs font-medium text-foreground-700">{item.themeName}</p><p className="text-[10px] font-mono text-foreground-400 mt-0.5">{item.slug}</p></td>
                    <td className="px-4 py-3 text-xs text-foreground-700">{item.ownerName ?? '未知用户'}</td>
                    <td className="px-4 py-3"><span className={`text-[10px] px-2 py-0.5 rounded-full ${statusStyles[item.status]}`}>{statusLabels[item.status]}</span></td>
                    <td className="px-4 py-3 text-xs text-foreground-500 whitespace-nowrap">{new Date(item.appliedAt).toLocaleString('zh-CN')}</td>
                    <td className="px-4 py-3 text-xs text-foreground-500 max-w-48 truncate" title={item.reason}>{item.reason || '—'}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        {item.status === 'pending' ? (
                          <>
                            <button onClick={() => setReview({ request: item, decision: 'approve' })} className="h-7 px-2.5 rounded-md text-xs text-accent-700 hover:bg-accent-50">批准</button>
                            <button onClick={() => setReview({ request: item, decision: 'reject' })} className="h-7 px-2.5 rounded-md text-xs text-red-600 hover:bg-red-50">拒绝</button>
                          </>
                        ) : (
                          <span className="text-xs text-foreground-300">已处理</span>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {totalPages > 1 ? (
          <div className="p-3 border-t border-background-200/70 flex items-center justify-between">
            <span className="text-xs text-foreground-400">第 {page} / {totalPages} 页</span>
            <div className="flex gap-1.5">
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={page <= 1} className="h-7 px-2.5 rounded-md border border-background-200 text-xs disabled:opacity-40">上一页</button>
              <button onClick={() => setPage(value => Math.min(totalPages, value + 1))} disabled={page >= totalPages} className="h-7 px-2.5 rounded-md border border-background-200 text-xs disabled:opacity-40">下一页</button>
            </div>
          </div>
        ) : null}
      </section>
      {review ? <ReviewDialog request={review.request} decision={review.decision} onClose={() => setReview(null)} /> : null}
    </>
  );
}
```

- [ ] **Step 2: Mount it in the admin themes page**

In `web/src/pages/admin/themes/page.tsx`, add the import (alongside the other imports):

```tsx
import CatalogRequestsSection from './components/CatalogRequestsSection';
```

Add `<CatalogRequestsSection />` right before the page's closing `</div>` (after the `{privateThemes.length > 0 && (...)}` block, still inside the outermost `<div>`).

- [ ] **Step 3: Type-check**

Run: `cd web && npx tsc --noEmit 2>&1 | tail -40`
Expected: no errors.

- [ ] **Step 4: Manual smoke test**

Since `/admin/theme-catalog-requests` is intentionally **not** mocked (matching the existing precedent that `/admin/subdomains` and `/admin/themes/{id}/versions` aren't mocked either), this section will show its `ErrorState` in `VITE_ENABLE_API_MOCKS=true` dev mode — that's expected and matches sibling admin sections. Verify the real flow instead via `make e2e` (Task 12) or by running the real binary locally (`go run ./cmd/navax`) and exercising `/admin/themes` in a browser against a real backend with a submitted request.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/admin/themes/components/CatalogRequestsSection.tsx web/src/pages/admin/themes/page.tsx
git commit -m "$(cat <<'EOF'
feat: admin catalog review queue on the theme library page

Approve/reject UI for pending private-theme catalog submissions,
structured like the existing subdomain review queue.
EOF
)"
```

---

## Task 12: E2E coverage + full verification pass

**Files:**
- Modify: `tests/e2e/specs/user.spec.ts` (extend the existing `'导入 zip 主题并应用'` test)
- Modify: `tests/e2e/specs/admin.spec.ts` (new test, self-seeded via API calls under the admin's own session — matches this file's existing pattern of not depending on state from other spec files, since Playwright runs spec files in parallel workers by default)

**Interfaces:**
- Consumes: everything from Tasks 1-11, against the real embedded-frontend binary launched by `server.mjs`.

- [ ] **Step 1: Extend the owner-side import test in `user.spec.ts`**

The existing `'导入 zip 主题并应用'` test (lines 135-149) already imports `THEME_ZIP` as a private theme and lands on a card whose accessible name starts with `Lilac`. Add the submit/cancel round-trip right after its last assertion:

```ts
  test('导入 zip 主题并应用', async ({ page }) => {
    await page.goto('/app/themes');
    await page.getByRole('button', { name: /导入主题/ }).click();
    await page.getByRole('button', { name: /上传 zip|zip/i }).click();
    await page.locator('[data-testid="theme-zip-input"]').setInputFiles(THEME_ZIP);
    await page.getByRole('button', { name: /导入/ }).last().click();
    await expect(page.getByText(/已导入主题/)).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('我的主题')).toBeVisible();
    await page.getByRole('button', { name: /^Lilac/ }).click();
    await expect(page.getByText(/主题已写入草稿：「Lilac」/)).toBeVisible();

    // 提交官方目录审核 → 撤回 → 恢复可再次提交。
    await page.getByRole('button', { name: '提交官方目录' }).click();
    await expect(page.getByText('审核中')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: '撤回' }).click();
    await expect(page.getByText('审核中')).toHaveCount(0, { timeout: 10000 });
    await expect(page.getByRole('button', { name: '提交官方目录' })).toBeVisible();
  });
```

- [ ] **Step 2: Add a self-seeded admin approval test to `admin.spec.ts`**

Add the two new imports at the top of `admin.spec.ts` (alongside the existing `import { test, expect } from '@playwright/test';`):

```ts
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';
import { USER } from './accounts';

const THEME_ZIP = fileURLToPath(new URL('../fixtures/theme-lilac.zip', import.meta.url));
```

Add a new test inside the `test.describe('管理员', ...)` block, after `'管理员查看主题历史版本'`:

```ts
  test('管理员批准目录审核申请', async ({ page }) => {
    // 用管理员自己的会话导入一个私有主题、提交审核——不依赖其它 spec 文件的
    // 状态(spec 文件默认并行 worker,不能假设执行顺序或共享服务端状态)。
    const imported = await page.request.post('/api/v1/me/themes/import', {
      multipart: { file: { name: 'lilac.zip', mimeType: 'application/zip', buffer: readFileSync(THEME_ZIP) } },
    });
    expect(imported.ok()).toBeTruthy();
    const themeId = (await imported.json()).data.id as string;

    const submitted = await page.request.post(`/api/v1/me/themes/${themeId}/catalog-request`);
    expect(submitted.ok()).toBeTruthy();

    await page.goto('/admin/themes');
    await expect(page.getByRole('heading', { name: '目录审核申请' })).toBeVisible();
    const row = page.getByRole('row').filter({ hasText: 'Lilac' });
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: '批准' }).click();
    await page.getByRole('button', { name: '批准', exact: true }).last().click(); // 确认对话框里的批准按钮
    await expect(page.getByText('目录申请已批准')).toBeVisible({ timeout: 10000 });

    // 队列行状态刷新为已处理;主题网格失效重取,该主题应移入「官方目录」分组。
    await expect(page.getByRole('row').filter({ hasText: 'Lilac' }).getByText('已处理')).toBeVisible({ timeout: 10000 });
    const catalogSection = page.locator('h3', { hasText: '官方目录' }).locator('xpath=..');
    await expect(catalogSection.getByText('Lilac')).toBeVisible({ timeout: 10000 });
  });
```

If the "批准" confirmation-dialog button's accessible name collides with the queue row's own "批准" button in a way `.last()` doesn't disambiguate cleanly once run, tighten the dialog button locator to scope inside the modal (e.g. `page.getByRole('dialog').getByRole('button', { name: '批准' })`, matching `ReviewDialog`'s DOM — it renders as a `fixed inset-0` overlay, not an ARIA `dialog` role, in the code from Task 11, so `page.locator('.fixed.inset-0')` or a more specific text-scoped locator may be needed; verify against the actual rendered DOM with `npx playwright test --debug` before trusting the selector blind).

- [ ] **Step 3: Run the E2E suite in isolation first**

Run: `make e2e-install` (once, if Playwright/Chromium aren't provisioned yet)
Run: `make e2e 2>&1 | tail -100`
Expected: PASS, including the two new/extended tests. If the dialog-button selector from Step 2's caveat needs adjusting, fix it here before moving on.

- [ ] **Step 4: Run the full verification suite**

Run each of the following and confirm all pass before considering the plan complete:

```bash
make check
go test -race ./...
make build
make test-contract
make test-mock
make e2e
```

Expected: all green. `go test -race ./...` in particular must catch any data race introduced by the new CAS-based `Review` transaction (it mirrors `internal/subdomains`' already-race-tested pattern, so this should be a confirmation, not a discovery step).

- [ ] **Step 5: Commit**

```bash
git add tests/e2e
git commit -m "$(cat <<'EOF'
test: e2e coverage for theme catalog submit/review

Owner submit → cancel round-trip on /app/themes; admin approve flow
on /admin/themes moving a theme into the catalog grid.
EOF
)"
```

---

## Delivery

- Branch: `feat/theme-catalog-review-b2c` off latest `main`, one PR covering all 12 tasks (per the design spec's §8 — the three bundled pieces ship together).
- Shipping path per `CLAUDE.md`: push branch → `gh pr create` → `gh pr merge --auto --rebase` → CI green (`verify`, `e2e`, `container`) → auto-merge → production CD.
- Not in scope (confirmed in the design spec, §8): `approved → revoked` (demote catalog back to private — use the existing kill switch/`enabled` toggle instead); starter official theme repo; background scheduled update polling; pre-account-deletion cleanup; 子项目 C (tier 2 声明式布局).
