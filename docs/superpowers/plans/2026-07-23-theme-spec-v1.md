# 主题规范 v1（子项目 A）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把主题从前端硬编码的 TS 包抽象成服务端校验、版本化入库、内容寻址供应的规范，并让现有 6 个内置主题成为该规范的第一批实现。

**Architecture:** 新增 Go 包 `internal/themes` 承载 manifest 解析、CSS 校验与编译、资产校验、版本存储。三条来源（内置 embed、后续的 GitHub 导入、zip 上传）共用同一条管线：解析 → 校验 → 编译（选择器加作用域、`asset()` 重写）→ 计算 content hash → 幂等落库为不可变版本。浏览器只拿编译产物：公开页通过 `<link rel="stylesheet">` 从内容寻址 URL 取样式，前端不再持有任何 CSS 字符串。

**Tech Stack:** Go 1.25 + chi + modernc.org/sqlite + 新增 `github.com/tdewolff/parse/v2`（纯 Go CSS 解析）；前端 React 19 + Vite + TanStack Query。

**设计依据：** `docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`

## Global Constraints

- 每次提交前必须通过 `make check` 与 `go test -race ./...`；涉及前端构建的任务额外跑 `make build`。
- Conventional Commit 主题行用英文；用户可见文案与文档用中文。
- 迁移文件 append-only，本计划只新增 `migrations/0014_theme_packages.sql`，不得修改任何既有迁移。
- `api/openapi.yaml` 是契约唯一来源；任何端点或响应字段变更必须同步该文件，并由 `tests/contract/` 覆盖。
- `internal/httpapi/` 只做路由、DTO、序列化；业务逻辑与事务边界放 `internal/themes` 与 `internal/navigation`。
- 不引入 ORM、DI 框架、事件总线、Redis、队列、PostgreSQL。本计划唯一新增依赖是 `github.com/tdewolff/parse/v2`。
- 体积上限（写成 `internal/themes` 的导出常量）：CSS ≤ 262144 字节；单个资产 ≤ 524288 字节；整包 ≤ 4194304 字节。
- `z-index` 上限 50；`specVersion` 固定为 1。
- 资产 MIME 白名单：`font/woff2`、`image/png`、`image/jpeg`、`image/webp`。**拒绝 SVG。**
- 主题 CSS 中不得出现外部 URL；仅允许 `asset("…")` 与 `data:image/*`（`data:` 单条 ≤ 8192 字节）。

---

## 文件结构

**新增 Go 包 `internal/themes/`（每个文件一个职责）**

| 文件 | 职责 |
|---|---|
| `manifest.go` | `theme.json` 的类型、解析、令牌校验 |
| `tokens.go` | 令牌 → CSS 变量块，含基线回落值 |
| `hooks.go` | `data-nx` 稳定选择器白名单（与 `docs/theme-api.md` 一一对应） |
| `cssvalidate.go` | CSS 拒绝规则（解析驱动） |
| `csscompile.go` | 选择器加作用域、`asset()` 重写、规范化输出 |
| `assets.go` | 资产 magic bytes 与体积校验 |
| `compile.go` | 串联上述步骤，产出 `Compiled`（含 content hash 与版本 ID） |
| `store.go` | `theme_versions` / `theme_assets` 的读写、幂等 upsert、themeId 解析回落 |
| `builtin/` | 6 个内置主题的 `theme.json` / `theme.css`，`//go:embed` |
| `builtin.go` | 启动时加载并 upsert 内置主题 |

**修改**

- `internal/httpapi/themes.go`（新增）：公开 CSS 与资产端点
- `internal/httpapi/catalog.go`、`internal/httpapi/admin.go`：主题列表扩展字段
- `internal/navigation/{types,sqlstore}.go`：发布锁版本
- `internal/app/run.go`：接线
- `api/openapi.yaml`、`tests/contract/`、`tests/e2e/`
- `web/src/themes/{types,registry}.ts`、`web/src/api/`、三处主题 UI 页面
- 删除 `web/src/themes/packages/`、`web/src/themes/manifest.ts`、`web/src/lib/themeResolve.ts`

**本计划不覆盖（留给子项目 B/C）**

- `preview.png` 的入库与供应：本阶段内置主题继续沿用 `themes.preview` 列的既有值，包内预览图的读取随导入管线一起做。
- GitHub / zip 导入、私有安装、目录审核、版本 kill switch、`POST /api/v1/themes/validate`。
- tier 2 声明式布局与公开页 slot 化。

---

### Task 1: `internal/themes` 骨架与 manifest 解析

**Files:**
- Create: `internal/themes/manifest.go`
- Create: `internal/themes/manifest_test.go`
- Modify: `go.mod`、`go.sum`

**Interfaces:**
- Consumes: 无
- Produces:
  - `type Manifest struct` 及字段 `SpecVersion int`、`ID string`、`Name string`、`Version string`、`Author string`、`License string`、`Homepage string`、`Mode string`、`Vibe string`、`Swatches [3]string`、`Tier int`、`Tokens Tokens`
  - `type Tokens struct { Font map[string]string; Radius map[string]string; Elevation map[string]string; Color map[string]map[string]string }`
  - `func ParseManifest(data []byte) (Manifest, error)`
  - `var ErrInvalidManifest error`

- [ ] **Step 1: 写失败测试**

创建 `internal/themes/manifest_test.go`：

```go
package themes

import (
	"errors"
	"strings"
	"testing"
)

const minimalManifest = `{
  "specVersion": 1,
  "id": "sample",
  "name": "Sample",
  "version": "1.0.0",
  "author": "nav.ax",
  "mode": "light",
  "vibe": "serious",
  "swatches": ["#ffffff", "#888888", "#111111"],
  "tier": 1,
  "tokens": {
    "font": {"heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace"},
    "color": {
      "background": {"50": "0.99 0.003 12"},
      "foreground": {"900": "0.15 0.008 12"},
      "primary": {"500": "0.55 0.12 250"},
      "accent": {"500": "0.70 0.14 145"}
    }
  }
}`

func TestParseManifestAcceptsMinimal(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.ID != "sample" || m.Tier != 1 || m.Tokens.Font["body"] != "system-ui" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestParseManifestRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantMsg string
	}{
		{"未知 specVersion", func(s string) string { return strings.Replace(s, `"specVersion": 1`, `"specVersion": 2`, 1) }, "specVersion"},
		{"非法 id", func(s string) string { return strings.Replace(s, `"id": "sample"`, `"id": "Sample_1"`, 1) }, "id"},
		{"非法 mode", func(s string) string { return strings.Replace(s, `"mode": "light"`, `"mode": "neon"`, 1) }, "mode"},
		{"tier 越界", func(s string) string { return strings.Replace(s, `"tier": 1`, `"tier": 4`, 1) }, "tier"},
		{"色值非 OKLCH 三通道", func(s string) string {
			return strings.Replace(s, `"50": "0.99 0.003 12"`, `"50": "#ffffff"`, 1)
		}, "color"},
		{"缺必填字体族", func(s string) string {
			return strings.Replace(s, `"mono": "monospace"`, `"mono": ""`, 1)
		}, "font"},
		{"缺必填颜色组", func(s string) string {
			return strings.Replace(s, `"accent": {"500": "0.70 0.14 145"}`, `"accent": {}`, 1)
		}, "accent"},
		{"字体族含注入字符", func(s string) string {
			return strings.Replace(s, `"body": "system-ui"`, `"body": "a;}body{display:none"`, 1)
		}, "font"},
		{"swatch 非 hex", func(s string) string {
			return strings.Replace(s, `"#888888"`, `"rgb(1,2,3)"`, 1)
		}, "swatches"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tc.mutate(minimalManifest)))
			if err == nil {
				t.Fatal("ParseManifest() expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("error = %v, want ErrInvalidManifest", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want to mention %q", err, tc.wantMsg)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/themes -run TestParseManifest -v`
