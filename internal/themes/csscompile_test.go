package themes

import (
	"strings"
	"testing"
)

func TestCompileCSSScopesSelectors(t *testing.T) {
	out, err := CompileCSS([]byte(`:root { --x: 1px; }
[data-nx="site-card"] { color: red; }
@media (max-width: 640px) { [data-nx="site-card"] { color: blue; } }`), "sakura")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `[data-theme="sakura"]{--x:1px;}`) {
		t.Fatalf(":root not rewritten to the theme root: %s", got)
	}
	if strings.Count(got, `[data-theme="sakura"] [data-nx="site-card"]`) != 2 {
		t.Fatalf("selectors not scoped inside and outside @media: %s", got)
	}
}

// 根引用要被改写为根自身，而不是根的后代——否则 [data-nx="page-root"]::after
// 会变成 [data-theme=x] [data-nx=page-root]::after，永远匹配不到。
func TestCompileCSSRewritesRootReferenceToRootItself(t *testing.T) {
	out, err := CompileCSS([]byte(`[data-nx="page-root"]::after { content: ""; }`), "terminal")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	if !strings.Contains(string(out), `[data-theme="terminal"]::after`) {
		t.Fatalf("root reference not collapsed onto the root: %s", out)
	}
}

func TestCompileCSSRewritesAssetURLs(t *testing.T) {
	out, err := CompileCSS([]byte(`@font-face { font-family: "S"; src: url("asset:fonts/s.woff2"); }`), "s")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	if !strings.Contains(string(out), AssetBasePlaceholder+"fonts/s.woff2") {
		t.Fatalf("asset url not rewritten: %s", out)
	}
	if strings.Contains(string(out), "asset:") {
		t.Fatalf("asset: scheme left in output: %s", out)
	}
}

func TestCompileCSSRewritesAssetURLsInCustomProperties(t *testing.T) {
	out, err := CompileCSS([]byte(`[data-nx="clock"] { --bg: url("asset:img/a.png"); }`), "s")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	if !strings.Contains(string(out), AssetBasePlaceholder+"img/a.png") {
		t.Fatalf("asset url in custom property not rewritten: %s", out)
	}
}

func TestCompileCSSNamespacesGlobalNames(t *testing.T) {
	out, err := CompileCSS([]byte(`
@keyframes pulse { from { opacity: 0; } to { opacity: 1; } }
[data-nx="clock"] { animation: pulse 2s infinite; }
@font-face { font-family: "Sample Sans"; src: url("asset:fonts/s.woff2"); }
[data-nx="clock"] { font-family: "Sample Sans", sans-serif; }`), "sakura")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	got := string(out)
	for _, want := range []string{
		"@keyframes sakura-pulse",
		"animation:sakura-pulse 2s infinite",
		`font-family:"sakura-Sample Sans"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	// keyframes 内部的 from/to 不加作用域前缀
	if strings.Contains(got, `[data-theme="sakura"] from`) {
		t.Fatalf("keyframe selectors must not be scoped: %s", got)
	}
	// 系统字体不受命名空间化影响
	if !strings.Contains(got, "sans-serif") {
		t.Fatalf("system font stack mangled: %s", got)
	}
}

func TestCompileCSSIsDeterministic(t *testing.T) {
	src := []byte(`[data-nx="clock"] { color: red; }`)
	first, err := CompileCSS(src, "s")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	second, _ := CompileCSS(src, "s")
	if string(first) != string(second) {
		t.Fatal("CompileCSS() is not deterministic")
	}
}

func TestTokensCSSEmitsVariablesAndBaselineFallback(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	out := TokensCSS(m, "sample", nil)
	for _, want := range []string{
		`[data-theme="sample"]`,
		"--font-body: system-ui;",
		"--background-50: 0.99 0.003 12;",
		"--radius-full: 9999px;", // manifest 未提供 radius，由基线补齐
		"--elevation-surface:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("TokensCSS() missing %q in:\n%s", want, out)
		}
	}
}

// 令牌里引用包内字体时必须写命名空间化后的名字，否则 @font-face 改了名
// 而令牌还指着原名，字体就找不到了。
func TestTokensCSSUsesNamespacedFontFamily(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	m.Tokens.Font["body"] = `"Sample Sans", system-ui`
	out := TokensCSS(m, "sakura", map[string]bool{"sample sans": true})
	if !strings.Contains(out, `--font-body: "sakura-Sample Sans", system-ui;`) {
		t.Fatalf("package font not namespaced in tokens:\n%s", out)
	}
}
