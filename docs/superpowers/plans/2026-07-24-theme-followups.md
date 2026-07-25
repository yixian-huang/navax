# 主题机制收尾(缺口 1-3 + 清理)实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐主题规范 v1 落地后的三个缺口——管理后台读不到 manifest 字段、草稿预览没有主题、`themeVersionId` 缺席 openapi 契约——并顺带清掉两处文档矛盾与一批死代码。

**Architecture:** 不引入新概念,全部是把既有管线接到位:管理后台复用 catalog 侧的 LEFT JOIN + manifest 解析模式;预览复用发布已注入的主题解析器(签名从 `*sql.Tx` 放宽为最小查询接口);前端预览页复用公开页渲染组件 `PublicNavigationView`;契约测试扩展既有的按序流程。

**Tech Stack:** Go 1.25 + chi + modernc.org/sqlite;React 19 + Vite + TanStack Query;Playwright E2E;libopenapi-validator 契约测试。

**设计依据:** `docs/superpowers/specs/2026-07-24-theme-followups-design.md`

## Global Constraints

- 分支 `fix/theme-followups`(已存在,spec 已提交),最终单 PR。
- 每个任务提交前该任务涉及的测试必须绿;推送前必须通过 `make check`、`go test -race ./...`、`make build`(CLAUDE.md 全局门槛)。
- Conventional Commit 主题行用英文;用户可见文案与注释用中文。
- `api/openapi.yaml` 是契约唯一来源;`internal/httpapi/` 只做路由/DTO/序列化。
- 迁移文件 append-only——本计划**不新增也不修改任何迁移**(`migrations/0008` 的过期注释因此保留不动)。
- 不引入任何新依赖。
- 前端二空格缩进,`@/` 别名指向 `web/src/`;React hooks / react-router hooks 自动导入,**`useQuery` 不在自动导入清单里**,需手动从 `@tanstack/react-query` 导入。
- E2E 测试文件不得互相 import。

---

## 文件结构总览

| 文件 | 动作 | 任务 |
|---|---|---|
| `api/openapi.yaml` | `PublishedPage` 补 `themeVersionId` | 1 |
| `tests/contract/client_test.go` | `apiResult` 增加 `header` | 1 |
| `tests/contract/api_contract_test.go` | 公开读断言 + 新 t.Run + 预览断言 | 1, 3 |
| `internal/admin/sqlstore.go` | 主题查询 LEFT JOIN + manifest 解析 | 2 |
| `internal/admin/sqlstore_test.go` | 新增字段接线测试 | 2 |
| `internal/navigation/sqlstore.go` | `ThemeQueryer` 接口 + Preview 解析 | 3 |
| `internal/navigation/theme_lock_test.go` | 新增预览一致性测试 | 3 |
| `internal/navigation/sqlstore_test.go` | `testNavigationService` 适配新签名 | 3 |
| `internal/app/run.go` | 接线闭包适配新签名 | 3 |
| `web/src/api/navigation.ts` | 新增 `getPreview` | 4 |
| `web/src/components/feature/PublicNavigationView.tsx` | 新增 `trackEvents` prop | 4 |
| `web/src/pages/app/preview/page.tsx` | 重写为复用公开渲染 | 4 |
| `web/src/api/mock-handlers.ts` | 预览端点 mock + `themeVersionId` | 4 |
| `web/tests/mock-contract.test.ts` | 新增预览 case | 4 |
| `tests/e2e/specs/guest.spec.ts` | link 断言改为无条件 | 5 |
| `tests/e2e/specs/user.spec.ts` | 新增预览主题 E2E | 5 |
| `docs/theme-api.md` | §5 SVG 矛盾修正 | 6 |
| `internal/themes/hooks_test.go` | 新增文档一致性测试 | 6 |
| `internal/themes/builtin/sakura/theme.css` | 头注释修正 | 6 |
| `internal/themes/store.go` | 注释修正;删 `ResolvePackageVersion`/`serviceableVersion` | 6, 7 |
| `internal/themes/store_test.go` | 删除对应测试 | 7 |
| `internal/themes/tokens.go` | 注释修正;删 `var _ = sort.Strings` | 6, 7 |
| `internal/themes/csscompile.go` | 删重言函数 `isFontFamilyProperty` | 7 |
| `web/src/components/base/ThemePicker.tsx` | 删除 | 7 |
| `web/src/api/types.ts` | 删 `ThemeManifest` 接口 | 7 |

---

### Task 1: openapi 补 `themeVersionId` + 公开主题端点契约测试

**Files:**
- Modify: `api/openapi.yaml`(`PublishedPage` schema,约 2290-2315 行)
- Modify: `tests/contract/client_test.go`(`apiResult`,约 55-68、127 行)
- Modify: `tests/contract/api_contract_test.go`(「发布与公开读取」t.Run,约 145-171 行)

**Interfaces:**
- Consumes: 既有 `apiClient.call`/`mustStatus`/`stringField`/`withHeader` 辅助函数。
- Produces: `apiResult.header http.Header` 字段;`TestAPIContract` 顶层变量 `publicThemeVersionID string`(Task 3 的预览断言也在同一文件)。

- [ ] **Step 1: openapi 补字段**

在 `api/openapi.yaml` 的 `PublishedPage.properties` 中、`subdomain` 与 `publishedAt` 之间插入(**不要**加进 `required` 列表——Go 侧是 `omitempty`,迁移前的旧快照没有该字段):