Expected: FAIL，`undefined: ParseManifest`。

- [ ] **Step 3: 实现 manifest.go**

创建 `internal/themes/manifest.go`：

```go
// Package themes 承载主题包的解析、校验、编译与版本化存储。
// 它是主题内容进入实例的唯一信任边界：浏览器只拿编译产物。
package themes

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidManifest 包裹所有 manifest 层面的校验失败。
var ErrInvalidManifest = errors.New("invalid theme manifest")

// SpecVersion 是本实现支持的规范版本。
const SpecVersion = 1

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)
	semverPattern   = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	oklchPattern    = regexp.MustCompile(`^\d(\.\d+)? \d(\.\d+)? \d{1,3}(\.\d+)?$`)
	hexPattern      = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontStackChars  = regexp.MustCompile(`^[A-Za-z0-9 ,'"\-_]+$`)
	lengthPattern   = regexp.MustCompile(`^[0-9a-zA-Z.%\- ()/]+$`)
	requiredFonts   = []string{"heading", "body", "label", "mono"}
	requiredColors  = []string{"background", "foreground", "primary", "accent"}
	allowedModes    = map[string]bool{"light": true, "dark": true, "both": true}
	allowedVibes    = map[string]bool{"serious": true, "cute": true}
)

type Tokens struct {
	Font      map[string]string            `json:"font"`
	Radius    map[string]string            `json:"radius,omitempty"`
	Elevation map[string]string            `json:"elevation,omitempty"`
	Color     map[string]map[string]string `json:"color"`
}

type Manifest struct {
	SpecVersion int       `json:"specVersion"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Author      string    `json:"author"`
	License     string    `json:"license,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	Mode        string    `json:"mode"`
	Vibe        string    `json:"vibe"`
	Swatches    [3]string `json:"swatches"`
	Tier        int       `json:"tier"`
	Tokens      Tokens    `json:"tokens"`
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
}

// ParseManifest 解析并全量校验 theme.json。
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	decoder := json.NewDecoder(newLimitedReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, invalid("json 解析失败: %v", err)
	}
	if m.SpecVersion != SpecVersion {
		return Manifest{}, invalid("specVersion 必须为 %d", SpecVersion)
	}
	if !slugPattern.MatchString(m.ID) {
		return Manifest{}, invalid("id 不符合 ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$")
	}
	if m.Name == "" || len(m.Name) > 100 {
		return Manifest{}, invalid("name 长度必须在 1..100")
	}
	if !semverPattern.MatchString(m.Version) {
		return Manifest{}, invalid("version 必须是语义化版本")
	}
	if m.Author == "" || len(m.Author) > 100 {
		return Manifest{}, invalid("author 长度必须在 1..100")
	}
	if !allowedModes[m.Mode] {
		return Manifest{}, invalid("mode 必须是 light|dark|both")
	}
	if !allowedVibes[m.Vibe] {
		return Manifest{}, invalid("vibe 必须是 serious|cute")
	}
	if m.Tier < 1 || m.Tier > 3 {
		return Manifest{}, invalid("tier 必须在 1..3")
	}
	for _, swatch := range m.Swatches {
		if !hexPattern.MatchString(swatch) {
			return Manifest{}, invalid("swatches 必须是 #rrggbb")
		}
	}
	if err := validateTokens(m.Tokens); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func validateTokens(tokens Tokens) error {
	for _, key := range requiredFonts {
		value := tokens.Font[key]
		if value == "" || !fontStackChars.MatchString(value) {
			return invalid("font.%s 缺失或含非法字符", key)
		}
	}
	for _, group := range requiredColors {
		values := tokens.Color[group]
		if len(values) == 0 {
			return invalid("color.%s 至少需要一个档位", group)
		}
		for step, value := range values {
			if !oklchPattern.MatchString(value) {
				return invalid("color.%s.%s 必须是 OKLCH 三通道，如 \"0.55 0.12 250\"", group, step)
			}
		}
	}
	for name, table := range map[string]map[string]string{"radius": tokens.Radius, "elevation": tokens.Elevation} {
		for step, value := range table {
			if value == "" || !lengthPattern.MatchString(value) {
				if name == "elevation" {
					continue // elevation 允许逗号与 oklch()，交由 CSS 校验器兜底
				}
				return invalid("%s.%s 含非法字符", name, step)
			}
		}
	}
	return nil
}
```

同时在 `manifest.go` 末尾加上 `newLimitedReader`，限制 manifest 体积：

```go
import "bytes"

const maxManifestBytes = 64 * 1024

