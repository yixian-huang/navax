# 主题导入与私有安装(B1)实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让第三方主题能以 zip 上传或 GitHub 导入的方式安装为用户私有主题,可升级、可卸载,并提供作者 dry-run 校验端点。

**Architecture:** 管线零改动——两条来源解包成 `themes.Package` 后走既有 `Compile → UpsertVersion` 路径。新增:`internal/themes/archive.go`(纯解包,无网络)、`internal/themeimport` 包(GitHub 拉取 + 导入编排,依赖 netguard)、store 的 `InstallPrivate/UninstallPrivate`(首个 `themes` 行创建路径)、HTTP 面三端点。元数据回写统一放进版本 upsert 的事务辅助函数,内置同步顺带消除既知漂移。

**Tech Stack:** Go 1.25(stdlib `archive/zip`/`archive/tar`/`compress/gzip`)+ chi + modernc.org/sqlite + 既有 `internal/netguard`;React 19 + Vite;Playwright。**不新增任何 Go/npm 依赖。**

**设计依据:** `docs/superpowers/specs/2026-07-24-theme-import-b1-design.md`

## Global Constraints

- 分支 `feat/theme-import-b1`(已存在,spec 已提交);最终单 PR。
- 每任务提交前该任务测试必须绿;推送前 `make check`、`go test -race ./...`、`make build` 全绿(CLAUDE.md)。
- Conventional Commit 英文主题行;用户可见文案与注释中文;gofmt 干净。
- `api/openapi.yaml` 是契约唯一来源;`internal/httpapi/` 只做路由/DTO/序列化,业务逻辑在域包。
- 迁移文件 append-only;**本计划不新增迁移**(0014 的表结构已够用)。
- 解包硬限(导出常量):压缩输入与解压总量 ≤ `16 << 20` 字节、文件数 ≤ 200;包体/CSS/资产体积沿用编译层既有常量,解包层不重复。
- 私有配额:`NAVAX_THEME_PRIVATE_QUOTA` 默认 10,按 owner 的 `themes` 行数计(含墓碑);升级不占新额度。
- GitHub:锁 commit sha 记入 `source_ref`;主机白名单默认 `github.com`,`NAVAX_THEME_IMPORT_HOSTS` 追加的主机按 Gitea 兼容布局(`/{owner}/{repo}/archive/{ref}.tar.gz`,须显式 ref);`NAVAX_GITHUB_TOKEN` 可选;全部出网走 `netguard.GuardedClient`。
- 限流(AbuseProtection 规则表):`POST /api/v1/me/themes/import` 10 次/小时/IP;`POST /api/v1/themes/validate` 20 次/小时/IP。
- 错误码沿用全仓字面量惯例:`QUOTA_EXCEEDED`(409)、`THEME_INVALID`(422)、`UPSTREAM_ERROR`(502)、`NOT_FOUND`(404)、`VALIDATION_FAILED`(422)。
- 默认主题必须是目录主题:管理端把私有主题设为默认 → 409。

## 文件结构总览

| 文件 | 动作 | 任务 |
|---|---|---|
| `internal/themes/archive.go` + `archive_test.go` | 新增:zip/tar.gz 解包 + 防护 + 组包 | 1 |
| `internal/themes/store.go` | `upsertVersionTx` 重构 + 元数据/preview 回写 | 2 |
| `internal/themes/sync_test.go` | 漂移消除断言 | 2 |
| `internal/themes/install.go` + `install_test.go` | 新增:InstallPrivate / UninstallPrivate | 3, 4 |
| `internal/themes/lifecycle_test.go` | 新增:用户删除 RESTRICT 不变量 | 4 |
| `internal/admin/sqlstore.go` + `service.go` + `internal/httpapi/admin.go` | 私有主题不可设默认 | 4 |
| `internal/themeimport/github.go` + `github_test.go` | 新增:GitHub/Gitea 拉取 | 5 |
| `internal/themeimport/service.go` + `service_test.go` | 新增:导入编排 + dry-run | 6 |
| `internal/config/config.go` | 三个新 env | 7 |
| `internal/httpapi/themeimport.go` + `rate_limit.go` + `internal/app/run.go` | HTTP 面 + 限流 + 接线 | 7 |
| `api/openapi.yaml` | 三端点 + schema + Theme 扩展 | 7, 9 |
| `tests/contract/client_test.go` + `api_contract_test.go` | multipart 泛化 + 导入流程 | 8 |
| `web/src/api/mock-handlers.ts` + `web/tests/mock-contract.test.ts` | mock 导入面 | 8 |
| `internal/catalog/service.go` + `internal/httpapi/catalog.go` | 私有主题透出 sourceType/sourceUrl | 9 |
| `web/src/api/themes.ts`(新)+ `types.ts` + `web/src/themes/types.ts` | 前端 API 与类型 | 9 |
| `web/src/pages/app/themes/page.tsx` + `web/src/components/base/ThemeImportDialog.tsx`(新) | 导入 UI + 我的主题分组 | 10 |
| `tests/e2e/fixtures/theme-lilac.zip`(新)+ `specs/user.spec.ts` | E2E | 11 |
| `docs/theme-api.md` | §6 导入与安装 | 12 |

**两处与 spec 的既定偏差(写给评审者):** (a) 墓碑主题不在 `GET /api/v1/themes` 出现(eligibility 谓词天然排除),B1 的 UI 不单独展示墓碑,配额 409 文案说明「含已卸载但仍被历史发布引用的主题」——保持「列表 = 可选」契约干净;(b) `NAVAX_THEME_IMPORT_HOSTS` 的追加主机按 Gitea 兼容 archive 布局实现并写入文档,GitHub 之外不做逐平台适配(spec §9 已排除)。

---

### Task 1: 解包层 `internal/themes/archive.go`

**Files:**
- Create: `internal/themes/archive.go`
- Test: `internal/themes/archive_test.go`

**Interfaces:**
- Consumes: `ParseManifest(data []byte) (Manifest, error)`(manifest.go)、`ValidateAsset(assetPath string, data []byte) (Asset, error)`(assets.go:66)、`Package{Manifest, CSS, Assets}`(compile.go:14)。
- Produces:
  - `const MaxArchiveBytes = 16 << 20`、`const MaxArchiveFiles = 200`
  - `var ErrInvalidArchive = errors.New("invalid theme archive")`
  - `func ExtractZip(data []byte) (map[string][]byte, error)`
  - `func ExtractTarGz(data []byte) (map[string][]byte, error)`
  - `func PackageFromFiles(files map[string][]byte) (Package, error)`

- [ ] **Step 1: 写失败测试**

`internal/themes/archive_test.go`(与既有测试同包 `themes`;`makeZip`/`makeTarGz` 是本文件的测试辅助):

```go
package themes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"
)

// makeZip 按序写入 entries(路径→内容)构造 zip 字节。
func makeZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

type tarEntry struct {
	name     string
	data     []byte
	typeflag byte
	linkname string
}

func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		flag := entry.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.data)), Typeflag: flag, Linkname: entry.linkname}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("tar header %s: %v", entry.name, err)
		}
		if _, err := tw.Write(entry.data); err != nil {
			t.Fatalf("tar write %s: %v", entry.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractZipRejectsHostileArchives(t *testing.T) {
	cases := []struct {
		name    string
		entries map[string][]byte
		wantSub string
	}{
		{"zip-slip 相对逃逸", map[string][]byte{"../evil.css": []byte("x")}, "路径"},
		{"绝对路径", map[string][]byte{"/etc/passwd": []byte("x")}, "路径"},
		{"反斜杠路径", map[string][]byte{`assets\evil.png`: []byte("x")}, "路径"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExtractZip(makeZip(t, tc.entries))
			if !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("err = %v, want ErrInvalidArchive", err)
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("err = %q, want contains %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestExtractZipEnforcesBudgets(t *testing.T) {
	// 文件数超限。
	many := map[string][]byte{}
	for i := 0; i < MaxArchiveFiles+1; i++ {
		many["f"+strings.Repeat("0", 3)+string(rune('a'+i%26))+strings.Repeat("x", i/26)+".txt"] = []byte("1")
	}
	if _, err := ExtractZip(makeZip(t, many)); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("file-count err = %v, want ErrInvalidArchive", err)
	}
	// 解压炸弹:高压缩比,解压总量超 MaxArchiveBytes。
	bomb := makeZip(t, map[string][]byte{"big.css": bytes.Repeat([]byte{0}, MaxArchiveBytes+1)})
	if _, err := ExtractZip(bomb); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("bomb err = %v, want ErrInvalidArchive", err)
	}
	// 压缩输入本身超限。
	if _, err := ExtractZip(make([]byte, MaxArchiveBytes+1)); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("input-size err = %v, want ErrInvalidArchive", err)
	}
}

func TestExtractTarGzRejectsLinks(t *testing.T) {
	for _, flag := range []byte{tar.TypeSymlink, tar.TypeLink} {
		archive := makeTarGz(t, []tarEntry{{name: "theme.css", typeflag: flag, linkname: "/etc/passwd"}})
		if _, err := ExtractTarGz(archive); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("typeflag %d err = %v, want ErrInvalidArchive", flag, err)
		}
	}
}

func TestExtractStripsSingleTopLevelDir(t *testing.T) {
	// GitHub tarball 形态:全部条目在 {repo}-{sha}/ 之下。
	archive := makeTarGz(t, []tarEntry{
		{name: "repo-abc123/theme.json", data: []byte("{}")},
		{name: "repo-abc123/theme.css", data: []byte("i{}")},
	})
	files, err := ExtractTarGz(archive)
	if err != nil {
		t.Fatalf("ExtractTarGz() error = %v", err)
	}
	if _, ok := files["theme.json"]; !ok {
		t.Fatalf("top-level dir not stripped: %v", keysOf(files))
	}
	// zip 同样剥离;两文件不共享顶层目录时不剥。
	mixed, err := ExtractZip(makeZip(t, map[string][]byte{"a/theme.json": []byte("{}"), "theme.css": []byte("i{}")}))
	if err != nil {
		t.Fatalf("ExtractZip() error = %v", err)
	}
	if _, ok := mixed["a/theme.json"]; !ok {
		t.Fatal("mixed layout must not be stripped")
	}
}

func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestPackageFromFilesAssemblesWhitelistedEntries(t *testing.T) {
	manifest := []byte(minimalManifest) // compile_test.go 既有的最小合法 manifest 字面量
	files := map[string][]byte{
		"theme.json":   manifest,
		"theme.css":    []byte("[data-nx=\"site-card\"] { color: var(--primary-500); }"),
		"README.md":    []byte("ignored"),
		".github/x.yml": []byte("ignored"),
	}
	pkg, err := PackageFromFiles(files)
	if err != nil {
		t.Fatalf("PackageFromFiles() error = %v", err)
	}
	if pkg.Manifest.ID == "" || len(pkg.CSS) == 0 || len(pkg.Assets) != 0 {
		t.Fatalf("unexpected package: %+v", pkg)
	}
	// 缺 theme.json 必须失败。
	if _, err := PackageFromFiles(map[string][]byte{"theme.css": []byte("i{}")}); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("missing manifest err = %v", err)
	}
	// assets/ 下的文件经 ValidateAsset 收录,路径剥掉 assets/ 前缀。
	withAsset := map[string][]byte{
		"theme.json":              manifest,
		"assets/fonts/sample.woff2": fixtureWOFF2, // compile_test.go 既有 fixture
	}
	pkg2, err := PackageFromFiles(withAsset)
	if err != nil {
		t.Fatalf("PackageFromFiles(asset) error = %v", err)
	}
	if len(pkg2.Assets) != 1 || pkg2.Assets[0].Path != "fonts/sample.woff2" {
		t.Fatalf("asset path = %+v", pkg2.Assets)
	}
	// preview.png 作为资产收录(最小合法 PNG 用 assets_test 既有 fixture;若命名不同以该文件为准)。
	withPreview := map[string][]byte{"theme.json": manifest, "preview.png": fixturePNG}
	pkg3, err := PackageFromFiles(withPreview)
	if err != nil {
		t.Fatalf("PackageFromFiles(preview) error = %v", err)
	}
	if len(pkg3.Assets) != 1 || pkg3.Assets[0].Path != "preview.png" {
		t.Fatalf("preview asset = %+v", pkg3.Assets)
	}
}
```

