package directoryadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

type SQLStore struct{ db *sql.DB }

var _ Store = (*SQLStore)(nil)

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.icon, c.sort_order, c.enabled, COUNT(s.id)
		FROM directory_categories c LEFT JOIN directory_sites s ON s.category_id = c.id
		GROUP BY c.id ORDER BY c.sort_order, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Name, &item.Icon, &item.SortOrder, &item.Enabled, &item.SiteCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) CreateCategory(ctx context.Context, item Category, now time.Time, audit AuditRecord) (Category, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM directory_categories").Scan(&item.SortOrder); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO directory_categories(id, name, icon, sort_order, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ID, item.Name, item.Icon, item.SortOrder, item.Enabled, dbTime(now), dbTime(now))
		if err != nil {
			return mapSQLError(err)
		}
		return insertAudit(ctx, tx, audit)
	})
	return item, err
}

func (s *SQLStore) UpdateCategory(ctx context.Context, categoryID string, input CategoryInput, now time.Time, audit AuditRecord) (Category, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE directory_categories SET name = ?, icon = ?, enabled = ?, updated_at = ? WHERE id = ?`,
			input.Name, input.Icon, input.Enabled, dbTime(now), categoryID)
		if err != nil {
			return mapSQLError(err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Category{}, err
	}
	return s.category(ctx, categoryID)
}

func (s *SQLStore) DeleteCategory(ctx context.Context, categoryID string, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM directory_sites WHERE category_id = ?", categoryID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return ErrCategoryInUse
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM directory_categories WHERE id = ?", categoryID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) category(ctx context.Context, categoryID string) (Category, error) {
	var item Category
	err := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.icon, c.sort_order, c.enabled, COUNT(s.id)
		FROM directory_categories c LEFT JOIN directory_sites s ON s.category_id = c.id
		WHERE c.id = ? GROUP BY c.id`, categoryID,
	).Scan(&item.ID, &item.Name, &item.Icon, &item.SortOrder, &item.Enabled, &item.SiteCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return item, err
}