func newLimitedReader(data []byte) *bytes.Reader {
	if len(data) > maxManifestBytes {
		data = data[:maxManifestBytes] // 超限时必然解析失败，错误信息由 Decode 给出
	}
	return bytes.NewReader(data)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/themes -run TestParseManifest -v`
Expected: PASS，全部子用例通过。

- [ ] **Step 5: 提交**

```bash
gofmt -w internal/themes
go vet ./internal/themes
git add internal/themes go.mod go.sum
git commit -m "feat: add theme manifest parsing and token validation"
```

---

### Task 2: `data-nx` 稳定选择器契约

**Files:**
- Create: `docs/theme-api.md`
- Create: `internal/themes/hooks.go`
- Create: `internal/themes/hooks_test.go`
- Modify: `web/src/components/feature/PublicShell.tsx`（公开页外壳、页脚源码链接）
- Modify: 公开页渲染站点卡片与分类的组件（用 `rg 'material-card|hairline' web/src` 定位全部使用点）

**Interfaces:**
- Consumes: 无
- Produces:
  - `func AllowedHooks() []string` — 已登记的 `data-nx` 值，按字典序
  - `func IsAllowedHook(name string) bool`
  - `const ProtectedAttr = "data-nx-protected"`

- [ ] **Step 1: 盘点现有主题实际用到的选择器**

Run: `rg -o '\.[a-z-]+' web/src/themes/packages/*.ts | sort -u`
把结果记进 `docs/theme-api.md` 的「迁移映射」小节；每个内部类名必须映射到一个 `data-nx` 钩子，否则内置主题迁移会丢效果。

- [ ] **Step 2: 写失败测试**

创建 `internal/themes/hooks_test.go`：

```go
package themes

import "testing"

func TestAllowedHooksAreSortedAndUnique(t *testing.T) {
	hooks := AllowedHooks()
	if len(hooks) == 0 {
		t.Fatal("AllowedHooks() is empty")
	}
	seen := map[string]bool{}
	for i, hook := range hooks {
		if seen[hook] {
			t.Fatalf("duplicate hook %q", hook)
		}
		seen[hook] = true
		if i > 0 && hooks[i-1] >= hook {
			t.Fatalf("hooks not sorted at %d: %q >= %q", i, hooks[i-1], hook)
		}
	}
}

func TestIsAllowedHook(t *testing.T) {
	if !IsAllowedHook("site-card") {
		t.Fatal("site-card should be an allowed hook")
	}
	if IsAllowedHook("material-card") {
		t.Fatal("internal class names must not be hooks")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/themes -run TestAllowedHooks -v`
Expected: FAIL，`undefined: AllowedHooks`。

- [ ] **Step 4: 实现 hooks.go**

```go
package themes

import "sort"

// ProtectedAttr 标记必须始终可见的元素（AGPL §13 源码链接等）。
// 任何命中它的主题规则一律拒绝。
const ProtectedAttr = "data-nx-protected"

// allowedHooks 是主题可以选择的稳定钩子。新增前先更新 docs/theme-api.md。
var allowedHooks = []string{
	"category-tab",
	"clock",
	"footer",
	"navbar",
	"page-root",
	"search-box",
	"section-title",
	"site-card",
	"site-card-icon",
	"site-card-title",
}

func init() { sort.Strings(allowedHooks) }

// AllowedHooks 返回已登记的 data-nx 值，按字典序。
func AllowedHooks() []string {
	out := make([]string, len(allowedHooks))
	copy(out, allowedHooks)
	return out
}

// IsAllowedHook 判断 data-nx 值是否已登记。
func IsAllowedHook(name string) bool {
	index := sort.SearchStrings(allowedHooks, name)
	return index < len(allowedHooks) && allowedHooks[index] == name
}
```

**注意：** 上面的钩子清单是初版骨架。Step 1 的盘点结果若发现现有主题依赖了别的视觉元素（如分隔线 `hairline`），必须在此补齐对应钩子，并在 `docs/theme-api.md` 记录，否则 Task 7 的内置主题迁移会视觉退化。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/themes -run 'TestAllowedHooks|TestIsAllowedHook' -v`
Expected: PASS。

- [ ] **Step 6: 前端组件挂钩子**

给公开页渲染路径上的元素补 `data-nx` 属性（保留现有 class，不破坏 Tailwind 样式）。例：

```tsx
<article data-nx="site-card" className="material-card …">
```

在 `PublicShell.tsx` 的页脚源码链接上加 `data-nx-protected`：

```tsx
<a data-nx-protected href={SOURCE_REPO_URL} …>
```

并在公开页样式入口加一条兜底规则（文件：`web/src/index.css` 或等价的全局样式入口）：

```css
[data-nx-protected] {
  display: revert !important;
  visibility: visible !important;
  opacity: 1 !important;
  pointer-events: auto !important;
}
```

- [ ] **Step 7: 写 docs/theme-api.md**

至少包含：钩子清单表（钩子名 / 出现位置 / stable 或 experimental）、`data-nx-protected` 的含义与不可覆盖性、Step 1 的内部类名 → 钩子迁移映射、以及「experimental 钩子可能在小版本变更」的声明。

- [ ] **Step 8: 验证与提交**

```bash
make check
go test -race ./internal/themes
git add docs/theme-api.md internal/themes web/src
git commit -m "feat: add stable data-nx theme hook contract"
```

---

### Task 3: CSS 校验器（拒绝规则）

**Files:**
- Create: `internal/themes/cssvalidate.go`
- Create: `internal/themes/cssvalidate_test.go`
- Modify: `go.mod`、`go.sum`（引入 `github.com/tdewolff/parse/v2`）

**Interfaces:**
- Consumes: Task 2 的 `IsAllowedHook`、`ProtectedAttr`
- Produces:
  - `func ValidateCSS(src []byte, fontFamilies []string) error` — `fontFamilies` 是 manifest `tokens.font` 中出现的字体族名，用于 `@font-face` 交叉检查
  - `var ErrInvalidCSS error`
  - `const MaxCSSBytes = 262144`

- [ ] **Step 1: 先用一个 spike 测试钉住解析器 API**

引入依赖：

```bash
go get github.com/tdewolff/parse/v2@latest
```

创建 `internal/themes/cssvalidate_test.go`，第一个测试只验证解析器行为符合预期（后续规则实现依赖这个形状）：

```go
package themes

import (
	"testing"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func TestCSSParserWalksRulesAndDeclarations(t *testing.T) {
	p := css.NewParser(parse.NewInputString(`a { color: red; }`), false)
	var sawRuleset, sawDeclaration bool
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorToken {
			break
		}
		switch gt {
		case css.BeginRulesetGrammar:
			sawRuleset = true
		case css.DeclarationGrammar:
			if string(data) == "color" {
				sawDeclaration = true
			}
		}
	}
	if !sawRuleset || !sawDeclaration {
		t.Fatalf("parser walk failed: ruleset=%v declaration=%v", sawRuleset, sawDeclaration)
	}
}
```

Run: `go test ./internal/themes -run TestCSSParserWalks -v`
若 API 形状与此不符（grammar 常量名、`p.Values()` 用法），先跑 `go doc github.com/tdewolff/parse/v2/css` 修正本测试，再继续——后续所有规则实现都以修正后的形状为准。

- [ ] **Step 2: 写规则拒绝的表驱动失败测试**

追加到 `internal/themes/cssvalidate_test.go`：

```go
func TestValidateCSSAcceptsRealisticTheme(t *testing.T) {
	src := []byte(`
:root { --radius-lg: 22px; }
[data-nx="site-card"] { border-radius: var(--radius-lg); box-shadow: 0 3px 10px rgb(0 0 0 / 0.08); }
[data-nx="site-card"]:hover { transform: translateY(-2px); }
body::after {
  content: "";
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  background-image: repeating-linear-gradient(0deg, transparent, transparent 3px, oklch(0.70 0.14 145 / 0.04) 3px);
}
@media (max-width: 640px) { [data-nx="site-card"] { border-radius: 14px; } }
@font-face { font-family: "Sample Sans"; src: asset("fonts/sample.woff2") format("woff2"); }
`)
	if err := ValidateCSS(src, []string{"Sample Sans"}); err != nil {
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
		{"外部 url()", `body { background-image: url("https://evil.example/p.png"); }`, "url"},
		{"注释绕过的外部 url()", `body { background-image: url(/*x*/"https://evil.example/p.png"); }`, "url"},
		{"@font-face 外链", `@font-face { font-family: "X"; src: url("https://evil.example/f.woff2"); }`, "font-face"},
		{"@font-face 族名未在令牌中引用", `@font-face { font-family: "Ghost"; src: asset("fonts/a.woff2"); }`, "font-family"},
		{"非空 content", `body::after { content: "请登录"; }`, "content"},
		{"attr() content", `body::after { content: attr(href); }`, "content"},
		{"fixed 缺 pointer-events", `body::after { content: ""; position: fixed; inset: 0; }`, "pointer-events"},
		{"z-index 越界", `[data-nx="navbar"] { z-index: 9999; }`, "z-index"},
		{"未知 at-rule", `@container (min-width: 10px) { body { color: red; } }`, "at-rule"},
		{"behavior", `body { behavior: url(x.htc); }`, "behavior"},
		{"-moz-binding", `body { -moz-binding: url(x.xml); }`, "binding"},
		{"expression", `body { width: expression(alert(1)); }`, "expression"},
		{"命中受保护元素", `[data-nx-protected] { display: none; }`, "data-nx-protected"},
		{"未登记的内部类名", `.material-card { color: red; }`, "material-card"},
		{"超大 data: URI", `body { background-image: url("data:image/png;base64,` + strings.Repeat("A", 9000) + `"); }`, "data:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCSS([]byte(tc.src), []string{"Sample Sans"})
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
```

