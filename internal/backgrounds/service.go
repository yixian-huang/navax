// Package backgrounds manages instance preset and per-user background media libraries.
package backgrounds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/assets"
	"github.com/yixian-huang/navax/internal/identity"
)

const (
	MaxInstancePresets = 12
	MaxUserLibrary     = 3
	MaxVideoSeconds    = 15
	// Max edge for video only (storage); still photos prefer full resolution.
	MaxVideoEdgePx = 1920
	// Soft cap before re-encode: if still image already smaller, keep original.
	TargetJPEGQuality = 85
)

var (
	ErrQuota          = errors.New("background media quota exceeded")
	ErrNotFound       = errors.New("background media not found")
	ErrForbidden      = errors.New("background media access denied")
	ErrInvalidFile    = errors.New("invalid background media file")
	ErrFFmpegRequired = errors.New("ffmpeg is required for video backgrounds")
	ErrVideoTooLong   = errors.New("video exceeds maximum duration")
)

type Scope string

const (
	ScopeInstance Scope = "instance"
	ScopeUser     Scope = "user"
)

type MediaKind string

const (
	MediaImage MediaKind = "image"
	MediaVideo MediaKind = "video"
)

type Media struct {
	ID          string    `json:"id"`
	Scope       Scope     `json:"scope"`
	OwnerUserID *string   `json:"ownerUserId,omitempty"`
	AssetID     string    `json:"assetId"`
	MediaKind   MediaKind `json:"mediaKind"`
	MIMEType    string    `json:"mimeType"`
	URL         string    `json:"url"`
	PosterURL   *string   `json:"posterUrl,omitempty"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	DurationMS  *int      `json:"durationMs,omitempty"`
	SizeBytes   int64     `json:"sizeBytes"`
	SortOrder   int       `json:"sortOrder"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Service struct {
	db     *sql.DB
	assets *assets.Service
	root   string
	now    func() time.Time
}

func NewService(db *sql.DB, assetService *assets.Service, dataDir string) (*Service, error) {
	if db == nil || assetService == nil {
		return nil, errors.New("backgrounds requires database and asset service")
	}
	root := filepath.Join(dataDir, "assets")
	return &Service{db: db, assets: assetService, root: root, now: time.Now}, nil
}

func (s *Service) ListPresets(ctx context.Context, includeDisabled bool) ([]Media, error) {
	query := `
		SELECT id, scope, owner_user_id, asset_id, media_kind, mime_type, url, poster_url,
		       width, height, duration_ms, size_bytes, sort_order, enabled, created_at
		FROM background_media WHERE scope = 'instance'`
	if !includeDisabled {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY sort_order ASC, created_at ASC`
	return s.queryList(ctx, query)
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]Media, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrForbidden
	}
	return s.queryList(ctx, `
		SELECT id, scope, owner_user_id, asset_id, media_kind, mime_type, url, poster_url,
		       width, height, duration_ms, size_bytes, sort_order, enabled, created_at
		FROM background_media WHERE scope = 'user' AND owner_user_id = ?
		ORDER BY created_at DESC`, userID)
}

func (s *Service) UploadPreset(ctx context.Context, adminUserID, filename, declaredMIME string, body io.Reader) (Media, error) {
	count, err := s.countScope(ctx, ScopeInstance, "")
	if err != nil {
		return Media{}, err
	}
	if count >= MaxInstancePresets {
		return Media{}, fmt.Errorf("%w: instance presets limited to %d", ErrQuota, MaxInstancePresets)
	}
	return s.upload(ctx, ScopeInstance, adminUserID, "", filename, declaredMIME, body, count)
}

func (s *Service) UploadMine(ctx context.Context, userID, filename, declaredMIME string, body io.Reader) (Media, error) {
	if strings.TrimSpace(userID) == "" {
		return Media{}, ErrForbidden
	}
	count, err := s.countScope(ctx, ScopeUser, userID)
	if err != nil {
		return Media{}, err
	}
	if count >= MaxUserLibrary {
		return Media{}, fmt.Errorf("%w: user library limited to %d", ErrQuota, MaxUserLibrary)
	}
	return s.upload(ctx, ScopeUser, userID, userID, filename, declaredMIME, body, 0)
}

func (s *Service) Delete(ctx context.Context, id, actorUserID string, actorIsAdmin bool) error {
	media, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	if media.Scope == ScopeInstance {
		if !actorIsAdmin {
			return ErrForbidden
		}
	} else if media.OwnerUserID == nil || *media.OwnerUserID != actorUserID {
		return ErrForbidden
	}

	// Auto-clear page drafts that reference this media URL / id.
	if err := s.clearBackgroundReferences(ctx, media); err != nil {
		slog.Warn("clear background references", "error", err, "media_id", id)
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM background_media WHERE id = ?`, id); err != nil {
		return err
	}
	// Best-effort: remove asset row (file GC not strict; assets package has no Delete yet).
	_, _ = s.db.ExecContext(ctx, `DELETE FROM assets WHERE id = ?`, media.AssetID)
	return nil
}

func (s *Service) upload(
	ctx context.Context,
	scope Scope,
	assetOwnerID string,
	userOwnerID string,
	filename, declaredMIME string,
	body io.Reader,
	sortOrder int,
) (Media, error) {
	processed, err := processUpload(ctx, s.root, filename, declaredMIME, body)
	if err != nil {
		return Media{}, err
	}
	defer os.Remove(processed.Path)

	file, err := os.Open(processed.Path)
	if err != nil {
		return Media{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return Media{}, err
	}
	ext := filepath.Ext(processed.Filename)
	if ext == "" {
		switch processed.MIMEType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "video/mp4":
			ext = ".mp4"
		case "video/webm":
			ext = ".webm"
		default:
			ext = ".bin"
		}
	}
	// Store via assets pipeline (local/S3). Kind remains "background".
	asset, err := s.assets.UploadPrepared(ctx, assetOwnerID, "background", ext, processed.MIMEType, file, info.Size())
	_ = file.Close()
	if err != nil {
		return Media{}, err
	}

	// Optional video poster as separate image asset.
	if processed.PosterPath != "" {
		if pf, openErr := os.Open(processed.PosterPath); openErr == nil {
			if pinfo, stErr := pf.Stat(); stErr == nil {
				if poster, upErr := s.assets.UploadPrepared(ctx, assetOwnerID, "background", ".jpg", "image/jpeg", pf, pinfo.Size()); upErr == nil {
					processed.PosterURL = poster.URL
				}
			}
			_ = pf.Close()
		}
		_ = os.Remove(processed.PosterPath)
	}

	id, err := identity.New("bgm")
	if err != nil {
		return Media{}, err
	}
	now := s.now().UTC()
	var owner any
	if scope == ScopeUser {
		owner = userOwnerID
	}
	var poster any
	if processed.PosterURL != "" {
		poster = processed.PosterURL
	}
	var duration any
	if processed.DurationMS > 0 {
		duration = processed.DurationMS
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO background_media(
			id, scope, owner_user_id, asset_id, media_kind, mime_type, url, poster_url,
			width, height, duration_ms, size_bytes, sort_order, enabled, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		id, string(scope), owner, asset.ID, string(processed.MediaKind), processed.MIMEType, asset.URL, poster,
		processed.Width, processed.Height, duration, asset.Size, sortOrder, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Media{}, fmt.Errorf("insert background media: %w", err)
	}

	media := Media{
		ID: id, Scope: scope, AssetID: asset.ID, MediaKind: processed.MediaKind,
		MIMEType: processed.MIMEType, URL: asset.URL, Width: processed.Width, Height: processed.Height,
		SizeBytes: asset.Size, SortOrder: sortOrder, Enabled: true, CreatedAt: now,
	}
	if scope == ScopeUser {
		media.OwnerUserID = &userOwnerID
	}
	if processed.PosterURL != "" {
		media.PosterURL = &processed.PosterURL
	}
	if processed.DurationMS > 0 {
		d := processed.DurationMS
		media.DurationMS = &d
	}
	return media, nil
}

func (s *Service) countScope(ctx context.Context, scope Scope, userID string) (int, error) {
	var n int
	var err error
	if scope == ScopeInstance {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM background_media WHERE scope = 'instance'`).Scan(&n)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM background_media WHERE scope = 'user' AND owner_user_id = ?`, userID).Scan(&n)
	}
	return n, err
}

func (s *Service) get(ctx context.Context, id string) (Media, error) {
	list, err := s.queryList(ctx, `
		SELECT id, scope, owner_user_id, asset_id, media_kind, mime_type, url, poster_url,
		       width, height, duration_ms, size_bytes, sort_order, enabled, created_at
		FROM background_media WHERE id = ?`, id)
	if err != nil {
		return Media{}, err
	}
	if len(list) == 0 {
		return Media{}, ErrNotFound
	}
	return list[0], nil
}

func (s *Service) queryList(ctx context.Context, query string, args ...any) ([]Media, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Media, 0)
	for rows.Next() {
		var m Media
		var owner sql.NullString
		var poster sql.NullString
		var duration sql.NullInt64
		var enabled int
		var created string
		var scope, kind string
		if err := rows.Scan(
			&m.ID, &scope, &owner, &m.AssetID, &kind, &m.MIMEType, &m.URL, &poster,
			&m.Width, &m.Height, &duration, &m.SizeBytes, &m.SortOrder, &enabled, &created,
		); err != nil {
			return nil, err
		}
		m.Scope = Scope(scope)
		m.MediaKind = MediaKind(kind)
		m.Enabled = enabled == 1
		if owner.Valid {
			m.OwnerUserID = &owner.String
		}
		if poster.Valid {
			m.PosterURL = &poster.String
		}
		if duration.Valid {
			d := int(duration.Int64)
			m.DurationMS = &d
		}
		m.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// clearBackgroundReferences sets appearance.background to none on draft pages that point at this media.
func (s *Service) clearBackgroundReferences(ctx context.Context, media Media) error {
	// Page settings live as JSON; rewrite with simple replace for url / mediaId patterns.
	rows, err := s.db.QueryContext(ctx, `SELECT id, settings_json, draft_revision FROM navigation_pages`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pageID, settings string
		var rev int64
		if err := rows.Scan(&pageID, &settings, &rev); err != nil {
			return err
		}
		if !strings.Contains(settings, media.URL) && !strings.Contains(settings, media.ID) {
			continue
		}
		// Surgical: if value field contains our URL or mediaId, reset background block via naive JSON patch.
		cleared, changed := clearBackgroundInSettingsJSON(settings, media.URL, media.ID)
		if !changed {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE navigation_pages SET settings_json = ?, draft_revision = draft_revision + 1, updated_at = ?
			WHERE id = ?`, cleared, time.Now().UTC().Format(time.RFC3339Nano), pageID); err != nil {
			return err
		}
	}
	return rows.Err()
}

func FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
