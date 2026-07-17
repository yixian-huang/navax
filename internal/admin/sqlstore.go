package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/database"
)

type SQLStore struct{ db *sql.DB }

var _ Store = (*SQLStore)(nil)

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) OverviewCounts(ctx context.Context, now time.Time) (Counts, error) {
	var counts Counts
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM users WHERE status = 'active'),
			(SELECT COUNT(*) FROM invitations WHERE revoked_at IS NULL AND expires_at > ? AND used_count < max_uses),
			(SELECT COUNT(*) FROM page_publications WHERE visibility = 'public' AND current_snapshot_id IS NOT NULL)`,
		dbTime(now),
	).Scan(&counts.TotalUsers, &counts.ActiveUsers, &counts.ActiveInvitations, &counts.PublicPages)
	return counts, err
}

func (s *SQLStore) ListUsers(ctx context.Context, filter UserFilter) (Page[auth.User], error) {
	conditions := make([]string, 0, 2)
	arguments := make([]any, 0, 5)
	if filter.Search != "" {
		conditions = append(conditions, "(username LIKE ? ESCAPE '\\' OR email LIKE ? ESCAPE '\\')")
		like := "%" + escapeLike(filter.Search) + "%"
		arguments = append(arguments, like, like)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		arguments = append(arguments, filter.Status)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users"+where, arguments...).Scan(&total); err != nil {
		return Page[auth.User]{}, err
	}
	arguments = append(arguments, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, email, password_hash, avatar_url, bio, role, status, created_at, updated_at
		FROM users`+where+` ORDER BY created_at DESC, id LIMIT ? OFFSET ?`, arguments...)
	if err != nil {
		return Page[auth.User]{}, err
	}
	defer rows.Close()
	items := make([]auth.User, 0, filter.PageSize)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return Page[auth.User]{}, err
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return Page[auth.User]{}, err
	}
	return Page[auth.User]{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (s *SQLStore) User(ctx context.Context, userID string) (auth.User, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, avatar_url, bio, role, status, created_at, updated_at
		FROM users WHERE id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, ErrNotFound
	}
	return user, err
}

func (s *SQLStore) SetUserStatus(ctx context.Context, userID, status string, now time.Time, audit AuditRecord) (auth.User, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "UPDATE users SET status = ?, updated_at = ? WHERE id = ?", status, dbTime(now), userID)
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
		if status == "disabled" {
			if _, err := tx.ExecContext(ctx, "UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", dbTime(now), userID); err != nil {
				return err
			}
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return auth.User{}, err
	}
	return s.User(ctx, userID)
}

func (s *SQLStore) RevokeUserSessions(ctx context.Context, userID string, now time.Time, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", userID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", dbTime(now), userID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) ListInvitations(ctx context.Context, page, pageSize int) (Page[Invitation], error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invitations").Scan(&total); err != nil {
		return Page[Invitation]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.token_preview, u.username, i.email, i.max_uses, i.used_count,
		       i.expires_at, i.revoked_at, i.created_at
		FROM invitations i JOIN users u ON u.id = i.creator_id
		ORDER BY i.created_at DESC, i.id LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page[Invitation]{}, err
	}
	defer rows.Close()
	items := make([]Invitation, 0, pageSize)
	for rows.Next() {
		item, err := scanInvitation(rows)
		if err != nil {
			return Page[Invitation]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[Invitation]{}, err
	}
	return Page[Invitation]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *SQLStore) InsertInvitation(ctx context.Context, input InvitationInsert, audit AuditRecord) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO invitations(id, token_hash, token_preview, creator_id, email, max_uses, used_count, expires_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			input.ID, input.TokenHash, input.TokenPreview, input.CreatorID, nullableString(input.Email), input.MaxUses,
			dbTime(input.ExpiresAt), dbTime(input.CreatedAt))
		if err != nil {
			return mapSQLError(err)
		}
		return insertAudit(ctx, tx, audit)
	})
}

func (s *SQLStore) RevokeInvitation(ctx context.Context, invitationID string, now time.Time, audit AuditRecord) (Invitation, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "UPDATE invitations SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL", dbTime(now), invitationID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			var exists int
			if err := tx.QueryRowContext(ctx, "SELECT 1 FROM invitations WHERE id = ?", invitationID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
			return ErrInvitationState
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Invitation{}, err
	}
	return s.invitation(ctx, invitationID)
}

