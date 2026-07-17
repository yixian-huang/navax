package navigation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *SQLStore) CurrentPage(ctx context.Context, actor Actor, kind PageKind) (Page, error) {
	var pageID string
	var err error
	if kind == PageKindPersonal {
		err = s.db.QueryRowContext(ctx, "SELECT id FROM navigation_pages WHERE kind = 'personal' AND owner_id = ?", actor.UserID).Scan(&pageID)
	} else {
		if !actor.IsAdmin() {
			return Page{}, ErrForbidden
		}
		err = s.db.QueryRowContext(ctx, "SELECT id FROM navigation_pages WHERE kind = 'system'").Scan(&pageID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, fmt.Errorf("find current navigation page: %w", err)
	}
	return s.pageDraft(ctx, s.db, actor, pageID)
}

func (s *SQLStore) PageDraft(ctx context.Context, actor Actor, pageID string) (Page, error) {
	return s.pageDraft(ctx, s.db, actor, pageID)
}

func (s *SQLStore) pageDraft(ctx context.Context, q queryer, actor Actor, pageID string) (Page, error) {
	page, err := loadPageBase(ctx, q, pageID)
	if err != nil {
		return Page{}, err
	}
	if err := authorize(actor, page); err != nil {
		return Page{}, err
	}
	page.Categories, err = loadCategories(ctx, q, pageID)
	if err != nil {
		return Page{}, err
	}
	page.Sites, err = loadSites(ctx, q, pageID, "", "")
	if err != nil {
		return Page{}, err
	}
	page.Publication, err = loadPublication(ctx, q, pageID, "")
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

func (s *SQLStore) UpdatePage(ctx context.Context, actor Actor, pageID string, patch PagePatch, now time.Time) (Page, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		page, err := loadPageBase(ctx, tx, pageID)
		if err != nil {
			return err
		}
		if err := authorize(actor, page); err != nil {
			return err
		}
		if page.DraftRevision != patch.ExpectedRevision {
			return ErrPrecondition
		}
		title, description := page.Title, page.Description
		if patch.Title != nil {
			title = *patch.Title
		}
		if patch.Description != nil {
			description = *patch.Description
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE navigation_pages
			SET title = ?, description = ?, draft_revision = draft_revision + 1,
			    draft_updated_at = ?, updated_at = ?
			WHERE id = ? AND draft_revision = ?`,
			title, description, dbTime(now), dbTime(now), pageID, patch.ExpectedRevision,
		)
		return expectRevisionResult(result, err)
	})
	if err != nil {
		return Page{}, err
	}
	return s.pageDraft(ctx, s.db, actor, pageID)
}

func (s *SQLStore) Categories(ctx context.Context, actor Actor, pageID string) ([]Category, error) {
	if err := s.authorizePage(ctx, s.db, actor, pageID); err != nil {
		return nil, err
	}
	return loadCategories(ctx, s.db, pageID)
}

func (s *SQLStore) CreateCategory(ctx context.Context, actor Actor, pageID string, category Category, now time.Time) (Category, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		var count, maximum, sortOrder int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*), (SELECT max_categories_per_page FROM system_settings WHERE id = 1),
			       COALESCE(MAX(sort_order), -1) + 1
			FROM categories WHERE page_id = ?`, pageID).Scan(&count, &maximum, &sortOrder); err != nil {
			return fmt.Errorf("count navigation categories: %w", err)
		}
		if count >= maximum {
			return fmt.Errorf("%w: category limit reached", ErrConflict)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
			category.ID, pageID, category.Name, category.Icon, sortOrder, dbTime(now), dbTime(now),
		)
		if err != nil {
			return mapWriteError(err)
		}
		return touchDraft(ctx, tx, pageID, now)
	})
	if err != nil {
		return Category{}, err
	}
	return loadCategory(ctx, s.db, pageID, category.ID)
}

func (s *SQLStore) UpdateCategory(ctx context.Context, actor Actor, pageID, categoryID string, patch CategoryPatch, now time.Time) (Category, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		current, err := loadCategory(ctx, tx, pageID, categoryID)
		if err != nil {
			return err
		}
		name, icon := current.Name, current.Icon
		if patch.Name != nil {
			name = *patch.Name
		}
		if patch.Icon != nil {
			icon = *patch.Icon
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE categories SET name = ?, icon = ?, updated_at = ? WHERE id = ? AND page_id = ?",
			name, icon, dbTime(now), categoryID, pageID,
		)
		if err != nil {
			return mapWriteError(err)
		}
		return touchDraft(ctx, tx, pageID, now)
	})
	if err != nil {
		return Category{}, err
	}
	return loadCategory(ctx, s.db, pageID, categoryID)
}

