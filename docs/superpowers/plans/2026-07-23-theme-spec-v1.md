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
- `z-index` 上限 50；`specVersion` 固定为 1；**校验器只接受 `tier: 1`**，`2`/`3` 拒绝。
- 资产 MIME 白名单：`font/woff2`、`image/png`、`image/jpeg`、`image/webp`。**拒绝 SVG，含 `data:image/svg+xml`。**
- 主题 CSS 中不得出现外部 URL；资产引用语法唯一：`url("asset:<包内路径>")`；`data:` 仅限 `image/png|jpeg|webp` 且单条 ≤ 8192 字节。
- **主题 CSS 的作用域根是 `[data-nx="page-root"]`，不是 `<html>`**；禁止选择 `html`/`body`；根之后禁 `+`/`~`，选择器 subject 必须在根内；`[data-nx-protected]` 元素位于宿主 wrapper 之外。
- **视觉边界是宿主 wrapper `[data-nx-frame]` 上的 `contain: paint`**，不是主题根——主题能选中主题根并用 `transform: none !important` 废掉放在那里的机制，但在语法上选不到 wrapper。
- 选择器标识符必须**先解码 CSS 转义并 ASCII 小写规范化**再比较；递归下钻**所有**接受 selector-list 的语法位置（不止 `:is/:where/:has/:not`，还有 `:nth-child(… of …)`）；拒绝 CSS nesting 与命名空间选择器。
- **值中的函数走正向白名单**，白名单外一律拒绝（`src()`、`image()`、`cross-fade()`、`element()`、`paint()`、`image-set()` 因此自然被拒）；禁 URL 形态的字符串字面量；自定义属性 `--*` 的值走与普通声明相同的完整校验；函数名同样先解码转义再匹配。
- 禁在 `font` 简写中引用包内字体。
- `z-index ≤ 50` 与 `position: fixed` 须带 `pointer-events: none` 是**纵深防御，不是边界**——不要因为它们存在就放松 wrapper 的实现。

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

### Task 2: 主题根与 `data-nx` 稳定选择器契约

> 这是整套安全模型的地基任务：把主题作用域从 `<html>` 收到公开页主题根，并把受许可证保护的元素移到该根之外。

**Files:**
- Create: `docs/theme-api.md`
- Create: `internal/themes/hooks.go`
- Create: `internal/themes/hooks_test.go`
- Modify: `web/src/components/feature/PublicShell.tsx`（新增主题根容器；页脚源码链接移到主题根之外）
- Modify: `web/src/pages/app/themes/page.tsx`（预览主题根限定在预览容器内）
- Modify: 公开页渲染站点卡片与分类的组件（用 `rg 'material-card|hairline' web/src` 定位全部使用点）

**Interfaces:**
- Consumes: 无
- Produces:
  - `func AllowedHooks() []string` — 已登记的 `data-nx` 值，按字典序
  - `func IsAllowedHook(name string) bool`
  - `const ProtectedAttr = "data-nx-protected"`

- [ ] **Step 1: 盘点现有主题实际用到的选择器**

```bash
rg -o '\.[a-z-]+' web/src/themes/packages/*.ts | sort -u
rg -o '\[class\*=[^]]*\]' web/src/themes/packages/   # sakura 用属性匹配选了 Tailwind 原子类
rg "content: *'" web/src/themes/packages/            # sakura 的 content: '✿'
rg -o 'url\("?data:[^;]*' web/src/themes/packages/   # slate/slate-dark 的 SVG 噪点
```

把结果记进 `docs/theme-api.md` 的「迁移映射」小节，内容以 spec §6.4 的差异表为准。每个内部类名（含 `[class*="w-11"]` 这类原子类匹配）必须映射到一个 `data-nx` 钩子；映射不出语义的记为视觉降级，不要为了迁就旧写法放宽规则。

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

// ThemeRootHook 是主题作用域的根。data-theme 设在它上面，
// 主题 CSS 只能触达它的后代；宿主外壳与受保护元素都在根之外。
const ThemeRootHook = "page-root"