func (s *SQLStore) invitation(ctx context.Context, invitationID string) (Invitation, error) {
	item, err := scanInvitation(s.db.QueryRowContext(ctx, `
		SELECT i.id, i.token_preview, u.username, i.email, i.max_uses, i.used_count,
		       i.expires_at, i.revoked_at, i.created_at
		FROM invitations i JOIN users u ON u.id = i.creator_id WHERE i.id = ?`, invitationID))
	if errors.Is(err, sql.ErrNoRows) {
		return Invitation{}, ErrNotFound
	}
	return item, err
}

func (s *SQLStore) ListThemes(ctx context.Context) ([]Theme, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, version, author, description, mode, preview, enabled, is_default
		FROM themes ORDER BY is_default DESC, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Theme, 0)
	for rows.Next() {
		item, err := scanTheme(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) Theme(ctx context.Context, themeID string) (Theme, error) {
	item, err := scanTheme(s.db.QueryRowContext(ctx, `
		SELECT id, name, version, author, description, mode, preview, enabled, is_default
		FROM themes WHERE id = ?`, themeID))
	if errors.Is(err, sql.ErrNoRows) {
		return Theme{}, ErrNotFound
	}
	return item, err
}

func (s *SQLStore) UpdateTheme(ctx context.Context, themeID string, patch ThemePatch, now time.Time, audit AuditRecord) (Theme, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var currentDefault bool
		if err := tx.QueryRowContext(ctx, "SELECT is_default FROM themes WHERE id = ?", themeID).Scan(&currentDefault); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if patch.Default != nil && *patch.Default {
			if _, err := tx.ExecContext(ctx, "UPDATE themes SET is_default = 0, updated_at = ? WHERE is_default = 1 AND id <> ?", dbTime(now), themeID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE themes SET is_default = 1, enabled = 1, updated_at = ? WHERE id = ?", dbTime(now), themeID); err != nil {
				return err
			}
		} else if patch.Default != nil && !*patch.Default && currentDefault {
			return ErrDefaultTheme
		}
		if patch.Enabled != nil {
			if currentDefault && !*patch.Enabled && (patch.Default == nil || !*patch.Default) {
				return ErrDefaultTheme
			}
			if _, err := tx.ExecContext(ctx, "UPDATE themes SET enabled = ?, updated_at = ? WHERE id = ?", *patch.Enabled, dbTime(now), themeID); err != nil {
				return err
			}
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return Theme{}, err
	}
	return s.Theme(ctx, themeID)
}

func (s *SQLStore) Settings(ctx context.Context) (SystemSettings, error) {
	var settings SystemSettings
	var rootDomain sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT instance_name, public_base_url, registration_mode,
		       max_categories_per_page, max_sites_per_page, max_upload_bytes,
		       analytics_enabled, analytics_retention_days, root_domain, subdomains_enabled
		FROM system_settings WHERE id = 1`).Scan(
		&settings.InstanceName, &settings.PublicBaseURL, &settings.RegistrationMode,
		&settings.Limits.MaxCategoriesPerPage, &settings.Limits.MaxSitesPerPage, &settings.Limits.MaxUploadBytes,
		&settings.Analytics.Enabled, &settings.Analytics.RetentionDays, &rootDomain, &settings.Domain.SubdomainsEnabled,
	)
	if rootDomain.Valid {
		settings.Domain.RootDomain = &rootDomain.String
	}
	return settings, err
}

func (s *SQLStore) UpdateSettings(ctx context.Context, patch SystemSettingsPatch, now time.Time, audit AuditRecord) (SystemSettings, error) {
	assignments := make([]string, 0, 10)
	arguments := make([]any, 0, 11)
	add := func(column string, value any) {
		assignments = append(assignments, column+" = ?")
		arguments = append(arguments, value)
	}
	if patch.InstanceName != nil {
		add("instance_name", *patch.InstanceName)
	}
	if patch.PublicBaseURL != nil {
		add("public_base_url", *patch.PublicBaseURL)
	}
	if patch.RegistrationMode != nil {
		add("registration_mode", *patch.RegistrationMode)
	}
	if patch.Limits != nil {
		if patch.Limits.MaxCategoriesPerPage != nil {
			add("max_categories_per_page", *patch.Limits.MaxCategoriesPerPage)
		}
		if patch.Limits.MaxSitesPerPage != nil {
			add("max_sites_per_page", *patch.Limits.MaxSitesPerPage)
		}
		if patch.Limits.MaxUploadBytes != nil {
			add("max_upload_bytes", *patch.Limits.MaxUploadBytes)
		}
	}
	if patch.Analytics != nil {
		if patch.Analytics.Enabled != nil {
			add("analytics_enabled", *patch.Analytics.Enabled)
		}
		if patch.Analytics.RetentionDays != nil {
			add("analytics_retention_days", *patch.Analytics.RetentionDays)
		}
	}
	if patch.Domain != nil {
		if patch.Domain.RootDomain != nil {
			if *patch.Domain.RootDomain == nil {
				add("root_domain", nil)
			} else {
				add("root_domain", **patch.Domain.RootDomain)
			}
		}
		if patch.Domain.SubdomainsEnabled != nil {
			add("subdomains_enabled", *patch.Domain.SubdomainsEnabled)
		}
	}
	add("updated_at", dbTime(now))
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE system_settings SET "+strings.Join(assignments, ", ")+" WHERE id = 1", arguments...); err != nil {
			return mapSQLError(err)
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return SystemSettings{}, err
	}
	return s.Settings(ctx)
}

func (s *SQLStore) AppendAudit(ctx context.Context, record AuditRecord) error {
	return insertAudit(ctx, s.db, record)
}

func (s *SQLStore) ListAudit(ctx context.Context, filter AuditFilter) (Page[AuditEntry], error) {
	where := ""
	arguments := make([]any, 0, 3)
	if filter.Action != "" {
		where = " WHERE action = ?"
		arguments = append(arguments, filter.Action)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs"+where, arguments...).Scan(&total); err != nil {
		return Page[AuditEntry]{}, err
	}
	arguments = append(arguments, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor_id, actor_name, action, target_type, target_id, detail_json, request_id, created_at
		FROM audit_logs`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, arguments...)
	if err != nil {
		return Page[AuditEntry]{}, err
	}
	defer rows.Close()
	items := make([]AuditEntry, 0, filter.PageSize)
	for rows.Next() {
		var item AuditEntry
		var actorID sql.NullString
		var createdAt string
		if err := rows.Scan(&item.ID, &actorID, &item.ActorName, &item.Action, &item.TargetType, &item.TargetID, &item.Detail, &item.RequestID, &createdAt); err != nil {
			return Page[AuditEntry]{}, err
		}
		item.ActorID = actorID.String
		var err error
		if item.CreatedAt, err = parseDBTime(createdAt); err != nil {
			return Page[AuditEntry]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[AuditEntry]{}, err
	}
	return Page[AuditEntry]{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

type rowScanner interface{ Scan(...any) error }

func scanUser(row rowScanner) (auth.User, error) {
	var user auth.User
	var createdAt, updatedAt string
	if err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.Bio, &user.Role, &user.Status, &createdAt, &updatedAt); err != nil {
		return auth.User{}, err
	}
	var err error
	if user.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return auth.User{}, err
	}
	if user.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return auth.User{}, err
	}
	return user, nil
}

