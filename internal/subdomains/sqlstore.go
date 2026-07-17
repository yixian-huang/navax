package subdomains

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

func (s *SQLStore) Policy(ctx context.Context) (Policy, error) {
	var policy Policy
	var rootDomain sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT subdomains_enabled, root_domain FROM system_settings WHERE id = 1").Scan(&policy.Enabled, &rootDomain); err != nil {
		return Policy{}, err
	}
	policy.RootDomain = rootDomain.String
	return policy, nil
}

func (s *SQLStore) LatestForUser(ctx context.Context, userID string) (*Request, error) {
	item, err := scanRequest(s.db.QueryRowContext(ctx, requestSelect+`
		WHERE s.user_id = ? ORDER BY s.applied_at DESC, s.id DESC LIMIT 1`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *SQLStore) Create(ctx context.Context, params CreateParams) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var reviewedAt any
		if params.ReviewedAt != nil {
			reviewedAt = dbTime(*params.ReviewedAt)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO subdomain_requests(
				id, user_id, label, full_domain, status, reason, applied_at, reviewed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			params.ID, params.UserID, params.Label, params.FullDomain, params.Status,
			params.Reason, dbTime(params.AppliedAt), reviewedAt)
		if err != nil {
			return mapSQLError(err)
		}
		return insertAudit(ctx, tx, params.Audit)
	})
}

func (s *SQLStore) CancelPending(ctx context.Context, userID string, now time.Time, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var requestID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM subdomain_requests
			WHERE user_id = ? AND status = 'pending'
			ORDER BY applied_at DESC, id DESC LIMIT 1`, userID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			var latestStatus string
			latestErr := tx.QueryRowContext(ctx, `
				SELECT status FROM subdomain_requests WHERE user_id = ?
				ORDER BY applied_at DESC, id DESC LIMIT 1`, userID).Scan(&latestStatus)
			if errors.Is(latestErr, sql.ErrNoRows) {
				return ErrNotFound
			}
			if latestErr != nil {
				return latestErr
			}
			return ErrInvalidTransition
		}
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE subdomain_requests
			SET status = 'revoked', reason = '用户取消申请', reviewed_at = ?
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
	arguments := make([]any, 0, 3)
	if status != "" {
		where = " WHERE s.status = ?"
		arguments = append(arguments, status)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subdomain_requests s"+where, arguments...).Scan(&total); err != nil {
		return Page{}, err
	}
	arguments = append(arguments, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, requestSelect+where+` ORDER BY s.applied_at DESC, s.id DESC LIMIT ? OFFSET ?`, arguments...)
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

func (s *SQLStore) Review(ctx context.Context, params ReviewParams) (Request, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, "SELECT status FROM subdomain_requests WHERE id = ?", params.RequestID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		targetStatus := ""
		switch params.Decision {
		case "approve":
			if status != "pending" {
				return ErrInvalidTransition
			}
			targetStatus = "approved"
		case "reject":
			if status != "pending" {
				return ErrInvalidTransition
			}
			targetStatus = "rejected"
		case "revoke":
			if status != "approved" {
				return ErrInvalidTransition
			}
			targetStatus = "revoked"
		default:
			return ErrInvalidInput
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE subdomain_requests
			SET status = ?, reason = ?, reviewer_id = ?, reviewed_at = ?
			WHERE id = ? AND status = ?`,
			targetStatus, params.Reason, params.ReviewerID, dbTime(params.ReviewedAt), params.RequestID, status)
		if err != nil {
			return mapSQLError(err)
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
	item, err := scanRequest(s.db.QueryRowContext(ctx, requestSelect+" WHERE s.id = ?", params.RequestID))
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return item, err
}

const requestSelect = `
	SELECT s.id, s.user_id, u.username, s.label, s.full_domain, s.custom_domain, s.status,
	       s.applied_at, s.reviewed_at, s.reason
	FROM subdomain_requests s JOIN users u ON u.id = s.user_id`

type rowScanner interface{ Scan(...any) error }

func scanRequest(row rowScanner) (Request, error) {
	var item Request
	var appliedAt string
	var reviewedAt, customDomain sql.NullString
	if err := row.Scan(
		&item.ID, &item.UserID, &item.Username, &item.Label, &item.FullDomain, &customDomain,
		&item.Status, &appliedAt, &reviewedAt, &item.Reason,
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
	if customDomain.Valid && customDomain.String != "" {
		value := customDomain.String
		item.CustomDomain = &value
	}
	return item, nil
}

func (s *SQLStore) SetCustomDomain(ctx context.Context, userID string, customDomain *string, now time.Time, audit AuditRecord) (Request, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var requestID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM subdomain_requests
			WHERE user_id = ? AND status = 'approved'
			ORDER BY reviewed_at DESC, id DESC LIMIT 1`, userID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidTransition
		}
		if err != nil {
			return err
		}
		var domain any
		if customDomain != nil {
			domain = *customDomain
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE subdomain_requests SET custom_domain = ?, reviewed_at = COALESCE(reviewed_at, ?)
			WHERE id = ? AND status = 'approved'`, domain, dbTime(now), requestID); err != nil {
			return mapSQLError(err)
		}
		audit.TargetID = requestID
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Request{}, err
	}
	item, err := s.LatestForUser(ctx, userID)
	if err != nil {
		return Request{}, err
	}
	if item == nil {
		return Request{}, ErrNotFound
	}
	return *item, nil
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
