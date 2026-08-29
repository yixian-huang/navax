package themes

import (
	"os"
	"path/filepath"
	"testing"
)

func starterExampleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "examples", "theme-starter")
}

func readStarterFiles(t *testing.T) map[string][]byte {
	t.Helper()
	dir := starterExampleDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		files[name] = data
	}
	return files
}

// TestStarterExampleCompiles 钉住官方示例包能走与第三方导入完全相同的
// 组包 + 编译路径。示例一坏，作者照抄就会导入失败。
func TestStarterExampleCompiles(t *testing.T) {
	pkg, err := PackageFromFiles(readStarterFiles(t))
	if err != nil {
		t.Fatalf("PackageFromFiles() error = %v", err)
	}
	if pkg.Manifest.ID != "starter" {
		t.Fatalf("starter id = %q, want starter", pkg.Manifest.ID)
	}
	if pkg.Manifest.Tier != 1 {
		t.Fatalf("starter tier = %d, want 1", pkg.Manifest.Tier)
	}
	compiled, err := Compile(pkg, pkg.Manifest.ID)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if compiled.VersionID == "" || len(compiled.CSS) == 0 {
		t.Fatalf("Compile() produced an empty version: %+v", compiled)
	}
}

func TestStarterExampleSlugIsReservedInCatalog(t *testing.T) {
	if !ReservedCatalogSlug("starter") {
		t.Fatal("catalog must reserve the official starter slug so the example id cannot be promoted")
	}
}