注意:`minimalManifest`、`fixtureWOFF2`、`fixturePNG` 是 `internal/themes` 既有测试 fixture(compile_test.go / assets 测试);动手前 grep 确认真实名字,以真实名字为准改测试引用,**不要**重复定义。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/themes -run 'TestExtract|TestPackageFromFiles'`
Expected: FAIL(`ExtractZip` 未定义,编译错误)

- [ ] **Step 3: 实现 `archive.go`**

```go
package themes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// 解包硬限。压缩输入与解压总量共用同一上限:它是包体上限(4 MiB)的 4 倍,
// 给 README、LICENSE 这类会被忽略但仍占解压预算的文件留余量。
const (
	MaxArchiveBytes = 16 << 20
	MaxArchiveFiles = 200
)

// ErrInvalidArchive 表示压缩包本身不可接受(路径逃逸、超限、格式损坏)。
// 与 ErrInvalidManifest/ErrInvalidCSS 并列,是导入错误分类的 archive 阶段。
var ErrInvalidArchive = errors.New("invalid theme archive")

func invalidArchive(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArchive, fmt.Sprintf(format, args...))
}

// ExtractZip 把 zip 解为路径→内容,应用全部防护。
func ExtractZip(data []byte) (map[string][]byte, error) {
	if len(data) > MaxArchiveBytes {
		return nil, invalidArchive("压缩包超过 %d 字节上限", MaxArchiveBytes)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, invalidArchive("无法解析 zip: %v", err)
	}
	files := map[string][]byte{}
	var total int
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		name, err := cleanArchivePath(entry.Name)
		if err != nil {
			return nil, err
		}
		if len(files) >= MaxArchiveFiles {
			return nil, invalidArchive("文件数超过 %d 上限", MaxArchiveFiles)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, invalidArchive("无法读取 %s: %v", name, err)
		}
		content, err := readCapped(rc, &total)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return stripTopLevelDir(files), nil
}

// ExtractTarGz 解 gzip tarball(GitHub codeload 产物),拒绝一切链接与设备条目。
func ExtractTarGz(data []byte) (map[string][]byte, error) {
	if len(data) > MaxArchiveBytes {
		return nil, invalidArchive("压缩包超过 %d 字节上限", MaxArchiveBytes)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, invalidArchive("无法解析 gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	var total int
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, invalidArchive("无法解析 tar: %v", err)
		}
		switch header.Typeflag {
		case tar.TypeDir, tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeReg:
		default:
			return nil, invalidArchive("拒绝非普通文件条目 %s(链接/设备)", header.Name)
		}
		name, err := cleanArchivePath(header.Name)
		if err != nil {
			return nil, err
		}
		if len(files) >= MaxArchiveFiles {
			return nil, invalidArchive("文件数超过 %d 上限", MaxArchiveFiles)
		}
		content, err := readCapped(tr, &total)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return stripTopLevelDir(files), nil
}

// readCapped 累计解压总量,越过 MaxArchiveBytes 立即失败——不是读完再量。
func readCapped(r io.Reader, total *int) ([]byte, error) {
	remaining := MaxArchiveBytes - *total
	content, err := io.ReadAll(io.LimitReader(r, int64(remaining)+1))
	if err != nil {
		return nil, invalidArchive("读取失败: %v", err)
	}
	if len(content) > remaining {
		return nil, invalidArchive("解压总量超过 %d 字节上限", MaxArchiveBytes)
	}
	*total += len(content)
	return content, nil
}

// cleanArchivePath 拒绝绝对路径、反斜杠与 .. 逃逸(zip-slip)。
func cleanArchivePath(name string) (string, error) {
	if strings.Contains(name, `\`) || strings.HasPrefix(name, "/") {
		return "", invalidArchive("非法路径 %q", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != name {
		return "", invalidArchive("非法路径 %q", name)
	}
	return cleaned, nil
}

// stripTopLevelDir:全部条目共享单一顶层目录时剥掉一层(GitHub tarball 的
// {repo}-{sha}/ 前缀与作者打 zip 时的习惯目录)。
func stripTopLevelDir(files map[string][]byte) map[string][]byte {
	var prefix string
	for name := range files {
		idx := strings.IndexByte(name, '/')
		if idx <= 0 {
			return files
		}
		dir := name[:idx+1]
		if prefix == "" {
			prefix = dir
		} else if dir != prefix {
			return files
		}
	}
	if prefix == "" {
		return files
	}
	stripped := make(map[string][]byte, len(files))
	for name, data := range files {
		stripped[strings.TrimPrefix(name, prefix)] = data
	}
	return stripped
}

// PackageFromFiles 从解包结果组装 Package:白名单提取 theme.json / theme.css /
// assets/** / preview.png,其余文件(README、LICENSE、.github/ 等)忽略。
func PackageFromFiles(files map[string][]byte) (Package, error) {
	manifestData, ok := files["theme.json"]
	if !ok {
		return Package{}, invalidArchive("缺少 theme.json")
	}
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return Package{}, err
	}
	pkg := Package{Manifest: manifest}
	if css, ok := files["theme.css"]; ok {
		pkg.CSS = css
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names) // 资产顺序确定性,与内容哈希稳定性一致
	for _, name := range names {
		var assetPath string
		switch {
		case strings.HasPrefix(name, "assets/"):
			assetPath = strings.TrimPrefix(name, "assets/")
		case name == "preview.png":
			assetPath = "preview.png"
		default:
			continue
		}
		asset, err := ValidateAsset(assetPath, files[name])
		if err != nil {
			return Package{}, err
		}
		pkg.Assets = append(pkg.Assets, asset)
	}
	return pkg, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/themes -run 'TestExtract|TestPackageFromFiles'`
Expected: PASS。随后跑全包:`go test ./internal/themes`,Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/themes/archive.go internal/themes/archive_test.go
git commit -m "feat: add hardened archive extraction for theme packages"
```

---

### Task 2: 版本 upsert 的事务化重构与元数据回写

**Files:**
- Modify: `internal/themes/store.go:55-135`(`UpsertVersion`)
- Test: `internal/themes/sync_test.go`(追加)

**Interfaces:**
- Consumes: `Compiled{VersionID, ContentHash, Manifest, CSS, Assets}`、`AssetBasePath(versionID string) string`(compile.go:30)、`database.WithinTx`。
- Produces:
  - `func upsertVersionTx(ctx context.Context, tx *sql.Tx, packageID string, compiled Compiled, sourceType, sourceRef string, now time.Time) (string, error)` — 未导出,Task 3 的 `InstallPrivate` 在同一事务里复用。
  - `UpsertVersion` 公开签名不变,但回写列扩为:`current_version_id, source_type, name, description, mode, version, preview, spec_version, updated_at`。`preview` 仅当包内含 `preview.png` 资产时为 `AssetBasePath(versionID) + "preview.png"`,否则空串。

- [ ] **Step 1: 写失败测试**

`internal/themes/sync_test.go` 追加:

```go
// UpsertVersion 必须把 manifest 的展示元数据回写 themes 行——否则列表 API
// 永远吐迁移种子的旧文案(slate 行 mode='both' vs manifest 'light' 一类漂移)。
func TestSyncBuiltinWritesBackManifestMetadata(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := SyncBuiltin(t.Context(), store, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	packages, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	for _, pkg := range packages {
		var name, description, mode, version string
		var specVersion int
		if err := db.QueryRow(`SELECT name, description, mode, version, spec_version FROM themes WHERE id = ?`,
			pkg.Manifest.ID).Scan(&name, &description, &mode, &version, &specVersion); err != nil {
			t.Fatalf("read %s: %v", pkg.Manifest.ID, err)
		}
		if name != pkg.Manifest.Name || mode != pkg.Manifest.Mode || version != pkg.Manifest.Version || specVersion != pkg.Manifest.SpecVersion {
			t.Fatalf("%s 行元数据与 manifest 漂移: name=%q mode=%q version=%q spec=%d", pkg.Manifest.ID, name, mode, version, specVersion)
		}
	}
	// 内置包都没有 preview.png,preview 列保持空串。
	var preview string
	if err := db.QueryRow(`SELECT preview FROM themes WHERE id = 'slate'`).Scan(&preview); err != nil {
		t.Fatal(err)
	}
	if preview != "" {
		t.Fatalf("slate preview = %q, want empty", preview)
	}
}
```

说明:`Manifest` 若无 `Description` 字段(以 manifest.go 实际字段为准),description 断言按实际字段裁剪——**动手前先读 `Manifest` 结构体**,断言只覆盖 manifest 真实携带的字段。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/themes -run TestSyncBuiltinWritesBackManifestMetadata`
Expected: FAIL(slate 的 mode 是种子值 `both`,manifest 是 `light`)

- [ ] **Step 3: 重构实现**

`store.go`:把 `UpsertVersion` 的 `WithinTx` 内部整体提取为 `upsertVersionTx(ctx, tx, packageID, compiled, sourceType, sourceRef, now)`,`UpsertVersion` 变成薄封装:

```go
func (s *Store) UpsertVersion(ctx context.Context, packageID string, compiled Compiled, sourceType, sourceRef string, now time.Time) (string, error) {
	// (原有的入参校验保持在这里,事务外)
	var versionID string
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		id, err := upsertVersionTx(ctx, tx, packageID, compiled, sourceType, sourceRef, now)
		if err != nil {
			return err
		}
		versionID = id
		return nil
	})
	return versionID, err
}
```

`upsertVersionTx` 末尾的回写 UPDATE 扩列(替换原 store.go:119-123 那条):

```go
	// 回写 manifest 展示元数据:themes 行的这些列否则永远停留在迁移种子值,
	// 列表 API 会吐与实际包不符的文案(内置与第三方同样受益)。
	preview := ""
	for _, asset := range compiled.Assets {
		if asset.Path == "preview.png" {
			preview = AssetBasePath(compiled.VersionID) + "preview.png"
			break
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE themes SET current_version_id = ?, source_type = ?, name = ?, description = ?,
		       mode = ?, version = ?, preview = ?, spec_version = ?, updated_at = ?
		WHERE id = ?`,
		compiled.VersionID, sourceType, compiled.Manifest.Name, compiled.Manifest.Description,
		compiled.Manifest.Mode, compiled.Manifest.Version, preview, compiled.Manifest.SpecVersion,
		dbTime(now), packageID); err != nil {
		return "", fmt.Errorf("update theme pointer: %w", err)
	}
```

(`Manifest` 无 `Description` 字段时删掉对应列;以实际结构体为准,保持编译通过。)

- [ ] **Step 4: 全包回归**

Run: `go test ./internal/themes && go test ./internal/admin ./internal/catalog ./internal/navigation`
Expected: PASS。注意:admin 的 `TestThemeListingIncludesSpecV1Fields` 与 catalog 测试若断言了旧种子文案会因回写而变——按新行为修正断言(回写后 `mode` 等以 manifest 为准),这是**预期的行为修正**,在提交信息里说明。

- [ ] **Step 5: Commit**

```bash
git add internal/themes/ internal/admin/ internal/catalog/
git commit -m "feat: write manifest metadata back to theme rows on version upsert"
```

---

### Task 3: `InstallPrivate` —— 首个 themes 行创建路径(配额 + 升级)

**Files:**
- Create: `internal/themes/install.go`
- Test: `internal/themes/install_test.go`

**Interfaces:**
- Consumes: `upsertVersionTx`(Task 2)、`identity.New("thm")`、`database.WithinTx`、触发器不变量(0014:catalog/private 与 owner 成对、current 指针必须指向本主题 active 版本)。
- Produces:
  - `var ErrQuotaExceeded = errors.New("private theme quota exceeded")`
  - `type InstalledTheme struct { ThemeID, Slug, VersionID string; Upgraded bool }`
  - `func (s *Store) InstallPrivate(ctx context.Context, ownerID, slug, sourceType, sourceURL, sourceRef string, quota int, compile func(themeID string) (Compiled, error), now time.Time) (InstalledTheme, error)`
  - compile 回调在事务内、themeID 确定后执行——CSS 作用域用 themes.id(ULID),与 v1 §7.2「作用域用包 ID,天然无碰撞」一致,升级与新装共用同一条路径。

- [ ] **Step 1: 写失败测试**

`internal/themes/install_test.go`:

```go
package themes

import (
	"errors"
	"testing"
	"time"
)

func seedUser(t *testing.T, store *Store, id string) {
	t.Helper()
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, 'x', 'user', 'active', ?, ?) ON CONFLICT(id) DO NOTHING`,
		id, id, id+"@example.com", stamp, stamp); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func installSample(t *testing.T, store *Store, ownerID, slug string, quota int) InstalledTheme {
	t.Helper()
	installed, err := store.InstallPrivate(t.Context(), ownerID, slug, "upload", "", "digest-"+slug, quota,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if err != nil {
		t.Fatalf("InstallPrivate(%s) error = %v", slug, err)
	}
	return installed
}

func TestInstallPrivateCreatesRowAndVersion(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0001")

	installed := installSample(t, store, "usr_inst_0001", "lilac", 10)
	if installed.Upgraded || installed.ThemeID == "" || installed.VersionID == "" || installed.Slug != "lilac" {
		t.Fatalf("unexpected result: %+v", installed)
	}
	var scope, owner, currentVersion, sourceType string
	if err := db.QueryRow(`SELECT scope, owner_id, current_version_id, source_type FROM themes WHERE id = ?`,
		installed.ThemeID).Scan(&scope, &owner, &currentVersion, &sourceType); err != nil {
		t.Fatal(err)
	}
	if scope != "private" || owner != "usr_inst_0001" || currentVersion != installed.VersionID || sourceType != "upload" {
		t.Fatalf("row = %s/%s/%s/%s", scope, owner, currentVersion, sourceType)
	}
	// 装完即可解析(owner 视角),匿名不可见。
	if got, err := store.ResolveEligibleVersion(t.Context(), installed.ThemeID, "usr_inst_0001"); err != nil || got != installed.VersionID {
		t.Fatalf("owner resolve = %q, %v", got, err)
	}
	var slateVersion string
	// 无内置主题时默认回落不可用——本测试库未跑 SyncBuiltin,匿名解析应报默认不可用,
	// 这里只断言拿不到私有版本即可。
	if got, _ := store.ResolveEligibleVersion(t.Context(), installed.ThemeID, ""); got == installed.VersionID {
		t.Fatalf("anonymous must not resolve a private theme, got %q (slate=%q)", got, slateVersion)
	}
}

func TestInstallPrivateSameSlugUpgrades(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0002")

	first := installSample(t, store, "usr_inst_0002", "lilac", 10)
	// 变更 CSS 产生新 content hash → 升级为新版本,行数不变。
	upgraded, err := store.InstallPrivate(t.Context(), "usr_inst_0002", "lilac", "upload", "", "digest-2", 10,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.9; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC())
	if err != nil {
		t.Fatalf("upgrade error = %v", err)
	}
	if !upgraded.Upgraded || upgraded.ThemeID != first.ThemeID || upgraded.VersionID == first.VersionID {
		t.Fatalf("upgrade result: first=%+v upgraded=%+v", first, upgraded)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE owner_id = 'usr_inst_0002'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("themes rows = %d, want 1", rows)
	}
}

func TestInstallPrivateEnforcesQuota(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_0003")

	installSample(t, store, "usr_inst_0003", "one", 2)
	installSample(t, store, "usr_inst_0003", "two", 2)
	_, err := store.InstallPrivate(t.Context(), "usr_inst_0003", "three", "upload", "", "d3", 2,
		func(themeID string) (Compiled, error) { return Compile(samplePackage(t), themeID) }, time.Now().UTC())
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	// 升级不占额度:配额已满仍可升级既有 slug。
	if _, err := store.InstallPrivate(t.Context(), "usr_inst_0003", "one", "upload", "", "d4", 2,
		func(themeID string) (Compiled, error) {
			pkg := samplePackage(t)
			pkg.CSS = append(pkg.CSS, []byte("\n[data-nx=\"clock\"] { opacity: 0.8; }")...)
			return Compile(pkg, themeID)
		}, time.Now().UTC()); err != nil {
		t.Fatalf("upgrade at quota error = %v", err)
	}
}

func TestInstallPrivateIsolatesSlugAcrossOwners(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_inst_a")
	seedUser(t, store, "usr_inst_b")
	a := installSample(t, store, "usr_inst_a", "lilac", 10)
	b := installSample(t, store, "usr_inst_b", "lilac", 10)
	if a.ThemeID == b.ThemeID {
		t.Fatal("different owners with same slug must get distinct theme rows")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/themes -run TestInstallPrivate`
Expected: FAIL(编译错误,`InstallPrivate` 未定义)

- [ ] **Step 3: 实现 `install.go`**

```go
package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
)

// ErrQuotaExceeded 表示该用户的私有主题数已达实例配额。
var ErrQuotaExceeded = errors.New("private theme quota exceeded")

// InstalledTheme 是一次导入(新装或升级)的结果。
type InstalledTheme struct {
	ThemeID   string
	Slug      string
	VersionID string
	Upgraded  bool
}

// InstallPrivate 安装或升级一个私有主题。themeID 在事务内确定(既有行复用、
// 新行用不透明 ULID),compile 回调据此产出以该 ID 为 CSS 作用域的编译产物——
// 作用域用行 ID 而不是 slug,两个用户同名主题天然无碰撞(设计 §7.2)。
// 同 owner 同 slug 重复导入即升级:不占新配额,切换 current 指针,已发布快照
// 因锁版本不受影响。
func (s *Store) InstallPrivate(ctx context.Context, ownerID, slug, sourceType, sourceURL, sourceRef string, quota int, compile func(themeID string) (Compiled, error), now time.Time) (InstalledTheme, error) {
	ownerID = strings.TrimSpace(ownerID)
	slug = strings.TrimSpace(slug)
	if ownerID == "" || slug == "" {
		return InstalledTheme{}, errors.New("themes: owner and slug are required")
	}
	var result InstalledTheme
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var themeID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM themes WHERE scope = 'private' AND owner_id = ? AND slug = ?`,
			ownerID, slug).Scan(&themeID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// 新装:配额按行数计(含墓碑——被历史发布引用的已卸载主题仍占位)。
			var owned int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE owner_id = ?`, ownerID).Scan(&owned); err != nil {
				return err
			}
			if owned >= quota {
				return ErrQuotaExceeded
			}
			themeID, err = identity.New("thm")
			if err != nil {
				return err
			}
			compiled, err := compile(themeID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
				                    created_at, updated_at, slug, scope, owner_id, source_type, source_url, spec_version)
				VALUES (?, ?, ?, ?, ?, ?, '', 1, 0, ?, ?, ?, 'private', ?, ?, ?, ?)`,
				themeID, compiled.Manifest.Name, compiled.Manifest.Version, compiled.Manifest.Author,
				compiled.Manifest.Description, compiled.Manifest.Mode, dbTime(now), dbTime(now),
				slug, ownerID, sourceType, sourceURL, compiled.Manifest.SpecVersion); err != nil {
				return fmt.Errorf("insert private theme: %w", err)
			}
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, now)
			if err != nil {
				return err
			}
			result = InstalledTheme{ThemeID: themeID, Slug: slug, VersionID: versionID}
			return nil
		case err != nil:
			return err
		default:
			// 升级:同一行换版本。source_url 跟随本次来源(github 重新拉取会刷新)。
			compiled, err := compile(themeID)
			if err != nil {
				return err
			}
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, now)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE themes SET source_url = ?, updated_at = ? WHERE id = ?`,
				sourceURL, dbTime(now), themeID); err != nil {
				return err
			}
			result = InstalledTheme{ThemeID: themeID, Slug: slug, VersionID: versionID, Upgraded: true}
			return nil
		}
	})
	if err != nil {
		return InstalledTheme{}, err
	}
	return result, nil
}
```