// allowedHooks 是主题可以选择的稳定钩子。新增前先更新 docs/theme-api.md。
var allowedHooks = []string{
	"category-tab",
	"clock",
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

- [ ] **Step 6: 重构 PublicShell 的 DOM 结构，划出主题根**

`PublicShell` 改为「外壳 → 主题根 → 内容」三层，页脚源码链接留在**主题根之外**：

结构是**三层**，隔离边界在中间那层：

```tsx
<div className="min-h-screen flex flex-col relative">
  {/* 宿主 wrapper：主题在语法上选不到它，因此这道边界不可被覆盖 */}
  <div data-nx-frame className="relative flex-1 flex flex-col" style={{ contain: 'paint' }}>
    <div data-nx="page-root" data-theme={themeId} className="relative flex-1 flex flex-col">
      {/* 导航栏、搜索、分类、站点卡片等全部内容 */}
    </div>
  </div>
  <footer data-nx-protected className="relative">
    <a href={SOURCE_REPO_URL} …>源码</a>
  </footer>
</div>
```

为什么必须是 wrapper 而不是主题根：

- `contain: paint` 一个属性同时给出三件事——成为绝对/固定定位后代的**包含块**、建立**独立层叠上下文**、把后代绘制**裁剪**在自己盒内。`transform: translateZ(0)` 只给前两件，阴影与 `filter` 仍会溢出。
- 把它放在主题根上不成立：`[data-nx="page-root"]` 是登记钩子，主题可以选中它写 `transform: none !important` / `contain: none !important` 直接废掉边界。wrapper 不是钩子，且所有主题选择器都被前缀到 `[data-theme=…]` 作用域内，**语法上无法命中 wrapper**。
- `data-nx-frame` **不得**加入 `allowedHooks`。Task 3 的校验器要显式拒绝命中它的选择器（和 `data-nx-protected` 同等对待）。
- 受保护页脚是 wrapper 的兄弟，不在被裁剪的绘制范围内，因此不需要靠 `z-index` 竞争。

- [ ] **Step 7: 其余元素挂钩子**

给公开页渲染路径上的元素补 `data-nx` 属性（保留现有 class，不破坏 Tailwind 样式）：

```tsx
<article data-nx="site-card" className="material-card …">
```

在全局样式入口（`web/src/index.css` 或等价文件）加一条兜底规则，作为 DOM 结构之外的第三道保险：

```css
[data-nx-protected] {
  display: revert !important;
  visibility: visible !important;
  opacity: 1 !important;
  pointer-events: auto !important;
}
```

- [ ] **Step 8: 预览页主题根限定**

`web/src/pages/app/themes/page.tsx` 当前调 `themeRegistry.activate(id)` 把 `data-theme` 写到 `<html>`，会让第三方 CSS 作用于整个已登录应用。改为只在预览容器上设置 `data-theme`，管理与编辑 UI 不受影响。Task 11 会把 `themeRegistry` 的实现一并改成"作用于指定根元素"。

- [ ] **Step 9: 写 docs/theme-api.md**

至少包含：主题根的定义与「主题够不到根之外」的说明、钩子清单表（钩子名 / 出现位置 / stable 或 experimental）、`data-nx-protected` 的含义与不可覆盖性、Step 1 的迁移映射（对齐 spec §6.4 差异表）、以及「experimental 钩子可能在小版本变更」的声明。

- [ ] **Step 10: 验证与提交**

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
		{"外部 url()", `[data-nx="clock"] { background-image: url("https://evil.example/p.png"); }`, "url"},
		{"注释绕过的外部 url()", `[data-nx="clock"] { background-image: url(/*x*/"https://evil.example/p.png"); }`, "url"},
		{"image-set 里的外部 url()", `[data-nx="clock"] { background-image: image-set(url("https://evil.example/p.png") 1x); }`, "url"},
		{"cursor 里的外部 url()", `[data-nx="clock"] { cursor: url("https://evil.example/c.cur"), auto; }`, "url"},
		{"@font-face 外链", `@font-face { font-family: "X"; src: url("https://evil.example/f.woff2"); }`, "url"},
		{"@font-face 族名未在令牌中引用", `@font-face { font-family: "Ghost"; src: url("asset:fonts/a.woff2"); }`, "font-family"},
		{"选择 body", `body { background: red; }`, "body"},
		{"选择 html", `html { background: red; }`, "html"},
		{"根后兄弟逃逸", `[data-nx="page-root"] + footer { display: none; }`, "兄弟"},
		{"根后通用兄弟逃逸", `[data-nx="page-root"] ~ footer { opacity: 0; }`, "兄弟"},
		{"逗号列表中混入越界项", `[data-nx="clock"], [data-nx="page-root"] + footer { color: red; }`, "兄弟"},
		{":is() 嵌套越界", `:is([data-nx="page-root"] + footer) { display: none; }`, "兄弟"},
		{"nth-child of 列表越界", `:nth-child(2 of [data-nx="page-root"] + footer) { display: none; }`, "兄弟"},
		{"命中宿主 wrapper", `[data-nx-frame] { contain: none; }`, "data-nx-frame"},
		{"转义的 html", `h\74ml { background: red; }`, "html"},
		{"转义的 class 属性选择器", `[cl\61ss*="w-11"] { width: 0; }`, "class"},
		{"命名空间选择器", `*|body { background: red; }`, "命名空间"},
		{"CSS nesting", `[data-nx="clock"] { & + footer { display: none; } }`, "nesting"},
		{"src() 函数外链", `[data-nx="clock"] { background-image: src("https://evil.example/p.png"); }`, "函数"},
		{"转义的 src()", `[data-nx="clock"] { background-image: \73 rc("https://evil.example/p.png"); }`, "函数"},
		{"image() 函数", `[data-nx="clock"] { background-image: image("https://evil.example/p.png"); }`, "函数"},
		{"cross-fade() 函数", `[data-nx="clock"] { background-image: cross-fade(url("asset:a.png") 50%); }`, "函数"},
		{"element() 函数", `[data-nx="clock"] { background: element(#x); }`, "函数"},
		{"image-set 字符串形式外链", `[data-nx="clock"] { background-image: image-set("https://evil.example/p.png" 1x); }`, "函数"},
		{"-webkit-image-set", `[data-nx="clock"] { background-image: -webkit-image-set(url("asset:a.png") 1x); }`, "函数"},
		{"自定义属性藏外链", `[data-nx="clock"] { --bg: url("https://evil.example/p.png"); background-image: var(--bg); }`, "url"},
		{"URL 形态字符串字面量", `[data-nx="clock"] { --bg: "https://evil.example/p.png"; }`, "字符串"},
		{"font 简写引用包内字体", `[data-nx="clock"] { font: bold 12px "Sample Sans"; }`, "font"},
		{"非空 content", `[data-nx="page-root"]::after { content: "请登录"; }`, "content"},
		{"装饰字符 content", `[data-nx="site-card"]::after { content: "✿"; }`, "content"},
		{"attr() content", `[data-nx="page-root"]::after { content: attr(href); }`, "content"},
		{"fixed 缺 pointer-events", `[data-nx="page-root"]::after { content: ""; position: fixed; inset: 0; }`, "pointer-events"},
		{"z-index 越界", `[data-nx="navbar"] { z-index: 9999; }`, "z-index"},
		{"未知 at-rule", `@container (min-width: 10px) { [data-nx="clock"] { color: red; } }`, "at-rule"},
		{"@layer 被禁", `@layer theme { [data-nx="clock"] { color: red; } }`, "@layer"},
		{"behavior", `[data-nx="clock"] { behavior: url(x.htc); }`, "behavior"},
		{"-moz-binding", `[data-nx="clock"] { -moz-binding: url(x.xml); }`, "binding"},
		{"expression", `[data-nx="clock"] { width: expression(alert(1)); }`, "expression"},
		{"命中受保护元素", `[data-nx-protected] { display: none; }`, "data-nx-protected"},
		{"未登记的内部类名", `.material-card { color: red; }`, "material-card"},
		{"Tailwind 原子类属性匹配", `[class*="w-11"] { width: 0; }`, "class"},
		{"未登记的 data-nx 钩子", `[data-nx="admin-panel"] { display: none; }`, "admin-panel"},
		{"data:image/svg+xml", `[data-nx="clock"] { background-image: url("data:image/svg+xml,%3Csvg%3E%3C/svg%3E"); }`, "svg"},
		{"超大 data: URI", `[data-nx="clock"] { background-image: url("data:image/png;base64,` + strings.Repeat("A", 9000) + `"); }`, "data:"},
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
3. `AtRuleGrammar` / `BeginAtRuleGrammar`：at-rule 名小写后必须落在 `{"media","supports","keyframes","font-face"}`，否则报 `不支持的 at-rule`。`@import` 与 `@layer` 各自给出更明确的报错文案（`@layer` 的理由是层序全局、无法靠改名隔离）。
4. `BeginRulesetGrammar` / `QualifiedRuleGrammar`：取 `p.Values()` 组成选择器文本，**解析成复合选择器 + 组合符的序列**（不要用字符串匹配判断组合符，注释与属性值里的 `+` 会骗过它），逗号列表逐条独立判定，任一条越界则整条规则拒绝——
   - 含 `data-nx-protected` → 拒绝；
   - 类型选择器命中 `html` / `body` → 拒绝，提示改用 `[data-nx="page-root"]`；
   - 出现任何类选择器 → 拒绝并回显类名（主题只能用钩子，不能用类）；
   - 出现 `[class…]` 形式的属性选择器 → 拒绝（`[class*="w-11"]` 是绕过类名限制的等价写法）；
   - `[data-nx="x"]` 中的 `x` 必须 `IsAllowedHook`，否则拒绝并回显 `x`；
   - 命中 `data-nx-frame`（宿主 wrapper）→ 拒绝，与 `data-nx-protected` 同等对待；
   - **包含规则**：出现 `[data-nx="page-root"]` 时，其后只允许后代（空格）与子代（`>`）组合符；出现 `+` / `~` → 拒绝（`[data-nx="page-root"] + footer` 加完前缀会命中根外的受保护页脚）；subject（最右复合选择器）必须是根自身或其后代；
   - **递归下钻所有接受 selector-list 的语法位置**，不要写成"检查这四个伪类"：`:is()` / `:where()` / `:has()` / `:not()` 之外还有 `:nth-child(… of <list>)` / `:nth-last-child(… of …)`。写成通用遍历，新伪类才不会自动成为缺口；
   - **标识符必须先解码 CSS 转义再 ASCII 小写规范化，然后才做比较**：`h\74ml` 就是 `html`、`[cl\61ss*=…]` 就是 `[class*=…]`。用 AST 不会自动带来规范化，这一步必须显式实现并测试；
   - 拒绝 CSS nesting（`&` / `@nest`）与命名空间选择器（`ns|el`、`*|body`）——v1 不承担它们的作用域语义。
5. `DeclarationGrammar`：`data` 是属性名，`p.Values()` 是值 token 序列——
   - 属性名命中 `behavior` / `-moz-binding` → 拒绝；值中出现 `expression(` 函数 token → 拒绝；
   - `content`：值必须是空字符串字面量或 `none`，其余（含 `attr()` 与任何非空字面量）一律拒绝；
   - `z-index`：解析为整数且 ≤ 50，非整数值（如 `var(...)`）一律拒绝。
6. **值走正向白名单**（不是"扫描 URL token"、也不是"拒绝已知的坏函数"——CSS 里能触发加载的写法不都产生 URL token，且新函数会持续出现）。对**每条声明的值，包括自定义属性 `--*` 的值**：
   - **函数名白名单**（先解码转义 + ASCII 小写后匹配）：`var` `calc` `min` `max` `clamp` `rgb` `rgba` `hsl` `hsla` `oklch` `oklab` `color-mix` `linear-gradient` `radial-gradient` `conic-gradient` 及 `repeating-` 变体 `cubic-bezier` `steps` `translate*` `scale*` `rotate*` `skew*` `matrix*` `blur` `brightness` `contrast` `drop-shadow` `grayscale` `hue-rotate` `invert` `opacity` `saturate` `sepia` `format` `local` `url`。**白名单外的函数一律拒绝**——`src()`、`image()`、`cross-fade()`、`element()`、`paint()`、`image-set()`、`-webkit-image-set()` 因此自然被拒，无需逐个枚举；
   - `url("asset:<path>")`：合法，记录 `<path>`，供 Task 4 重写与资产存在性检查；
   - `url("data:image/png|jpeg|webp,…")`：总长 ≤ 8192 才合法；`data:image/svg+xml` 明确拒绝（与资产层拒绝 SVG 一致）；
   - 其余 `url()` 一律拒绝，错误文案回显被拒的 URL 前 64 字符；
   - **任何字符串字面量若形如 URL**（含 `://`、以 `//` 开头，或经反斜杠转义拼接后成立）→ 拒绝；
   - 自定义属性必须走以上全部规则，否则 `--bg: url(https://…)` + `background: var(--bg)` 是等价的外链通道。
   - `font` 简写中出现包内 `@font-face` 族名 → 拒绝，提示改用 `font-family`。
7. 在当前 ruleset 内累积声明，ruleset 结束（`EndRulesetGrammar`）时做跨声明检查：出现 `position: fixed` 而没有 `pointer-events: none` → 拒绝。
8. `@font-face` 块：`src` 只允许 `url("asset:…")`；`font-family` 的族名必须出现在传入的 `fontFamilies` 中（去引号、去首尾空格后比较）。

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
  - `func CompileCSS(src []byte, scope string) ([]byte, error)` — 选择器加作用域、全局名命名空间化、`url("asset:p")` → `url("<AssetBasePlaceholder>p")`
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
	for _, want := range []string{"@keyframes sakura-pulse", "animation: sakura-pulse", `"sakura-Sample Sans"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	// keyframes 内部的 from/to 不加作用域前缀
	if strings.Contains(got, `[data-theme="sakura"] from`) {
		t.Fatalf("keyframe selectors must not be scoped: %s", got)
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

- 每个 ruleset 的选择器列表逐个前缀化：`:root` 与 `[data-nx="page-root"]` → `[data-theme="<scope>"]` 自身；其余 `sel` → `[data-theme="<scope>"] sel`；逗号分隔的多选择器分别处理。
- `@media` / `@supports` 内部的 ruleset 同样前缀化（这是测试里 `strings.Count(...) != 2` 要覆盖的点）。
- **全局名命名空间化**（选择器前缀不隔离它们），三处引用面必须全覆盖，漏一处就会出现"通过校验但字体/动画找不到"：
  - `@keyframes` 名加 `<scope>-` 前缀，并同步改写引用它的 `animation-name` / `animation` 值；`@keyframes` 内部的 `from`/`to`/百分比选择器**不**加前缀；
  - `@font-face` 的 `font-family` descriptor 加 `<scope>-` 前缀，并同步改写 `font-family` 声明中对该族名的引用；系统字体名不动；`font` 简写已在校验阶段拒绝引用包内字体，编译期无需解析简写；
  - **`TokensCSS` 必须用重命名后的族名**：`tokens.font.*` 引用包内字体时，生成的 `--font-*` 变量要写 `<scope>-<family>`，否则 `theme.json` 与 `theme.css` 对不上。这条由 `TestTokensCSSUsesNamespacedFontFamily` 覆盖。
  - `@layer` 已在校验阶段拒绝，编译期无需处理。
- `@font-face` 规则本身不加选择器前缀。
- `url("asset:p")` → `url("<AssetBasePlaceholder>p")`，遍历所有 URL token 位置（含 `image-set()`、`cursor`）。
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
	pkg.CSS = []byte(`@font-face { font-family: "Sample Sans"; src: url("asset:fonts/missing.woff2"); }`)
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
		CSS:      []byte(`@font-face { font-family: "Sample Sans"; src: url("asset:fonts/sample.woff2"); }`),
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
3. 交叉检查：CSS 中出现的每个 `url("asset:p")` 都必须在 `pkg.Assets` 中有 `Path == p`，否则报错并回显路径。
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

`migrations/0014_theme_packages.sql`，内容严格照 spec §7.1，三处易错点：

1. `UPDATE themes SET slug = id WHERE slug = '';` 必须在两个唯一索引之前执行，否则既有行的空串 slug 会冲突。
2. `theme_versions.theme_id` 用 `ON DELETE RESTRICT`（**不是 CASCADE**）——已发布快照按 version_id 引用编译产物，删包不得毁掉线上样式。
3. SQLite 不能给已有表补 `CHECK`，`scope`/`owner_id` 的配对不变量用 `themes_scope_owner_insert` / `themes_scope_owner_update` 两个 `BEFORE` 触发器实现（照抄 spec §7.1 的 SQL）。缺了它，`owner_id IS NULL` 的私有主题会因 NULL 在唯一索引中互不相等而绕过 slug 唯一性。
4. **`published_snapshots` 必须增列 `theme_version_id TEXT REFERENCES theme_versions(id) ON DELETE RESTRICT`**（可空，NULL = 迁移前的旧快照，读取时回落默认主题）。只把 versionId 写进 `payload_json` 数据库管不着——`DELETE FROM theme_versions` 能直接抽掉线上公开页的样式。同时建部分索引 `idx_published_snapshots_theme_version`。
5. **两个触发器把 `current_version_id` 变成数据库级不变量**（照抄 spec §7.1）：`themes_current_version_valid`（必须存在、属于本主题、`status='active'`）与 `theme_versions_current_guard`（仍被引用为当前版本的行不得删除）。只靠应用层校验挡不住绕过服务层的写入；快照外键也覆盖不到"当前版本尚未被任何快照引用"这段空窗。

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

再补三个约束回归测试（对应 codex 评审指出的隔离与生命周期漏洞）：

```go
func TestThemesScopeOwnerTriggers(t *testing.T) {
	db := newTestDB(t)
	// scope='private' 且 owner_id IS NULL → 必须被触发器 ABORT
	// scope='catalog' 且 owner_id 非空 → 必须被触发器 ABORT
	// 合法组合 → 成功
}

func TestUpsertVersionRejectsForeignVersionPointer(t *testing.T) {
	db := newTestDB(t)
	// 构造属于主题 A 的 version，尝试把它写进主题 B 的 current_version_id → 必须失败
}

func TestDeleteThemeWithVersionsIsRestricted(t *testing.T) {
	db := newTestDB(t)
	// 已有 theme_versions 行的主题执行 DELETE → 必须因 RESTRICT 失败
}

func TestDeleteVersionReferencedBySnapshotIsRestricted(t *testing.T) {
	db := newTestDB(t)
	// 插入一条 published_snapshots.theme_version_id 指向该版本的行
	// DELETE FROM theme_versions WHERE id = ? → 必须因 RESTRICT 失败
	// 这是 codex 第二轮指出的缺口：themes 上的 RESTRICT 只挡删包，挡不住删版本
}

func TestCurrentVersionTriggers(t *testing.T) {
	db := newTestDB(t)
	// 1. current_version_id 指向他主题的版本 → 触发器 ABORT
	// 2. current_version_id 指向 status='disabled' 的版本 → 触发器 ABORT
	// 3. 删除仍是某主题 current_version_id 的版本（且无任何快照引用）→ 触发器 ABORT
	//    第 3 条是快照外键覆盖不到的空窗：当前版本可能还没被发布过
}

func TestDefaultThemeInvariant(t *testing.T) {
	db := newTestDB(t)
	// SyncBuiltin 后：is_default=1 的行唯一，且 scope='catalog'、enabled=1、
	// current_version_id 指向 active 版本。破坏该不变量后启动断言必须失败。
}
```

`ErrNotFound`、`NewStore` 与 `newTestDB` 辅助函数一并在本任务中定义（`newTestDB` 建临时 SQLite 库并执行 `migrations/*.sql`，写法照抄 `internal/admin/sqlstore_test.go`；注意必须开启 `PRAGMA foreign_keys = ON`，否则 `RESTRICT` 不生效，测试会假通过）。

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

- [ ] **Step 3: 逐个迁移主题（不是无损迁移）**

对每个 `web/src/themes/packages/<id>.ts`：

- `meta` → `theme.json` 的 `name/subtitle/description/swatches/vibe`（`subtitle`、`description` 作为 `name` 之外的展示字段一并放进 manifest，字段名沿用现有 API 响应）。
- CSS 中的 `[data-theme="<id>"] { --… }` 令牌块 → `theme.json` 的 `tokens`。
- CSS 中的视觉覆盖规则 → `theme.css`，**去掉手写的 `[data-theme="<id>"]` 前缀**，并把内部类名换成 Task 2 登记的 `data-nx` 钩子。

按 spec §6.4 的差异表逐条处置，**不要为了让旧写法通过而放宽规则**：

| 主题 | 现状 | 处置 |
|---|---|---|
| `terminal` | `body::after` 扫描线 | 选择器改 `[data-nx="page-root"]::after`；`content: ""`、`pointer-events: none`、`z-index: 1` 原实现已合规 |
| `sakura` | `body::before` 全屏柔光 | 改 `[data-nx="page-root"]::before` |
| `sakura` | `content: '✿'` | 改 `background-image: url("asset:deco/blossom.png")`（需新增该资产）或 mask 实现 |
| `sakura` | `[class*="relative"]`、`[class*="w-11"]` | 换 `data-nx` 钩子；补不出语义的删除该规则并在 `docs/theme-api.md` 记为视觉降级 |
| `slate` / `slate-dark` | `url("data:image/svg+xml,…")` 噪点 | 改 `assets/noise.png` 或纯 CSS 渐变 |

- 每迁移完一个就跑 `go test ./internal/themes -run TestBuiltinPackages/<id> -v`。
- 迁移完成后把每处实现方式变更与视觉降级补进 `docs/theme-api.md` 的迁移映射小节。

- [ ] **Step 4: 实现 builtin.go 与启动同步**

```go
//go:embed builtin
var builtinFS embed.FS
```

`SyncBuiltin` 遍历 `BuiltinPackages()`，逐个 `Compile` 后 `Store.UpsertVersion(ctx, pkg.Manifest.ID, compiled, "builtin", "builtin", now)`。在 `internal/app/run.go` 迁移执行之后、HTTP 服务启动之前调用一次，失败则启动失败（内置主题不合规属于构建缺陷，不应静默降级）。

同步完成后**断言默认主题不变量**：`is_default = 1` 的行唯一，且 `scope='catalog'`、`enabled=1`、`current_version_id` 指向 active 版本。不成立则启动失败——回落目标自身不可用的话，`Publish` 的回落只是把问题推后一步。

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

`internal/httpapi/themes_test.go`：已知 versionId 返回 200、`Content-Type: text/css; charset=utf-8`、`Cache-Control: public, max-age=31536000, immutable`、ETag 为 content hash；带 `If-None-Match` 返回 304；未知 versionId 返回 404；`status='disabled'` 的版本返回 **410 Gone**（语义是"曾存在、已撤销"，与 404 区分）；资产路径含 `..` 返回 404；未登记路径返回 404。

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

在 `internal/catalog/service_test.go` 断言：

- `Themes()` 返回的每一项都带非空 `CurrentVersionID`，且 `CSSHref == "/api/v1/public/themes/" + CurrentVersionID + ".css"`；
- **单一 eligibility 谓词**（spec §8.1）——列表、预览、发布三处必须复用同一个判定函数，不各写一份 SQL。测试覆盖：普通用户看到目录启用主题 + 自己启用的私有主题；看不到他人私有主题；**看不到自己已卸载（`enabled=0`）的私有主题**（私有分支漏掉 `enabled=1` 是 codex 第二轮指出的具体矛盾——会造成"能选、发布却静默回落"）；看不到 `current_version_id` 已被撤销（`status='disabled'`）的主题；管理员走单独的全量只读谓词。
- 子项目 A 尚不能创建私有主题，测试直接向表里插入 `scope='private'` 的夹具覆盖这些边界，**不要等到 B 才补**——谓词写错的代价是跨租户泄露。

- [ ] **Step 2: 运行确认失败** — `go test ./internal/catalog -v`

- [ ] **Step 3: 实现**

`catalog.Service.Themes` 的 SQL 从 `themes` 联 `theme_versions` 取当前版本，并按调用主体施加可见性谓词；`themeData` 补齐新字段；`api/openapi.yaml` 的 `ThemesResponse` 同步。

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
- Modify: `internal/navigation/sqlstore.go`（`Publish` 与 `Preview`；`Publish` 须**同时**写 `published_snapshots.theme_version_id` 列与 payload）
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

补一条跨租户回归（codex 评审指出的授权缺口）：

```go
func TestPublishRejectsForeignPrivateTheme(t *testing.T) {
	env := newNavigationTestEnv(t)
	foreign := env.seedPrivateTheme(t, env.otherUserID) // 属于另一个用户的私有主题
	pageID := env.seedPageWithTheme(t, foreign)

	published, err := env.store.Publish(t.Context(), env.actor, pageID, 1, "https://nav.ax", time.Now().UTC())
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Page.ThemeVersionID != env.slateVersionID {
		t.Fatalf("cross-tenant theme leaked into snapshot: %q", published.Page.ThemeVersionID)
	}
}
```

- [ ] **Step 3: 实现**

`Publish` 事务内调用 Task 9 的**同一个** eligibility 谓词取版本，写入 `published_snapshots.theme_version_id` 列与快照 payload 两处。谓词不成立则回落默认主题（而不是报错——避免用户被他人的下架操作卡住发布）；**若默认主题本身也不可用**（不变量被破坏），则明确返回 `503` 并附可诊断信息，不要静默产出引用空版本的快照。`Preview` 用同样逻辑（预览取当前版本，不落库）。

不要在这里另写一份判定 SQL：列表与发布各写一份正是第二轮评审里"能选、发布却静默回落"的成因。

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
- Consumes: Task 9 的 `GET /api/v1/themes` 新字段、Task 10 的快照 `themeVersionId`、Task 2 的主题根 DOM 结构
- Produces: `ThemePackage = { id: string; meta: ThemeMeta; cssHref: string }`；`themeRegistry.activate(id, root: HTMLElement)` 切换 `<link>` 并在 **root** 上设置 `data-theme`

- [ ] **Step 1: 改类型与 registry，实现原子切换状态机**

`activate(id, root)` 不是简单的 onload 回调，四条行为各写一个单测：

1. **序号防竞态**：模块内维护一个自增 `switchSeq`，每次 `activate` 递增并捕获当前值；`load`/`error` 回调先比对序号，过期回调直接 `return`，避免慢请求覆盖用户更新的选择。
2. **先加载后替换**：新 link 成功加载前保留旧 link（先移除会闪烁），成功后同一帧内移除旧 link 并设置 `root.dataset.theme = id`。
3. **失败与超时**：`error` 或 5 秒超时 → 移除失败的 link，保持当前主题不变并提示用户；若当前无任何主题（首次加载失败），显式回落基线令牌。已撤销版本返回 `410` 走同一条失败路径。
4. **`data-theme` 设在传入的 root 上，不再是 `document.documentElement`**；`deactivate(root)` 移除 link 与该 root 上的属性。

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
浏览器冒烟：加载态、空态、错误态、移动端、键盘导航、深色主题；并逐项确认——

- 切主题时无样式闪烁，旧 `<link>` 被移除；
- 快速连点多个主题后，最终生效的是最后一次点击；
- 断网/请求失败时保持当前主题并给出提示，不出现裸样式；
- `/app/themes` 预览时管理与编辑 UI 不受主题影响（`data-theme` 只在预览容器上）；
- 页脚源码链接在任何主题下都可见（它在主题根之外）。

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

在 guest 规格中断言：公开页存在 `link[data-theme-style]`；`[data-nx="page-root"][data-theme]` 与快照主题一致（**不是 `html[data-theme]`**）；`[data-nx-protected]` 的源码链接可见且位于主题根之外。

- [ ] **Step 3b: 恶意主题视觉隔离 E2E**

造一个只用于测试的主题夹具（**直接构造编译产物注入，绕过校验器**——目的是验证 wrapper 这道边界本身，而不是验证校验器能拦住它们），用以下手段尝试遮挡 wrapper 之外的 UI，全部必须失败：

```css
[data-nx="page-root"]::after { content: ""; position: fixed; inset: 0; background: red; }
[data-nx="site-card"] { position: absolute; top: -9999px; height: 20000px; }
[data-nx="site-card"] { box-shadow: 0 0 0 9999px rgba(255,0,0,1); }
[data-nx="page-root"] { filter: invert(1); }
[data-nx="page-root"] { width: 300vw; height: 300vh; }
/* 试图废掉边界本身 */
[data-nx="page-root"] { transform: none !important; contain: none !important; z-index: 2147483647 !important; }
```

断言：`[data-nx-protected]` 的源码链接在每种情形下都可见且可点击（用 Playwright 的可见性与命中测试，不只查 DOM 存在）。最后一条尤其关键——它验证边界确实在主题够不到的 wrapper 上，而不是在主题能覆盖的根上。这是本设计里最容易被实现细节悄悄破坏的一环（第二轮方案就是栽在这里）。

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
- 6 个内置主题全部经服务端校验器编译并入库；公开页视觉与迁移前一致，**§6.4 记录的实现方式变更除外**，且这些变更已写进 `docs/theme-api.md`。
- 主题作用域封闭：`data-theme` 只出现在 `[data-nx="page-root"]` 上，`rg 'documentElement.*data-theme' web/src` 无结果；`[data-nx-protected]` 位于宿主 wrapper 之外；恶意主题夹具的全部遮挡手段（含试图覆盖 `transform`/`contain`/`z-index`）均失效。
- 已发布快照携带 `themeVersionId` 并写入 `published_snapshots.theme_version_id` 列；删除仍被引用的主题包、被快照引用的版本、以及仍是当前版本的版本，三者都被拒绝。
- 归属与可见性：触发器拒绝 `scope`/`owner_id` 错配与非法 `current_version_id`；列表/预览/发布复用同一个 eligibility 谓词；跨租户主题不会进入他人快照；默认主题不变量在启动时被断言。
- `docs/theme-api.md` 存在且钩子清单与 `internal/themes/hooks.go` 一致。
