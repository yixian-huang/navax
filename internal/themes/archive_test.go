package themes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
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

// TestExtractZipRejectsDuplicateEntries 确认同名条目被拒绝，而不是静默地
// 后者覆盖前者——静默覆盖会让校验/编译看到的内容与作者实际打包的内容不一致，
// 且两个条目各自都占用了解压预算，不能被 len(files) 掩盖。
func TestExtractZipRejectsDuplicateEntries(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, data := range [][]byte{[]byte("first"), []byte("second")} {
		f, err := w.Create("theme.css")
		if err != nil {
			t.Fatalf("zip create: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	_, err := ExtractZip(buf.Bytes())
	if !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("err = %v, want ErrInvalidArchive", err)
	}
	if !strings.Contains(err.Error(), "重复条目") {
		t.Fatalf("err = %q, want mention of 重复条目", err.Error())
	}
}

// TestExtractTarGzRejectsDuplicateEntries 是上面 zip 用例的 tar.gz 版本。
func TestExtractTarGzRejectsDuplicateEntries(t *testing.T) {
	archive := makeTarGz(t, []tarEntry{
		{name: "theme.css", data: []byte("first")},
		{name: "theme.css", data: []byte("second")},
	})
	_, err := ExtractTarGz(archive)
	if !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("err = %v, want ErrInvalidArchive", err)
	}
	if !strings.Contains(err.Error(), "重复条目") {
		t.Fatalf("err = %q, want mention of 重复条目", err.Error())
	}
}

// TestExtractDiscardsMacOSJunkBeforeStrippingTopLevelDir 确认 __MACOSX/ 资源
// 分支与散落各处的 .DS_Store 被丢弃，且这一步发生在“共享单一顶层目录”判定
// 之前——垃圾条目引入了第二个顶层目录，若不先丢弃就会让本该被剥离的
// GitHub 风格外层目录保留下来，theme.json 落在错误的深度。
func TestExtractDiscardsMacOSJunkBeforeStrippingTopLevelDir(t *testing.T) {
	files, err := ExtractZip(makeZip(t, map[string][]byte{
		"lilac/theme.json":            []byte("{}"),
		"lilac/theme.css":             []byte("i{}"),
		"lilac/.DS_Store":             []byte("junk"),
		".DS_Store":                   []byte("junk"),
		"__MACOSX/lilac/._theme.json": []byte("resource-fork"),
		"__MACOSX/._lilac":            []byte("resource-fork"),
	}))
	if err != nil {
		t.Fatalf("ExtractZip() error = %v", err)
	}
	if _, ok := files["theme.json"]; !ok {
		t.Fatalf("top-level dir not stripped after discarding junk: %v", keysOf(files))
	}
	if _, ok := files["theme.css"]; !ok {
		t.Fatalf("theme.css missing after discarding junk: %v", keysOf(files))
	}
	for name := range files {
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasSuffix(name, ".DS_Store") {
			t.Fatalf("junk entry survived extraction: %q", name)
		}
	}
}

func TestCleanArchivePathAcceptsDotSlashPrefix(t *testing.T) {
	files, err := ExtractZip(makeZip(t, map[string][]byte{"./theme.json": []byte("{}")}))
	if err != nil {
		t.Fatalf("ExtractZip() error = %v", err)
	}
	if _, ok := files["theme.json"]; !ok {
		t.Fatalf("./ prefix must normalize to theme.json: %v", keysOf(files))
	}

	for _, name := range []string{"../x", "/etc/x"} {
		if _, err := ExtractZip(makeZip(t, map[string][]byte{name: []byte("x")})); !errors.Is(err, ErrInvalidArchive) {
			t.Fatalf("%q err = %v, want ErrInvalidArchive", name, err)
		}
	}
}

func TestExtractZipEnforcesBudgets(t *testing.T) {
	// 文件数超限。
	many := map[string][]byte{}
	for i := 0; i < MaxArchiveFiles+1; i++ {
		many[fmt.Sprintf("f%03d.txt", i)] = []byte("1")
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
	// minimalManifest 是 manifest_test.go 既有的最小合法 manifest 字面量。
	manifest := []byte(minimalManifest)
	files := map[string][]byte{
		"theme.json":    manifest,
		"theme.css":     []byte("[data-nx=\"site-card\"] { color: var(--primary-500); }"),
		"README.md":     []byte("ignored"),
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
	// fixtureWOFF2 是 compile_test.go 既有 fixture。
	withAsset := map[string][]byte{
		"theme.json":                manifest,
		"assets/fonts/sample.woff2": fixtureWOFF2,
	}
	pkg2, err := PackageFromFiles(withAsset)
	if err != nil {
		t.Fatalf("PackageFromFiles(asset) error = %v", err)
	}
	if len(pkg2.Assets) != 1 || pkg2.Assets[0].Path != "fonts/sample.woff2" {
		t.Fatalf("asset path = %+v", pkg2.Assets)
	}
	// preview.png 作为资产收录(最小合法 PNG 用 compile_test.go 既有 fixture fixturePNG)。
	withPreview := map[string][]byte{"theme.json": manifest, "preview.png": fixturePNG}
	pkg3, err := PackageFromFiles(withPreview)
	if err != nil {
		t.Fatalf("PackageFromFiles(preview) error = %v", err)
	}
	if len(pkg3.Assets) != 1 || pkg3.Assets[0].Path != "preview.png" {
		t.Fatalf("preview asset = %+v", pkg3.Assets)
	}
}