注意:`Manifest` 若无 `Author`/`Description` 字段,以实际结构体为准调整 INSERT 的取值(缺失字段用空串字面量),保持编译通过。

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/themes`
Expected: PASS(含既有触发器测试——INSERT 走 private+owner 成对,触发器放行)

- [ ] **Step 5: Commit**

```bash
git add internal/themes/install.go internal/themes/install_test.go
git commit -m "feat: install and upgrade private themes with quota enforcement"
```

---

### Task 4: 卸载、用户删除不变量、私有主题不可设默认

**Files:**
- Modify: `internal/themes/install.go`(追加 UninstallPrivate)
- Test: `internal/themes/install_test.go`(追加)、Create: `internal/themes/lifecycle_test.go`
- Modify: `internal/admin/sqlstore.go:237-262`(UpdateTheme)、`internal/admin/service.go`(错误变量)、`internal/httpapi/admin.go`(409 映射)

**Interfaces:**
- Consumes: 触发器 `theme_versions_current_guard`(删版本前必须先把 current 指针置 NULL——`themes_current_version_valid` 触发器 `WHEN NEW.current_version_id IS NOT NULL`,置空被放行)、`published_snapshots.theme_version_id`(ON DELETE RESTRICT)。
- Produces:
  - `func (s *Store) UninstallPrivate(ctx context.Context, ownerID, themeID string, now time.Time) (removed bool, err error)` — `removed=true` 物理删除(配额释放),`false` 墓碑(`enabled=0`,仍被快照引用)。不存在或非本人 → `ErrNotFound`(不区分,防枚举)。
  - admin 包:`var ErrPrivateDefault = errors.New("private themes cannot be the instance default")`;`UpdateTheme` 对 `scope='private'` 且 `patch.Default=true` 返回它;httpapi 映射 409 `PRIVATE_THEME_DEFAULT`。

- [ ] **Step 1: 写失败测试(卸载两分支)**

`internal/themes/install_test.go` 追加:

```go
func TestUninstallPrivateDeletesUnreferencedTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_0001")
	installed := installSample(t, store, "usr_uni_0001", "lilac", 10)

	removed, err := store.UninstallPrivate(t.Context(), "usr_uni_0001", installed.ThemeID, time.Now().UTC())
	if err != nil || !removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want removed", removed, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM themes WHERE id = ?`, installed.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("theme row must be physically deleted")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE theme_id = ?`, installed.ThemeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("versions must be deleted with the theme")
	}
}

func TestUninstallPrivateTombstonesReferencedTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_0002")
	installed := installSample(t, store, "usr_uni_0002", "lilac", 10)

	// 造一条引用该版本的快照(最小行;published_snapshots 的其余 NOT NULL 列按 schema 补)。
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO pages (id, owner_id, kind, created_at, updated_at)
		VALUES ('pg_uni_0002', 'usr_uni_0002', 'personal', ?, ?)`, stamp, stamp); err != nil {
		t.Skipf("pages schema differs; adjust seed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO published_snapshots (id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at, theme_version_id)
		VALUES ('snp_uni_0002', 'pg_uni_0002', 1, 'uni', 'public', '{}', 'W/"x"', ?, ?)`,
		stamp, installed.VersionID); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	removed, err := store.UninstallPrivate(t.Context(), "usr_uni_0002", installed.ThemeID, time.Now().UTC())
	if err != nil || removed {
		t.Fatalf("UninstallPrivate() = %v, %v; want tombstone", removed, err)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT enabled FROM themes WHERE id = ?`, installed.ThemeID).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("tombstoned theme must be disabled")
	}
	// 公开供应不受影响:版本行仍在且 active。
	if _, _, status, err := store.VersionCSS(t.Context(), installed.VersionID); err != nil || status != VersionStatusActive {
		t.Fatalf("css after tombstone: status=%q err=%v", status, err)
	}
}

func TestUninstallPrivateRejectsForeignTheme(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_uni_a")
	seedUser(t, store, "usr_uni_b")
	installed := installSample(t, store, "usr_uni_a", "lilac", 10)
	if _, err := store.UninstallPrivate(t.Context(), "usr_uni_b", installed.ThemeID, time.Now().UTC()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign uninstall err = %v, want ErrNotFound", err)
	}
}
```

注意:`pages` 表的真实列以 `migrations/0001_initial.sql` 为准——写测试前先看该表定义,种子行补齐全部 NOT NULL 列;`t.Skipf` 只是脚手架期防御,落地时必须替换为正确种子(不许留 Skip)。

- [ ] **Step 2: 确认失败后实现 `UninstallPrivate`**

Run: `go test ./internal/themes -run TestUninstallPrivate` → FAIL(未定义)。然后在 `install.go` 追加:

```go
// UninstallPrivate 卸载私有主题。无任何已发布快照引用其版本时物理删除
// (配额立即释放);仍被引用时转墓碑(enabled=0):版本与资产保留,公开页
// 继续可用——撤销语义见设计 §8.1.1。不存在与非本人统一返回 ErrNotFound。
func (s *Store) UninstallPrivate(ctx context.Context, ownerID, themeID string, now time.Time) (bool, error) {
	var removed bool
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var one int
		err := tx.QueryRowContext(ctx, `
			SELECT 1 FROM themes WHERE id = ? AND scope = 'private' AND owner_id = ?`,
			themeID, ownerID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var refs int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM published_snapshots
			WHERE theme_version_id IN (SELECT id FROM theme_versions WHERE theme_id = ?)`,
			themeID).Scan(&refs); err != nil {
			return err
		}
		if refs > 0 {
			_, err := tx.ExecContext(ctx, `UPDATE themes SET enabled = 0, updated_at = ? WHERE id = ?`, dbTime(now), themeID)
			return err
		}
		// 物理删除:先摘 current 指针(theme_versions_current_guard 才放行删版本)。
		if _, err := tx.ExecContext(ctx, `UPDATE themes SET current_version_id = NULL, updated_at = ? WHERE id = ?`, dbTime(now), themeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM theme_versions WHERE theme_id = ?`, themeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, themeID); err != nil {
			return err
		}
		removed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return removed, nil
}
```

Run: `go test ./internal/themes -run TestUninstallPrivate` → PASS。

- [ ] **Step 3: 用户删除不变量测试**

Create `internal/themes/lifecycle_test.go`:

```go
package themes

import (
	"testing"
	"time"
)

// 系统目前没有任何删除用户的代码路径(admin 只能改状态)。这里钉住数据库级
// 不变量:拥有版本行的用户被 DELETE 时,themes 行的 ON DELETE CASCADE 会传导
// 到 theme_versions 的 ON DELETE RESTRICT 而被整体拒绝——未来实现账号删除的
// 人会先撞上这个测试,而不是生产事故。清理顺序见 UninstallPrivate。
func TestDeletingUserWithPrivateThemeVersionsIsBlocked(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	seedUser(t, store, "usr_life_0001")
	installSample(t, store, "usr_life_0001", "lilac", 10)

	if _, err := db.Exec(`DELETE FROM users WHERE id = 'usr_life_0001'`); err == nil {
		t.Fatal("deleting a user whose private theme has versions must be rejected by RESTRICT")
	}
	// 正确路径:先卸载(物理删),用户行才可删。
	var themeID string
	if err := db.QueryRow(`SELECT id FROM themes WHERE owner_id = 'usr_life_0001'`).Scan(&themeID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UninstallPrivate(t.Context(), "usr_life_0001", themeID, time.Now().UTC()); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = 'usr_life_0001'`); err != nil {
		t.Fatalf("user delete after cleanup: %v", err)
	}
}
```

Run: `go test ./internal/themes -run TestDeletingUser` → PASS(不变量本就成立,此测试是回归钉)。

- [ ] **Step 4: 管理端私有主题不可设默认**

`internal/admin/service.go` 错误变量区追加:

```go
// ErrPrivateDefault:默认主题必须是目录主题(设计 §7.1 不变量)。
var ErrPrivateDefault = errors.New("private themes cannot be the instance default")
```

`internal/admin/sqlstore.go` 的 `UpdateTheme` 事务内,读 `is_default` 的同一查询扩为同时读 `scope`:

```go
		var currentDefault bool
		var scope string
		if err := tx.QueryRowContext(ctx, "SELECT is_default, scope FROM themes WHERE id = ?", themeID).Scan(&currentDefault, &scope); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if patch.Default != nil && *patch.Default && scope == "private" {
			return ErrPrivateDefault
		}
```

`internal/httpapi/admin.go` 的 updateTheme 错误映射(找到现有 `ErrDefaultTheme` → 409 的分支,同样式追加):

```go
	case errors.Is(err, adminpkg.ErrPrivateDefault):
		WriteError(w, r, http.StatusConflict, "PRIVATE_THEME_DEFAULT", "私有主题不能设为实例默认主题", nil)
```

测试:`internal/admin/sqlstore_test.go` 追加(种子一个私有主题行:INSERT users + themes,scope=private+owner,参照 `internal/themes/eligibility_test.go:12-32` 的 seedPrivateTheme 写法,但 admin 测试自备最小 SQL):

```go
func TestUpdateThemeRejectsPrivateDefault(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_pd_01', 'pd', 'pd@example.com', 'x', 'user', 'active', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at, slug, scope, owner_id, source_type)
		VALUES ('thm_pd_01', 'PD', '1.0.0', 'pd', '', 'light', '', 1, 0, ?, ?, 'pd', 'private', 'usr_pd_01', 'upload')`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	store := NewSQLStore(db)
	enable := true
	_, err = store.UpdateTheme(ctx, "thm_pd_01", ThemePatch{Default: &enable}, time.Now().UTC(), AuditRecord{ID: "aud_pd_01", ActorName: "t", Action: "theme.update", TargetType: "theme", TargetID: "thm_pd_01", CreatedAt: time.Now().UTC()})
	if !errors.Is(err, ErrPrivateDefault) {
		t.Fatalf("err = %v, want ErrPrivateDefault", err)
	}
}
```

(`AuditRecord` 的必填字段以 admin 包实际定义为准;若 UpdateTheme 在错误路径不落审计,构造最小值即可。)

- [ ] **Step 5: 回归与提交**

Run: `go test ./internal/themes ./internal/admin && go vet ./...`
Expected: PASS

```bash
git add internal/themes/ internal/admin/ internal/httpapi/admin.go
git commit -m "feat: uninstall private themes and guard lifecycle invariants"
```

---

### Task 5: `internal/themeimport` —— GitHub/Gitea 拉取器

**Files:**
- Create: `internal/themeimport/github.go`
- Test: `internal/themeimport/github_test.go`

**Interfaces:**
- Consumes: `netguard.NewValidator/GuardedClient/Transport/Resolver`(netguard.go、client.go)、linkcheck 的测试范式(fakeResolver 映射到公网 IP + 注入 RoundTripper,见 `internal/linkcheck/service_test.go:37-54,278-284`)。
- Produces:
  - `type Fetched struct { Data []byte; SHA string; CanonicalURL string }`(SHA 对 Gitea 兼容主机为原样 ref)
  - `type GitHubClient struct { … }`
  - `func NewGitHubClient(resolver netguard.Resolver, transport http.RoundTripper, extraHosts []string, token string) *GitHubClient` — resolver/transport 为 nil 时用生产默认(`netguard.NewValidator(nil)` + `netguard.GuardedClient(validator, 30*time.Second, 3)`);测试注入两者。
  - `func (c *GitHubClient) FetchTarball(ctx context.Context, rawURL, ref string) (Fetched, error)`
  - `var ErrHostNotAllowed`、`var ErrUpstream`(区分 422 与 502 映射)。

- [ ] **Step 1: 写失败测试**

`internal/themeimport/github_test.go`:

```go
package themeimport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type fakeResolver struct{ addresses map[string][]netip.Addr }

func (f *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if addrs, ok := f.addresses[host]; ok {
		return addrs, nil
	}
	return nil, errors.New("host not found")
}

func publicResolver(hosts ...string) *fakeResolver {
	addresses := make(map[string][]netip.Addr, len(hosts))
	for _, host := range hosts {
		addresses[host] = []netip.Addr{netip.MustParseAddr("8.8.8.8")}
	}
	return &fakeResolver{addresses: addresses}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestFetchTarballResolvesRefAndDownloads(t *testing.T) {
	var apiHit, tarHit bool
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.github.com":
			apiHit = true
			if r.URL.Path != "/repos/alice/lilac/commits/main" {
				t.Fatalf("api path = %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
				t.Fatalf("auth header = %q", got)
			}
			return respond(200, `{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), nil
		case "codeload.github.com":
			tarHit = true
			if r.URL.Path != "/alice/lilac/tar.gz/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
				t.Fatalf("codeload path = %s", r.URL.Path)
			}
			return respond(200, "tarball-bytes"), nil
		}
		t.Fatalf("unexpected host %s", r.URL.Host)
		return nil, nil
	})
	client := NewGitHubClient(publicResolver("api.github.com", "codeload.github.com"), transport, nil, "tok123")
	fetched, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", "main")
	if err != nil {
		t.Fatalf("FetchTarball() error = %v", err)
	}
	if !apiHit || !tarHit || fetched.SHA != strings.Repeat("a", 40) || string(fetched.Data) != "tarball-bytes" {
		t.Fatalf("fetched = %+v (api=%v tar=%v)", fetched, apiHit, tarHit)
	}
	if fetched.CanonicalURL != "https://github.com/alice/lilac" {
		t.Fatalf("canonical = %q", fetched.CanonicalURL)
	}
}

func TestFetchTarballSkipsAPIForExplicitSHA(t *testing.T) {
	sha := strings.Repeat("b", 40)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "codeload.github.com" {
			t.Fatalf("unexpected host %s", r.URL.Host)
		}
		return respond(200, "x"), nil
	})
	client := NewGitHubClient(publicResolver("codeload.github.com"), transport, nil, "")
	fetched, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", sha)
	if err != nil || fetched.SHA != sha {
		t.Fatalf("fetched = %+v, err = %v", fetched, err)
	}
}

func TestFetchTarballRejectsDisallowedHostAndScheme(t *testing.T) {
	client := NewGitHubClient(publicResolver(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("must not reach network")
		return nil, nil
	}), nil, "")
	for _, target := range []string{
		"https://evil.example.com/alice/lilac",
		"http://github.com/alice/lilac",          // 非 https
		"https://github.com/alice",               // 缺 repo 段
		"https://user:pass@github.com/alice/lilac", // 内嵌凭据
	} {
		if _, err := client.FetchTarball(context.Background(), target, "main"); !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("%s err = %v, want ErrHostNotAllowed", target, err)
		}
	}
}

func TestFetchTarballGiteaCompatibleHost(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "git.example.com" || r.URL.Path != "/alice/lilac/archive/v1.2.0.tar.gz" {
			t.Fatalf("url = %s", r.URL.String())
		}
		return respond(200, "x"), nil
	})
	client := NewGitHubClient(publicResolver("git.example.com"), transport, []string{"git.example.com"}, "")
	fetched, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", "v1.2.0")
	if err != nil || fetched.SHA != "v1.2.0" {
		t.Fatalf("fetched = %+v, err = %v", fetched, err)
	}
	// Gitea 兼容主机必须显式 ref。
	if _, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", ""); err == nil {
		t.Fatal("empty ref on gitea-compatible host must fail")
	}
}