func (s *SQLStore) DeleteCategory(ctx context.Context, actor Actor, pageID, categoryID string, mode DeleteCategoryMode, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		category, err := loadCategory(ctx, tx, pageID, categoryID)
		if err != nil {
			return err
		}
		if category.IsUncategorized {
			return ErrUncategorized
		}
		var siteCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sites WHERE page_id = ? AND category_id = ?", pageID, categoryID).Scan(&siteCount); err != nil {
			return fmt.Errorf("count category sites: %w", err)
		}
		if siteCount > 0 && mode == DeleteCategoryRejectIfNotEmpty {
			return ErrCategoryNotEmpty
		}
		if siteCount > 0 && mode == DeleteCategoryMoveSites {
			var targetID string
			var nextOrder int
			if err := tx.QueryRowContext(ctx, `
				SELECT id, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM sites WHERE page_id = ? AND category_id = categories.id)
				FROM categories WHERE page_id = ? AND is_uncategorized = 1`, pageID, pageID).Scan(&targetID, &nextOrder); err != nil {
				return fmt.Errorf("find uncategorized category: %w", err)
			}
			rows, err := tx.QueryContext(ctx, "SELECT id FROM sites WHERE page_id = ? AND category_id = ? ORDER BY sort_order, id", pageID, categoryID)
			if err != nil {
				return fmt.Errorf("list category sites: %w", err)
			}
			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					_ = rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			for index, id := range ids {
				if _, err := tx.ExecContext(ctx,
					"UPDATE sites SET category_id = ?, sort_order = ?, updated_at = ? WHERE id = ? AND page_id = ?",
					targetID, nextOrder+index, dbTime(now), id, pageID,
				); err != nil {
					return fmt.Errorf("move category site: %w", err)
				}
			}
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM categories WHERE id = ? AND page_id = ?", categoryID, pageID); err != nil {
			return fmt.Errorf("delete navigation category: %w", err)
		}
		if err := normalizeCategoryOrder(ctx, tx, pageID, now); err != nil {
			return err
		}
		return touchDraft(ctx, tx, pageID, now)
	})
}

func (s *SQLStore) Sites(ctx context.Context, actor Actor, pageID, categoryID, search string) ([]Site, error) {
	if err := s.authorizePage(ctx, s.db, actor, pageID); err != nil {
		return nil, err
	}
	return loadSites(ctx, s.db, pageID, categoryID, search)
}

func (s *SQLStore) CreateSite(ctx context.Context, actor Actor, pageID string, site Site, now time.Time) (Site, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		if _, err := loadCategory(ctx, tx, pageID, site.CategoryID); err != nil {
			return err
		}
		var count, maximum, sortOrder int
		if err := tx.QueryRowContext(ctx, `
			SELECT (SELECT COUNT(*) FROM sites WHERE page_id = ?),
			       (SELECT max_sites_per_page FROM system_settings WHERE id = 1),
			       COALESCE(MAX(sort_order), -1) + 1
			FROM sites WHERE page_id = ? AND category_id = ?`, pageID, pageID, site.CategoryID).Scan(&count, &maximum, &sortOrder); err != nil {
			return fmt.Errorf("count navigation sites: %w", err)
		}
		if count >= maximum {
			return fmt.Errorf("%w: site limit reached", ErrConflict)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sites(id, page_id, category_id, title, url, icon, description, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			site.ID, pageID, site.CategoryID, site.Title, site.URL, site.Icon, site.Description, sortOrder, dbTime(now), dbTime(now),
		)
		if err != nil {
			return mapWriteError(err)
		}
		return touchDraft(ctx, tx, pageID, now)
	})
	if err != nil {
		return Site{}, err
	}
	return loadSite(ctx, s.db, pageID, site.ID)
}

