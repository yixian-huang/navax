package themes

import (
	"errors"
	"strings"
	"testing"
)

var sampleFonts = []string{"Sample Sans"}

func TestValidateCSSAcceptsRealisticTheme(t *testing.T) {
	src := []byte(`
:root { --radius-lg: 22px; }
[data-nx="site-card"] { border-radius: var(--radius-lg); box-shadow: 0 3px 10px rgb(0 0 0 / 0.08); }
[data-nx="site-card"]:hover { transform: translateY(-2px); }
[data-nx="site-grid"] > [data-nx="site-card"]:nth-child(2) { animation-delay: 40ms; }
[data-nx="page-root"] [data-nx="clock"] { color: red; }
[data-nx="page-root"]::after {
  content: "";
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  background-image: repeating-linear-gradient(0deg, transparent, transparent 3px, oklch(0.70 0.14 145 / 0.04) 3px);
}
@media (max-width: 640px) { [data-nx="site-card"] { border-radius: 14px; } }
@font-face { font-family: "Sample Sans"; src: url("asset:fonts/sample.woff2") format("woff2"); }
@keyframes pulse { from { opacity: 0; } to { opacity: 1; } }
[data-nx="clock"] { animation: pulse 2s infinite; font-family: "Sample Sans", sans-serif; }
`)
	if err := ValidateCSS(src, sampleFonts); err != nil {
		t.Fatalf("ValidateCSS() error = %v, want nil", err)
	}
}

