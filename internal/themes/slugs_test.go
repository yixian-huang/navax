package themes

import "testing"

func TestReservedCatalogSlugCoversBuiltinsAndOfficialNames(t *testing.T) {
	packages, err := BuiltinPackages()
	if err != nil {
		t.Fatalf("BuiltinPackages() error = %v", err)
	}
	for _, pkg := range packages {
		if !ReservedCatalogSlug(pkg.Manifest.ID) {
			t.Errorf("builtin id %q must be reserved in the catalog namespace", pkg.Manifest.ID)
		}
	}

	culled := []string{"kyoto", "terracotta", "mochi", "pastelsky", "mono", "cyber"}
	for _, id := range culled {
		if !ReservedCatalogSlug(id) {
			t.Errorf("culled first-party id %q must stay reserved so it can be revived", id)
		}
	}

	for _, id := range []string{"default", "official", "builtin", "navax", "system", "catalog", "theme"} {
		if !ReservedCatalogSlug(id) {
			t.Errorf("official namespace slug %q must be reserved", id)
		}
	}

	if ReservedCatalogSlug("aurora") || ReservedCatalogSlug("solstice") {
		t.Fatal("ordinary third-party slugs must not be reserved")
	}
}