func (s *SQLStore) UpdateSite(ctx context.Context, actor Actor, pageID, siteID string, patch SitePatch, now time.Time) (Site, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		current, err := loadSite(ctx, tx, pageID, siteID)
		if err != nil {
			return err
		}
		categoryID, title, rawURL, icon, description := current.CategoryID, current.Title, current.URL, current.Icon, current.Description
		sortOrder := current.SortOrder
		if patch.CategoryID != nil && *patch.CategoryID != current.CategoryID {
			if _, err := loadCategory(ctx, tx, pageID, *patch.CategoryID); err != nil {
				return err
			}
			categoryID = *patch.CategoryID
			if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM sites WHERE page_id = ? AND category_id = ?", pageID, categoryID).Scan(&sortOrder); err != nil {
				return err
			}
		}
		if patch.Title != nil {
			title = *patch.Title
		}
		if patch.URL != nil {
			rawURL = *patch.URL
		}
		if patch.Icon != nil {
			icon = *patch.Icon
		}
		if patch.Description != nil {
			description = *patch.Description
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE sites SET category_id = ?, title = ?, url = ?, icon = ?, description = ?, sort_order = ?, updated_at = ?
			WHERE id = ? AND page_id = ?`,
			categoryID, title, rawURL, icon, description, sortOrder, dbTime(now), siteID, pageID,
		)
		if err != nil {
			return mapWriteError(err)
		}
		if categoryID != current.CategoryID {
			if err := normalizeSiteOrder(ctx, tx, pageID, current.CategoryID, now); err != nil {
				return err
			}
		}
		return touchDraft(ctx, tx, pageID, now)
	})
	if err != nil {
		return Site{}, err
	}
	return loadSite(ctx, s.db, pageID, siteID)
}

func (s *SQLStore) DeleteSite(ctx context.Context, actor Actor, pageID, siteID string, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		site, err := loadSite(ctx, tx, pageID, siteID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sites WHERE id = ? AND page_id = ?", siteID, pageID); err != nil {
			return fmt.Errorf("delete navigation site: %w", err)
		}
		if err := normalizeSiteOrder(ctx, tx, pageID, site.CategoryID, now); err != nil {
			return err
		}
		return touchDraft(ctx, tx, pageID, now)
	})
}

func (s *SQLStore) ReplaceContentOrder(ctx context.Context, actor Actor, pageID string, expectedRevision int, order []CategoryOrder, now time.Time) (int, error) {
	newRevision := 0
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		page, err := loadPageBase(ctx, tx, pageID)
		if err != nil {
			return err
		}
		if err := authorize(actor, page); err != nil {
			return err
		}
		if page.DraftRevision != expectedRevision {
			return ErrPrecondition
		}
		categories, err := loadCategories(ctx, tx, pageID)
		if err != nil {
			return err
		}
		sites, err := loadSites(ctx, tx, pageID, "", "")
		if err != nil {
			return err
		}
		if err := validateCompleteOrder(categories, sites, order); err != nil {
			return err
		}
		if err := bumpExpected(ctx, tx, pageID, expectedRevision, now); err != nil {
			return err
		}
		for categoryIndex, category := range order {
			if _, err := tx.ExecContext(ctx, "UPDATE categories SET sort_order = ?, updated_at = ? WHERE id = ? AND page_id = ?", categoryIndex, dbTime(now), category.ID, pageID); err != nil {
				return fmt.Errorf("reorder category: %w", err)
			}
			for siteIndex, siteID := range category.SiteIDs {
				if _, err := tx.ExecContext(ctx, "UPDATE sites SET category_id = ?, sort_order = ?, updated_at = ? WHERE id = ? AND page_id = ?", category.ID, siteIndex, dbTime(now), siteID, pageID); err != nil {
					return fmt.Errorf("reorder site: %w", err)
				}
			}
		}
		newRevision = expectedRevision + 1
		return nil
	})
	return newRevision, err
}

func (s *SQLStore) Settings(ctx context.Context, actor Actor, pageID string) (PageSettings, error) {
	if err := s.authorizePage(ctx, s.db, actor, pageID); err != nil {
		return PageSettings{}, err
	}
	return loadSettings(ctx, s.db, pageID)
}

func (s *SQLStore) ReplaceSettings(ctx context.Context, actor Actor, pageID string, expectedRevision int, settings PageSettings, now time.Time) (PageSettings, error) {
	payload, err := json.Marshal(settings)
	if err != nil {
		return PageSettings{}, fmt.Errorf("encode page settings: %w", err)
	}
	err = database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		page, err := loadPageBase(ctx, tx, pageID)
		if err != nil {
			return err
		}
		if err := authorize(actor, page); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE navigation_pages SET settings_json = ?, draft_revision = draft_revision + 1,
			    draft_updated_at = ?, updated_at = ? WHERE id = ? AND draft_revision = ?`,
			string(payload), dbTime(now), dbTime(now), pageID, expectedRevision,
		)
		return expectRevisionResult(result, err)
	})
	if err != nil {
		return PageSettings{}, err
	}
	return settings, nil
}

