package themeimport

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/themes"
)

// 最小合法主题包。manifest 字面量以 internal/themes 的 manifest 校验规则为准:
// 颜色四组 + 字体四族 + 三个 swatch + tier 1。与 manifest_test.go 里的
// minimalManifest 保持同等约束——不一致时以那边为准。
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

// fixturePNG 是最小合法 PNG 字节:仅 magic number + 填充,足以通过
// internal/themes 的 magic-bytes 检测。与 internal/themes/compile_test.go
// 里的 fixturePNG 构造方式一致(该文件是 themes 包内部测试,不可跨包导入,
// 故在此就地内联同样的字节)。
var fixturePNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)

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

// makeSampleTarGz 用 archive/tar + compress/gzip 打包 repo-c/theme.json +
// repo-c/theme.css(内容同 sampleZip),模拟 GitHub codeload 产物的顶层目录。
func makeSampleTarGz(t *testing.T) []byte {
	t.Helper()
	entries := map[string][]byte{
		"repo-c/theme.json": []byte(sampleManifest),
		"repo-c/theme.css":  []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newServiceDB 与 newService 共享同一套建库逻辑,但额外把 *sql.DB 交还给
// 调用方——newService 本身遵循简报约定只返回 (*Service, *themes.Store),
// 而 preview.png 回写断言需要直接查 themes 表,themes.Store 未导出 db 字段。
func newServiceDB(t *testing.T) (*Service, *themes.Store, *sql.DB) {
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
	return NewService(store, NewGitHubClient(publicResolver(), nil, nil, ""), 10), store, db
}

func newService(t *testing.T) (*Service, *themes.Store) {
	t.Helper()
	service, store, _ := newServiceDB(t)
	return service, store
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

// TestImportZipWritesPreviewAssetURL 确认 zip 内含合法 preview.png 时,
// 导入后 themes.preview 回写为该版本的资产同源 URL——补上 Task 2
// preview.png 正向回写分支此前缺失的测试覆盖。
func TestImportZipWritesPreviewAssetURL(t *testing.T) {
	service, _, db := newServiceDB(t)
	zipData := buildZip(t, map[string][]byte{
		"theme.json":  []byte(sampleManifest),
		"theme.css":   []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
		"preview.png": fixturePNG,
	})
	installed, err := service.ImportZip(context.Background(), "usr_svc_0001", zipData)
	if err != nil {
		t.Fatalf("ImportZip() error = %v", err)
	}
	var preview string
	if err := db.QueryRow(`SELECT preview FROM themes WHERE id = ?`, installed.ThemeID).Scan(&preview); err != nil {
		t.Fatalf("query preview: %v", err)
	}
	want := "/api/v1/public/themes/" + installed.VersionID + "/assets/preview.png"
	if preview != want {
		t.Fatalf("preview = %q, want %q", preview, want)
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
	tarball := makeSampleTarGz(t)
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
