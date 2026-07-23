package themes

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// 最小合法夹具：类型按 magic bytes 判定，因此不需要真实字形/像素数据。
var (
	fixtureWOFF2 = append([]byte("wOF2"), bytes.Repeat([]byte{0}, 32)...)
	fixturePNG   = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	fixtureJPEG  = append([]byte("\xff\xd8\xff"), bytes.Repeat([]byte{0}, 32)...)
)

func fixtureWebP() []byte {
	out := []byte("RIFF")
	out = append(out, 0, 0, 0, 0)
	out = append(out, []byte("WEBP")...)
	return append(out, bytes.Repeat([]byte{0}, 16)...)
}

func TestValidateAssetAcceptsSupportedTypes(t *testing.T) {
	tests := []struct {
		path string
		data []byte
		mime string
	}{
		{"fonts/sample.woff2", fixtureWOFF2, "font/woff2"},
		{"img/noise.png", fixturePNG, "image/png"},
		{"img/photo.jpg", fixtureJPEG, "image/jpeg"},
		{"img/shot.webp", fixtureWebP(), "image/webp"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			asset, err := ValidateAsset(tc.path, tc.data)
			if err != nil {
				t.Fatalf("ValidateAsset() error = %v", err)
			}
			if asset.MIME != tc.mime || asset.SHA256 == "" {
				t.Fatalf("unexpected asset: %+v", asset)
			}
		})
	}
}

func TestValidateAssetRejects(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		data    []byte
		wantMsg string
	}{
		{"扩展名与内容不符", "fonts/sample.woff2", fixturePNG, "实际内容"},
		{"svg 拒绝", "img/icon.svg", []byte("<svg xmlns='x'></svg>"), "svg"},
		{"未知扩展名", "img/icon.gif", []byte("GIF89a"), "扩展名"},
		{"内容无法识别", "img/noise.png", []byte("not an image at all"), "无法识别"},
		{"越界路径", "../../etc/passwd.png", fixturePNG, "相对路径"},
		{"绝对路径", "/etc/passwd.png", fixturePNG, "/ 开头"},
		{"反斜杠路径", `img\noise.png`, fixturePNG, "反斜杠"},
		{"空内容", "img/noise.png", nil, "为空"},
		{"超过单文件上限", "fonts/big.woff2", append([]byte("wOF2"), bytes.Repeat([]byte{0}, MaxAssetBytes)...), "上限"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateAsset(tc.path, tc.data); err == nil {
				t.Fatal("ValidateAsset() expected error, got nil")
			} else if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want to mention %q", err, tc.wantMsg)
			}
		})
	}
}

func samplePackage(t *testing.T) Package {
	t.Helper()
	manifest, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	manifest.Tokens.Font["body"] = `"Sample Sans", system-ui`
	asset, err := ValidateAsset("fonts/sample.woff2", fixtureWOFF2)
	if err != nil {
		t.Fatalf("ValidateAsset() error = %v", err)
	}
	return Package{
		Manifest: manifest,
		CSS:      []byte(`@font-face { font-family: "Sample Sans"; src: url("asset:fonts/sample.woff2"); }`),
		Assets:   []Asset{asset},
	}
}

func TestCompileProducesStableVersionID(t *testing.T) {
	pkg := samplePackage(t)
	first, err := Compile(pkg, "sample")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	second, err := Compile(pkg, "sample")
	if err != nil {
		t.Fatalf("Compile() second call error = %v", err)
	}
	if first.VersionID != second.VersionID || first.ContentHash != second.ContentHash {
		t.Fatalf("Compile() not deterministic: %q vs %q", first.VersionID, second.VersionID)
	}
	if !strings.HasPrefix(first.VersionID, "v") || len(first.VersionID) != 33 {
		t.Fatalf("unexpected version id %q", first.VersionID)
	}
}

// 资产 URL 在定下版本 ID 之后才落地，因此哈希不依赖自身。
func TestCompileResolvesAssetURLsAgainstVersionID(t *testing.T) {
	compiled, err := Compile(samplePackage(t), "sample")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	want := AssetBasePath(compiled.VersionID) + "fonts/sample.woff2"
	if !strings.Contains(string(compiled.CSS), want) {
		t.Fatalf("asset url not resolved to %q:\n%s", want, compiled.CSS)
	}
	if strings.Contains(string(compiled.CSS), AssetBasePlaceholder) {
		t.Fatalf("placeholder left in output:\n%s", compiled.CSS)
	}
}

func TestCompileRejectsCSSReferencingMissingAsset(t *testing.T) {
	pkg := samplePackage(t)
	pkg.CSS = []byte(`@font-face { font-family: "Sample Sans"; src: url("asset:fonts/missing.woff2"); }`)
	if _, err := Compile(pkg, "sample"); err == nil || !strings.Contains(err.Error(), "missing.woff2") {
		t.Fatalf("error = %v, want missing asset rejection", err)
	}
}

func TestCompileRejectsOversizePackage(t *testing.T) {
	pkg := samplePackage(t)
	blob := append([]byte("wOF2"), bytes.Repeat([]byte{0}, MaxAssetBytes-4)...)
	for i := 0; i < 9; i++ {
		pkg.Assets = append(pkg.Assets, Asset{
			Path: fmt.Sprintf("fonts/pad-%d.woff2", i),
			MIME: "font/woff2",
			Data: blob,
		})
	}
	if _, err := Compile(pkg, "sample"); err == nil || !strings.Contains(err.Error(), "整包") {
		t.Fatalf("error = %v, want package size rejection", err)
	}
}

func TestCompileEmitsTokensBeforeThemeCSS(t *testing.T) {
	compiled, err := Compile(samplePackage(t), "sample")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	css := string(compiled.CSS)
	tokenIndex := strings.Index(css, "--font-body")
	themeIndex := strings.Index(css, "@font-face")
	if tokenIndex < 0 || themeIndex < 0 || tokenIndex > themeIndex {
		t.Fatalf("tokens must precede theme rules so themes can override them:\n%s", css)
	}
}