```yaml
        subdomain: { type: [string, 'null'] }
        themeVersionId: { $ref: '#/components/schemas/ThemeVersionId' }
        publishedAt: { $ref: '#/components/schemas/Timestamp' }
```

`ThemeVersionId` schema 已存在(`^v[0-9a-f]{32}$`),无需新增。

- [ ] **Step 2: `apiResult` 暴露响应头**

`tests/contract/client_test.go`:

```go
type apiResult struct {
	status int
	header http.Header
	body   []byte
	json   map[string]any
}
```

`call` 中构造处(约 127 行)改为:

```go
	result := apiResult{status: response.StatusCode, header: response.Header, body: responseBody}
```

- [ ] **Step 3: 扩展契约测试**

`tests/contract/api_contract_test.go`:

1. 在 `TestAPIContract` 内、`systemPageID` 声明的同一位置补一个共享变量:

```go
	var publicThemeVersionID string
```

2. 在「发布与公开读取」t.Run 中 `mustStatus(t, publicPage, http.StatusOK, "公开读取个人页面")` 之后追加:

```go
		// 发布必然锁定主题版本;该字段自本次起登记进 openapi。
		publicThemeVersionID = stringField(t, publicPage.data(), "themeVersionId", "公开页主题版本")
		if !regexp.MustCompile(`^v[0-9a-f]{32}$`).MatchString(publicThemeVersionID) {
			t.Fatalf("themeVersionId 形状不合法: %q", publicThemeVersionID)
		}
```

3. 在「发布与公开读取」与「系统页发布与公开首页」两个 t.Run 之间插入新 t.Run:

```go
	t.Run("主题目录与公开样式表", func(t *testing.T) {
		list := guest.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, list, http.StatusOK, "主题目录")

		cssPath := "/api/v1/public/themes/" + publicThemeVersionID + ".css"
		css := guest.call(t, http.MethodGet, cssPath, nil)
		mustStatus(t, css, http.StatusOK, "主题样式表")
		if cacheControl := css.header.Get("Cache-Control"); !strings.Contains(cacheControl, "immutable") {
			t.Fatalf("样式表缺少 immutable 缓存头: %q", cacheControl)
		}
		etag := css.header.Get("ETag")
		if etag == "" {
			t.Fatal("样式表缺少 ETag")
		}
		cached := guest.call(t, http.MethodGet, cssPath, nil, withHeader("If-None-Match", etag))
		mustStatus(t, cached, http.StatusNotModified, "样式表 304")

		missing := guest.call(t, http.MethodGet,
			"/api/v1/public/themes/v00000000000000000000000000000000.css", nil)
		mustStatus(t, missing, http.StatusNotFound, "未知版本 404")

		missingAsset := guest.call(t, http.MethodGet,
			"/api/v1/public/themes/"+publicThemeVersionID+"/assets/nope.png", nil)
		mustStatus(t, missingAsset, http.StatusNotFound, "未知资产 404")
	})
```

4. import 块按需补 `regexp`、`strings`(`gofmt` 会排序)。

- [ ] **Step 4: 运行契约测试验证通过**

Run: `make test-contract`
Expected: PASS(这些行为服务端早已实现,本任务是契约补登记;若 css 响应校验报错,先检查 openapi 中 `text/css` 定义与实际 `Content-Type` 是否一致,再修 spec 而不是跳过校验)

- [ ] **Step 5: mock 契约守卫回归**

Run: `make test-mock`
Expected: PASS(`themeVersionId` 是可选字段,mock 暂未返回它也合法;Task 4 会补齐)

- [ ] **Step 6: Commit**

```bash
git add api/openapi.yaml tests/contract/
git commit -m "fix: register themeVersionId in the OpenAPI contract and cover public theme endpoints"
```

---

### Task 2: 管理后台主题列表接线 manifest 字段

**Files:**
- Test: `internal/admin/sqlstore_test.go`
- Modify: `internal/admin/sqlstore.go:208-235`(`ListThemes`/`Theme`)、`:438-442`(`scanTheme`)

**Interfaces:**
- Consumes: `themes.SyncBuiltin(ctx, themes.NewStore(db), now)`、`themes.Manifest`(字段 `Subtitle string`、`Tier int`、`Vibe string`、`Swatches [3]string`)。
- Produces: `adminpkg.Theme` 的 `CurrentVersionID/CSSHref/Subtitle/Tier/Scope/Vibe/Swatches` 有值(有编译版本时);`internal/httpapi/admin.go` 的 `themeData` 无需改动——它已按「零值省略」序列化。

- [ ] **Step 1: 写失败测试**

在 `internal/admin/sqlstore_test.go` 末尾追加(import 块补 `strings` 与 `"github.com/yixian-huang/navax/internal/themes"`):