func TestValidateCSSRejects(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{"@import", `@import url("https://evil.example/x.css");`, "@import"},
		{"外部 url()", `[data-nx="clock"] { background-image: url("https://evil.example/p.png"); }`, "url"},
		{"注释绕过的外部 url()", `[data-nx="clock"] { background-image: url(/*x*/"https://evil.example/p.png"); }`, "url"},
		{"@font-face 外链", `@font-face { font-family: "Sample Sans"; src: url("https://evil.example/f.woff2"); }`, "url"},
		{"@font-face 族名未在令牌中引用", `@font-face { font-family: "Ghost"; src: url("asset:fonts/a.woff2"); }`, "font-family"},
		{"选择 body", `body { background: red; }`, "body"},
		{"选择 html", `html { background: red; }`, "html"},
		{"转义的 html", `h\74 ml { background: red; }`, "html"},
		{"根后兄弟逃逸", `[data-nx="page-root"] + footer { display: none; }`, "兄弟"},
		{"根后通用兄弟逃逸", `[data-nx="page-root"] ~ footer { opacity: 0; }`, "兄弟"},
		{"逗号列表中混入越界项", `[data-nx="clock"], [data-nx="page-root"] + footer { color: red; }`, "兄弟"},
		{":is() 嵌套越界", `:is([data-nx="page-root"] + footer) { display: none; }`, "兄弟"},
		{"nth-child of 列表越界", `:nth-child(2 of [data-nx="page-root"] ~ x) { display: none; }`, "兄弟"},
		{"命中宿主 wrapper", `[data-nx-frame] { contain: none; }`, "data-nx-frame"},
		{"命中受保护元素", `[data-nx-protected] { display: none; }`, "data-nx-protected"},
		{"未登记的内部类名", `.material-card { color: red; }`, "material-card"},
		{"Tailwind 原子类属性匹配", `[class*="w-11"] { width: 0; }`, "class"},
		{"转义的 class 属性选择器", `[cl\61 ss*="w-11"] { width: 0; }`, "class"},
		{"未登记的 data-nx 钩子", `[data-nx="admin-panel"] { display: none; }`, "admin-panel"},
		{"命名空间选择器", `*|body { background: red; }`, "命名空间"},
		{"CSS nesting", `[data-nx="clock"] { & + footer { display: none; } }`, "嵌套"},
		{"非空 content", `[data-nx="page-root"]::after { content: "请登录"; }`, "content"},
		{"装饰字符 content", `[data-nx="site-card"]::after { content: "✿"; }`, "content"},
		{"attr() content", `[data-nx="page-root"]::after { content: attr(href); }`, "content"},
		{"fixed 缺 pointer-events", `[data-nx="page-root"]::after { content: ""; position: fixed; inset: 0; }`, "pointer-events"},
		{"z-index 越界", `[data-nx="navbar"] { z-index: 9999; }`, "z-index"},
		{"z-index 非整数", `[data-nx="navbar"] { z-index: var(--z); }`, "z-index"},
		{"未知 at-rule", `@container (min-width: 10px) { [data-nx="clock"] { color: red; } }`, "at-rule"},
		{"@layer 被禁", `@layer theme { [data-nx="clock"] { color: red; } }`, "@layer"},
		{"behavior", `[data-nx="clock"] { behavior: url(x.htc); }`, "behavior"},
		{"-moz-binding", `[data-nx="clock"] { -moz-binding: url(x.xml); }`, "binding"},
		{"expression", `[data-nx="clock"] { width: expression(alert(1)); }`, "函数"},
		{"src() 函数外链", `[data-nx="clock"] { background-image: src("https://evil.example/p.png"); }`, "函数"},
		{"转义的 src()", `[data-nx="clock"] { background-image: \73 rc("https://evil.example/p.png"); }`, "函数"},
		{"image() 函数", `[data-nx="clock"] { background-image: image("https://evil.example/p.png"); }`, "函数"},
		{"cross-fade() 函数", `[data-nx="clock"] { background-image: cross-fade(url("asset:a.png") 50%); }`, "函数"},
		{"element() 函数", `[data-nx="clock"] { background: element(#x); }`, "函数"},
		{"image-set 字符串形式外链", `[data-nx="clock"] { background-image: image-set("https://evil.example/p.png" 1x); }`, "函数"},
		{"-webkit-image-set", `[data-nx="clock"] { background-image: -webkit-image-set(url("asset:a.png") 1x); }`, "函数"},
		{"自定义属性藏外链", `[data-nx="clock"] { --bg: url("https://evil.example/p.png"); }`, "url"},
		{"URL 形态字符串字面量", `[data-nx="clock"] { --bg: "https://evil.example/p.png"; }`, "字符串"},
		{
			"font 简写引用包内字体",
			`@font-face { font-family: "Sample Sans"; src: url("asset:fonts/s.woff2"); }
			 [data-nx="clock"] { font: bold 12px "Sample Sans"; }`,
			"font",
		},
		{"data:image/svg+xml", `[data-nx="clock"] { background-image: url("data:image/svg+xml,%3Csvg%3E%3C/svg%3E"); }`, "svg"},
		{"超大 data: URI", `[data-nx="clock"] { background-image: url("data:image/png;base64,` + strings.Repeat("A", 9000) + `"); }`, "data:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCSS([]byte(tc.src), sampleFonts)
			if err == nil {
				t.Fatal("ValidateCSS() expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidCSS) {
				t.Fatalf("error = %v, want ErrInvalidCSS", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want to mention %q", err, tc.wantMsg)
			}
		})
	}
}

func TestValidateCSSRejectsOversize(t *testing.T) {
	src := []byte(strings.Repeat("a{color:red}\n", MaxCSSBytes))
	if err := ValidateCSS(src, nil); err == nil || !strings.Contains(err.Error(), "体积") {
		t.Fatalf("error = %v, want size rejection", err)
	}
}

func TestDecodeCSSIdent(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"html", "html"},
		{`h\74 ml`, "html"},
		{`h\000074ml`, "html"},
		{`cl\61 ss`, "class"},
		{`\73 rc`, "src"},
		{"HTML", "html"},
		{`a\-b`, "a-b"},
	}
	for _, tc := range tests {
		if got := decodeCSSIdent(tc.raw); got != tc.want {
			t.Fatalf("decodeCSSIdent(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