func (s *SQLStore) Publication(ctx context.Context, actor Actor, pageID, publicBaseURL string) (Publication, error) {
	if err := s.authorizePage(ctx, s.db, actor, pageID); err != nil {
		return Publication{}, err
	}
	return loadPublication(ctx, s.db, pageID, publicBaseURL)
}

func (s *SQLStore) ReplacePublication(ctx context.Context, actor Actor, pageID string, input PublicationSettingsInput, publicBaseURL string, now time.Time) (Publication, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		if err := ensurePublishedSlugAvailable(ctx, tx, pageID, input.Slug); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE page_publications
			SET visibility = ?, slug = ?, show_author = ?, seo_title = ?, seo_description = ?,
			    current_snapshot_id = CASE WHEN ? = 'private' THEN NULL ELSE current_snapshot_id END,
			    updated_at = ?
			WHERE page_id = ?`,
			input.Visibility, input.Slug, input.ShowAuthor, input.SEOTitle, input.SEODescription,
			input.Visibility, dbTime(now), pageID,
		)
		return mapWriteError(err)
	})
	if err != nil {
		return Publication{}, err
	}
	return loadPublication(ctx, s.db, pageID, publicBaseURL)
}

func (s *SQLStore) Preview(ctx context.Context, actor Actor, pageID, _ string, now time.Time) (PublishedPage, error) {
	page, err := s.pageDraft(ctx, s.db, actor, pageID)
	if err != nil {
		return PublishedPage{}, err
	}
	visibility := page.Publication.Visibility
	if visibility == VisibilityPrivate {
		visibility = VisibilityUnlisted
	}
	published := buildPublishedPage(page, "preview_"+page.ID, visibility, page.Publication.Slug, now)
	if err := attachApprovedSubdomain(ctx, s.db, page, &published); err != nil {
		return PublishedPage{}, err
	}
	published.ETag = makeETag(published)
	return published, nil
}

func (s *SQLStore) Publish(ctx context.Context, actor Actor, pageID string, expectedRevision int, publicBaseURL string, now time.Time) (Publication, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		page, err := s.pageDraft(ctx, tx, actor, pageID)
		if err != nil {
			return err
		}
		if page.DraftRevision != expectedRevision {
			return ErrPrecondition
		}
		if page.Publication.Visibility == VisibilityPrivate {
			return validation("visibility", "private pages cannot be published")
		}
		if err := ensurePublishedSlugAvailable(ctx, tx, pageID, page.Publication.Slug); err != nil {
			return err
		}
		snapshotID, err := identity.New("snp")
		if err != nil {
			return err
		}
		published := buildPublishedPage(page, snapshotID, page.Publication.Visibility, page.Publication.Slug, now)
		if err := attachApprovedSubdomain(ctx, tx, page, &published); err != nil {
			return err
		}
		published.ETag = makeETag(published)
		payload, err := json.Marshal(published)
		if err != nil {
			return fmt.Errorf("encode published navigation snapshot: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO published_snapshots(id, page_id, draft_revision, slug, visibility, payload_json, etag, published_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshotID, pageID, page.DraftRevision, page.Publication.Slug, page.Publication.Visibility,
			string(payload), published.ETag, dbTime(now),
		); err != nil {
			return mapWriteError(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE page_publications SET current_snapshot_id = ?, updated_at = ? WHERE page_id = ?", snapshotID, dbTime(now), pageID); err != nil {
			return fmt.Errorf("activate navigation snapshot: %w", err)
		}
		return nil
	})
	if err != nil {
		return Publication{}, err
	}
	return loadPublication(ctx, s.db, pageID, publicBaseURL)
}

func (s *SQLStore) Unpublish(ctx context.Context, actor Actor, pageID, publicBaseURL string, now time.Time) (Publication, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := s.authorizePage(ctx, tx, actor, pageID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE page_publications SET visibility = 'private', current_snapshot_id = NULL, updated_at = ?
			WHERE page_id = ?`, dbTime(now), pageID)
		if err != nil {
			return fmt.Errorf("unpublish navigation page: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Publication{}, err
	}
	return loadPublication(ctx, s.db, pageID, publicBaseURL)
}