```go
// 管理后台是全量视图:既要读出有编译版本主题的 manifest 字段(色板、vibe
// 分组都依赖它们),也要保留停用且无版本的行——那是它与 eligibility 谓词
// 的职责区别。
func TestThemeListingIncludesSpecV1Fields(t *testing.T) {
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
	items, err := store.ListThemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Theme{}
	for _, item := range items {
		byID[item.ID] = item
	}

	sakura, ok := byID["sakura"]
	if !ok {
		t.Fatal("列表缺少 sakura")
	}
	if sakura.Vibe != "cute" {
		t.Fatalf("sakura vibe = %q, want cute", sakura.Vibe)
	}
	for i, swatch := range sakura.Swatches {
		if !strings.HasPrefix(swatch, "#") {
			t.Fatalf("sakura swatch[%d] = %q,不是真实色板", i, swatch)
		}
	}
	if sakura.CurrentVersionID == "" ||
		sakura.CSSHref != "/api/v1/public/themes/"+sakura.CurrentVersionID+".css" {
		t.Fatalf("sakura 版本字段未接线: %+v", sakura)
	}
	if sakura.Scope != "catalog" || sakura.Tier != 1 || sakura.Subtitle == "" {
		t.Fatalf("sakura manifest 字段未接线: %+v", sakura)
	}

	// migration 0013 停用的 mono 没有编译版本,必须保留在管理列表且 v1 字段为零值。
	mono, ok := byID["mono"]
	if !ok {
		t.Fatal("管理列表必须包含已停用的 mono")
	}
	if mono.Enabled {
		t.Fatal("mono 应处于停用状态")
	}
	if mono.CurrentVersionID != "" || mono.CSSHref != "" || mono.Vibe != "" || mono.Swatches[0] != "" {
		t.Fatalf("无版本主题的 v1 字段应为零值: %+v", mono)
	}

	single, err := store.Theme(ctx, "sakura")
	if err != nil {
		t.Fatal(err)
	}
	if single.Vibe != "cute" || single.CurrentVersionID == "" {
		t.Fatalf("单行查询未接线: %+v", single)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/admin -run TestThemeListingIncludesSpecV1Fields`
Expected: FAIL,`sakura vibe = "", want cute`

- [ ] **Step 3: 实现**

`internal/admin/sqlstore.go`,import 块补 `encoding/json` 与 `"github.com/yixian-huang/navax/internal/themes"`。把 `ListThemes`/`Theme` 的 SQL 与 `scanTheme` 改为:

