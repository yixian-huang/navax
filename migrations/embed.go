// Package migrations exposes the SQL migrations embedded in the nav.ax binary.
package migrations

import "embed"

// Files contains every immutable, ordered SQL migration in this directory.
//
//go:embed *.sql
var Files embed.FS
