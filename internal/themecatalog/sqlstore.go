package themecatalog

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

const requestSelect = `
	SELECT r.id, r.theme_id, t.name, t.slug, r.owner_id, u.username, r.status, r.reason,
	       r.version_id, r.reviewer_id, r.applied_at, r.reviewed_at
	FROM theme_catalog_requests r
	JOIN themes t ON t.id = r.theme_id
	JOIN users u ON u.id = r.owner_id`

func (s *SQLStore) Create(ctx context.Context, params CreateParams) (Request, error) {
	var requestID string
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var slug, scope string
		var ownerID sql.NullString
		var enabled int
		var currentVersionID sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT slug, scope, owner_id, enabled, current_version_id
			FROM themes WHERE id = ?`, params.ThemeID).Scan(&slug, &scope, &ownerID, &enabled, &currentVersionID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// 非本人/非私有统一 404,不暴露主题是否存在或归属于谁。
		if scope != "private" || !ownerID.Valid || ownerID.String != params.OwnerID {
			return ErrNotFound
		}
		if enabled == 0 || !currentVersionID.Valid || currentVersionID.String == "" {
			return ErrThemeNotEligible
		}
		var conflict int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM themes WHERE scope = 'catalog' AND slug = ?`, slug).Scan(&conflict); err == nil {
			return ErrSlugConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO theme_catalog_requests(id, theme_id, owner_id, status, reason, version_id, applied_at)
			VALUES (?, ?, ?, 'pending', '', ?, ?)`,
			params.ID, params.ThemeID, params.OwnerID, currentVersionID.String, dbTime(params.AppliedAt)); err != nil {
			return mapSQLError(err)
		}
		if err := insertAudit(ctx, tx, params.Audit); err != nil {
			return err
		}
		requestID = params.ID
		return nil
	})
	if err != nil {
		return Request{}, err
	}
	return s.byID(ctx, requestID)
}

func (s *SQLStore) CancelPending(ctx context.Context, ownerID, themeID string, now time.Time, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var requestID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM theme_catalog_requests
			WHERE theme_id = ? AND owner_id = ? AND status = 'pending'`, themeID, ownerID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE theme_catalog_requests SET status = 'revoked', reason = '用户撤回申请', reviewed_at = ?
			WHERE id = ? AND status = 'pending'`, dbTime(now), requestID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrInvalidTransition
		}
		audit.TargetID = requestID
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) List(ctx context.Context, status string, page, pageSize int) (Page, error) {
	where := ""
	args := make([]any, 0, 3)
	if status != "" {
		where = " WHERE r.status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM theme_catalog_requests r"+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, requestSelect+where+` ORDER BY r.applied_at DESC, r.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Request, 0, pageSize)
	for rows.Next() {
		item, err := scanRequest(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// Review is the approval/rejection transaction. On approve, it re-checks
// the request's version_id still matches themes.current_version_id (the
// upgrade lock in internal/themes should make a mismatch impossible, this
// is a defensive backstop) and that the slug still has no catalog conflict
// (another request could have been approved with the same slug in the
// meantime), then flips themes.scope/owner_id in the same statement the
// scope/owner trigger requires.
func (s *SQLStore) Review(ctx context.Context, params ReviewParams) (Request, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var status, themeID, versionID string
		if err := tx.QueryRowContext(ctx, `
			SELECT status, theme_id, version_id FROM theme_catalog_requests WHERE id = ?`,
			params.RequestID).Scan(&status, &themeID, &versionID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if status != "pending" {
			return ErrInvalidTransition
		}
		targetStatus := "rejected"
		switch params.Decision {
		case "approve":
			targetStatus = "approved"
			var currentVersionID, slug sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT current_version_id, slug FROM themes WHERE id = ?`, themeID).
				Scan(&currentVersionID, &slug); err != nil {
				return err
			}
			if !currentVersionID.Valid || currentVersionID.String != versionID {
				return ErrInvalidTransition
			}
			var conflict int
			if err := tx.QueryRowContext(ctx, `
				SELECT 1 FROM themes WHERE scope = 'catalog' AND slug = ? AND id <> ?`,
				slug.String, themeID).Scan(&conflict); err == nil {
				return ErrSlugConflict
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE themes SET scope = 'catalog', owner_id = NULL, updated_at = ? WHERE id = ?`,
				dbTime(params.ReviewedAt), themeID); err != nil {
				return mapSQLError(err)
			}
		case "reject":
			// targetStatus 已是 "rejected",无需改 themes 表。
		default:
			return ErrInvalidInput
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE theme_catalog_requests
			SET status = ?, reason = ?, reviewer_id = ?, reviewed_at = ?
			WHERE id = ? AND status = 'pending'`,
			targetStatus, params.Reason, params.ReviewerID, dbTime(params.ReviewedAt), params.RequestID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrInvalidTransition
		}
		return insertAudit(ctx, tx, params.Audit)
	})
	if err != nil {
		return Request{}, err
	}
	return s.byID(ctx, params.RequestID)
}

func (s *SQLStore) byID(ctx context.Context, id string) (Request, error) {
	item, err := scanRequest(s.db.QueryRowContext(ctx, requestSelect+" WHERE r.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return item, err
}

type rowScanner interface{ Scan(...any) error }

func scanRequest(row rowScanner) (Request, error) {
	var item Request
	var appliedAt string
	var reviewedAt, reviewerID sql.NullString
	if err := row.Scan(
		&item.ID, &item.ThemeID, &item.ThemeName, &item.Slug, &item.OwnerID, &item.OwnerName,
		&item.Status, &item.Reason, &item.VersionID, &reviewerID, &appliedAt, &reviewedAt,
	); err != nil {
		return Request{}, err
	}
	var err error
	if item.AppliedAt, err = parseDBTime(appliedAt); err != nil {
		return Request{}, err
	}
	if reviewedAt.Valid {
		value, err := parseDBTime(reviewedAt.String)
		if err != nil {
			return Request{}, err
		}
		item.ReviewedAt = &value
	}
	item.ReviewerID = reviewerID.String
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