```go
// 管理后台是全量只读视图:停用的、尚无编译版本的主题也要出现,因此这里
// 是 LEFT JOIN 而不是 catalog 侧的 eligibility 谓词(设计文档 §8.1:
// 管理员目录不复用 eligible)。
const themeSelect = `
	SELECT themes.id, themes.name, themes.version, themes.author, themes.description,
	       themes.mode, themes.preview, themes.enabled, themes.is_default,
	       themes.current_version_id, themes.scope, theme_versions.manifest_json
	FROM themes
	LEFT JOIN theme_versions ON theme_versions.id = themes.current_version_id`

func (s *SQLStore) ListThemes(ctx context.Context) ([]Theme, error) {
	rows, err := s.db.QueryContext(ctx, themeSelect+`
		ORDER BY themes.is_default DESC, themes.name, themes.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Theme, 0)
	for rows.Next() {
		item, err := scanTheme(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) Theme(ctx context.Context, themeID string) (Theme, error) {
	item, err := scanTheme(s.db.QueryRowContext(ctx, themeSelect+`
		WHERE themes.id = ?`, themeID))
	if errors.Is(err, sql.ErrNoRows) {
		return Theme{}, ErrNotFound
	}
	return item, err
}
```

`scanTheme`(约 438 行)改为:

```go
func scanTheme(row rowScanner) (Theme, error) {
	var item Theme
	var currentVersionID, manifestJSON sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.Version, &item.Author, &item.Description,
		&item.Mode, &item.Preview, &item.Enabled, &item.Default,
		&currentVersionID, &item.Scope, &manifestJSON); err != nil {
		return Theme{}, err
	}
	// 无编译版本的主题保持零值,序列化层据此省略字段——缺省即「不可选用」。
	if currentVersionID.Valid && currentVersionID.String != "" {
		item.CurrentVersionID = currentVersionID.String
		item.CSSHref = "/api/v1/public/themes/" + currentVersionID.String + ".css"
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

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/admin`
Expected: PASS(含既有 `TestAdminManagementLifecycle`——若它对 `UpdateTheme` 后的返回值有断言,新增列不影响其读取路径)

- [ ] **Step 5: 契约与 mock 回归**

Run: `make test-contract && make test-mock`
Expected: PASS——`GET /api/v1/admin/themes` 现在返回完整字段,与 openapi `Theme` schema 及 mock 的 `mockThemes` 形状对齐。

- [ ] **Step 6: Commit**

```bash
git add internal/admin/
git commit -m "fix: surface theme manifest fields in the admin listing"
```

---

### Task 3: 服务端草稿预览解析主题版本

**Files:**
- Test: `internal/navigation/theme_lock_test.go`
- Modify: `internal/navigation/sqlstore.go`(resolver 类型约 20-37 行;`Preview` 约 545-560 行)
- Modify: `internal/navigation/sqlstore_test.go:450-470`(`testNavigationService`)
- Modify: `internal/app/run.go:162-164`
- Modify: `tests/contract/api_contract_test.go`(「发布与公开读取」t.Run)

**Interfaces:**
- Consumes: `themes.ResolveEligibleVersion(ctx context.Context, q themes.Queryer, themeID, actorID string) (string, error)`,其中 `themes.Queryer` 只要求 `QueryRowContext`。
- Produces: `navigation.ThemeQueryer` 接口;`ThemeVersionResolver` 新签名 `func(ctx context.Context, q ThemeQueryer, themeID, actorID string) (string, error)`。`*sql.DB` 与 `*sql.Tx` 都满足 `ThemeQueryer`;`ThemeQueryer` 的方法集覆盖 `themes.Queryer`,接口值可直接透传。

- [ ] **Step 1: 写失败测试**

`internal/navigation/theme_lock_test.go` 末尾追加:

```go
// 预览与发布共用同一份主题解析:用户在预览里看到的主题必须与发布后一致,
// 否则「所见即所得」只是口号。
func TestPreviewCarriesThemeVersion(t *testing.T) {
	db, service := testNavigationService(t)
	ctx := context.Background()
	actor := insertTestPersonalPage(t, db, "usr_prev_0001", "previewer", "pg_prev_0001", "cat_prev_001", "prev-page")

	preview, err := service.Preview(ctx, actor, "pg_prev_0001", "https://nav.ax")
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.ThemeVersionID == "" {
		t.Fatal("preview carries no theme version")
	}

	if _, err := service.Publish(ctx, actor, "pg_prev_0001", 0, "https://nav.ax"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	published, err := service.PublicBySlug(ctx, "prev-page")
	if err != nil {
		t.Fatalf("PublicBySlug() error = %v", err)
	}
	if preview.ThemeVersionID != published.ThemeVersionID {
		t.Fatalf("preview theme %q != published theme %q", preview.ThemeVersionID, published.ThemeVersionID)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/navigation -run TestPreviewCarriesThemeVersion`
Expected: FAIL,`preview carries no theme version`

- [ ] **Step 3: 放宽 resolver 签名并在 Preview 中解析**

`internal/navigation/sqlstore.go`——替换现有 `ThemeVersionResolver` 类型定义及其注释:

```go
// ThemeQueryer 是主题解析所需的最小查询能力,*sql.DB 与 *sql.Tx 都满足。
type ThemeQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ThemeVersionResolver 解析出某主体可用的主题版本。发布必须传**发布事务**:
// 解析与写快照之间若隔着事务边界,主题在这个空档被撤销就会留下一条指向
// 已撤销版本的快照。预览只读、不写快照,没有这个窗口,在连接池上解析即可。
type ThemeVersionResolver func(ctx context.Context, q ThemeQueryer, themeID, actorID string) (string, error)
```

`Preview`(约 545 行)在 `attachApprovedSubdomain` 成功之后、`published.ETag = makeETag(published)` 之前插入:

```go
	if s.resolveThemeVersion == nil {
		return PublishedPage{}, fmt.Errorf("navigation: theme version resolver is not wired")
	}
	// 与发布共用同一份解析,预览里看到的主题才与发布后一致。
	themeVersionID, err := s.resolveThemeVersion(ctx, s.db, page.Settings.Appearance.ThemeID, actor.UserID)
	if err != nil {
		return PublishedPage{}, err
	}
	published.ThemeVersionID = themeVersionID
```

`Publish` 内的调用无需改动(`tx` 满足 `ThemeQueryer`)。

- [ ] **Step 4: 适配两处接线闭包**

`internal/app/run.go:162` 与 `internal/navigation/sqlstore_test.go` 的 `testNavigationService` 中,把闭包参数类型从 `tx *sql.Tx` 改为 `q navigation.ThemeQueryer`(测试文件在包内,写 `ThemeQueryer` 即可):

```go
	navigationStore.SetThemeVersionResolver(func(ctx context.Context, q navigation.ThemeQueryer, themeID, actorID string) (string, error) {
		return themes.ResolveEligibleVersion(ctx, q, themeID, actorID)
	})
```

`run.go` 如因此不再直接引用 `database/sql`,同步清理 import(先 `go build ./...` 看报错再动)。

- [ ] **Step 5: 运行测试验证通过**

Run: `go test ./internal/navigation ./internal/app/... && go build ./...`
Expected: PASS

- [ ] **Step 6: 契约测试补预览断言**

`tests/contract/api_contract_test.go` 「发布与公开读取」t.Run 中、`publicThemeVersionID` 断言之后追加:

```go
		previewResult := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/preview", adminPageID), nil)
		mustStatus(t, previewResult, http.StatusOK, "草稿预览")
		// 预览与发布解析出同一个版本(同一份草稿、同一个谓词)。
		if got := stringField(t, previewResult.data(), "themeVersionId", "预览主题版本"); got != publicThemeVersionID {
			t.Fatalf("预览主题版本 %q != 发布主题版本 %q", got, publicThemeVersionID)
		}
```

Run: `make test-contract`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/navigation/ internal/app/ tests/contract/
git commit -m "fix: resolve the locked theme version for draft previews"
```

---

### Task 4: 前端预览页复用公开渲染

**Files:**
- Modify: `web/src/api/navigation.ts`(`getPublicPage` 之后)
- Modify: `web/src/components/feature/PublicNavigationView.tsx:57-90`
- Rewrite: `web/src/pages/app/preview/page.tsx`
- Modify: `web/src/api/mock-handlers.ts`(`contractPublishedResponse` 约 154-182 行、`mapContractUrlToLegacy` 约 1077 行、公开页 handler 约 811 行)
- Modify: `web/tests/mock-contract.test.ts`(`cases` 数组)

**Interfaces:**
- Consumes: `navigationApi`(`web/src/api/navigation.ts` 导出)、`normalizePublishedPage`、`PublicNavigationView`、`useMyPage`/`usePublish`/`usePublishUiState`、`mockThemes`(来自 `@/mocks/data`,已在 mock-handlers 中导入)。
- Produces: `navigationApi.getPreview(pageId: string): Promise<ApiResponse<PublishedNavigationPage>>`;`PublicNavigationViewProps.trackEvents?: boolean`(默认 `true`)。

- [ ] **Step 1: API 层新增 `getPreview`**

`web/src/api/navigation.ts`,紧跟 `getPublicPage` 之后:

```ts
  // 草稿预览与公开读取共用同一契约形状与 normalize 逻辑。
  getPreview: async (pageId: string) => {
    const response = await request<ApiResponse<PublishedPageContract>>(
      `/pages/${encodeURIComponent(pageId)}/preview`,
    );
    return envelope(response, normalizePublishedPage(response.data));
  },
```

- [ ] **Step 2: `PublicNavigationView` 增加 `trackEvents`**

`web/src/components/feature/PublicNavigationView.tsx`:

props 接口(57 行起)追加:

```ts
  /** 预览态置 false:page_view 与 site_click 都不上报公开统计。 */
  trackEvents?: boolean;
```

函数签名解构追加 `trackEvents = true`,并把 87 行的 tracker 调用改为:

```ts
  const recordSiteClick = usePublicEventTracker(trackEvents ? page?.id : undefined, page?.snapshotId);
```

(`usePublicEventTracker` 对 `pageId` 为空时两条路径都直接 no-op,无需改 hook。)

- [ ] **Step 3: 重写预览页**

`web/src/pages/app/preview/page.tsx` 整体替换为(保留原有工具栏与发布逻辑,内容区换成公开渲染;`PreviewCategorySection`/`PreviewSiteCard` 删除):

```tsx
// ============================================================
// nav.ax Draft Preview — /app/preview
// 复用公开页渲染组件:预览端点返回的就是 PublishedPage 契约形状,
// 主题、壁纸、布局模板与发布后一致(所见即所得)。
// ============================================================

import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowLeft, ExternalLink, Globe, Loader2 } from 'lucide-react';
import { useMyPage, usePublish } from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import {
  handlePublishError,
  navigateToVisibilityFix,
  publishSettingsPath,
  resolvePrimaryPublishIntent,
  toastForPublishSuccess,
} from '@/lib/publish-actions';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { navigationApi } from '@/api/navigation';
import PublicNavigationView from '@/components/feature/PublicNavigationView';

export default function PreviewPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const pageQuery = useMyPage();
  const {
    state,
    scope,
    slug,
    publication,
    refetch,
  } = usePublishUiState('preview');
  const { mutate: publishMutation, isPending: publishing } = usePublish();

  const pageId = pageQuery.data?.id;
  const previewQuery = useQuery({
    queryKey: ['preview', pageId],
    queryFn: async () => {
      const response = await navigationApi.getPreview(pageId!);
      return response.data;
    },
    enabled: !!pageId,
    staleTime: 0,
  });

  if (pageQuery.isLoading) return <LoadingSkeleton count={4} />;
  if (pageQuery.isError) {
    return (
      <ErrorState
        message={pageQuery.error?.message || '无法加载草稿预览'}
        onRetry={() => pageQuery.refetch()}
      />
    );
  }

  const isPublished = state.showUnpublish || publication?.published === true;
  const liveSlug = slug || publication?.slug || previewQuery.data?.title;

  const handlePrimaryPublish = () => {
    const intent = resolvePrimaryPublishIntent(state);
    if (intent === 'noop') return;
    if (intent === 'redirect_visibility') {
      navigateToVisibilityFix(navigate, scope);
      return;
    }

    const stateBefore = state;
    publishMutation(undefined, {
      onSuccess: () => {
        toast('success', toastForPublishSuccess(stateBefore));
      },
      onError: (cause: Error) => {
        handlePublishError(cause, {
          toast,
          refetch: () => { void refetch(); },
          navigateToVisibilityFix: () => navigateToVisibilityFix(navigate, scope),
        });
      },
    });
  };

  return (
    <div>
      <div className="mb-4 flex items-center justify-between gap-3 flex-wrap">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Link
              to={publishSettingsPath(scope)}
              className="inline-flex items-center gap-1 text-xs text-foreground-400 hover:text-foreground-600"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              返回发布
            </Link>
          </div>
          <h1 className="text-2xl font-bold font-heading text-foreground-950">草稿预览 · 非公开</h1>
          <p className="text-sm text-foreground-400 mt-1">
            与发布后完全一致的渲染(主题、壁纸、布局),未发布内容不会影响公开页
          </p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {state.primaryAction !== 'none' && (
            <button
              type="button"
              onClick={handlePrimaryPublish}
              disabled={publishing || state.primaryDisabled}
              className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed inline-flex items-center gap-1.5 transition-colors duration-150"
            >
              {publishing ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <Globe className="w-3.5 h-3.5" />
              )}
              {publishing ? '发布中…' : state.primaryLabel}
            </button>
          )}
          {isPublished && liveSlug && (
            <Link
              to={`/u/${liveSlug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="h-9 px-3 rounded-lg border border-background-200 text-sm text-foreground-600 hover:bg-background-100 inline-flex items-center gap-1.5"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              打开线上版
            </Link>
          )}
        </div>
      </div>

      {/* 全宽渲染公开页组件:负 margin 抵消 AppShell 内容区的 p-4/md:p-5。 */}
      <div className="-mx-4 md:-mx-5 -mb-4 md:-mb-5 border-t border-background-200/70">
        <PublicNavigationView
          page={previewQuery.data}
          isLoading={previewQuery.isLoading}
          error={previewQuery.error ?? undefined}
          onRetry={() => { void previewQuery.refetch(); }}
          displayName={previewQuery.data?.ownerName || '朋友'}
          showBrowserGuide={false}
          share={null}
          trackEvents={false}
        />
      </div>
    </div>
  );
}
```

注意:`liveSlug` 原实现取自 `preview.slug`,`PublishedNavigationPage` 没有 `slug` 字段——以 `slug || publication?.slug` 为准,兜底 `previewQuery.data?.title` 仅防空,行为与原页面一致(原页面的 `preview.slug` 兜底在有 publication 时同样不会命中)。

- [ ] **Step 4: mock 补预览端点与 `themeVersionId`**

`web/src/api/mock-handlers.ts` 三处:

1. `contractPublishedResponse`(约 158 行)返回对象中、`etag` 之前加一行,并在文件内(该函数上方)加派生函数:

```ts
// 与真实后端一致:发布/预览响应携带锁定的主题版本。mock 直接复用
// mockThemes 里的稳定假哈希,满足契约的 ^v[0-9a-f]{32}$。
function mockThemeVersionId(themeId: string): string {
  return mockThemes.find(theme => theme.id === themeId)?.currentVersionId
    ?? 'v00000000000000000000000000000001';
}
```

```ts
    themeVersionId: mockThemeVersionId(source.themeId),
```

2. `mapContractUrlToLegacy`(约 1077 行的 pages 分支之前)追加映射:

```ts
  } else if (/^\/api\/v1\/pages\/[^/]+\/preview$/.test(path)) {
    legacyPath = `${API_BASE}/navigation/preview`;
    keepSearch = false;
```

3. 公开页 handler(约 811 行 `if (url.startsWith(...navigation/public/...))`)之前追加:

```ts
  if (url === `${API_BASE}/navigation/preview`) {
    // 草稿预览的契约形状与公开读取一致;mock 保真度以形状为准。
    const currentSub = getMockSubdomain();
    const previewPage = contractPublishedResponse(mockPublishedPage, 'personal', currentSub?.subdomain || '');
    return Promise.resolve(jsonResponse({
      code: 'OK',
      data: { ...previewPage, snapshotId: `preview_${previewPage.id}` },
      meta: { message: '', detail: '' },
    }));
  }
```

- [ ] **Step 5: mock 契约守卫补 case**

`web/tests/mock-contract.test.ts` 的 `cases` 数组(「当前草稿页」之后)追加:

```ts
  { name: '草稿预览', path: '/api/v1/pages/{pageId}/preview', method: 'get', status: '200', url: '/api/v1/pages/pg_mock_001/preview' },
```

- [ ] **Step 6: 类型检查与守卫**

Run: `make check && make test-mock`
Expected: PASS(若 `snapshotId` 的 `preview_` 前缀撞上 `Id` schema 约束,守卫已放宽长度、未放宽 pattern——`Id` schema 若含 pattern 导致失败,则去掉 `snapshotId` 覆盖行,直接用 `contractPublishedResponse` 原值)

- [ ] **Step 7: 浏览器冒烟(六态)**

Run: `cd web && VITE_ENABLE_API_MOCKS=true npm run dev`,浏览 `http://localhost:3000/app/preview`
Expected:
- 加载态:骨架屏(PublicNavigationView 内置);
- 正常态:公开页渲染 + 主题 `<link>` 生效(mock CSS),`[data-nx="page-root"]` 上有 `data-theme`;
- 空态:mock 清空分类后显示公开组件空态;
- 错误态:断网/改 mock 返回 500 后出现重试;
- 移动端:视口 375px 无横向滚动;
- 暗色:切 Slate Dark 后预览区域随主题变化,工具栏(宿主 UI)不受影响;
- 统计静默:Network 面板确认预览态没有 `/public/events` 请求。

- [ ] **Step 8: Commit**

```bash
git add web/src/ web/tests/
git commit -m "feat: render draft preview with the real public page pipeline"
```

---

### Task 5: E2E——guest 断言收紧 + 预览主题用例

**Files:**
- Modify: `tests/e2e/specs/guest.spec.ts:84-94`
- Modify: `tests/e2e/specs/user.spec.ts`(「切换主题为 Slate Dark」之后)

**Interfaces:**
- Consumes: registry 在激活成功后于主题根写 `data-theme`、样式表 link 带 `data-theme-style` 属性;user 流程中上一个用例已把 Slate Dark 写入草稿。
- Produces: 无。

- [ ] **Step 1: guest link 断言改为无条件**

`tests/e2e/specs/guest.spec.ts` 把整个 `if (await link.count()) { … }` 块替换为:

```ts
  test('主题样式经内容寻址的 link 供应', async ({ page }) => {
    await page.goto('/');
    // 发布路径必然锁定主题版本——条件断言会静默空过,这里必须有 link。
    const link = page.locator('link[data-theme-style]');
    await expect(link).toHaveCount(1);
    const href = await link.first().getAttribute('href');
    expect(href).toMatch(/\/api\/v1\/public\/themes\/v[0-9a-f]{32}\.css$/);
    const response = await page.request.get(href!);
    expect(response.status()).toBe(200);
    expect(response.headers()['cache-control']).toContain('immutable');
  });
```

- [ ] **Step 2: user 增加预览主题用例**

`tests/e2e/specs/user.spec.ts`,紧跟「切换主题为 Slate Dark」用例之后(同文件内用例按声明顺序执行,依赖上一用例已把主题写入草稿):

```ts
  test('草稿预览呈现所选主题', async ({ page }) => {
    await page.goto('/app/preview');
    const root = page.locator('[data-nx="page-root"]');
    // registry 在样式表加载成功后才写 data-theme,断言它即断言整条链路。
    await expect(root).toHaveAttribute('data-theme', 'slate-dark', { timeout: 10000 });
    await expect(page.locator('link[data-theme-style]')).toHaveCount(1);
  });
```

- [ ] **Step 3: 运行 E2E**

Run: `make e2e`(首次需 `make e2e-install`)
Expected: PASS(guest/user/admin 全部)

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/
git commit -m "test: assert theme link unconditionally and cover themed draft preview"
```

---

### Task 6: 文档矛盾修正 + 钩子清单一致性测试

**Files:**
- Modify: `docs/theme-api.md`(§5,约 133 行)
- Test: `internal/themes/hooks_test.go`
- Modify: `internal/themes/builtin/sakura/theme.css:1-12`
- Modify: `internal/themes/store.go:31-32`、`internal/themes/tokens.go:9`

**Interfaces:**
- Consumes: `AllowedHooks() []string`(`hooks.go:51`,返回已排序副本)。
- Produces: 无新接口;`AllowedHooks` 自此有生产级消费者(文档一致性测试),不再是疑似死代码。

- [ ] **Step 1: 写钩子一致性测试**

`internal/themes/hooks_test.go` 末尾追加(import 补 `os`、`regexp`、`slices`、`sort`、`strings`;`path/filepath` 如缺同补):

```go
// docs/theme-api.md 第 6 行向主题作者承诺「钩子清单与 hooks.go 一致,有测试
// 保证」——本测试兑现该承诺。只解析 §2 钩子表格:文档其余表格(迁移映射等)
// 的首列不是钩子,必须先按章节切片再匹配。
func TestHooksMatchThemeAPIDoc(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "theme-api.md"))
	if err != nil {
		t.Fatalf("read theme-api.md: %v", err)
	}
	content := string(raw)
	start := strings.Index(content, "## 2. 钩子清单")
	if start < 0 {
		t.Fatal("docs/theme-api.md 缺少「## 2. 钩子清单」章节")
	}
	section := content[start:]
	if end := strings.Index(section, "\n### "); end >= 0 {
		section = section[:end]
	} else if end := strings.Index(section, "\n## 3"); end >= 0 {
		section = section[:end]
	}

	rowPattern := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|")
	documented := []string{}
	for _, match := range rowPattern.FindAllStringSubmatch(section, -1) {
		documented = append(documented, match[1])
	}
	sort.Strings(documented)

	allowed := AllowedHooks()
	sort.Strings(allowed)
	if !slices.Equal(documented, allowed) {
		t.Fatalf("docs/theme-api.md §2 = %v\nhooks.go = %v", documented, allowed)
	}
}
```

- [ ] **Step 2: 运行(应当直接通过)**

Run: `go test ./internal/themes -run TestHooksMatchThemeAPIDoc`
Expected: PASS——两边当前各 22 项且一致。若 FAIL,输出会给出两侧差集;修文档而不是改 `hooks.go`(钩子清单是既定契约)。再做一次变异验证:临时在文档表格加一行假钩子,确认测试变红,然后还原。

- [ ] **Step 3: 修正 §5 的 SVG 矛盾**

`docs/theme-api.md` §5「被拒绝的写法」中,把

```
- 外部 URL、形如 URL 的字符串字面量、`data:image/svg+xml`
```

替换为(与 §3/§4.1 的放宽规则对齐,替换前先读 §3 对应条目保持措辞一致):

```
- 外部 URL、形如 URL 的字符串字面量;`data:` 图片仅限 png/jpeg/webp/svg+xml 且单条 ≤ 8 KB(SVG 需通过净化检查;`.svg` 资产**文件**仍一律拒绝——图片上下文与文档上下文的风险不同)
```

- [ ] **Step 4: 修正 sakura 头注释**

`internal/themes/builtin/sakura/theme.css` 头部,把

```
 * - 卡片右上角的 `content: '✿'` 装饰字符被规则拒绝（content 只允许 "" 与 none），
 *   本次不引入资产，因此整条规则删除 —— 记录为视觉降级。
