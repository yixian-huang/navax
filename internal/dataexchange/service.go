package dataexchange

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/security"
)

const (
	defaultPreviewTTL     = 15 * time.Minute
	defaultIdempotencyTTL = 24 * time.Hour
)

type Service struct {
	db             *sql.DB
	now            func() time.Time
	previewTTL     time.Duration
	idempotencyTTL time.Duration
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now, previewTTL: defaultPreviewTTL, idempotencyTTL: defaultIdempotencyTTL}
}

func (s *Service) MaxUploadBytes(ctx context.Context) (int64, error) {
	var maximum int64
	if err := s.db.QueryRowContext(ctx, "SELECT max_upload_bytes FROM system_settings WHERE id = 1").Scan(&maximum); err != nil {
		return 0, fmt.Errorf("read import upload limit: %w", err)
	}
	return maximum, nil
}

func (s *Service) Preview(ctx context.Context, actor navigation.Actor, pageID, format string, content []byte) (Preview, error) {
	maximum, err := s.MaxUploadBytes(ctx)
	if err != nil {
		return Preview{}, err
	}
	if int64(len(content)) > maximum {
		return Preview{}, ErrPayloadTooLarge
	}
	if len(content) == 0 {
		return Preview{}, fmt.Errorf("%w: import file is empty", ErrValidation)
	}
	if err := authorizePage(ctx, s.db, actor, pageID); err != nil {
		return Preview{}, err
	}
	parsed, err := parseImport(format, content)
	if err != nil {
		return Preview{}, err
	}
	existingURLs, err := pageURLs(ctx, s.db, pageID)
	if err != nil {
		return Preview{}, err
	}

	categories, totals := buildPreview(parsed, existingURLs)
	plainToken, tokenHash, err := security.NewToken()
	if err != nil {
		return Preview{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.previewTTL)
	payloadJSON, err := json.Marshal(previewPayload{Categories: categories})
	if err != nil {
		return Preview{}, fmt.Errorf("encode import preview: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO import_previews(token_hash, page_id, user_id, format, payload_json, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tokenHash, pageID, actor.UserID, format, payloadJSON, dbTime(expiresAt), dbTime(now),
	)
	if err != nil {
		return Preview{}, fmt.Errorf("store import preview: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM import_previews WHERE expires_at <= ?", dbTime(now))
	return Preview{ImportToken: plainToken, ExpiresAt: expiresAt, Categories: categories, Totals: totals}, nil
}

func buildPreview(parsed []parsedCategory, existingURLs map[string]struct{}) ([]ImportCategory, PreviewTotals) {
	categories := make([]ImportCategory, 0, len(parsed))
	totals := PreviewTotals{Categories: len(parsed)}
	seen := make(map[string]struct{}, len(existingURLs))
	for value := range existingURLs {
		seen[value] = struct{}{}
	}
	for _, sourceCategory := range parsed {
		category := ImportCategory{
			SourceID: sourceCategory.SourceID,
			Name:     strings.TrimSpace(sourceCategory.Name),
			Enabled:  sourceCategory.Enabled,
			Sites:    make([]ImportSite, 0, len(sourceCategory.Sites)),
		}
		categoryError := ""
		if category.Name == "" || len(category.Name) > 60 {
			categoryError = "分类名称长度必须为 1-60 个字符"
		}
		for _, sourceSite := range sourceCategory.Sites {
			totals.Sites++
			site := ImportSite{
				SourceID: sourceSite.SourceID,
				Title:    strings.TrimSpace(sourceSite.Title),
				URL:      strings.TrimSpace(sourceSite.URL),
				Enabled:  sourceSite.Enabled,
			}
			if categoryError != "" {
				site.Error = categoryError
			} else if site.Title == "" || len(site.Title) > 100 {
				site.Error = "站点标题长度必须为 1-100 个字符"
			} else {
				cleaned, err := cleanHTTPURL(site.URL)
				if err != nil {
					site.Error = err.Error()
				} else {
					site.URL = cleaned
					site.Valid = true
					if _, duplicate := seen[cleaned]; duplicate {
						site.Duplicate = true
						totals.Duplicates++
					} else {
						seen[cleaned] = struct{}{}
					}
				}
			}
			if !site.Valid {
				totals.Invalid++
			}
			category.Sites = append(category.Sites, site)
		}
		categories = append(categories, category)
	}
	return categories, totals
}

func (s *Service) Commit(ctx context.Context, actor navigation.Actor, pageID, idempotencyKey string, input CommitInput) (ImportResult, error) {
	if err := validateCommit(idempotencyKey, input); err != nil {
		return ImportResult{}, err
	}
	requestHash, err := hashCommitRequest(pageID, actor.UserID, input)
	if err != nil {
		return ImportResult{}, err
	}
	now := s.now().UTC()
	scope := "page-import:" + pageID
	var result ImportResult
	err = database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		cached, found, err := loadIdempotentResult(ctx, tx, scope, idempotencyKey, actor.UserID, requestHash, now)
		if err != nil {
			return err
		}
		if found {
			result = cached
			return nil
		}
		if err := authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		var currentRevision int
		if err := tx.QueryRowContext(ctx, "SELECT draft_revision FROM navigation_pages WHERE id = ?", pageID).Scan(&currentRevision); err != nil {
			return fmt.Errorf("read import page revision: %w", err)
		}
		if currentRevision != input.ExpectedRevision {
			return navigation.ErrPrecondition
		}

		payload, err := loadPreviewPayload(ctx, tx, actor, pageID, input.ImportToken, now)
		if err != nil {
			return err
		}
		selected, err := selectSites(payload, input.SelectedSiteIDs)
		if err != nil {
			return err
		}
		if input.SitesEnabled != nil {
			for index := range selected {
				selected[index].Site.Enabled = *input.SitesEnabled
			}
		}
		result, err = applyImport(ctx, tx, pageID, input.Mode, selected, currentRevision, now)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM import_previews WHERE token_hash = ?", security.HashToken(input.ImportToken)); err != nil {
			return fmt.Errorf("consume import preview: %w", err)
		}
		responseJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode idempotent import response: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO idempotency_records(
				scope, idempotency_key, actor_id, request_hash, response_status, response_json, created_at, expires_at
			) VALUES (?, ?, ?, ?, 200, ?, ?, ?)`,
			scope, idempotencyKey, actor.UserID, requestHash, responseJSON, dbTime(now), dbTime(now.Add(s.idempotencyTTL)),
		)
		if err != nil {
			return fmt.Errorf("store import idempotency result: %w", err)
		}
		return nil
	})
	return result, err
}

func validateCommit(idempotencyKey string, input CommitInput) error {
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return fmt.Errorf("%w: Idempotency-Key length must be between 16 and 128", ErrValidation)
	}
	if len(input.ImportToken) < 16 || len(input.ImportToken) > 256 {
		return fmt.Errorf("%w: importToken length must be between 16 and 256", ErrValidation)
	}
	if input.Mode != ModeMerge && input.Mode != ModeReplace {
		return fmt.Errorf("%w: mode must be merge or replace", ErrValidation)
	}
	if input.ExpectedRevision < 0 {
		return fmt.Errorf("%w: expectedRevision must not be negative", ErrValidation)
	}
	seen := make(map[string]struct{}, len(input.SelectedSiteIDs))
	for _, sourceID := range input.SelectedSiteIDs {
		if strings.TrimSpace(sourceID) == "" {
			return fmt.Errorf("%w: selectedSiteIds must not contain an empty ID", ErrValidation)
		}
		if _, duplicate := seen[sourceID]; duplicate {
			return fmt.Errorf("%w: selectedSiteIds must be unique", ErrValidation)
		}
		seen[sourceID] = struct{}{}
	}
	return nil
}

func hashCommitRequest(pageID, actorID string, input CommitInput) ([]byte, error) {
	payload, err := json.Marshal(struct {
		PageID  string      `json:"pageId"`
		ActorID string      `json:"actorId"`
		Input   CommitInput `json:"input"`
	}{PageID: pageID, ActorID: actorID, Input: input})
	if err != nil {
		return nil, fmt.Errorf("hash import request: %w", err)
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}

func loadIdempotentResult(ctx context.Context, tx *sql.Tx, scope, key, actorID string, requestHash []byte, now time.Time) (ImportResult, bool, error) {
	var storedActor sql.NullString
	var storedHash []byte
	var responseJSON, expiresAt string
	err := tx.QueryRowContext(ctx, `
		SELECT actor_id, request_hash, response_json, expires_at
		FROM idempotency_records WHERE scope = ? AND idempotency_key = ?`, scope, key,
	).Scan(&storedActor, &storedHash, &responseJSON, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ImportResult{}, false, nil
	}
	if err != nil {
		return ImportResult{}, false, fmt.Errorf("read import idempotency result: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return ImportResult{}, false, fmt.Errorf("parse import idempotency expiry: %w", err)
	}
	if !expires.After(now) {
		if _, err := tx.ExecContext(ctx, "DELETE FROM idempotency_records WHERE scope = ? AND idempotency_key = ?", scope, key); err != nil {
			return ImportResult{}, false, fmt.Errorf("delete expired import idempotency result: %w", err)
		}
		return ImportResult{}, false, nil
	}
	if !storedActor.Valid || storedActor.String != actorID || !bytes.Equal(storedHash, requestHash) {
		return ImportResult{}, false, fmt.Errorf("%w: idempotency key was used for a different request", ErrConflict)
	}
	var result ImportResult
	if err := json.Unmarshal([]byte(responseJSON), &result); err != nil {
		return ImportResult{}, false, fmt.Errorf("decode import idempotency result: %w", err)
	}
	return result, true, nil
}

func loadPreviewPayload(ctx context.Context, tx *sql.Tx, actor navigation.Actor, pageID, token string, now time.Time) (previewPayload, error) {
	var raw, expiresAt string
	err := tx.QueryRowContext(ctx, `
		SELECT payload_json, expires_at FROM import_previews
		WHERE token_hash = ? AND page_id = ? AND user_id = ?`, security.HashToken(token), pageID, actor.UserID,
	).Scan(&raw, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return previewPayload{}, ErrImportExpired
	}
	if err != nil {
		return previewPayload{}, fmt.Errorf("read import preview: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || !expires.After(now) {
		return previewPayload{}, ErrImportExpired
	}
	var payload previewPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return previewPayload{}, fmt.Errorf("decode import preview: %w", err)
	}
	return payload, nil
}

type selectedSite struct {
	CategoryName    string
	CategoryEnabled bool
	Site            ImportSite
}

func selectSites(payload previewPayload, selectedIDs []string) ([]selectedSite, error) {
	available := make(map[string]selectedSite)
	for _, category := range payload.Categories {
		for _, site := range category.Sites {
			available[site.SourceID] = selectedSite{
				CategoryName: category.Name, CategoryEnabled: category.Enabled, Site: site,
			}
		}
	}
	selected := make([]selectedSite, 0, len(selectedIDs))
	for _, sourceID := range selectedIDs {
		item, ok := available[sourceID]
		if !ok {
			return nil, fmt.Errorf("%w: selected site %q is not present in the preview", ErrValidation, sourceID)
		}
		selected = append(selected, item)
	}
	return selected, nil
}

func applyImport(ctx context.Context, tx *sql.Tx, pageID, mode string, selected []selectedSite, revision int, now time.Time) (ImportResult, error) {
	result := ImportResult{DraftRevision: revision}
	if mode == ModeReplace {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sites WHERE page_id = ?", pageID); err != nil {
			return result, fmt.Errorf("clear imported sites: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM categories WHERE page_id = ? AND is_uncategorized = 0", pageID); err != nil {
			return result, fmt.Errorf("clear imported categories: %w", err)
		}
	}

	categories, categoryCount, categorySort, err := loadCategoryMap(ctx, tx, pageID)
	if err != nil {
		return result, err
	}
	existingURLs, err := pageURLs(ctx, tx, pageID)
	if err != nil {
		return result, err
	}
	var siteCount, maxCategories, maxSites int
	if err := tx.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM sites WHERE page_id = ?), max_categories_per_page, max_sites_per_page
		FROM system_settings WHERE id = 1`, pageID).Scan(&siteCount, &maxCategories, &maxSites); err != nil {
		return result, fmt.Errorf("read import limits: %w", err)
	}

	siteSort := make(map[string]int)
	changed := mode == ModeReplace
	for _, item := range selected {
		if !item.Site.Valid {
			result.InvalidSkipped++
			continue
		}
		if _, duplicate := existingURLs[item.Site.URL]; duplicate {
			result.DuplicatesSkipped++
			continue
		}
		categoryKey := strings.ToLower(strings.TrimSpace(item.CategoryName))
		categoryID, exists := categories[categoryKey]
		if !exists {
			if categoryCount >= maxCategories {
				return result, fmt.Errorf("%w: category limit reached", ErrConflict)
			}
			categoryID, err = identity.New("cat")
			if err != nil {
				return result, err
			}
			categoryEnabled := 1
			if !item.CategoryEnabled {
				categoryEnabled = 0
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, enabled, created_at, updated_at)
				VALUES (?, ?, ?, '', ?, 0, ?, ?, ?)`,
				categoryID, pageID, strings.TrimSpace(item.CategoryName), categorySort, categoryEnabled, dbTime(now), dbTime(now),
			)
			if err != nil {
				return result, fmt.Errorf("create imported category: %w", err)
			}
			categories[categoryKey] = categoryID
			categoryCount++
			categorySort++
			result.CategoriesCreated++
		}
		if siteCount >= maxSites {
			return result, fmt.Errorf("%w: site limit reached", ErrConflict)
		}
		siteID, err := identity.New("site")
		if err != nil {
			return result, err
		}
		sortOrder, ok := siteSort[categoryID]
		if !ok {
			if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM sites WHERE category_id = ?", categoryID).Scan(&sortOrder); err != nil {
				return result, fmt.Errorf("read imported site order: %w", err)
			}
		}
		siteEnabled := 0
		if item.Site.Enabled {
			siteEnabled = 1
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sites(id, page_id, category_id, title, url, icon, description, sort_order, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, '', '', ?, ?, ?, ?)`,
			siteID, pageID, categoryID, item.Site.Title, item.Site.URL, sortOrder, siteEnabled, dbTime(now), dbTime(now),
		)
		if err != nil {
			return result, fmt.Errorf("create imported site: %w", err)
		}
		siteSort[categoryID] = sortOrder + 1
		existingURLs[item.Site.URL] = struct{}{}
		siteCount++
		result.SitesCreated++
		changed = true
	}

	if changed {
		update, err := tx.ExecContext(ctx, `
			UPDATE navigation_pages SET draft_revision = draft_revision + 1, draft_updated_at = ?, updated_at = ?
			WHERE id = ? AND draft_revision = ?`, dbTime(now), dbTime(now), pageID, revision)
		if err != nil {
			return result, fmt.Errorf("advance imported page revision: %w", err)
		}
		affected, err := update.RowsAffected()
		if err != nil {
			return result, fmt.Errorf("read imported page revision result: %w", err)
		}
		if affected != 1 {
			return result, navigation.ErrPrecondition
		}
		result.DraftRevision++
	}
	return result, nil
}

func loadCategoryMap(ctx context.Context, q queryer, pageID string) (map[string]string, int, int, error) {
	rows, err := q.QueryContext(ctx, "SELECT id, name, sort_order FROM categories WHERE page_id = ? ORDER BY sort_order, id", pageID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list import categories: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	count := 0
	nextSort := 0
	for rows.Next() {
		var id, name string
		var sortOrder int
		if err := rows.Scan(&id, &name, &sortOrder); err != nil {
			return nil, 0, 0, fmt.Errorf("scan import category: %w", err)
		}
		result[strings.ToLower(name)] = id
		count++
		if sortOrder >= nextSort {
			nextSort = sortOrder + 1
		}
	}
	return result, count, nextSort, rows.Err()
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func pageURLs(ctx context.Context, q queryer, pageID string) (map[string]struct{}, error) {
	rows, err := q.QueryContext(ctx, "SELECT url FROM sites WHERE page_id = ?", pageID)
	if err != nil {
		return nil, fmt.Errorf("list page URLs: %w", err)
	}
	defer rows.Close()
	urls := make(map[string]struct{})
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan page URL: %w", err)
		}
		urls[value] = struct{}{}
	}
	return urls, rows.Err()
}

func authorizePage(ctx context.Context, q queryer, actor navigation.Actor, pageID string) error {
	var kind navigation.PageKind
	var ownerID sql.NullString
	err := q.QueryRowContext(ctx, "SELECT kind, owner_id FROM navigation_pages WHERE id = ?", pageID).Scan(&kind, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return navigation.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("authorize import page: %w", err)
	}
	if kind == navigation.PageKindSystem {
		if !actor.IsAdmin() {
			return navigation.ErrForbidden
		}
		return nil
	}
	if !ownerID.Valid || actor.UserID == "" || ownerID.String != actor.UserID {
		return navigation.ErrForbidden
	}
	return nil
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