func TestFetchTarballMapsUpstreamFailures(t *testing.T) {
	client := NewGitHubClient(publicResolver("api.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return respond(404, `{"message":"Not Found"}`), nil
	}), nil, "")
	if _, err := client.FetchTarball(context.Background(), "https://github.com/alice/gone", "main"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}
```

- [ ] **Step 2: 确认失败后实现 `github.go`**

Run: `go test ./internal/themeimport` → FAIL(包不存在)。实现:

```go
// Package themeimport 负责第三方主题的获取与导入编排。信任边界仍在
// internal/themes 的校验/编译管线;本包只做「拿到字节」与「串起流程」。
package themeimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
	"github.com/yixian-huang/navax/internal/themes"
)

var (
	// ErrHostNotAllowed:URL 不在导入白名单或形状非法——用户输入问题(422)。
	ErrHostNotAllowed = errors.New("theme import host not allowed")
	// ErrUpstream:上游仓库不可达/不存在——上游问题(502)。
	ErrUpstream = errors.New("theme import upstream failed")
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Fetched 是一次拉取的产物。SHA 对 GitHub 是解析后的 commit sha;
// 对 Gitea 兼容主机是调用方显式给出的 ref(这些主机不做 API 解析)。
type Fetched struct {
	Data         []byte
	SHA          string
	CanonicalURL string
}

type GitHubClient struct {
	client     *http.Client
	extraHosts map[string]bool
	token      string
}

// NewGitHubClient 构造拉取器。resolver/transport 供测试注入;生产传 nil,
// 使用严格 netguard 校验器(公网单播 only)与 30s 超时、3 跳重定向的守护 client。
func NewGitHubClient(resolver netguard.Resolver, transport http.RoundTripper, extraHosts []string, token string) *GitHubClient {
	validator := netguard.NewValidator(resolver)
	var client *http.Client
	if transport != nil {
		client = &http.Client{Timeout: 30 * time.Second, Transport: netguard.Transport{Validator: validator, Base: transport}}
	} else {
		client = netguard.GuardedClient(validator, 30*time.Second, 3)
	}
	extras := make(map[string]bool, len(extraHosts))
	for _, host := range extraHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			extras[host] = true
		}
	}
	return &GitHubClient{client: client, extraHosts: extras, token: token}
}

// FetchTarball 解析仓库 URL 并拉取 ref 对应的 tarball。
// github.com:ref(缺省默认分支 HEAD)经 api.github.com 解析为 commit sha,
// tarball 走 codeload。白名单追加主机走 Gitea 兼容布局且必须显式 ref。
func (c *GitHubClient) FetchTarball(ctx context.Context, rawURL, ref string) (Fetched, error) {
	owner, repo, host, err := parseRepoURL(rawURL, c.extraHosts)
	if err != nil {
		return Fetched{}, err
	}
	canonical := "https://" + host + "/" + owner + "/" + repo
	if host != "github.com" {
		if strings.TrimSpace(ref) == "" {
			return Fetched{}, fmt.Errorf("%w: 该主机需要显式 ref", ErrHostNotAllowed)
		}
		data, err := c.download(ctx, "https://"+host+"/"+owner+"/"+repo+"/archive/"+url.PathEscape(ref)+".tar.gz")
		if err != nil {
			return Fetched{}, err
		}
		return Fetched{Data: data, SHA: ref, CanonicalURL: canonical}, nil
	}
	sha := strings.ToLower(strings.TrimSpace(ref))
	if !shaPattern.MatchString(sha) {
		refPath := "HEAD"
		if strings.TrimSpace(ref) != "" {
			refPath = ref
		}
		resolved, err := c.resolveSHA(ctx, owner, repo, refPath)
		if err != nil {
			return Fetched{}, err
		}
		sha = resolved
	}
	data, err := c.download(ctx, "https://codeload.github.com/"+owner+"/"+repo+"/tar.gz/"+sha)
	if err != nil {
		return Fetched{}, err
	}
	return Fetched{Data: data, SHA: sha, CanonicalURL: canonical}, nil
}

func parseRepoURL(rawURL string, extras map[string]bool) (owner, repo, host string, err error) {
	parsed, parseErr := url.Parse(strings.TrimSpace(rawURL))
	if parseErr != nil || parsed.Scheme != "https" || parsed.User != nil {
		return "", "", "", fmt.Errorf("%w: 仓库地址必须是 https 且不含凭据", ErrHostNotAllowed)
	}
	host = strings.ToLower(parsed.Hostname())
	if host != "github.com" && !extras[host] {
		return "", "", "", fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", "", "", fmt.Errorf("%w: 地址需为 https://%s/{owner}/{repo}", ErrHostNotAllowed, host)
	}
	repo = strings.TrimSuffix(segments[1], ".git")
	return segments[0], repo, host, nil
}

func (c *GitHubClient) resolveSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	target := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/commits/" + url.PathEscape(ref)
	body, err := c.get(ctx, target)
	if err != nil {
		return "", err
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !shaPattern.MatchString(payload.SHA) {
		return "", fmt.Errorf("%w: 无法解析 commit sha", ErrUpstream)
	}
	return payload.SHA, nil
}

func (c *GitHubClient) download(ctx context.Context, target string) ([]byte, error) {
	return c.get(ctx, target)
}

