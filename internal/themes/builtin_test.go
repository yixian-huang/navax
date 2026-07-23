package themes

import (
	"sort"
	"testing"
)

// builtinIDs 是首批随二进制分发的内置主题。它写死在测试里：新增或删除一个
// 内置主题必须是一次自觉的改动，而不是目录里多/少一个文件夹的副作用。
var builtinIDs = []string{"noir", "orbit", "sakura", "slate", "slate-dark", "terminal"}

// TestBuiltinPackagesCompile 是内置主题的黄金文件测试：它证明这 6 个包能通过
// 与第三方包完全相同的校验与编译路径。规则一旦收紧而主题没跟上，这里先红。
func TestBuiltinPackagesCompile(t *testing.T) {
	packages, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	if len(packages) != len(builtinIDs) {
		t.Fatalf("BuiltinPackages() returned %d packages, want %d", len(packages), len(builtinIDs))
	}

	gotIDs := make([]string, 0, len(packages))
	for _, pkg := range packages {
		gotIDs = append(gotIDs, pkg.Manifest.ID)
	}
	if !sort.StringsAreSorted(gotIDs) {
		t.Fatalf("BuiltinPackages() must return packages sorted by id, got %v", gotIDs)
	}
	for i, want := range builtinIDs {
		if gotIDs[i] != want {
			t.Fatalf("BuiltinPackages() ids = %v, want %v", gotIDs, builtinIDs)
		}
	}

	for _, pkg := range packages {
		t.Run(pkg.Manifest.ID, func(t *testing.T) {
			compiled, err := Compile(pkg, pkg.Manifest.ID)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if compiled.VersionID == "" || compiled.ContentHash == "" {
				t.Fatalf("Compile() produced an empty version: %+v", compiled)
			}
			if len(compiled.CSS) == 0 {
				t.Fatal("Compile() produced empty CSS — tokens must always be emitted")
			}
		})
	}
}

// 编译产物的哈希是版本 ID，必须与 embed FS 的遍历顺序、map 迭代顺序无关。
func TestBuiltinPackagesCompileDeterministically(t *testing.T) {
	first, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	second, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() second call error = %v", err)
	}
	for i := range first {
		a, err := Compile(first[i], first[i].Manifest.ID)
		if err != nil {
			t.Fatalf("Compile(%s) error = %v", first[i].Manifest.ID, err)
		}
		b, err := Compile(second[i], second[i].Manifest.ID)
		if err != nil {
			t.Fatalf("Compile(%s) second error = %v", second[i].Manifest.ID, err)
		}
		if a.VersionID != b.VersionID {
			t.Fatalf("%s: version id not stable: %q vs %q", first[i].Manifest.ID, a.VersionID, b.VersionID)
		}
	}
}