测试文件顶部的 import 需补上 `errors` 与 `strings`。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/themes -run TestValidateCSS -v`
Expected: FAIL，`undefined: ValidateCSS`。

- [ ] **Step 4: 实现 cssvalidate.go**

实现要点（逐条对应上面的用例，全部基于解析出的 token，不用正则匹配原始字符串）：

1. 入口先判体积：`len(src) > MaxCSSBytes` → 报「CSS 体积超过上限」。
2. 用 `css.NewParser(parse.NewInputString(string(src)), false)` 遍历。
3. `AtRuleGrammar` / `BeginAtRuleGrammar`：at-rule 名小写后必须落在 `{"media","supports","keyframes","font-face","layer"}`，否则报 `不支持的 at-rule`。`@import` 单独给出更明确的报错文案。
4. `BeginRulesetGrammar` / `QualifiedRuleGrammar`：取 `p.Values()` 组成选择器文本并解析——
   - 含 `data-nx-protected` → 拒绝；
   - 含类选择器且该类名不是主题私有前缀（约定 `nx-` 之外的类一律拒绝）→ 拒绝并回显类名；
   - `[data-nx="x"]` 中的 `x` 必须 `IsAllowedHook`。
5. `DeclarationGrammar`：`data` 是属性名，`p.Values()` 是值 token 序列——
   - 属性名命中 `behavior` / `-moz-binding` → 拒绝；值中出现 `expression(` 函数 token → 拒绝；
   - `content`：值必须是空字符串字面量或 `none`，其余（含 `attr()`）一律拒绝；
   - `z-index`：解析为整数且 ≤ 50，非整数值（如 `var(...)`）一律拒绝；
   - `url` 函数 token：参数必须是 `data:image/` 开头且总长 ≤ 8192，否则拒绝；`asset(` 函数在此阶段视为合法并记录其参数（供 Task 4 重写与资产存在性检查）。
6. 在当前 ruleset 内累积声明，ruleset 结束（`EndRulesetGrammar`）时做跨声明检查：出现 `position: fixed` 而没有 `pointer-events: none` → 拒绝。
7. `@font-face` 块：`src` 只允许 `asset(...)`；`font-family` 的族名必须出现在传入的 `fontFamilies` 中（去引号、去首尾空格后比较）。

所有拒绝路径统一走 `fmt.Errorf("%w: …", ErrInvalidCSS, …)`，错误文案里带上触发的选择器或属性，便于主题作者定位。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/themes -run TestValidateCSS -v`
Expected: PASS，全部子用例通过。

- [ ] **Step 6: 提交**

```bash
gofmt -w internal/themes && go vet ./internal/themes
go test -race ./internal/themes
git add internal/themes go.mod go.sum
git commit -m "feat: add theme CSS validator with parser-driven rejection rules"
```

---

### Task 4: CSS 编译器（作用域改写 + asset 重写 + 令牌块）

**Files:**
- Create: `internal/themes/csscompile.go`
- Create: `internal/themes/tokens.go`
- Create: `internal/themes/csscompile_test.go`
- Create: `internal/themes/tokens_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Manifest`/`Tokens`，Task 3 的 `ValidateCSS`
- Produces:
  - `func TokensCSS(m Manifest, scope string) string` — 生成 `[data-theme="<scope>"] { --… }` 变量块，缺失的 `radius`/`elevation` 用基线值补齐
  - `func CompileCSS(src []byte, scope string) ([]byte, error)` — 选择器加作用域、`asset("p")` → `AssetBasePlaceholder + "p"`
  - `const AssetBasePlaceholder = "__NAVAX_THEME_ASSET_BASE__/"`

- [ ] **Step 1: 写失败测试**

`internal/themes/csscompile_test.go`：

```go
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
	if !strings.Contains(got, `[data-theme="sakura"]{--x:1px}`) &&
		!strings.Contains(got, `[data-theme="sakura"] {`) {
		t.Fatalf(":root not scoped: %s", got)
	}
	if strings.Count(got, `[data-theme="sakura"] [data-nx="site-card"]`) != 2 {
		t.Fatalf("selectors not scoped inside and outside @media: %s", got)
	}
}

func TestCompileCSSRewritesAssetCalls(t *testing.T) {
	out, err := CompileCSS([]byte(`@font-face { font-family: "S"; src: asset("fonts/s.woff2"); }`), "s")
	if err != nil {
		t.Fatalf("CompileCSS() error = %v", err)
	}
	if !strings.Contains(string(out), AssetBasePlaceholder+"fonts/s.woff2") {
		t.Fatalf("asset() not rewritten: %s", out)
	}
	if strings.Contains(string(out), "asset(") {
		t.Fatalf("asset() call left in output: %s", out)
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
```

`internal/themes/tokens_test.go`：

```go
package themes

import (
	"strings"
	"testing"
)

func TestTokensCSSEmitsVariablesAndBaselineFallback(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	out := TokensCSS(m, "sample")
	for _, want := range []string{
		`[data-theme="sample"]`,
		"--font-body: system-ui;",
		"--background-50: 0.99 0.003 12;",
		"--radius-lg:", // manifest 未提供 radius，必须由基线补齐
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("TokensCSS() missing %q in:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/themes -run 'TestCompileCSS|TestTokensCSS' -v`
Expected: FAIL，`undefined: CompileCSS` / `undefined: TokensCSS`。

- [ ] **Step 3: 实现 tokens.go**

基线值直接取自现有 `web/src/themes/packages/slate.ts` 的 `--radius-*` 与 `--elevation-*`，逐条抄写为 Go 常量表 `baselineRadius` / `baselineElevation`。`TokensCSS` 按固定顺序（字体 → radius → elevation → color，组内按键名字典序）输出，保证确定性：

```go
func TokensCSS(m Manifest, scope string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[data-theme=%q] {\n", scope)
	writeSorted(&b, "--font-", m.Tokens.Font, nil)
	writeSorted(&b, "--radius-", m.Tokens.Radius, baselineRadius)
	writeSorted(&b, "--elevation-", m.Tokens.Elevation, baselineElevation)
	for _, group := range sortedKeys(m.Tokens.Color) {
		writeSorted(&b, "--"+group+"-", m.Tokens.Color[group], nil)
	}
	b.WriteString("}\n")
	return b.String()
}
```

`writeSorted` 先写 fallback 表中未被覆盖的键，再写 manifest 提供的键，全部按键名排序。

- [ ] **Step 4: 实现 csscompile.go**

复用 Task 3 的解析遍历，改为输出模式：

- 每个 ruleset 的选择器列表逐个前缀化：`:root` → `[data-theme="<scope>"]`；其余 `sel` → `[data-theme="<scope>"] sel`；逗号分隔的多选择器分别处理。
- `@media` / `@supports` / `@layer` 内部的 ruleset 同样前缀化（这是测试里 `strings.Count(...) != 2` 要覆盖的点）。
- `@keyframes` 内部的 `from`/`to`/百分比选择器**不**加前缀；`@keyframes` 名加 `<scope>-` 前缀，并同步改写引用它的 `animation-name` / `animation` 值，防跨主题串扰。
- `@font-face` 不加前缀。
- `asset("p")` → `url("<AssetBasePlaceholder>p")`。
- 输出规范化（统一缩进与分号），保证同输入逐字节一致。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/themes -run 'TestCompileCSS|TestTokensCSS' -v`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
gofmt -w internal/themes && go vet ./internal/themes
go test -race ./internal/themes
git add internal/themes
git commit -m "feat: compile theme CSS with scope prefixing and asset rewriting"
```

---

### Task 5: 资产校验与包编译串联

**Files:**
- Create: `internal/themes/assets.go`
- Create: `internal/themes/compile.go`
- Create: `internal/themes/assets_test.go`
- Create: `internal/themes/compile_test.go`
- Create: `internal/themes/testdata/`（最小合法 woff2/png 夹具）

**Interfaces:**
- Consumes: Task 1、3、4 的全部导出
- Produces:
  - `type Asset struct { Path, MIME string; Data []byte; SHA256 string }`
  - `type Package struct { Manifest Manifest; CSS []byte; Assets []Asset }`
  - `type Compiled struct { VersionID, ContentHash string; Manifest Manifest; CSS []byte; Assets []Asset }`
  - `func ValidateAsset(path string, data []byte) (Asset, error)`
  - `func Compile(pkg Package, packageID string) (Compiled, error)`
  - `const MaxAssetBytes = 524288`、`const MaxPackageBytes = 4194304`

- [ ] **Step 1: 写失败测试**

`internal/themes/assets_test.go` 覆盖：woff2 magic（`wOF2`）通过；png 通过；扩展名与内容不符拒绝；SVG 拒绝；超 `MaxAssetBytes` 拒绝；路径含 `..` 或以 `/` 开头拒绝；路径不在 `assets/` 下拒绝。

`internal/themes/compile_test.go` 覆盖：

```go
func TestCompileProducesStableVersionID(t *testing.T) {
	pkg := samplePackage(t) // 从 testdata 组装
	first, err := Compile(pkg, "sample")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	second, _ := Compile(pkg, "sample")
	if first.VersionID != second.VersionID || first.ContentHash != second.ContentHash {
		t.Fatalf("Compile() not deterministic: %q vs %q", first.VersionID, second.VersionID)
	}
	if !strings.HasPrefix(first.VersionID, "v") || len(first.VersionID) != 33 {
		t.Fatalf("unexpected version id %q", first.VersionID)
	}
}

func TestCompileRejectsCSSReferencingMissingAsset(t *testing.T) {
	pkg := samplePackage(t)
	pkg.CSS = []byte(`@font-face { font-family: "Sample Sans"; src: asset("fonts/missing.woff2"); }`)
	if _, err := Compile(pkg, "sample"); err == nil || !strings.Contains(err.Error(), "missing.woff2") {
		t.Fatalf("error = %v, want missing asset rejection", err)
	}
}

func TestCompileRejectsOversizePackage(t *testing.T) {
	pkg := samplePackage(t)
	blob := make([]byte, MaxAssetBytes)
	copy(blob, []byte("wOF2"))
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
```

`samplePackage` 辅助函数放在同文件底部，从 `internal/themes/testdata/` 读取夹具：

```go
func samplePackage(t *testing.T) Package {
	t.Helper()
	manifest, err := ParseManifest([]byte(minimalManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	manifest.Tokens.Font["body"] = "Sample Sans"
	font, err := os.ReadFile("testdata/sample.woff2")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	asset, err := ValidateAsset("fonts/sample.woff2", font)
	if err != nil {
		t.Fatalf("ValidateAsset() error = %v", err)
	}
	return Package{
		Manifest: manifest,
		CSS:      []byte(`@font-face { font-family: "Sample Sans"; src: asset("fonts/sample.woff2"); }`),
		Assets:   []Asset{asset},
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/themes -run 'TestValidateAsset|TestCompile' -v`
Expected: FAIL，`undefined: ValidateAsset` / `undefined: Compile`。

- [ ] **Step 3: 实现 assets.go 与 compile.go**

`Compile` 的顺序（每一步失败即返回，错误可直接展示给主题作者）：

1. 校验资产总体积 ≤ `MaxPackageBytes`，逐个 `ValidateAsset`。
2. `ValidateCSS(pkg.CSS, fontFamilies(pkg.Manifest))`。
3. 交叉检查：CSS 中出现的每个 `asset("p")` 都必须在 `pkg.Assets` 中有 `Path == p`，否则报错并回显路径。
4. `TokensCSS(manifest, packageID) + CompileCSS(pkg.CSS, packageID)` 拼接为最终 CSS（令牌块在前，保证主题 CSS 能覆盖令牌）。
5. `ContentHash = sha256(canonicalJSON(manifest) || finalCSS || 每个资产的 "path\x00sha256\n"（按 path 排序）)`，十六进制。
6. `VersionID = "v" + ContentHash[:32]`。
7. 把最终 CSS 里的 `AssetBasePlaceholder` 替换成 `/api/v1/public/themes/<VersionID>/assets/`，替换后的结果放进 `Compiled.CSS`（哈希基于替换前的形式，因此 ID 不自我依赖）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/themes -run 'TestValidateAsset|TestCompile' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
gofmt -w internal/themes && go vet ./internal/themes
go test -race ./internal/themes
git add internal/themes
git commit -m "feat: validate theme assets and compile packages to immutable versions"
```

---

### Task 6: 迁移与版本存储

**Files:**
- Create: `migrations/0014_theme_packages.sql`
- Create: `internal/themes/store.go`
- Create: `internal/themes/store_test.go`

**Interfaces:**
- Consumes: Task 5 的 `Compiled`
- Produces:
  - `type Store struct` + `func NewStore(db *sql.DB) *Store`
  - `func (s *Store) UpsertVersion(ctx context.Context, packageID string, compiled Compiled, sourceType, sourceRef string, now time.Time) (string, error)` — 返回 versionID，按 `content_hash` 幂等
  - `func (s *Store) VersionCSS(ctx context.Context, versionID string) ([]byte, string, error)` — 返回 CSS 与 content hash
  - `func (s *Store) VersionAsset(ctx context.Context, versionID, path string) (Asset, error)`
  - `func (s *Store) ResolvePackageVersion(ctx context.Context, themeID string) (string, error)` — themeID → current_version_id，未知则回落默认主题

- [ ] **Step 1: 写迁移**

`migrations/0014_theme_packages.sql`，内容严格照 spec §7.1（含 `UPDATE themes SET slug = id WHERE slug = '';` 必须在两个唯一索引之前执行）。

- [ ] **Step 2: 写失败测试**

`internal/themes/store_test.go`（SQLite 集成测试，参照 `internal/admin/sqlstore_test.go` 的建库方式）：

```go
func TestUpsertVersionIsIdempotent(t *testing.T) {
	db := newTestDB(t) // 建库 + 跑 migrations，写法照抄 internal/admin/sqlstore_test.go
	store := NewStore(db)
	compiled, err := Compile(samplePackage(t), "slate")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

	first, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", now)
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}
	second, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", now)
	if err != nil {
		t.Fatalf("UpsertVersion() second call error = %v", err)
	}
	if first != second {
		t.Fatalf("version id changed: %q → %q", first, second)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM theme_versions WHERE theme_id = 'slate'`).Scan(&count); err != nil {
		t.Fatalf("count error = %v", err)
	}
	if count != 1 {
		t.Fatalf("theme_versions rows = %d, want 1", count)
	}
}

func TestUpsertVersionStoresAssets(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	pkg := samplePackage(t)
	compiled, err := Compile(pkg, "slate")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	versionID, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}

	asset, err := store.VersionAsset(t.Context(), versionID, "fonts/sample.woff2")
	if err != nil {
		t.Fatalf("VersionAsset() error = %v", err)
	}
	if asset.MIME != "font/woff2" || !bytes.Equal(asset.Data, pkg.Assets[0].Data) {
		t.Fatalf("asset roundtrip mismatch: mime=%q bytes=%d", asset.MIME, len(asset.Data))
	}
	if _, err := store.VersionAsset(t.Context(), versionID, "fonts/absent.woff2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestResolvePackageVersionFallsBack(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	compiled, err := Compile(samplePackage(t), "slate")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	slateVersion, err := store.UpsertVersion(t.Context(), "slate", compiled, "builtin", "builtin", time.Now().UTC())
	if err != nil {
		t.Fatalf("UpsertVersion() error = %v", err)
	}

	tests := []struct {
		name    string
		themeID string
		want    string
	}{
		{"已知主题", "slate", slateVersion},
		{"未知主题回落默认", "does-not-exist", slateVersion},
		{"culled 别名回落", "kyoto", slateVersion},
		{"空 themeId 回落默认", "", slateVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.ResolvePackageVersion(t.Context(), tc.themeID)
			if err != nil {
				t.Fatalf("ResolvePackageVersion() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}
```

`ErrNotFound`、`NewStore` 与 `newTestDB` 辅助函数一并在本任务中定义（`newTestDB` 建临时 SQLite 库并执行 `migrations/*.sql`，写法照抄 `internal/admin/sqlstore_test.go`）。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/themes -run TestUpsertVersion -v`
Expected: FAIL，`undefined: NewStore`。

- [ ] **Step 4: 实现 store.go**

`UpsertVersion` 在单个短事务内：`INSERT … ON CONFLICT (theme_id, content_hash) DO NOTHING` → 查回 versionID → 若为新插入则写 `theme_assets` → `UPDATE themes SET current_version_id = ?, updated_at = ?`。别名表（`kyoto → slate` 等，照搬 `web/src/lib/themeResolve.ts` 的 `THEME_ID_ALIASES`）作为 Go 常量 map 放在 `store.go` 顶部。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -race ./internal/themes`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
gofmt -w internal/themes && go vet ./...
go test -race ./...
git add migrations internal/themes
git commit -m "feat: persist immutable theme versions and assets"
```

---

### Task 7: 内置主题迁移为规范格式

**Files:**
- Create: `internal/themes/builtin/<id>/theme.json` 与 `theme.css`（6 个：slate、slate-dark、noir、sakura、orbit、terminal）
- Create: `internal/themes/builtin.go`
- Create: `internal/themes/builtin_test.go`
- Modify: `internal/app/run.go`
- Delete: `web/src/themes/packages/`（整目录）、`web/src/themes/manifest.ts`

**Interfaces:**
- Consumes: Task 5 的 `Compile`、Task 6 的 `Store`
- Produces:
  - `func BuiltinPackages() ([]Package, error)` — 从 embed FS 解析
  - `func SyncBuiltin(ctx context.Context, store *Store, now time.Time) error`

- [ ] **Step 1: 写「所有内置主题必须通过校验器」的失败测试**

```go
func TestBuiltinPackagesCompile(t *testing.T) {
	pkgs, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	if len(pkgs) != 6 {
		t.Fatalf("got %d builtin packages, want 6", len(pkgs))
	}
	for _, pkg := range pkgs {
		t.Run(pkg.Manifest.ID, func(t *testing.T) {
			if _, err := Compile(pkg, pkg.Manifest.ID); err != nil {
				t.Fatalf("Compile(%s) error = %v", pkg.Manifest.ID, err)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/themes -run TestBuiltinPackages -v`
Expected: FAIL，`undefined: BuiltinPackages`。

- [ ] **Step 3: 逐个迁移主题**

对每个 `web/src/themes/packages/<id>.ts`：

- `meta` → `theme.json` 的 `name/subtitle/description/swatches/vibe`（`subtitle`、`description` 作为 `name` 之外的展示字段一并放进 manifest，字段名沿用现有 API 响应）。
- CSS 中的 `[data-theme="<id>"] { --… }` 令牌块 → `theme.json` 的 `tokens`。
- CSS 中的视觉覆盖规则 → `theme.css`，**去掉手写的 `[data-theme="<id>"]` 前缀**，并把内部类名换成 Task 2 登记的 `data-nx` 钩子。
- `terminal` 的 `body::after` 扫描线：确认 `content: ""` 与 `pointer-events: none` 齐备（原实现已有），`z-index: 1` 合规，可原样保留。
- 每迁移完一个就跑 `go test ./internal/themes -run TestBuiltinPackages/<id> -v`。

- [ ] **Step 4: 实现 builtin.go 与启动同步**

```go
//go:embed builtin
var builtinFS embed.FS
```

`SyncBuiltin` 遍历 `BuiltinPackages()`，逐个 `Compile` 后 `Store.UpsertVersion(ctx, pkg.Manifest.ID, compiled, "builtin", "builtin", now)`。在 `internal/app/run.go` 迁移执行之后、HTTP 服务启动之前调用一次，失败则启动失败（内置主题不合规属于构建缺陷，不应静默降级）。

- [ ] **Step 5: 写幂等性集成测试并运行**

```go
func TestSyncBuiltinIsIdempotent(t *testing.T) {
	// 连续调用两次 SyncBuiltin，theme_versions 行数不变
}
```

Run: `go test -race ./internal/themes ./internal/app`
Expected: PASS。

- [ ] **Step 6: 删除前端主题包并跑全量检查**

```bash
git rm -r web/src/themes/packages web/src/themes/manifest.ts
make check
```
此时前端会因引用缺失而报错——这是预期的，Task 11 修复。**本任务先只提交 Go 侧与主题资源文件，前端删除留到 Task 11 一起提交**，保持每次提交可构建：

```bash
git restore --staged web/src/themes && git checkout -- web/src/themes
gofmt -w internal && go vet ./...
go test -race ./...
git add internal/themes internal/app
git commit -m "feat: ship builtin themes through the theme spec pipeline"
```

---

### Task 8: 公开 CSS 与资产端点

**Files:**
- Create: `internal/httpapi/themes.go`
- Create: `internal/httpapi/themes_test.go`
- Modify: `internal/app/run.go`、`api/openapi.yaml`、`tests/contract/`

**Interfaces:**
- Consumes: Task 6 的 `Store.VersionCSS` / `VersionAsset`
- Produces:
  - `GET /api/v1/public/themes/{versionId}.css`
  - `GET /api/v1/public/themes/{versionId}/assets/*`

- [ ] **Step 1: 写失败测试**

`internal/httpapi/themes_test.go`：已知 versionId 返回 200、`Content-Type: text/css; charset=utf-8`、`Cache-Control: public, max-age=31536000, immutable`、ETag 为 content hash；带 `If-None-Match` 返回 304；未知 versionId 返回 404；资产路径含 `..` 返回 404；未登记路径返回 404。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpapi -run TestTheme -v`
Expected: FAIL。

- [ ] **Step 3: 实现 handler 并接线**

参照 `internal/httpapi/assets.go` 的只读供应写法（先查库、再输出，绝不用请求路径直接拼文件路径）。在 `run.go` 的 `MountAPI` 中于 `catalogHandler.Mount(router)` 附近加 `themeHandler.MountPublic(router)`。

- [ ] **Step 4: 更新 openapi 与契约测试**

在 `api/openapi.yaml` 增加两个路径与 `ThemeManifestV1` schema；在 `tests/contract/` 的流程中加入「读取默认主题 CSS」步骤。

- [ ] **Step 5: 运行验证**

```bash
go test -race ./internal/httpapi
make test-contract
```
Expected: 全部 PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/httpapi internal/app api tests/contract
git commit -m "feat: serve immutable theme css and assets"
```

---

### Task 9: 主题列表扩展与服务端 themeId 解析

**Files:**
- Modify: `internal/catalog/service.go`、`internal/httpapi/admin.go`（`themeData`）、`api/openapi.yaml`
- Modify: `internal/catalog/service_test.go`

**Interfaces:**
- Consumes: Task 6 的 `Store`
- Produces: `GET /api/v1/themes` 响应新增 `currentVersionId`、`cssHref`、`tier`、`scope`、`swatches`、`vibe`、`subtitle`

- [ ] **Step 1: 写失败测试**

在 `internal/catalog/service_test.go` 断言 `Themes()` 返回的每一项都带非空 `CurrentVersionID`，且 `CSSHref == "/api/v1/public/themes/" + CurrentVersionID + ".css"`。

- [ ] **Step 2: 运行确认失败** — `go test ./internal/catalog -v`

- [ ] **Step 3: 实现**

`catalog.Service.Themes` 的 SQL 从 `themes` 联 `theme_versions` 取当前版本；`themeData` 补齐新字段；`api/openapi.yaml` 的 `ThemesResponse` 同步。

- [ ] **Step 4: 运行验证** — `go test -race ./internal/catalog ./internal/httpapi && make test-contract`

- [ ] **Step 5: 提交**

```bash
git add internal api
git commit -m "feat: expose current theme version in theme listing"
```

---

### Task 10: 发布锁定主题版本

**Files:**
- Modify: `internal/navigation/types.go`（`PublishedPage` 增 `ThemeVersionID string \`json:"themeVersionId"\``）
- Modify: `internal/navigation/sqlstore.go`（`Publish` 与 `Preview`）
- Modify: `internal/navigation/service.go`（注入 themes 解析器）
- Modify: `api/openapi.yaml`
- Modify: `internal/navigation/service_test.go` 或对应 sqlstore 测试

**Interfaces:**
- Consumes: Task 6 的 `ResolvePackageVersion`
- Produces: 快照中的 `themeVersionId`

- [ ] **Step 1: 写失败测试**

```go
func TestPublishLocksThemeVersion(t *testing.T) {
	env := newNavigationTestEnv(t) // 复用本包既有的测试夹具构造方式
	pageID := env.seedPageWithTheme(t, "slate")

	published, err := env.store.Publish(t.Context(), env.actor, pageID, 1, "https://nav.ax", time.Now().UTC())
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	locked := published.Page.ThemeVersionID
	if locked == "" {
		t.Fatal("published snapshot has empty themeVersionId")
	}

	env.upsertNewSlateVersion(t) // 改写 themes.current_version_id

	reread, err := env.store.PublicBySlug(t.Context(), published.Page.Slug)
	if err != nil {
		t.Fatalf("PublicBySlug() error = %v", err)
	}
	if reread.ThemeVersionID != locked {
		t.Fatalf("snapshot theme version drifted: %q → %q", locked, reread.ThemeVersionID)
	}
}

func TestPublishFallsBackForUnknownTheme(t *testing.T) {
	env := newNavigationTestEnv(t)
	pageID := env.seedPageWithTheme(t, "kyoto") // 0013 已停用的 culled 主题

	published, err := env.store.Publish(t.Context(), env.actor, pageID, 1, "https://nav.ax", time.Now().UTC())
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Page.ThemeVersionID != env.slateVersionID {
		t.Fatalf("themeVersionId = %q, want default theme version %q",
			published.Page.ThemeVersionID, env.slateVersionID)
	}
}
```

`newNavigationTestEnv` / `seedPageWithTheme` / `upsertNewSlateVersion` 是本任务新增的测试辅助函数，建库方式沿用 `internal/navigation` 现有测试；`env.slateVersionID` 是夹具中 `slate` 的当前版本 ID。`Publish` 的返回类型是 `Publication`，其 `Page` 字段为 `PublishedPage`——若现有签名与此不同，以实际签名为准调整断言。

- [ ] **Step 2: 运行确认失败** — `go test ./internal/navigation -run TestPublish -v`

- [ ] **Step 3: 实现**

`Publish` 事务内调用解析器取版本并写入快照 JSON；`Preview` 用同样逻辑（预览取当前版本，不落库）。

- [ ] **Step 4: 运行验证** — `go test -race ./internal/navigation && make test-contract`

- [ ] **Step 5: 提交**

```bash
git add internal/navigation api
git commit -m "feat: lock theme version into published snapshots"
```

---

### Task 11: 前端运行时改造

**Files:**
- Modify: `web/src/themes/types.ts`、`web/src/themes/registry.ts`
- Modify: `web/src/components/feature/PublicShell.tsx`、`web/src/components/base/ThemePicker.tsx`、`web/src/pages/app/themes/page.tsx`、`web/src/pages/admin/themes/page.tsx`
- Modify: `web/src/api/types.ts`、`web/src/api/` 中主题相关模块
- Delete: `web/src/themes/packages/`、`web/src/themes/manifest.ts`、`web/src/lib/themeResolve.ts`

**Interfaces:**
- Consumes: Task 9 的 `GET /api/v1/themes` 新字段、Task 10 的快照 `themeVersionId`
- Produces: `ThemePackage = { id: string; meta: ThemeMeta; cssHref: string }`；`themeRegistry.activate(id)` 切换 `<link>`

- [ ] **Step 1: 改类型与 registry**

`activate` 改为：创建 `<link rel="stylesheet" data-theme-style="<id>">`，`onload` 后设置 `document.documentElement.dataset.theme = id` 并移除旧 link；`deactivate` 移除 link 与属性。

- [ ] **Step 2: 三处 UI 改数据驱动**

主题列表来自 `GET /api/v1/themes`（TanStack Query）；`PublicShell` 从快照拿 `themeVersionId`，直接用 `/api/v1/public/themes/{versionId}.css` 作为 href，不再经过注册表查表。

- [ ] **Step 3: 删除前端主题包与兜底**

```bash
git rm -r web/src/themes/packages web/src/themes/manifest.ts web/src/lib/themeResolve.ts
rg 'themeResolve|themes/packages' web/src   # 必须无结果
```

- [ ] **Step 4: 验证**

```bash
make check
make build
```
浏览器冒烟：加载态、空态、错误态、移动端、键盘导航、深色主题；确认切主题时无样式闪烁、旧 `<link>` 被移除、页脚源码链接始终可见。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "feat: load theme css from content-addressed stylesheet links"
```

---

### Task 12: mock 契约与端到端覆盖

**Files:**
- Modify: `web/src/api/mock-handlers.ts`
- Modify: `web/tests/mock-contract.test.ts`（如需新增断言）
- Modify: `tests/e2e/specs/`（guest 规格）

**Interfaces:**
- Consumes: Task 8、9 的端点契约
- Produces: 绿色的 `make test-mock` 与 `make e2e`

- [ ] **Step 1: mock 补齐新端点**

`GET /api/v1/themes` 返回带 `currentVersionId`/`cssHref` 的数据；`GET /api/v1/public/themes/:versionId.css` 返回一段最小合法 CSS。

- [ ] **Step 2: 运行** — `make test-mock`，Expected: PASS。

- [ ] **Step 3: E2E 断言**

在 guest 规格中断言公开页存在 `link[data-theme-style]` 且 `html[data-theme]` 与快照主题一致。

- [ ] **Step 4: 运行** — `make e2e`，Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add web tests/e2e
git commit -m "test: cover theme css delivery in mock and e2e suites"
```

---

### Task 13: CSP 收紧（核实后执行）

**Files:**
- Modify: `internal/httpapi/security_headers.go`
- Modify: `internal/httpapi/router_test.go` 或新增断言

**Interfaces:**
- Consumes: Task 7 之后的自托管字体现状
- Produces: 更严格的 `style-src` / `font-src`

- [ ] **Step 1: 核实外部主机是否仍被使用**

```bash
rg 'fonts.googleapis|fonts.gstatic|cdnjs' web/src web/index.html internal
```
若仅出现在 `security_headers.go`，说明放行已无用途，继续；若有真实引用，**本任务终止并在计划中记录原因**，不要强行收紧。

- [ ] **Step 2: 写期望新 CSP 的失败测试**

断言响应头的 `style-src` 与 `font-src` 不再包含上述外部主机。

- [ ] **Step 3: 运行确认失败** — `go test ./internal/httpapi -run TestSecurityHeaders -v`

- [ ] **Step 4: 修改并验证**

```bash
go test -race ./internal/httpapi
make build && make e2e
```
浏览器控制台必须无 CSP 违规报错。

- [ ] **Step 5: 提交**

```bash
git add internal/httpapi
git commit -m "chore: tighten style and font CSP after self-hosting theme assets"
```

---

## 完成标准

- `make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、`make e2e` 全绿。
- `web/src/themes/packages/`、`web/src/themes/manifest.ts`、`web/src/lib/themeResolve.ts` 已删除，`rg 'themes/packages'` 无结果。
- 6 个内置主题全部经服务端校验器编译并入库，公开页视觉与迁移前一致。
- 已发布快照携带 `themeVersionId`，主题版本变更不影响既有快照。
- `docs/theme-api.md` 存在且钩子清单与 `internal/themes/hooks.go` 一致。