func scanInvitation(row rowScanner) (Invitation, error) {
	var item Invitation
	var email, revokedAt sql.NullString
	var expiresAt, createdAt string
	if err := row.Scan(&item.ID, &item.TokenPreview, &item.CreatorName, &email, &item.MaxUses, &item.UsedCount, &expiresAt, &revokedAt, &createdAt); err != nil {
		return Invitation{}, err
	}
	if email.Valid {
		item.Email = &email.String
	}
	var err error
	if item.ExpiresAt, err = parseDBTime(expiresAt); err != nil {
		return Invitation{}, err
	}
	if item.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return Invitation{}, err
	}
	if revokedAt.Valid {
		value, err := parseDBTime(revokedAt.String)
		if err != nil {
			return Invitation{}, err
		}
		item.RevokedAt = &value
	}
	return item, nil
}

func scanTheme(row rowScanner) (Theme, error) {
	var item Theme
	err := row.Scan(&item.ID, &item.Name, &item.Version, &item.Author, &item.Description, &item.Mode, &item.Preview, &item.Enabled, &item.Default)
	return item, err
}

type auditExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertAudit(ctx context.Context, execer auditExecer, record AuditRecord) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_id, actor_name, action, target_type, target_id, detail_json, request_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, nullable(record.ActorID), record.ActorName, record.Action, record.TargetType,
		record.TargetID, record.Detail, record.RequestID, dbTime(record.CreatedAt))
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return replacer.Replace(value)
}