func (s *SQLStore) PublicHome(ctx context.Context) (PublishedPage, error) {
	return loadPublicSnapshot(ctx, s.db, `
		SELECT s.payload_json FROM published_snapshots s
		JOIN page_publications p ON p.current_snapshot_id = s.id
		JOIN navigation_pages n ON n.id = p.page_id
		WHERE n.kind = 'system' AND s.visibility IN ('unlisted', 'public')`)
}

func (s *SQLStore) PublicHomeForHost(ctx context.Context, host string) (PublishedPage, error) {
	normalizedHost := strings.ToLower(strings.Trim(host, "."))
	page, err := loadPublicSnapshot(ctx, s.db, `
		SELECT ps.payload_json FROM subdomain_requests sr
		JOIN navigation_pages n ON n.owner_id = sr.user_id AND n.kind = 'personal'
		JOIN page_publications pp ON pp.page_id = n.id
		JOIN published_snapshots ps ON ps.id = pp.current_snapshot_id
		JOIN users u ON u.id = sr.user_id
		WHERE sr.status = 'approved'
		  AND (sr.full_domain = ? COLLATE NOCASE OR sr.custom_domain = ? COLLATE NOCASE)
		  AND ps.visibility IN ('unlisted', 'public') AND u.status = 'active'
		ORDER BY sr.reviewed_at DESC LIMIT 1`, normalizedHost, normalizedHost)
	if err == nil {
		return page, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return PublishedPage{}, err
	}
	var rootDomain sql.NullString
	if settingsErr := s.db.QueryRowContext(ctx, "SELECT root_domain FROM system_settings WHERE id = 1").Scan(&rootDomain); settingsErr != nil {
		return PublishedPage{}, settingsErr
	}
	root := strings.ToLower(strings.Trim(rootDomain.String, "."))
	if root != "" && host != root && strings.HasSuffix(host, "."+root) {
		return PublishedPage{}, ErrNotFound
	}
	return s.PublicHome(ctx)
}

func (s *SQLStore) PublicBySlug(ctx context.Context, slug string) (PublishedPage, error) {
	return loadPublicSnapshot(ctx, s.db, `
		SELECT s.payload_json FROM published_snapshots s
		JOIN page_publications p ON p.current_snapshot_id = s.id
		JOIN navigation_pages n ON n.id = p.page_id
		LEFT JOIN users u ON u.id = n.owner_id
		WHERE s.slug = ? COLLATE NOCASE AND s.visibility IN ('unlisted', 'public')
		  AND (n.kind = 'system' OR u.status = 'active')`, slug)
}

func (s *SQLStore) authorizePage(ctx context.Context, q queryer, actor Actor, pageID string) error {
	page, err := loadPageBase(ctx, q, pageID)
	if err != nil {
		return err
	}
	return authorize(actor, page)
}

func authorize(actor Actor, page Page) error {
	if page.Kind == PageKindSystem {
		if !actor.IsAdmin() {
			return ErrForbidden
		}
		return nil
	}
	if page.OwnerID == nil || actor.UserID == "" || actor.UserID != *page.OwnerID {
		return ErrForbidden
	}
	return nil
}

func loadPageBase(ctx context.Context, q queryer, pageID string) (Page, error) {
	var page Page
	var ownerID sql.NullString
	var settingsJSON, draftUpdatedAt, createdAt, updatedAt string
	err := q.QueryRowContext(ctx, `
		SELECT p.id, p.kind, p.owner_id, COALESCE(u.username, ss.instance_name), COALESCE(u.avatar_url, ''),
		       p.title, p.description, p.draft_revision, p.settings_json,
		       p.draft_updated_at, p.created_at, p.updated_at
		FROM navigation_pages p
		LEFT JOIN users u ON u.id = p.owner_id
		CROSS JOIN system_settings ss
		WHERE p.id = ? AND ss.id = 1`, pageID).Scan(
		&page.ID, &page.Kind, &ownerID, &page.OwnerName, &page.OwnerAvatarURL,
		&page.Title, &page.Description, &page.DraftRevision, &settingsJSON,
		&draftUpdatedAt, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, fmt.Errorf("read navigation page: %w", err)
	}
	if ownerID.Valid {
		page.OwnerID = &ownerID.String
	}
	if err := json.Unmarshal([]byte(settingsJSON), &page.Settings); err != nil {
		return Page{}, fmt.Errorf("decode navigation page settings: %w", err)
	}
	if page.DraftUpdatedAt, err = parseDBTime(draftUpdatedAt); err != nil {
		return Page{}, err
	}
	if page.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return Page{}, err
	}
	if page.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return Page{}, err
	}
	return page, nil
}