```

替换为:

```
 * - 卡片右上角的 `content: '✿'` 装饰：content 规则已放宽为「≤ 2 个符号/
 *   标点码位」，本主题恢复使用（见文末 :hover 规则），不再是视觉降级。
```

注意:`theme.css` 变更会改变 sakura 的 content hash,启动时会幂等生成一个新版本行——这是设计内行为,`SyncBuiltin` 测试覆盖。

- [ ] **Step 5: 修正两处过期 Go 注释**

`internal/themes/store.go:31-32`,把

```go
// 与 web/src/lib/themeResolve.ts 的 THEME_ID_ALIASES 保持一致：解析统一收敛到
// 服务端一处，前端不再自行兜底。
```

替换为:

```go
// 前端旧实现（themeResolve.ts，已随主题规范 v1 删除）曾在浏览器里做同样的
// 映射；现在解析统一收敛到服务端这一处，前端不再自行兜底。
```

`internal/themes/tokens.go:9`,把

```go
// 基线令牌取自默认主题 slate（web/src/themes/packages/slate.ts）。
```

替换为:

```go
// 基线令牌取自默认主题 slate 的令牌集（internal/themes/builtin/slate/theme.json）。
```

- [ ] **Step 6: 运行主题包全部测试**

Run: `go test ./internal/themes`
Expected: PASS(含 builtin 编译确定性——注释变更改变 hash 是预期,相关测试断言的是「同一输入两次编译一致」,不是固定哈希值)

- [ ] **Step 7: Commit**

```bash
git add docs/theme-api.md internal/themes/
git commit -m "docs: fix theme-api contradictions and enforce hook list consistency with a test"
```

---

### Task 7: 死代码清理

**Files:**
- Delete: `web/src/components/base/ThemePicker.tsx`
- Modify: `web/src/api/types.ts`(约 370 行 `ThemeManifest` 接口)
- Modify: `internal/themes/store.go`(删 `ResolvePackageVersion` + `serviceableVersion`,约 201-260 行)
- Modify: `internal/themes/store_test.go`(删除仅覆盖上述两函数的测试)
- Modify: `internal/themes/tokens.go:104`(删 `var _ = sort.Strings`)
- Modify: `internal/themes/csscompile.go:219、230-232`(删重言函数)

**Interfaces:**
- Consumes: 无。
- Produces: 无(纯删除;`ResolveEligibleVersion` 是全部生产路径的解析入口,别名与默认回落语义由 `eligibility_test.go` 继续覆盖)。

- [ ] **Step 1: 删除前先核实零引用**

Run:
```bash
grep -rn "ThemePicker" web/src --include="*.tsx" --include="*.ts"
grep -rn "ThemeManifest" web/src --include="*.tsx" --include="*.ts"
grep -rn "ResolvePackageVersion\|serviceableVersion" --include="*.go" internal/ cmd/ tests/
```
Expected: `ThemePicker` 只有自身文件;`ThemeManifest` 只有 `types.ts` 定义处;`ResolvePackageVersion`/`serviceableVersion` 只在 `internal/themes/store.go` 与 `internal/themes/store_test.go`。**任何一条出现额外引用,该项跳过删除并在 PR 描述里说明。**

- [ ] **Step 2: 执行删除**

1. `git rm web/src/components/base/ThemePicker.tsx`;
2. `web/src/api/types.ts` 删除 `ThemeManifest` 接口整块;
3. `internal/themes/store.go` 删除 `ResolvePackageVersion`、`serviceableVersion` 两个函数及各自注释;
4. `internal/themes/store_test.go` 删除引用它们的测试函数(以 `grep -n "ResolvePackageVersion" internal/themes/store_test.go` 定位,整函数删除;删除前确认 `eligibility_test.go` 已覆盖同等语义:culled 别名回落、默认主题不可用时 `ErrDefaultThemeUnavailable`——已确认存在);
5. `internal/themes/tokens.go` 删除 `var _ = sort.Strings` 行;若 `sort` 自此无引用(`grep -n "sort\." internal/themes/tokens.go`),同时删除 import;
6. `internal/themes/csscompile.go:219` 的 case 条件 `isFontFamilyProperty(property, c.inAtRule("font-face"))` 改为 `property == "font-family"`,并删除 `isFontFamilyProperty` 函数(230-232 行,`inFontFace` 分支是重言)。**先确认**该 case 与相邻 case 的行为等价:函数体是 `property == "font-family" || (inFontFace && property == "font-family")`,与 `property == "font-family"` 恒等;若实现时发现与此不符,保留原样并记录。

- [ ] **Step 3: 全量回归**

Run: `make check && go test ./... && make build`
Expected: PASS,无未使用 import 报错

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore: remove dead theme code left over from the spec v1 rewrite"
```