func (s *SQLStore) Sites(ctx context.Context, filter SiteFilter) (Page[Site], error) {
	where, arguments := directorySiteWhere(filter.Search, filter.CategoryID)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM directory_sites s JOIN directory_categories c ON c.id = s.category_id"+where, arguments...).Scan(&total); err != nil {
		return Page[Site]{}, err
	}
	arguments = append(arguments, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.category_id, c.name, s.title, s.url, s.icon, s.description, s.sort_order, s.enabled
		FROM directory_sites s JOIN directory_categories c ON c.id = s.category_id`+where+`
		ORDER BY c.sort_order, s.sort_order, s.id LIMIT ? OFFSET ?`, arguments...)
	if err != nil {
		return Page[Site]{}, err
	}
	defer rows.Close()
	items := make([]Site, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanSite(rows)
		if err != nil {
			return Page[Site]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[Site]{}, err
	}
	return Page[Site]{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (s *SQLStore) CreateSite(ctx context.Context, item Site, now time.Time, audit AuditRecord) (Site, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := requireCategory(ctx, tx, item.CategoryID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM directory_sites WHERE category_id = ?", item.CategoryID).Scan(&item.SortOrder); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO directory_sites(id, category_id, title, url, icon, description, sort_order, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.CategoryID, item.Title, item.URL, item.Icon, item.Description,
			item.SortOrder, item.Enabled, dbTime(now), dbTime(now))
		if err != nil {
			return mapSQLError(err)
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Site{}, err
	}
	return s.site(ctx, item.ID)
}

func (s *SQLStore) UpdateSite(ctx context.Context, siteID string, patch SitePatch, now time.Time, audit AuditRecord) (Site, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		assignments := make([]string, 0, 8)
		arguments := make([]any, 0, 9)
		add := func(column string, value any) {
			assignments = append(assignments, column+" = ?")
			arguments = append(arguments, value)
		}
		if patch.CategoryID != nil {
			if err := requireCategory(ctx, tx, *patch.CategoryID); err != nil {
				return err
			}
			var order int
			if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM directory_sites WHERE category_id = ?", *patch.CategoryID).Scan(&order); err != nil {
				return err
			}
			add("category_id", *patch.CategoryID)
			add("sort_order", order)
		}
		if patch.Title != nil {
			add("title", *patch.Title)
		}
		if patch.URL != nil {
			add("url", *patch.URL)
		}
		if patch.Icon != nil {
			add("icon", *patch.Icon)
		}
		if patch.Description != nil {
			add("description", *patch.Description)
		}
		if patch.Enabled != nil {
			add("enabled", *patch.Enabled)
		}
		add("updated_at", dbTime(now))
		arguments = append(arguments, siteID)
		result, err := tx.ExecContext(ctx, "UPDATE directory_sites SET "+strings.Join(assignments, ", ")+" WHERE id = ?", arguments...)
		if err != nil {
			return mapSQLError(err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Site{}, err
	}
	return s.site(ctx, siteID)
}

func (s *SQLStore) DeleteSite(ctx context.Context, siteID string, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "DELETE FROM directory_sites WHERE id = ?", siteID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) site(ctx context.Context, siteID string) (Site, error) {
	item, err := scanSite(s.db.QueryRowContext(ctx, `
		SELECT s.id, s.category_id, c.name, s.title, s.url, s.icon, s.description, s.sort_order, s.enabled
		FROM directory_sites s JOIN directory_categories c ON c.id = s.category_id WHERE s.id = ?`, siteID))
	if errors.Is(err, sql.ErrNoRows) {
		return Site{}, ErrNotFound
	}
	return item, err
}

func (s *SQLStore) Links(ctx context.Context, filter LinkFilter) (Page[AdminLink], error) {
	// Include personal + system navigation sites (not the public directory catalog).
	// System pages have NULL owner_id, so users must be LEFT JOIN'd.
	where := " WHERE p.kind IN ('personal', 'system')"
	arguments := make([]any, 0, 6)
	if filter.OwnerID != "" {
		if filter.OwnerID == "system" {
			where += " AND p.kind = 'system'"
		} else {
			where += " AND u.id = ?"
			arguments = append(arguments, filter.OwnerID)
		}
	}
	if filter.Search != "" {
		where += " AND (s.title LIKE ? ESCAPE '\\' OR s.url LIKE ? ESCAPE '\\' OR s.description LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLike(filter.Search) + "%"
		arguments = append(arguments, pattern, pattern, pattern)
	}
	joins := ` FROM sites s JOIN categories c ON c.id = s.category_id
		JOIN navigation_pages p ON p.id = s.page_id
		LEFT JOIN users u ON u.id = p.owner_id`
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+joins+where, arguments...).Scan(&total); err != nil {
		return Page[AdminLink]{}, err
	}
	arguments = append(arguments, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.page_id, s.category_id, c.name, u.id, u.username,
		       s.title, s.url, s.icon, s.description, s.sort_order, s.enabled, s.created_at, s.updated_at`+
		joins+where+` ORDER BY s.updated_at DESC, s.id LIMIT ? OFFSET ?`, arguments...)
	if err != nil {
		return Page[AdminLink]{}, err
	}
	defer rows.Close()
	items := make([]AdminLink, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanAdminLink(rows)
		if err != nil {
			return Page[AdminLink]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[AdminLink]{}, err
	}
	return Page[AdminLink]{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (s *SQLStore) DeletePersonalLink(ctx context.Context, siteID string, now time.Time, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var pageID string
		err := tx.QueryRowContext(ctx, `
			SELECT s.page_id FROM sites s JOIN navigation_pages p ON p.id = s.page_id
			WHERE s.id = ? AND p.kind IN ('personal', 'system')`, siteID).Scan(&pageID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sites WHERE id = ?", siteID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE navigation_pages SET draft_revision = draft_revision + 1, draft_updated_at = ?, updated_at = ?
			WHERE id = ?`, dbTime(now), dbTime(now), pageID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return insertAudit(ctx, tx, audit)
	})
}

func directorySiteWhere(search, categoryID string) (string, []any) {
	conditions := make([]string, 0, 2)
	arguments := make([]any, 0, 4)
	if categoryID != "" {
		conditions = append(conditions, "s.category_id = ?")
		arguments = append(arguments, categoryID)
	}
	if search != "" {
		conditions = append(conditions, "(s.title LIKE ? ESCAPE '\\' OR s.url LIKE ? ESCAPE '\\' OR s.description LIKE ? ESCAPE '\\')")
		pattern := "%" + escapeLike(search) + "%"
		arguments = append(arguments, pattern, pattern, pattern)
	}
	if len(conditions) == 0 {
		return "", arguments
	}
	return " WHERE " + strings.Join(conditions, " AND "), arguments
}

func requireCategory(ctx context.Context, tx *sql.Tx, categoryID string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM directory_categories WHERE id = ?", categoryID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanSite(row rowScanner) (Site, error) {
	var item Site
	err := row.Scan(&item.ID, &item.CategoryID, &item.CategoryName, &item.Title, &item.URL, &item.Icon, &item.Description, &item.SortOrder, &item.Enabled)
	return item, err
}

func scanAdminLink(row rowScanner) (AdminLink, error) {
	var item AdminLink
	var ownerID, ownerName sql.NullString
	var enabled int
	var createdAt, updatedAt string
	if err := row.Scan(
		&item.ID, &item.PageID, &item.CategoryID, &item.CategoryName, &ownerID, &ownerName,
		&item.Title, &item.URL, &item.Icon, &item.Description, &item.SortOrder, &enabled, &createdAt, &updatedAt,
	); err != nil {
		return AdminLink{}, err
	}
	item.Enabled = enabled != 0
	if ownerID.Valid {
		item.OwnerID = ownerID.String
		item.OwnerName = ownerName.String
	} else {
		item.OwnerID = "system"
		item.OwnerName = "主站"
	}
	var err error
	if item.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return AdminLink{}, err
	}
	if item.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return AdminLink{}, err
	}
	return item, nil
}

type auditExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertAudit(ctx context.Context, execer auditExecer, record AuditRecord) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_id, actor_name, action, target_type, target_id, detail_json, request_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.ActorID, record.ActorName,
		record.Action, record.TargetType, record.TargetID, record.DetailJSON, record.RequestID, dbTime(record.CreatedAt))
	return err
}

func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed") {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseDBTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time %q: %w", value, err)
	}
	return parsed, nil
}
