package themes

import "strings"

// reservedCatalogSlugs holds catalog names that must not be claimed by a
// user-promoted theme. Builtin package IDs are reserved dynamically via
// BuiltinPackages so adding a first-party theme cannot silently omit it.
//
// Culled IDs from migration 0013 stay reserved so they can be revived
// without colliding with a promoted third-party slug. The remaining names
// are the official namespace (default theme, product identity, catalog
// itself).
var reservedCatalogSlugs = map[string]struct{}{
	"kyoto": {}, "terracotta": {}, "mochi": {}, "pastelsky": {}, "mono": {}, "cyber": {},
	"default": {}, "official": {}, "builtin": {}, "navax": {}, "system": {}, "catalog": {}, "theme": {},
}

// ReservedCatalogSlug reports whether slug is kept for official / builtin
// catalog themes. Private installs may still use these names; promotion
// into scope=catalog must not.
func ReservedCatalogSlug(slug string) bool {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return false
	}
	if _, ok := reservedCatalogSlugs[slug]; ok {
		return true
	}
	packages, err := BuiltinPackages()
	if err != nil {
		// Fail closed: a catalog promotion must not proceed if we cannot
		// load the first-party set that the unique index also protects.
		return true
	}
	for _, pkg := range packages {
		if pkg.Manifest.ID == slug {
			return true
		}
	}
	return false
}