func loadCategories(ctx context.Context, q queryer, pageID string) ([]Category, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at
		FROM categories WHERE page_id = ? ORDER BY sort_order, id`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list navigation categories: %w", err)
	}
	defer rows.Close()
	categories := make([]Category, 0)
	for rows.Next() {
		category, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func loadCategory(ctx context.Context, q queryer, pageID, categoryID string) (Category, error) {
	category, err := scanCategory(q.QueryRowContext(ctx, `
		SELECT id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at
		FROM categories WHERE page_id = ? AND id = ?`, pageID, categoryID))
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return category, err
}

type rowScanner interface{ Scan(...any) error }

func scanCategory(row rowScanner) (Category, error) {
	var category Category
	var createdAt, updatedAt string
	if err := row.Scan(&category.ID, &category.PageID, &category.Name, &category.Icon, &category.SortOrder, &category.IsUncategorized, &createdAt, &updatedAt); err != nil {
		return Category{}, err
	}
	var err error
	category.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return Category{}, err
	}
	category.UpdatedAt, err = parseDBTime(updatedAt)
	return category, err
}

func loadSites(ctx context.Context, q queryer, pageID, categoryID, search string) ([]Site, error) {
	query := `SELECT id, page_id, category_id, title, url, icon, description, sort_order, created_at, updated_at FROM sites WHERE page_id = ?`
	args := []any{pageID}
	if categoryID != "" {
		query += " AND category_id = ?"
		args = append(args, categoryID)
	}
	if search != "" {
		query += " AND (title LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\' OR url LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLike(search) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	query += " ORDER BY category_id, sort_order, id"
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list navigation sites: %w", err)
	}
	defer rows.Close()
	sites := make([]Site, 0)
	for rows.Next() {
		site, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func loadSite(ctx context.Context, q queryer, pageID, siteID string) (Site, error) {
	site, err := scanSite(q.QueryRowContext(ctx, `
		SELECT id, page_id, category_id, title, url, icon, description, sort_order, created_at, updated_at
		FROM sites WHERE page_id = ? AND id = ?`, pageID, siteID))
	if errors.Is(err, sql.ErrNoRows) {
		return Site{}, ErrNotFound
	}
	return site, err
}

func scanSite(row rowScanner) (Site, error) {
	var site Site
	var createdAt, updatedAt string
	if err := row.Scan(&site.ID, &site.PageID, &site.CategoryID, &site.Title, &site.URL, &site.Icon, &site.Description, &site.SortOrder, &createdAt, &updatedAt); err != nil {
		return Site{}, err
	}
	var err error
	site.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return Site{}, err
	}
	site.UpdatedAt, err = parseDBTime(updatedAt)
	return site, err
}

func loadSettings(ctx context.Context, q queryer, pageID string) (PageSettings, error) {
	var raw string
	if err := q.QueryRowContext(ctx, "SELECT settings_json FROM navigation_pages WHERE id = ?", pageID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PageSettings{}, ErrNotFound
		}
		return PageSettings{}, err
	}
	var settings PageSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return PageSettings{}, fmt.Errorf("decode page settings: %w", err)
	}
	return settings, nil
}

func loadPublication(ctx context.Context, q queryer, pageID, publicBaseURL string) (Publication, error) {
	var publication Publication
	var currentSnapshotID, snapshotID, snapshotPublishedAt, snapshotSlug, snapshotVisibility sql.NullString
	var publishedRevision sql.NullInt64
	var publicationUpdatedAt string
	var pageKind PageKind
	var draftRevision int
	err := q.QueryRowContext(ctx, `
		SELECT p.visibility, p.slug, p.show_author, p.seo_title, p.seo_description,
		       p.current_snapshot_id, p.updated_at,
		       s.id, s.draft_revision, s.published_at, s.slug, s.visibility,
		       n.kind, n.draft_revision
		FROM page_publications p
		JOIN navigation_pages n ON n.id = p.page_id
		LEFT JOIN published_snapshots s ON s.id = p.current_snapshot_id
		WHERE p.page_id = ?`, pageID).Scan(
		&publication.Visibility, &publication.Slug, &publication.ShowAuthor, &publication.SEOTitle, &publication.SEODescription,
		&currentSnapshotID, &publicationUpdatedAt, &snapshotID, &publishedRevision, &snapshotPublishedAt, &snapshotSlug, &snapshotVisibility,
		&pageKind, &draftRevision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrNotFound
	}
	if err != nil {
		return Publication{}, fmt.Errorf("read navigation publication: %w", err)
	}
	publication.Published = currentSnapshotID.Valid && snapshotID.Valid
	// Unpublished drafts must never advertise index; only a live public snapshot may.
	publication.Robots = "noindex,follow"
	if publication.Published {
		publication.SnapshotID = &snapshotID.String
		revision := int(publishedRevision.Int64)
		publication.PublishedRevision = &revision
		publishedAt, err := parseDBTime(snapshotPublishedAt.String)
		if err != nil {
			return Publication{}, err
		}
		publication.PublishedAt = &publishedAt
		if Visibility(snapshotVisibility.String) == VisibilityPublic {
			publication.Robots = "index,follow"
		}
		canonical := strings.TrimRight(publicBaseURL, "/") + "/u/" + snapshotSlug.String
		if pageKind == PageKindSystem {
			canonical = strings.TrimRight(publicBaseURL, "/") + "/"
		}
		if publicBaseURL != "" {
			publication.CanonicalURL = &canonical
		}
		settingsUpdatedAt, err := parseDBTime(publicationUpdatedAt)
		if err != nil {
			return Publication{}, err
		}
		publication.HasUnpublishedChanges = draftRevision != revision || settingsUpdatedAt.After(publishedAt)
	}
	return publication, nil
}

func buildPublishedPage(page Page, snapshotID string, visibility Visibility, slug string, publishedAt time.Time) PublishedPage {
	byCategory := make(map[string][]Site, len(page.Categories))
	for _, site := range page.Sites {
		byCategory[site.CategoryID] = append(byCategory[site.CategoryID], site)
	}
	categories := make([]PublicCategory, 0, len(page.Categories))
	for _, category := range page.Categories {
		sites := byCategory[category.ID]
		if sites == nil {
			sites = make([]Site, 0)
		}
		categories = append(categories, PublicCategory{Category: category, Sites: sites})
	}
	owner := PublishedOwner{Visible: page.Publication.ShowAuthor}
	if owner.Visible {
		owner.Name = page.OwnerName
		owner.AvatarURL = page.OwnerAvatarURL
	}
	ogImage := ""
	if page.Settings.Appearance.Background.Type == "image" {
		ogImage = strings.TrimSpace(page.Settings.Appearance.Background.Value)
	}
	return PublishedPage{
		ID: page.ID, SnapshotID: snapshotID, Kind: page.Kind, Title: page.Title, Description: page.Description,
		SEOTitle: page.Publication.SEOTitle, SEODescription: page.Publication.SEODescription, OGImage: ogImage,
		Slug: slug, Visibility: visibility,
		Owner:    owner,
		Settings: page.Settings, Categories: categories, PublishedAt: publishedAt,
	}
}

func attachApprovedSubdomain(ctx context.Context, q queryer, page Page, published *PublishedPage) error {
	if page.OwnerID == nil {
		return nil
	}
	var domain string
	err := q.QueryRowContext(ctx, `
		SELECT full_domain FROM subdomain_requests
		WHERE user_id = ? AND status = 'approved'
		ORDER BY reviewed_at DESC LIMIT 1`, *page.OwnerID).Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read approved subdomain: %w", err)
	}
	published.Subdomain = &domain
	return nil
}

func makeETag(page PublishedPage) string {
	page.ETag = ""
	payload, _ := json.Marshal(page)
	sum := sha256.Sum256(payload)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func loadPublicSnapshot(ctx context.Context, q queryer, query string, args ...any) (PublishedPage, error) {
	var payload string
	if err := q.QueryRowContext(ctx, query, args...).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PublishedPage{}, ErrNotFound
		}
		return PublishedPage{}, fmt.Errorf("read public navigation snapshot: %w", err)
	}
	var page PublishedPage
	if err := json.Unmarshal([]byte(payload), &page); err != nil {
		return PublishedPage{}, fmt.Errorf("decode public navigation snapshot: %w", err)
	}
	// Subdomain is live state, not immutable snapshot content. Clear any embedded
	// value then re-attach the currently approved domain (if any) so revoke is
	// visible without requiring a republish.
	page.Subdomain = nil
	if err := attachApprovedSubdomainByPageID(ctx, q, page.ID, &page); err != nil {
		return PublishedPage{}, err
	}
	page.ETag = makeETag(page)
	return page, nil
}

func attachApprovedSubdomainByPageID(ctx context.Context, q queryer, pageID string, published *PublishedPage) error {
	var domain string
	err := q.QueryRowContext(ctx, `
		SELECT sr.full_domain FROM subdomain_requests sr
		JOIN navigation_pages n ON n.owner_id = sr.user_id AND n.id = ? AND n.kind = 'personal'
		WHERE sr.status = 'approved'
		ORDER BY sr.reviewed_at DESC LIMIT 1`, pageID).Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read approved subdomain for public page: %w", err)
	}
	published.Subdomain = &domain
	return nil
}

func ensurePublishedSlugAvailable(ctx context.Context, q queryer, pageID, slug string) error {
	var otherPageID string
	err := q.QueryRowContext(ctx, `
		SELECT p.page_id FROM page_publications p
		JOIN published_snapshots s ON s.id = p.current_snapshot_id
		WHERE s.slug = ? COLLATE NOCASE AND p.page_id <> ? LIMIT 1`, slug, pageID).Scan(&otherPageID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check published slug: %w", err)
	}
	return fmt.Errorf("%w: slug is already published", ErrConflict)
}

func validateCompleteOrder(categories []Category, sites []Site, order []CategoryOrder) error {
	if len(categories) != len(order) {
		return ErrInvalidOrder
	}
	categorySet := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		categorySet[category.ID] = struct{}{}
	}
	siteSet := make(map[string]struct{}, len(sites))
	for _, site := range sites {
		siteSet[site.ID] = struct{}{}
	}
	seenCategories := make(map[string]struct{}, len(order))
	seenSites := make(map[string]struct{}, len(sites))
	for _, category := range order {
		if _, exists := categorySet[category.ID]; !exists {
			return ErrInvalidOrder
		}
		if _, duplicate := seenCategories[category.ID]; duplicate {
			return ErrInvalidOrder
		}
		seenCategories[category.ID] = struct{}{}
		for _, siteID := range category.SiteIDs {
			if _, exists := siteSet[siteID]; !exists {
				return ErrInvalidOrder
			}
			if _, duplicate := seenSites[siteID]; duplicate {
				return ErrInvalidOrder
			}
			seenSites[siteID] = struct{}{}
		}
	}
	if len(seenSites) != len(siteSet) {
		return ErrInvalidOrder
	}
	return nil
}

func touchDraft(ctx context.Context, tx *sql.Tx, pageID string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE navigation_pages SET draft_revision = draft_revision + 1, draft_updated_at = ?, updated_at = ?
		WHERE id = ?`, dbTime(now), dbTime(now), pageID)
	if err != nil {
		return fmt.Errorf("advance navigation revision: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

func bumpExpected(ctx context.Context, tx *sql.Tx, pageID string, expectedRevision int, now time.Time) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE navigation_pages SET draft_revision = draft_revision + 1, draft_updated_at = ?, updated_at = ?
		WHERE id = ? AND draft_revision = ?`, dbTime(now), dbTime(now), pageID, expectedRevision)
	return expectRevisionResult(result, err)
}

func expectRevisionResult(result sql.Result, err error) error {
	if err != nil {
		return fmt.Errorf("update navigation revision: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrPrecondition
	}
	return nil
}

func normalizeCategoryOrder(ctx context.Context, tx *sql.Tx, pageID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM categories WHERE page_id = ? ORDER BY sort_order, id", pageID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for index, id := range ids {
		if _, err := tx.ExecContext(ctx, "UPDATE categories SET sort_order = ?, updated_at = ? WHERE id = ?", index, dbTime(now), id); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSiteOrder(ctx context.Context, tx *sql.Tx, pageID, categoryID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM sites WHERE page_id = ? AND category_id = ? ORDER BY sort_order, id", pageID, categoryID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for index, id := range ids {
		if _, err := tx.ExecContext(ctx, "UPDATE sites SET sort_order = ?, updated_at = ? WHERE id = ?", index, dbTime(now), id); err != nil {
			return err
		}
	}
	return nil
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	if strings.Contains(message, "foreign key constraint") {
		return fmt.Errorf("%w: referenced resource does not exist", ErrNotFound)
	}
	return err
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseDBTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time %q: %w", value, err)
	}
	return parsed.UTC(), nil
}
