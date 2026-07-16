// Package dataexchange implements navigation page import previews, atomic
// import commits, and portable exports.
package dataexchange

import (
	"errors"
	"time"

	"github.com/yixian-huang/navax/internal/navigation"
)

const (
	FormatBookmarksHTML = "bookmarks-html"
	FormatNavaxJSON     = "navax-json"

	ModeMerge   = "merge"
	ModeReplace = "replace"
)

var (
	ErrValidation      = errors.New("data exchange validation failed")
	ErrPayloadTooLarge = errors.New("import payload is too large")
	ErrImportExpired   = errors.New("import preview token is invalid or expired")
	ErrConflict        = errors.New("data exchange conflict")
)

type Preview struct {
	ImportToken string           `json:"importToken"`
	ExpiresAt   time.Time        `json:"expiresAt"`
	Categories  []ImportCategory `json:"categories"`
	Totals      PreviewTotals    `json:"totals"`
}

type PreviewTotals struct {
	Categories int `json:"categories"`
	Sites      int `json:"sites"`
	Duplicates int `json:"duplicates"`
	Invalid    int `json:"invalid"`
}

type ImportCategory struct {
	SourceID string       `json:"sourceId"`
	Name     string       `json:"name"`
	Sites    []ImportSite `json:"sites"`
}

type ImportSite struct {
	SourceID  string `json:"sourceId"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Duplicate bool   `json:"duplicate"`
	Valid     bool   `json:"valid"`
	Error     string `json:"error,omitempty"`
}

type CommitInput struct {
	ImportToken      string   `json:"importToken"`
	Mode             string   `json:"mode"`
	SelectedSiteIDs  []string `json:"selectedSiteIds"`
	ExpectedRevision int      `json:"expectedRevision"`
}

type ImportResult struct {
	CategoriesCreated int `json:"categoriesCreated"`
	SitesCreated      int `json:"sitesCreated"`
	DuplicatesSkipped int `json:"duplicatesSkipped"`
	InvalidSkipped    int `json:"invalidSkipped"`
	DraftRevision     int `json:"draftRevision"`
}

type PortableExport struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exportedAt"`
	Page       navigation.Page `json:"page"`
}

type ExportFile struct {
	ContentType string
	Filename    string
	Content     []byte
}

type previewPayload struct {
	Categories []ImportCategory `json:"categories"`
}