---

### Task 8: 全量验证与交付

**Files:** 无新增修改(只跑验证与 PR 流程)。

- [ ] **Step 1: 全量门槛**

Run:
```bash
make check
go test -race ./...
make build
make test-contract
make test-mock
make e2e
```
Expected: 全部 PASS。任何一项失败,回到对应任务修复后重跑,不带失败推送。

- [ ] **Step 2: 推送并创建 PR**

```bash
git push -u origin fix/theme-followups
gh pr create --title "fix: close the theme mechanism follow-up gaps" --body "$(cat <<'EOF'
## Summary
- 管理后台主题列表接线 manifest 字段(色板/vibe/版本,LEFT JOIN 保留停用与无版本行)
- 草稿预览所见即所得:服务端 Preview 解析锁定主题版本(与发布共用 eligibility 谓词),前端 /app/preview 复用公开页渲染组件,预览态关闭公开统计
- openapi 补登记 PublishedPage.themeVersionId;契约测试覆盖公开主题端点(200/304/404)与预览/发布版本一致性
- guest E2E 主题 link 断言改为无条件;新增预览主题 E2E
- 修正 theme-api.md §5 与 §3 的 SVG 矛盾、sakura 头注释、两处过期 Go 注释;新增钩子清单与文档一致性测试
- 清理死代码:ThemePicker.tsx、ThemeManifest 接口、ResolvePackageVersion/serviceableVersion 等

设计文档:docs/superpowers/specs/2026-07-24-theme-followups-design.md

## Test plan
- [x] make check / go test -race ./... / make build
- [x] make test-contract / make test-mock / make e2e
- [x] 浏览器六态冒烟(/app/preview:加载/空/错误/移动/键盘/暗色)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr merge --auto --rebase
```

- [ ] **Step 3: 确认 CI 绿灯与自动合并**

Run: `gh pr checks --watch`
Expected: `verify`、`e2e`、`container` 全绿,PR 自动合并,分支自动删除。合并后 `deploy-production` 自动跑,无需人工确认。
