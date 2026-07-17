package navigation

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("navigation resource not found")
	ErrForbidden        = errors.New("navigation access forbidden")
	ErrPrecondition     = errors.New("navigation revision precondition failed")
	ErrConflict         = errors.New("navigation resource conflict")
	ErrValidation       = errors.New("navigation validation failed")
	ErrCategoryNotEmpty = errors.New("navigation category is not empty")
	ErrInvalidOrder     = errors.New("navigation content order is incomplete or invalid")
	ErrUncategorized    = errors.New("navigation uncategorized category cannot be deleted")
)

type PageKind string

const (
	PageKindPersonal PageKind = "personal"
	PageKindSystem   PageKind = "system"
)

type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPublic   Visibility = "public"
)

type DeleteCategoryMode string

const (
	DeleteCategoryRejectIfNotEmpty DeleteCategoryMode = "reject-if-not-empty"
	DeleteCategoryDeleteSites      DeleteCategoryMode = "delete-sites"
	DeleteCategoryMoveSites        DeleteCategoryMode = "move-to-uncategorized"
)

// Actor is the authenticated identity used for navigation authorization.
type Actor struct {
	UserID    string
	Username  string
	AvatarURL string
	Role      string
}

func (a Actor) IsAdmin() bool { return a.Role == "admin" }

type PageSettings struct {
	Layout      LayoutSettings     `json:"layout"`
	Appearance  AppearanceSettings `json:"appearance"`
	Search      SearchSettings     `json:"search"`
	Display     DisplaySettings    `json:"display"`
	Preferences PreferenceSettings `json:"preferences"`
}

type LayoutSettings struct {
	Template      string `json:"template"`
	Density       string `json:"density"`
	Columns       int    `json:"columns"`
	CategoryStyle string `json:"categoryStyle"`
}

type AppearanceSettings struct {
	ThemeID    string             `json:"themeId"`
	Background BackgroundSettings `json:"background"`
}

type BackgroundSettings struct {
	Type    string  `json:"type"`
	Value   string  `json:"value"`
	Opacity float64 `json:"opacity"`
	// MediaID links to background_media library entry (optional).
	MediaID *string `json:"mediaId,omitempty"`
	// Poster is a still frame URL for video backgrounds.
	Poster *string `json:"poster,omitempty"`
}

type SearchSettings struct {
	DefaultEngine      string `json:"defaultEngine"`
	ShowEngineSelector bool   `json:"showEngineSelector"`
}

type DisplaySettings struct {
	ShowClock    bool `json:"showClock"`
	ShowDate     bool `json:"showDate"`
	ShowGreeting bool `json:"showGreeting"`
}

type PreferenceSettings struct {
	Locale            string `json:"locale"`
	Timezone          string `json:"timezone"`
	OpenLinksInNewTab bool   `json:"openLinksInNewTab"`
}

type Category struct {
	ID              string    `json:"id"`
	PageID          string    `json:"pageId"`
	Name            string    `json:"name"`
	Icon            string    `json:"icon"`
	SortOrder       int       `json:"sortOrder"`
	IsUncategorized bool      `json:"-"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Site struct {
	ID          string    `json:"id"`
	PageID      string    `json:"-"`
	CategoryID  string    `json:"categoryId"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Icon        string    `json:"icon"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Publication struct {
	Visibility     Visibility `json:"visibility"`
	Slug           string     `json:"slug"`
	ShowAuthor     bool       `json:"showAuthor"`
	SEOTitle       string     `json:"seoTitle"`
	SEODescription string     `json:"seoDescription"`
	// SEOImage is an optional dedicated Open Graph / share image URL.
	// Empty means publish falls back to the page background (or video poster).
	SEOImage              string     `json:"seoImage"`
	Published             bool       `json:"published"`
	CanonicalURL          *string    `json:"canonicalUrl"`
	Robots                string     `json:"robots"`
	SnapshotID            *string    `json:"snapshotId"`
	PublishedRevision     *int       `json:"publishedRevision"`
	PublishedAt           *time.Time `json:"publishedAt"`
	HasUnpublishedChanges bool       `json:"hasUnpublishedChanges"`
}

type Page struct {
	ID             string       `json:"id"`
	Kind           PageKind     `json:"kind"`
	OwnerID        *string      `json:"ownerId"`
	OwnerName      string       `json:"ownerName"`
	OwnerAvatarURL string       `json:"-"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	DraftRevision  int          `json:"draftRevision"`
	Settings       PageSettings `json:"settings"`
	Categories     []Category   `json:"categories"`
	Sites          []Site       `json:"sites"`
	Publication    Publication  `json:"publication"`
	DraftUpdatedAt time.Time    `json:"draftUpdatedAt"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

type PublishedOwner struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Visible   bool   `json:"visible"`
}

type PublicCategory struct {
	Category
	Sites []Site `json:"sites"`
}

// PublishedPage is the complete immutable public payload stored in a snapshot.
type PublishedPage struct {
	ID             string           `json:"id"`
	SnapshotID     string           `json:"snapshotId"`
	Kind           PageKind         `json:"kind"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	SEOTitle       string           `json:"seoTitle,omitempty"`
	SEODescription string           `json:"seoDescription,omitempty"`
	OGImage        string           `json:"ogImage,omitempty"`
	Slug           string           `json:"slug"`
	Visibility     Visibility       `json:"visibility"`
	Owner          PublishedOwner   `json:"owner"`
	Settings       PageSettings     `json:"settings"`
	Categories     []PublicCategory `json:"categories"`
	Subdomain      *string          `json:"subdomain,omitempty"`
	PublishedAt    time.Time        `json:"publishedAt"`
	ETag           string           `json:"etag"`
}

type PagePatch struct {
	ExpectedRevision int
	Title            *string
	Description      *string
}

type CategoryInput struct {
	Name string
	Icon string
}

type CategoryPatch struct {
	Name *string
	Icon *string
}

type SiteInput struct {
	CategoryID  string
	Title       string
	URL         string
	Icon        string
	Description string
}

type SitePatch struct {
	CategoryID  *string
	Title       *string
	URL         *string
	Icon        *string
	Description *string
}

type CategoryOrder struct {
	ID      string
	SiteIDs []string
}

type PublicationSettingsInput struct {
	Visibility     Visibility
	Slug           string
	ShowAuthor     bool
	SEOTitle       string
	SEODescription string
	SEOImage       string
}