func (s *SQLStore) ListDiscoverPages(ctx context.Context, filter DiscoverFilter) (Page[DiscoverPage], error) {
	where := ` WHERE p.kind = 'personal' AND pp.visibility = 'public' AND pp.current_snapshot_id IS NOT NULL AND u.status = 'active'`
	args := make([]any, 0, 4)
	if filter.Search != "" {
		where += ` AND (pp.slug LIKE ? ESCAPE '\' OR u.username LIKE ? ESCAPE '\' OR json_extract(s.payload_json, '$.title') LIKE ? ESCAPE '\')`
		pattern := "%" + escapeLike(filter.Search) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	joins := ` FROM page_publications pp
		JOIN navigation_pages p ON p.id = pp.page_id
		JOIN users u ON u.id = p.owner_id
		JOIN published_snapshots s ON s.id = pp.current_snapshot_id`
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+joins+where, args...).Scan(&total); err != nil {
		return Page[DiscoverPage]{}, err
	}
	queryArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, pp.slug, COALESCE(json_extract(s.payload_json, '$.title'), pp.slug), u.id, u.username,
		       pp.featured, pp.tags_json, s.published_at`+joins+where+`
		ORDER BY pp.featured DESC, s.published_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page[DiscoverPage]{}, err
	}
	defer rows.Close()
	items := make([]DiscoverPage, 0)
	for rows.Next() {
		var item DiscoverPage
		var tagsJSON, publishedAt string
		if err := rows.Scan(&item.PageID, &item.Slug, &item.Title, &item.OwnerID, &item.OwnerName, &item.Featured, &tagsJSON, &publishedAt); err != nil {
			return Page[DiscoverPage]{}, err
		}
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			item.Tags = []string{}
		}
		if item.Tags == nil {
			item.Tags = []string{}
		}
		item.PublishedAt, err = parseDBTime(publishedAt)
		if err != nil {
			return Page[DiscoverPage]{}, err
		}
		items = append(items, item)
	}
	return Page[DiscoverPage]{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, rows.Err()
}

func (s *SQLStore) UpdateDiscoverPage(ctx context.Context, pageID string, patch DiscoverPatch, now time.Time, audit AuditRecord) (DiscoverPage, error) {
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM page_publications pp
			JOIN navigation_pages p ON p.id = pp.page_id
			WHERE pp.page_id = ? AND p.kind = 'personal' AND pp.visibility = 'public' AND pp.current_snapshot_id IS NOT NULL`,
			pageID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		sets := make([]string, 0, 3)
		args := make([]any, 0, 4)
		if patch.Featured != nil {
			sets = append(sets, "featured = ?")
			if *patch.Featured {
				args = append(args, 1)
			} else {
				args = append(args, 0)
			}
		}
		if patch.Tags != nil {
			encoded, err := json.Marshal(*patch.Tags)
			if err != nil {
				return err
			}
			sets = append(sets, "tags_json = ?")
			args = append(args, string(encoded))
		}
		sets = append(sets, "updated_at = ?")
		args = append(args, dbTime(now), pageID)
		_, err := tx.ExecContext(ctx, "UPDATE page_publications SET "+strings.Join(sets, ", ")+" WHERE page_id = ?", args...)
		if err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit)
	})
	if err != nil {
		return DiscoverPage{}, err
	}
	var item DiscoverPage
	var tagsJSON, publishedAt string
	err = s.db.QueryRowContext(ctx, `
		SELECT p.id, pp.slug, COALESCE(json_extract(s.payload_json, '$.title'), pp.slug), u.id, u.username,
		       pp.featured, pp.tags_json, s.published_at
		FROM page_publications pp
		JOIN navigation_pages p ON p.id = pp.page_id
		JOIN users u ON u.id = p.owner_id
		JOIN published_snapshots s ON s.id = pp.current_snapshot_id
		WHERE p.id = ?`, pageID).Scan(
		&item.PageID, &item.Slug, &item.Title, &item.OwnerID, &item.OwnerName, &item.Featured, &tagsJSON, &publishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return DiscoverPage{}, ErrNotFound
	}
	if err != nil {
		return DiscoverPage{}, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &item.Tags)
	if item.Tags == nil {
		item.Tags = []string{}
	}
	item.PublishedAt, err = parseDBTime(publishedAt)
	return item, err
}

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "constraint failed") || strings.Contains(message, "unique constraint") {
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