func (c *GitHubClient) get(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		if errors.Is(err, netguard.ErrBlocked) {
			return nil, fmt.Errorf("%w: 目标地址被拒绝", ErrHostNotAllowed)
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: 上游返回 %d", ErrUpstream, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(themes.MaxArchiveBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if len(body) > themes.MaxArchiveBytes {
		return nil, fmt.Errorf("%w: 响应体超过 %d 字节上限", ErrUpstream, themes.MaxArchiveBytes)
	}
	return body, nil
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test ./internal/themeimport`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/themeimport/
git commit -m "feat: fetch theme tarballs from github with ssrf guards"
```

---

### Task 6: `internal/themeimport` —— 导入编排与 dry-run

**Files:**
- Create: `internal/themeimport/service.go`
- Test: `internal/themeimport/service_test.go`

**Interfaces:**
- Consumes: Task 1 `ExtractZip/ExtractTarGz/PackageFromFiles/ErrInvalidArchive`、Task 3 `Store.InstallPrivate/InstalledTheme/ErrQuotaExceeded`、Task 4 `Store.UninstallPrivate`、Task 5 `GitHubClient.FetchTarball`、`themes.Compile/ErrInvalidManifest/ErrInvalidCSS/ErrInvalidAsset`。
- Produces:
  - `type Service struct { … }`、`func NewService(store *themes.Store, github *GitHubClient, quota int) *Service`(内部 `now func() time.Time` 默认 `time.Now`,测试可覆写)
  - `func (s *Service) ImportZip(ctx context.Context, ownerID string, zipData []byte) (themes.InstalledTheme, error)`
  - `func (s *Service) ImportGitHub(ctx context.Context, ownerID, repoURL, ref string) (themes.InstalledTheme, error)`
  - `func (s *Service) Uninstall(ctx context.Context, ownerID, themeID string) (bool, error)`
  - `type ValidationIssue struct { Stage, Path, Message string }`;`func (s *Service) ValidatePackage(zipData []byte) []ValidationIssue`(空切片 = 合法;当前粒度为「首个错误」,stage ∈ archive/manifest/css/asset)
  - `func classifyIssue(err error) ValidationIssue`(错误 → 阶段映射,导入与 dry-run 共用)

- [ ] **Step 1: 写失败测试**

`internal/themeimport/service_test.go`(用真实 SQLite + 真实管线;zip 用 Go 现场构造):

```go
package themeimport

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/themes"
)

// 最小合法主题包。manifest 字面量以 internal/themes 的 manifest 校验规则为准:
// 颜色四组 + 字体四族 + 三个 swatch + tier 1。动手前对照 manifest_test.go 里
// 的最小合法样例(minimalManifest),保持两处一致——不一致时以那边为准。
const sampleManifest = `{
  "specVersion": 1, "id": "lilac", "name": "Lilac", "version": "1.0.0",
  "author": "e2e", "license": "MIT", "mode": "light", "vibe": "serious",
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

func buildZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range entries {
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

func sampleZip(t *testing.T) []byte {
	return buildZip(t, map[string][]byte{
		"theme.json": []byte(sampleManifest),
		"theme.css":  []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
	})
}

func newService(t *testing.T) (*Service, *themes.Store) {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stamp := "2026-07-24T00:00:00Z"
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('usr_svc_0001', 'svc', 'svc@example.com', 'x', 'user', 'active', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	store := themes.NewStore(db)
	return NewService(store, NewGitHubClient(publicResolver(), nil, nil, ""), 10), store
}

func TestImportZipInstallsPrivateTheme(t *testing.T) {
	service, store := newService(t)
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", sampleZip(t))
	if err != nil {
		t.Fatalf("ImportZip() error = %v", err)
	}
	if installed.Slug != "lilac" || installed.Upgraded {
		t.Fatalf("installed = %+v", installed)
	}
	if got, err := store.ResolveEligibleVersion(context.Background(), installed.ThemeID, "usr_svc_0001"); err != nil || got != installed.VersionID {
		t.Fatalf("resolve = %q, %v", got, err)
	}
}

func TestImportZipClassifiesFailures(t *testing.T) {
	service, _ := newService(t)
	cases := []struct {
		name  string
		zip   []byte
		stage string
	}{
		{"损坏压缩包", []byte("not a zip"), "archive"},
		{"缺 manifest", buildZip(t, map[string][]byte{"theme.css": []byte("i{}")}), "archive"},
		{"坏 manifest", buildZip(t, map[string][]byte{"theme.json": []byte(`{"specVersion":2}`)}), "manifest"},
		{"外链 CSS", buildZip(t, map[string][]byte{
			"theme.json": []byte(sampleManifest),
			"theme.css":  []byte(`[data-nx="site-card"] { background: url("https://evil.example/x.png"); }`),
		}), "css"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.ImportZip(context.Background(), "usr_svc_0001", tc.zip)
			if err == nil {
				t.Fatal("want error")
			}
			issues := service.ValidatePackage(tc.zip)
			if len(issues) == 0 || issues[0].Stage != tc.stage {
				t.Fatalf("issues = %+v, want stage %q", issues, tc.stage)
			}
		})
	}
	// 合法包 dry-run:零 issue,且不落库。
	if issues := service.ValidatePackage(sampleZip(t)); len(issues) != 0 {
		t.Fatalf("valid package issues = %+v", issues)
	}
}

func TestImportGitHubUsesFetchedTarball(t *testing.T) {
	// 假 transport 返回内存 tarball(用 Task 1 的 tar 构造逻辑等价物)。
	service, _ := newService(t)
	tarball := makeSampleTarGz(t) // 见下方辅助
	sha := "cccccccccccccccccccccccccccccccccccccccc"
	service.github = NewGitHubClient(publicResolver("codeload.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(tarball)), Header: http.Header{}}, nil
	}), nil, "")
	installed, err := service.ImportGitHub(context.Background(), "usr_svc_0001", "https://github.com/e2e/lilac", sha)
	if err != nil {
		t.Fatalf("ImportGitHub() error = %v", err)
	}
	if installed.Slug != "lilac" {
		t.Fatalf("installed = %+v", installed)
	}
}

func TestUninstallDelegates(t *testing.T) {
	service, _ := newService(t)
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", sampleZip(t))
	if err != nil {
		t.Fatal(err)
	}
	removed, err := service.Uninstall(context.Background(), "usr_svc_0001", installed.ThemeID)
	if err != nil || !removed {
		t.Fatalf("Uninstall() = %v, %v", removed, err)
	}
	if _, err := service.Uninstall(context.Background(), "usr_svc_0001", installed.ThemeID); !errors.Is(err, themes.ErrNotFound) {
		t.Fatalf("second uninstall err = %v", err)
	}
}
```

`makeSampleTarGz(t)`:用 `archive/tar`+`compress/gzip` 打包 `repo-c/theme.json` + `repo-c/theme.css`(内容同 sampleZip),写在本测试文件底部;`service.github` 字段需可从包内测试赋值(小写字段,同包)。补 import:`"io"`、`"net/http"`。

- [ ] **Step 2: 确认失败后实现 `service.go`**

```go
package themeimport

import (
	"context"
	"errors"
	"time"

	"github.com/yixian-huang/navax/internal/themes"
)

// Service 编排主题导入:解包 → 组包 → (店内)编译并落库。
type Service struct {
	store  *themes.Store
	github *GitHubClient
	quota  int
	now    func() time.Time
}

func NewService(store *themes.Store, github *GitHubClient, quota int) *Service {
	return &Service{store: store, github: github, quota: quota, now: time.Now}
}

// ValidationIssue 是 dry-run 校验的结构化错误。当前粒度为首个错误。
type ValidationIssue struct {
	Stage   string `json:"stage"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func classifyIssue(err error) ValidationIssue {
	issue := ValidationIssue{Message: err.Error()}
	switch {
	case errors.Is(err, themes.ErrInvalidArchive):
		issue.Stage = "archive"
	case errors.Is(err, themes.ErrInvalidManifest):
		issue.Stage = "manifest"
	case errors.Is(err, themes.ErrInvalidCSS):
		issue.Stage = "css"
		issue.Path = "theme.css"
	case errors.Is(err, themes.ErrInvalidAsset):
		issue.Stage = "asset"
	default:
		issue.Stage = "archive"
	}
	if issue.Stage == "manifest" {
		issue.Path = "theme.json"
	}
	return issue
}

func packageFromZip(zipData []byte) (themes.Package, error) {
	files, err := themes.ExtractZip(zipData)
	if err != nil {
		return themes.Package{}, err
	}
	return themes.PackageFromFiles(files)
}

func (s *Service) install(ctx context.Context, ownerID string, pkg themes.Package, sourceType, sourceURL, sourceRef string) (themes.InstalledTheme, error) {
	return s.store.InstallPrivate(ctx, ownerID, pkg.Manifest.ID, sourceType, sourceURL, sourceRef, s.quota,
		func(themeID string) (themes.Compiled, error) { return themes.Compile(pkg, themeID) }, s.now().UTC())
}

// ImportZip 安装 zip 上传的主题包。
func (s *Service) ImportZip(ctx context.Context, ownerID string, zipData []byte) (themes.InstalledTheme, error) {
	pkg, err := packageFromZip(zipData)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	return s.install(ctx, ownerID, pkg, "upload", "", themes.ContentDigest(zipData))
}

// ImportGitHub 拉取仓库 tarball 并安装;source_ref 锁定 commit sha。
func (s *Service) ImportGitHub(ctx context.Context, ownerID, repoURL, ref string) (themes.InstalledTheme, error) {
	fetched, err := s.github.FetchTarball(ctx, repoURL, ref)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	files, err := themes.ExtractTarGz(fetched.Data)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	pkg, err := themes.PackageFromFiles(files)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	return s.install(ctx, ownerID, pkg, "github", fetched.CanonicalURL, fetched.SHA)
}

// Uninstall 转发到 store(removed=false 表示墓碑)。
func (s *Service) Uninstall(ctx context.Context, ownerID, themeID string) (bool, error) {
	return s.store.UninstallPrivate(ctx, ownerID, themeID, s.now().UTC())
}

// ValidatePackage 对 zip 做 dry-run:走完整管线但不落库。空切片 = 合法。
func (s *Service) ValidatePackage(zipData []byte) []ValidationIssue {
	pkg, err := packageFromZip(zipData)
	if err != nil {
		return []ValidationIssue{classifyIssue(err)}
	}
	if _, err := themes.Compile(pkg, "validate-preview"); err != nil {
		return []ValidationIssue{classifyIssue(err)}
	}
	return nil
}
```

`themes.ContentDigest`:在 `internal/themes/archive.go` 追加一个小导出函数(zip 上传的 source_ref 用内容摘要,与设计「上传摘要」一致):

```go
// ContentDigest 给上传内容一个稳定标识,记入 source_ref。
func ContentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}
```

(archive.go 的 import 相应补 `crypto/sha256`、`encoding/hex`。)

- [ ] **Step 3: 运行测试验证通过**

Run: `go test ./internal/themeimport ./internal/themes`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/themeimport/ internal/themes/archive.go
git commit -m "feat: orchestrate theme imports with dry-run validation"
```

---

### Task 7: 配置、HTTP 面、限流与接线 + openapi

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/httpapi/themeimport.go`
- Modify: `internal/httpapi/rate_limit.go:32-50`、`internal/app/run.go`
- Modify: `api/openapi.yaml`

**Interfaces:**
- Consumes: Task 6 的 `Service` 全部方法、`catalog.Service.Themes(ctx, actorID)`(internal/catalog/service.go:114,用于 201 返回完整 Theme 形状)、`SessionFromContext`(session.go:37)、`WriteJSON/WriteError`(response.go)、`decodeJSON`(auth.go:529)、assets.go:37-91 的 multipart 模式。
- Produces:
  - `Config` 新字段:`ThemePrivateQuota int`、`ThemeImportHosts []string`、`GitHubToken string`;`func envInt(name string, fallback int) (int, error)`
  - `type ThemeImportHandler struct { service *themeimport.Service; catalog *catalog.Service }`;`func NewThemeImportHandler(service *themeimport.Service, catalogService *catalog.Service) *ThemeImportHandler`;`MountProtected(router chi.Router)` 挂 `POST /me/themes/import`、`DELETE /me/themes/{themeId}`、`POST /themes/validate`
  - openapi 三端点 + `ThemeImportRequest`/`ThemeValidationEnvelope` schema;`ThemeResponse`/`ThemeEnvelope`(单实体信封,若不存在则新增,命名沿用惯例)

- [ ] **Step 1: config 扩展**

`internal/config/config.go`:`Config` 追加三字段;`Load()` 内(其它 env 读取旁):

```go
	quota, err := envInt("NAVAX_THEME_PRIVATE_QUOTA", 10)
	if err != nil {
		return Config{}, err
	}
	cfg.ThemePrivateQuota = quota
	cfg.GitHubToken = strings.TrimSpace(os.Getenv("NAVAX_GITHUB_TOKEN"))
	if hosts := strings.TrimSpace(os.Getenv("NAVAX_THEME_IMPORT_HOSTS")); hosts != "" {
		for _, host := range strings.Split(hosts, ",") {
			if host = strings.TrimSpace(host); host != "" {
				cfg.ThemeImportHosts = append(cfg.ThemeImportHosts, host)
			}
		}
	}
```

```go
func envInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s 必须是正整数: %q", name, raw)
	}
	return value, nil
}
```

(赋值语法融入 `Load()` 的实际结构——它可能是构造字面量而非逐字段赋值,以现文为准。)`.env.example` 追加三行注释说明新变量。

- [ ] **Step 2: handler**

`internal/httpapi/themeimport.go`:

```go
package httpapi

import (
	"errors"
	"mime"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/themeimport"
	"github.com/yixian-huang/navax/internal/themes"
)

// ThemeImportHandler 提供主题导入、卸载与 dry-run 校验。
type ThemeImportHandler struct {
	service        *themeimport.Service
	catalogService *catalog.Service
}

func NewThemeImportHandler(service *themeimport.Service, catalogService *catalog.Service) *ThemeImportHandler {
	return &ThemeImportHandler{service: service, catalogService: catalogService}
}

func (h *ThemeImportHandler) MountProtected(router chi.Router) {
	router.Post("/me/themes/import", h.importTheme)
	router.Delete("/me/themes/{themeId}", h.uninstall)
	router.Post("/themes/validate", h.validate)
}

const themeArchiveOverhead int64 = 1 << 20

type themeImportRequest struct {
	GitHubURL string `json:"githubUrl"`
	Ref       string `json:"ref"`
}

func (h *ThemeImportHandler) importTheme(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var (
		installed themes.InstalledTheme
		err       error
	)
	switch {
	case mediaType == "multipart/form-data":
		data, ok := h.readArchive(w, r)
		if !ok {
			return
		}
		installed, err = h.service.ImportZip(r.Context(), session.User.ID, data)
	case mediaType == "application/json":
		var payload themeImportRequest
		if !decodeJSON(w, r, &payload) {
			return
		}
		if payload.GitHubURL == "" {
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "githubUrl 不能为空", nil)
			return
		}
		installed, err = h.service.ImportGitHub(r.Context(), session.User.ID, payload.GitHubURL, payload.Ref)
	default:
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请使用 multipart/form-data 上传 zip,或 application/json 提供 githubUrl", nil)
		return
	}
	if err != nil {
		h.writeImportError(w, r, err)
		return
	}
	// 201 返回与列表一致的 Theme 形状,前端免二次映射。
	items, listErr := h.catalogService.Themes(r.Context(), session.User.ID)
	if listErr == nil {
		for _, item := range items {
			if item.ID == installed.ThemeID {
				WriteJSON(w, r, http.StatusCreated, themeData(item))
				return
			}
		}
	}
	WriteJSON(w, r, http.StatusCreated, map[string]any{"id": installed.ThemeID, "slug": installed.Slug, "currentVersionId": installed.VersionID})
}

// readArchive 按 assets.go 的模式取 multipart 的 file 字段,压缩包硬上限。
func (h *ThemeImportHandler) readArchive(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	maximum := int64(themes.MaxArchiveBytes)
	r.Body = http.MaxBytesReader(w, r.Body, maximum+themeArchiveOverhead)
	if err := r.ParseMultipartForm(themeArchiveOverhead); err != nil {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
		return nil, false
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "file 字段必须提供且仅一次", nil)
		return nil, false
	}
	defer file.Close()
	if header.Size > maximum {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
		return nil, false
	}
	return data, true
}

func (h *ThemeImportHandler) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, themes.ErrQuotaExceeded):
		WriteError(w, r, http.StatusConflict, "QUOTA_EXCEEDED", "私有主题数量已达上限(含已卸载但仍被历史发布引用的主题)", nil)
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

func (h *ThemeImportHandler) uninstall(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	if _, err := h.service.Uninstall(r.Context(), session.User.ID, chi.URLParam(r, "themeId")); err != nil {
		if errors.Is(err, themes.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "主题不存在", nil)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "卸载失败", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ThemeImportHandler) validate(w http.ResponseWriter, r *http.Request) {
	data, ok := h.readArchive(w, r)
	if !ok {
		return
	}
	issues := h.service.ValidatePackage(data)
	if issues == nil {
		issues = []themeimport.ValidationIssue{}
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"valid": len(issues) == 0, "errors": issues})
}
```

补 import `"io"`。注意:`themeData` 是 admin.go 里的既有序列化函数,同包直接复用;`catalog.Themes` 返回 `[]adminpkg.Theme`,类型对上。**WriteError 的 422 分支带 err**(status<500 会把 `err.Error()` 放 meta.detail,作者能看到具体规则文案)。

- [ ] **Step 3: 限流与接线**

`rate_limit.go` 规则表(assets 那条之后)追加:

```go
		{http.MethodPost, exactPath("/api/v1/me/themes/import"), 10, time.Hour},
		{http.MethodPost, exactPath("/api/v1/themes/validate"), 20, time.Hour},
```

`internal/app/run.go`:在 `themeStore`/`SyncBuiltin` 之后、受保护路由组内:

```go
	themeImportService := themeimport.NewService(themeStore,
		themeimport.NewGitHubClient(nil, nil, cfg.ThemeImportHosts, cfg.GitHubToken), cfg.ThemePrivateQuota)
	themeImportHandler := httpapi.NewThemeImportHandler(themeImportService, catalogService)
```

(`catalogService` 用 run.go 里给 `httpapi.NewCatalogHandler` 的同一个变量,以现文实际名字为准。)受保护组内 `themeImportHandler.MountProtected(protected)`。

- [ ] **Step 4: openapi**

三端点(路径区按现有排序插入)+ schemas。`/api/v1/me/themes/import`:

```yaml
  /api/v1/me/themes/import:
    post:
      tags: [Themes]
      operationId: importTheme
      security: [{ sessionCookie: [] }]
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              required: [file]
              properties:
                file: { type: string, format: binary, description: 主题包 zip(≤ 16 MiB) }
          application/json:
            schema: { $ref: '#/components/schemas/ThemeImportRequest' }
      responses:
        '201': { $ref: '#/components/responses/ThemeResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '413': { $ref: '#/components/responses/ErrorResponse' }
        '415': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
        '502': { $ref: '#/components/responses/ErrorResponse' }
  /api/v1/me/themes/{themeId}:
    delete:
      tags: [Themes]
      operationId: uninstallTheme
      security: [{ sessionCookie: [] }]
      parameters:
        - $ref: '#/components/parameters/ThemeId'
      responses:
        '204': { description: 已卸载(物理删除或转墓碑) }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
  /api/v1/themes/validate:
    post:
      tags: [Themes]
      operationId: validateTheme
      security: [{ sessionCookie: [] }]
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              required: [file]
              properties:
                file: { type: string, format: binary }
      responses:
        '200': { $ref: '#/components/responses/ThemeValidationResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '413': { $ref: '#/components/responses/ErrorResponse' }
        '422': { $ref: '#/components/responses/ErrorResponse' }
```

schemas / responses(信封模式照抄现有惯例;`ThemeResponse` 若已存在则复用):

```yaml
    ThemeImportRequest:
      type: object
      required: [githubUrl]
      properties:
        githubUrl: { type: string, maxLength: 300, description: 'https://github.com/{owner}/{repo}' }
        ref: { type: string, maxLength: 120, description: 分支/标签/40 位 commit sha,缺省默认分支 }
    ThemeValidationIssue:
      type: object
      required: [stage, path, message]
      properties:
        stage: { type: string, enum: [archive, manifest, css, asset] }
        path: { type: string }
        message: { type: string }
    ThemeValidationResult:
      type: object
      required: [valid, errors]
      properties:
        valid: { type: boolean }
        errors: { type: array, items: { $ref: '#/components/schemas/ThemeValidationIssue' } }
```

`ThemeValidationEnvelope`/`ThemeEnvelope` 按既有 `<X>Envelope` 模式(code/data/meta)新增,`ThemeValidationResponse`/`ThemeResponse` 按单行 responses 惯例新增(先 grep `ThemeResponse` 是否已存在)。

- [ ] **Step 5: 编译与既有测试回归**

Run: `go build ./... && go test ./internal/httpapi ./internal/app/... ./internal/config 2>/dev/null; go test ./internal/...`
Expected: PASS(httpapi 的路由级单测若有 route 快照断言需按新路由更新)

- [ ] **Step 6: Commit**

```bash
git add internal/config/ internal/httpapi/ internal/app/ api/openapi.yaml .env.example
git commit -m "feat: expose theme import, uninstall and validate endpoints"
```

---

### Task 8: 契约测试与 mock

**Files:**
- Modify: `tests/contract/client_test.go`(multipart 泛化)、`tests/contract/api_contract_test.go`、`tests/contract/main_test.go`(配额 env)
- Modify: `web/src/api/mock-handlers.ts`、`web/tests/mock-contract.test.ts`

**Interfaces:**
- Consumes: Task 7 的端点与 openapi;既有 user 客户端(「邀请注册新用户”流程产出);`mockThemes`(web/src/mocks/data.ts:365)。
- Produces:
  - `func (c *apiClient) uploadMultipart(t *testing.T, path string, fields map[string]string, fileField, filename string, content []byte) apiResult`;`uploadPNG` 改为对它的薄封装(`uploadMultipart(t, "/api/v1/assets", map[string]string{"kind": kind}, "file", "background.png", png)`)。
  - contract 端 `buildThemeZip(t, slug string) []byte` 辅助(archive/zip 现场构造,manifest 与 Task 6 的 `sampleManifest` 同款、id 用参数 slug)。
  - mock:`mockPrivateThemes` 数组 + import/uninstall/validate handler + 列表合并。

- [ ] **Step 1: 契约客户端泛化**

`client_test.go`:新增 `uploadMultipart`(签名如上;主体从 `uploadPNG` 抽出,URL/字段名/文件名参数化,响应校验与 JSON 解析逻辑不变),`uploadPNG` 委托之。

- [ ] **Step 2: 配额 env**

`tests/contract/main_test.go`:找到启动二进制的 `exec.Command`/env 组装处,追加 `NAVAX_THEME_PRIVATE_QUOTA=2`(让配额分支可低成本触达)。

- [ ] **Step 3: 新 t.Run(放在「个人资料」之后)**

```go
	t.Run("主题导入与私有安装", func(t *testing.T) {
		// zip 导入 → 201 且响应过 Theme schema 校验。
		imported := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "lilac.zip", buildThemeZip(t, "lilac"))
		mustStatus(t, imported, http.StatusCreated, "导入主题")
		importedID := stringField(t, imported.data(), "id", "导入主题 ID")

		// 列表出现(带会话)。
		list := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, list, http.StatusOK, "含私有主题的列表")
		if !strings.Contains(string(list.body), importedID) {
			t.Fatalf("列表缺少导入的主题 %s", importedID)
		}

		// 应用到自己的页面并发布,公开读锁定其版本。
		page := user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		userPageID := stringField(t, page.data(), "id", "用户页面")
		settings, _ := page.data()["settings"].(map[string]any)
		appearance, _ := settings["appearance"].(map[string]any)
		appearance["themeId"] = importedID
		updated := user.call(t, http.MethodPut, fmt.Sprintf("/api/v1/pages/%s/settings", userPageID), settings)
		mustStatus(t, updated, http.StatusOK, "应用私有主题")
		// (settings 更新端点的方法与请求形状以 openapi 现文为准——PATCH/PUT、
		// 全量/局部,动手前先查 /pages/{pageId}/settings 定义,按契约构造。)

		revision := numberField(t, user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil).data(), "draftRevision", "修订号")
		published := user.call(t, http.MethodPost, fmt.Sprintf("/api/v1/pages/%s/publish", userPageID),
			map[string]any{"expectedRevision": revision}, withHeader("Idempotency-Key", "contract-theme-import-0001"))
		mustStatus(t, published, http.StatusOK, "发布私有主题页面")

		// 配额 = 2:第二个成功,第三个 409。
		second := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "second.zip", buildThemeZip(t, "second"))
		mustStatus(t, second, http.StatusCreated, "第二个私有主题")
		secondID := stringField(t, second.data(), "id", "第二主题 ID")
		third := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "third.zip", buildThemeZip(t, "third"))
		mustStatus(t, third, http.StatusConflict, "配额 409")

		// 同 slug 重复导入 = 升级,不占额度。
		again := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "lilac2.zip", buildThemeZip(t, "lilac"))
		mustStatus(t, again, http.StatusCreated, "重复导入即升级")

		// dry-run:坏包 200 + valid=false。
		invalid := user.uploadMultipart(t, "/api/v1/themes/validate", nil, "file", "bad.zip", []byte("not a zip"))
		mustStatus(t, invalid, http.StatusOK, "校验坏包")
		if valid, _ := invalid.data()["valid"].(bool); valid {
			t.Fatal("坏包 valid 应为 false")
		}

		// 卸载未被引用的第二主题 → 204;再删 → 404。
		removed := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+secondID, nil)
		mustStatus(t, removed, http.StatusNoContent, "卸载私有主题")
		missing := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+secondID, nil)
		mustStatus(t, missing, http.StatusNotFound, "重复卸载 404")

		// 坏包导入 → 422。
		bad := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "bad.zip", []byte("not a zip"))
		mustStatus(t, bad, http.StatusUnprocessableEntity, "坏包 422")
	})
```

`buildThemeZip` 写在 api_contract_test.go 底部(archive/zip + manifest 字面量,id 参数化;css 一行合法规则)。**注意两点**:(a) 配额按行数计——本 t.Run 结束时该用户有 lilac(已发布引用?未引用也保留)与 third 失败、second 已删,后续既有 t.Run 不受影响;(b) 「用户编辑与发布」t.Run 在前面已用 user 发布过页面,本处再次发布用新的 Idempotency-Key 与最新 revision,若冲突以实际流程状态调整(实现时跑一遍按真实状态微调,原则:不改既有步骤,只追加)。

- [ ] **Step 4: mock**

`mock-handlers.ts`:
1. `// ---- Theme import ----` 区段(backgrounds handler 之后):模块级 `const mockPrivateThemes: Theme[] = [];`;POST `${API_BASE}/me/themes/import`(FormData 或 JSON 都返回一个固定形状私有主题,id `thm_mock_priv_${序号}`,scope 'private',currentVersionId 满足 `^v[0-9a-f]{32}$`,push 进数组,201);DELETE `${API_BASE}/me/themes/{id}`(splice,204 空响应);POST `${API_BASE}/themes/validate`(200 `{valid:true, errors:[]}` 信封)。
2. 列表 handler(:861-863 与 admin :1031-1033)返回 `[...mockThemes, ...mockPrivateThemes]`。
3. `mapContractUrlToLegacy` 无需新映射(这三个契约路径不与旧 pages 正则冲突,handler 直接匹配契约 URL;确认 `/me/themes/import` 不被 `/me/subdomain` 之类的既有映射误吞)。

`mock-contract.test.ts` cases 追加:

```ts
  { name: '导入主题', path: '/api/v1/me/themes/import', method: 'post', status: '201', url: '/api/v1/me/themes/import', init: { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ githubUrl: 'https://github.com/e2e/lilac' }) } },
  { name: '主题包校验', path: '/api/v1/themes/validate', method: 'post', status: '200', url: '/api/v1/themes/validate', init: { method: 'POST' } },
```

- [ ] **Step 5: 运行**

Run: `make test-contract && make test-mock && make check`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tests/contract/ web/src/api/mock-handlers.ts web/tests/mock-contract.test.ts
git commit -m "test: cover theme import contract end to end and in the dev mock"
```

---

### Task 9: Theme 契约扩展(sourceType/sourceUrl)与前端 API 模块

**Files:**
- Modify: `internal/admin/service.go:96-115`(Theme 结构体)、`internal/catalog/service.go:114-150`、`internal/admin/sqlstore.go`(themeSelect)、`internal/httpapi/admin.go`(themeData)
- Modify: `api/openapi.yaml`(Theme schema)、`web/src/mocks/data.ts`(mockThemes 补字段)
- Create: `web/src/api/themes.ts`
- Modify: `web/src/api/types.ts`(Theme 接口)、`web/src/themes/types.ts`(ThemeMeta 透出)

**Interfaces:**
- Consumes: Task 2 之后 `themes.source_type/source_url` 列已有可靠数据。
- Produces:
  - `adminpkg.Theme` 增 `SourceType string`、`SourceURL string`;catalog 与 admin 查询都 SELECT 这两列;`themeData` 在非空时序列化 `sourceType`/`sourceUrl`。
  - openapi `Theme` schema 增可选 `sourceType: { type: string, enum: [builtin, github, upload] }`、`sourceUrl: { type: string }`。
  - `web/src/api/themes.ts`:
    ```ts
    export const themesApi = {
      importZip: (file: File) => { const body = new FormData(); body.set('file', file);
        return request<ApiResponse<Theme>>('/me/themes/import', { method: 'POST', body }); },
      importGitHub: (githubUrl: string, ref?: string) =>
        request<ApiResponse<Theme>>('/me/themes/import', { method: 'POST', body: ref ? { githubUrl, ref } : { githubUrl } }),
      uninstall: (themeId: string) =>
        request<ApiResponse<null>>(`/me/themes/${encodeURIComponent(themeId)}`, { method: 'DELETE' }),
    };
    ```
  - `web/src/api/types.ts` 的 `Theme` 增 `sourceType?: 'builtin' | 'github' | 'upload'; sourceUrl?: string;`
  - `web/src/themes/types.ts`:`ThemeMeta` 增 `scope?: 'catalog' | 'private'; sourceType?: 'builtin' | 'github' | 'upload'; sourceUrl?: string;`,`themePackageFromApi`/`themeDisplayFromApi` 透传三字段。

- [ ] **Step 1: 后端字段接线(测试先行)**

`internal/catalog/service_test.go` 追加断言(在既有主题列表测试旁):种子后 `Themes(ctx, "")` 返回的 slate 有 `SourceType == "builtin"`。Run → FAIL → 实现:结构体加字段、两处 SELECT 补 `themes.source_type, themes.source_url`、Scan 对应、`themeData` 补:

```go
	if item.SourceType != "" {
		data["sourceType"] = item.SourceType
	}
	if item.SourceURL != "" {
		data["sourceUrl"] = item.SourceURL
	}
```

openapi `Theme` schema 补两个可选字段。Run: `go test ./internal/catalog ./internal/admin && make test-contract` → PASS。

- [ ] **Step 2: 前端模块与类型**

按上方 Produces 原样创建/修改;`web/src/mocks/data.ts` 的 `mockThemes` 三条补 `sourceType: 'builtin'`。Run: `make check && make test-mock` → PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/ api/openapi.yaml web/src/
git commit -m "feat: expose theme source metadata through the contract"
```

---

### Task 10: 导入 UI 与「我的主题」分组

**Files:**
- Create: `web/src/components/base/ThemeImportDialog.tsx`
- Modify: `web/src/pages/app/themes/page.tsx`

**Interfaces:**
- Consumes: `themesApi`(Task 9)、`useToast`、`ConfirmDialog`(SharedUI.tsx:137)、`FormField/FormInput`(AddDialogs.tsx 同款)、`useQueryClient`(invalidate `['navigation','themes']`)、`ThemePackage.meta.scope/sourceType/sourceUrl`。
- Produces: `ThemeImportDialog({ open, onClose, onImported }: { open: boolean; onClose: () => void; onImported: () => void })`。

- [ ] **Step 1: 对话框组件**

`ThemeImportDialog.tsx` 按 `AddCategoryDialog`(AddDialogs.tsx:22-107)的手写模态模式实现,要点(样式细节沿用该文件的类名惯例):

- 两个 tab:`'github' | 'zip'`(useState;顶部两枚 segment 按钮)。
- GitHub tab:`FormInput` 仓库地址(placeholder `https://github.com/owner/repo`)+ 可选 ref 输入;提交 → `themesApi.importGitHub(url.trim(), ref.trim() || undefined)`。
- zip tab:隐藏 `<input type="file" accept=".zip" data-testid="theme-zip-input">` + 「选择 zip 文件」按钮;选中即显示文件名;提交 → `themesApi.importZip(file)`。
- 提交中禁用按钮显示 Loader;成功 → `toast('success', 已导入主题「…」)` + `onImported()` + `onClose()`;失败 → `toast('error', cause.message)`(client.ts 的错误 message 已含服务端文案)。
- `if (!open) return null;` 开头,overlay 点击关闭,与既有对话框一致。

- [ ] **Step 2: themes 页接线**

`web/src/pages/app/themes/page.tsx`:

1. `const myThemes = useMemo(() => themes.filter(t => t.meta.scope === 'private'), [themes]);` 且 `seriousThemes`/`cuteThemes` 的 filter 追加 `t.meta.scope !== 'private'`(私有主题只出现在自己的分组)。
2. 页面标题区(`:500-512`)加「导入主题」按钮(`useState` 控制对话框开关);对话框 `onImported` 里 `queryClient.invalidateQueries({ queryKey: ['navigation', 'themes'] })`。
3. 新分组「我的主题」渲染在 Classic 分组之前:复用 `renderThemeCard`,卡片右上角对私有主题追加两枚小操作(不改 `renderThemeCard` 签名——包一层 `renderMyThemeCard(pkg)`,在卡片容器外叠操作条):
   - 「升级」:`meta.sourceType === 'github' && meta.sourceUrl` → 点击调 `themesApi.importGitHub(meta.sourceUrl)`(重新解析默认分支)+ invalidate + toast;`sourceType === 'upload'` → 触发一个对话框外的隐藏 zip input(`data-testid="theme-upgrade-input"`)重新上传。
   - 「卸载」:`ConfirmDialog`(danger)确认后 `themesApi.uninstall(pkg.id)` + invalidate + toast(文案:「已卸载。若曾被历史发布引用,名额暂不释放。」)。当前草稿正用该主题时不拦截——发布回落默认主题,与目录主题下架同语义(设计 §4)。
4. 空态:`myThemes.length === 0` 时分组不渲染(不加占位)。

**注意**:本页已有两个隐藏 file input(背景上传,E2E 依赖 `.first()` 顺序,user.spec.ts:157-159)。新增的 zip input 一个在对话框内(仅 open 时渲染)、一个升级用(仅私有 upload 主题存在时渲染)——**都必须用 `data-testid` 定位,且不得渲染在背景 input 之前**;完成后跑一遍既有 E2E 背景用例确认顺序未破坏。

- [ ] **Step 3: 冒烟与守卫**

Run: `make check && make test-mock`,然后 `cd web && VITE_ENABLE_API_MOCKS=true npm run dev` 浏览 `/app/themes`:导入对话框两个 tab 提交(mock 返回固定私有主题)、我的主题分组出现、卸载走确认框、移动端(375px)与暗色主题不破版、键盘 Tab 可达对话框控件。六态记录进报告。
Expected: 全绿。

- [ ] **Step 4: Commit**

```bash
git add web/src/
git commit -m "feat: theme import dialog and private theme management ui"
```

---

### Task 11: E2E —— fixture 包与导入用例

**Files:**
- Create: `tests/e2e/fixtures/theme-lilac.zip`
- Modify: `tests/e2e/specs/user.spec.ts`

**Interfaces:**
- Consumes: Task 10 的 `data-testid="theme-zip-input"`;既有 `USER` 账号与 `.auth/user.json`。
- Produces: 无。

- [ ] **Step 1: 生成 fixture zip**

用一次性脚本生成并提交(内容与 Task 6 的 sampleManifest 同款、id=lilac;**zip 不含顶层目录**):

```bash
cd "$(mktemp -d)" && cat > theme.json <<'EOF'
{
  "specVersion": 1, "id": "lilac", "name": "Lilac", "version": "1.0.0",
  "author": "e2e", "license": "MIT", "mode": "light", "vibe": "serious",
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
}
EOF
printf '[data-nx="site-card"] { border-radius: var(--radius-md); }\n' > theme.css
zip -X theme-lilac.zip theme.json theme.css
cp theme-lilac.zip /Users/isian/workspace/navax/tests/e2e/fixtures/
```

先用契约测试的校验器口径自证:`go run ./cmd/navax --help >/dev/null 2>&1 || true`(可选);更直接的自证是在 Task 8 的 `buildThemeZip` 单测已覆盖同款 manifest——两处字面量保持一致。

- [ ] **Step 2: user.spec 追加用例(「草稿预览呈现所选主题」之后)**

```ts
  test('导入 zip 主题并应用', async ({ page }) => {
    await page.goto('/app/themes');
    await page.getByRole('button', { name: /导入主题/ }).click();
    await page.getByRole('button', { name: /上传 zip|zip/i }).click();
    await page.locator('[data-testid="theme-zip-input"]').setInputFiles(THEME_ZIP);
    await page.getByRole('button', { name: /导入/ }).last().click();
    await expect(page.getByText(/已导入主题/)).toBeVisible({ timeout: 15000 });
    // 我的主题分组出现,卡片可选用。
    await expect(page.getByText('我的主题')).toBeVisible();
    await page.getByRole('button', { name: /Lilac/ }).click();
    await expect(page.getByText(/主题已写入草稿：「Lilac」/)).toBeVisible();
  });
```

文件顶部常量区追加 `const THEME_ZIP = fileURLToPath(new URL('../fixtures/theme-lilac.zip', import.meta.url));`。按钮的可访问名以 Task 10 实际渲染为准微调(原则:用 role+name,不用 CSS 类)。

- [ ] **Step 3: 运行**

Run: `make e2e`
Expected: 全部 PASS(含既有背景上传用例——input 顺序未破坏)

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/
git commit -m "test: e2e import a zip theme and apply it"
```

---

### Task 12: 文档、全量验证与交付

**Files:**
- Modify: `docs/theme-api.md`(新 §6)、`docs/architecture.md`(themes 表清单一行)
- 验证与 PR,无其他代码。

- [ ] **Step 1: theme-api.md 新章节「6. 导入与安装」**

内容要点(作者视角,中文,与实现严格一致):包布局(theme.json/theme.css/assets/preview.png,顶层目录会被剥离);zip 上传与 GitHub 导入(锁 commit sha;私有安装仅自己可用;同 slug 重复导入即升级);体积与数量硬限(16 MiB/200 文件/包体 4 MiB);配额(默认 10,实例可调);`POST /api/v1/themes/validate` dry-run 用法与错误分类(archive/manifest/css/asset);GitHub 匿名 API 限额说明(sha 直填可绕过、`NAVAX_GITHUB_TOKEN`、zip 兜底);`NAVAX_THEME_IMPORT_HOSTS` 追加主机按 Gitea 兼容 archive 布局且须显式 ref。`docs/architecture.md` 核心表列举处补 `theme_versions`、`theme_assets`(顺带清掉既知过期项)。

- [ ] **Step 2: 全量门槛**

Run:
```bash
make check && go test -race ./... && make build && make test-contract && make test-mock && make e2e
```
Expected: 全部 PASS;任何失败回对应任务修复,不带失败推送。

- [ ] **Step 3: 推送与 PR**

```bash
git push -u origin feat/theme-import-b1
gh pr create --title "feat: theme import and private install (B1)" --body "$(cat <<'EOF'
## Summary
- zip 上传与 GitHub 一键导入共用既有校验/编译管线;解包层四项硬限(zip-slip/炸弹/文件数/顶层剥离)
- 首个 themes 行创建路径:私有安装(配额默认 10,可调)、同 slug 重复导入即升级、卸载分物理删/墓碑
- GitHub 锁 commit sha,netguard SSRF 防护,可选 token,Gitea 兼容主机白名单
- preview.png 入库为内容寻址资产,themes.preview 首次有真值;版本 upsert 回写 manifest 元数据,消除行/manifest 漂移
- POST /api/v1/themes/validate 作者 dry-run;导入与校验独立限流
- 管理端私有主题不可设默认;用户删除 RESTRICT 不变量测试钉住(系统尚无删除用户路径,清理接线随该功能立项)
- /app/themes 导入对话框 + 我的主题分组 + 卸载/升级;契约/mock/E2E 全链路覆盖

设计文档:docs/superpowers/specs/2026-07-24-theme-import-b1-design.md

## Test plan
- [x] make check / go test -race ./... / make build
- [x] make test-contract / make test-mock / make e2e
- [x] /app/themes 浏览器六态冒烟

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr merge --auto --rebase
```

- [ ] **Step 4: 确认 CI**

Run: `gh pr checks --watch`(或后台轮询)
Expected: `verify`、`e2e`、`container` 全绿自动合并;`deploy-production` 随 main 自动执行。
