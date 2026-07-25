package themes

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

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
	if !IsAllowedHook(ThemeRootHook) {
		t.Fatalf("%q must be selectable by themes", ThemeRootHook)
	}
	if IsAllowedHook("material-card") {
		t.Fatal("internal class names must not be hooks")
	}
}

// 宿主 wrapper 必须不可被主题选择——它承载 contain: paint，是视觉隔离的
// 唯一边界。一旦它变成钩子，主题就能覆盖 contain 把边界废掉。
func TestFrameIsNotSelectableByThemes(t *testing.T) {
	if IsAllowedHook("frame") || IsAllowedHook(FrameAttr) {
		t.Fatal("host frame must never be an allowed hook")
	}
}

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
