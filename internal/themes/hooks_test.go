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
