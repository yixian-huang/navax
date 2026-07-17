package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/navigation"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Initialized(ctx context.Context) (bool, error) {
	var initialized bool
	if err := s.db.QueryRowContext(ctx, "SELECT initialized FROM system_settings WHERE id = 1").Scan(&initialized); err != nil {
		return false, fmt.Errorf("read initialization state: %w", err)
	}
	return initialized, nil
}

func (s *SQLStore) Bootstrap(ctx context.Context, params BootstrapParams) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE system_settings
			SET initialized = 1, instance_name = ?, public_base_url = ?, registration_mode = 'invite', updated_at = ?
			WHERE id = 1 AND initialized = 0`,
			params.InstanceName, params.PublicBaseURL, dbTime(params.User.CreatedAt),
		)
		if err != nil {
			return fmt.Errorf("initialize settings: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("inspect initialization update: %w", err)
		}
		if changed != 1 {
			return ErrAlreadyInitialized
		}
		if err := insertUser(ctx, tx, params.User); err != nil {
			return mapConflict(err)
		}
		if err := insertPersonalPage(ctx, tx, params.PersonalPageID, params.UncategorizedID, params.User, params.Slug); err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE navigation_pages SET title = ?, updated_at = ? WHERE kind = 'system'",
			params.InstanceName, dbTime(params.User.CreatedAt),
		); err != nil {
			return fmt.Errorf("update system page: %w", err)
		}
		return insertSession(ctx, tx, params.Session)
	})
}

func (s *SQLStore) UserByEmail(ctx context.Context, email string) (User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+" WHERE email = ?", email)
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *SQLStore) UserBySessionHash(ctx context.Context, hash string, now time.Time) (Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.expires_at,
		       u.id, u.username, u.email, u.password_hash, u.avatar_url, u.bio, u.role, u.status, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?`, hash, dbTime(now))
	var session Session
	var expiresAt string
	var userCreatedAt, userUpdatedAt string
	err := row.Scan(
		&session.ID, &expiresAt,
		&session.User.ID, &session.User.Username, &session.User.Email, &session.User.PasswordHash,
		&session.User.AvatarURL, &session.User.Bio, &session.User.Role, &session.User.Status,
		&userCreatedAt, &userUpdatedAt,
	)
	if err != nil {
		return Session{}, err
	}
	if session.ExpiresAt, err = parseDBTime(expiresAt); err != nil {
		return Session{}, err
	}
	if session.User.CreatedAt, err = parseDBTime(userCreatedAt); err != nil {
		return Session{}, err
	}
	if session.User.UpdatedAt, err = parseDBTime(userUpdatedAt); err != nil {
		return Session{}, err
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE id = ?", dbTime(now), session.ID)
	return session, nil
}

func (s *SQLStore) CreateSession(ctx context.Context, input SessionInput) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, user_id, token_hash, device, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.UserID, input.TokenHash, input.Device, dbTime(input.Now), dbTime(input.Now), dbTime(input.ExpiresAt),
	)
	return err
}

func (s *SQLStore) DeleteSessionByHash(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE sessions SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL",
		dbTime(time.Now().UTC()), hash,
	)
	return err
}

func (s *SQLStore) InvitationByHash(ctx context.Context, hash string, now time.Time) (InvitationInfo, error) {
	var info InvitationInfo
	var expiresAt string
	var revokedAt sql.NullString
	var maxUses, usedCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT u.username, i.expires_at, i.revoked_at, i.max_uses, i.used_count
		FROM invitations i JOIN users u ON u.id = i.creator_id
		WHERE i.token_hash = ?`, hash,
	).Scan(&info.InviterName, &expiresAt, &revokedAt, &maxUses, &usedCount)
	if errors.Is(err, sql.ErrNoRows) {
		return InvitationInfo{}, ErrInvitationInvalid
	}
	if err != nil {
		return InvitationInfo{}, err
	}
	if revokedAt.Valid {
		return InvitationInfo{}, ErrInvitationInvalid
	}
	info.ExpiresAt, err = parseDBTime(expiresAt)
	if err != nil {
		return InvitationInfo{}, err
	}
	if !now.Before(info.ExpiresAt) {
		return InvitationInfo{}, ErrInvitationExpired
	}
	if usedCount >= maxUses {
		return InvitationInfo{}, ErrInvitationExhausted
	}
	return info, nil
}

func (s *SQLStore) RegistrationMode(ctx context.Context) (string, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, "SELECT registration_mode FROM system_settings WHERE id = 1").Scan(&mode); err != nil {
		return "", fmt.Errorf("read registration mode: %w", err)
	}
	return mode, nil
}

func (s *SQLStore) RegisterWithInvitation(ctx context.Context, params RegistrationParams, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var invitationID string
		var invitationEmail, revokedAt sql.NullString
		var expiresAt string
		var maxUses, usedCount int
		err := tx.QueryRowContext(ctx, `
			SELECT id, email, max_uses, used_count, expires_at, revoked_at
			FROM invitations WHERE token_hash = ?`, params.InvitationHash,
		).Scan(&invitationID, &invitationEmail, &maxUses, &usedCount, &expiresAt, &revokedAt)
		if errors.Is(err, sql.ErrNoRows) || revokedAt.Valid {
			return ErrInvitationInvalid
		}
		if err != nil {
			return err
		}
		expires, err := parseDBTime(expiresAt)
		if err != nil {
			return err
		}
		if !now.Before(expires) {
			return ErrInvitationExpired
		}
		if usedCount >= maxUses {
			return ErrInvitationExhausted
		}
		if invitationEmail.Valid && !strings.EqualFold(invitationEmail.String, params.User.Email) {
			return ErrInvitationInvalid
		}
		if err := insertUser(ctx, tx, params.User); err != nil {
			return mapConflict(err)
		}
		if err := insertPersonalPage(ctx, tx, params.PageID, params.UncategorizedID, params.User, params.Slug); err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO invitation_redemptions(invitation_id, user_id, redeemed_at) VALUES (?, ?, ?)",
			invitationID, params.User.ID, dbTime(now),
		); err != nil {
			return mapConflict(err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE invitations SET used_count = used_count + 1
			WHERE id = ? AND revoked_at IS NULL AND used_count < max_uses AND expires_at > ?`,
			invitationID, dbTime(now),
		)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return ErrInvitationExhausted
		}
		return insertSession(ctx, tx, params.Session)
	})
}

func (s *SQLStore) RegisterOpen(ctx context.Context, params RegistrationParams, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var mode string
		if err := tx.QueryRowContext(ctx, "SELECT registration_mode FROM system_settings WHERE id = 1").Scan(&mode); err != nil {
			return err
		}
		if mode != "open" {
			return ErrRegistrationClosed
		}
		if err := insertUser(ctx, tx, params.User); err != nil {
			return mapConflict(err)
		}
		if err := insertPersonalPage(ctx, tx, params.PageID, params.UncategorizedID, params.User, params.Slug); err != nil {
			return mapConflict(err)
		}
		return insertSession(ctx, tx, params.Session)
	})
}

const userSelect = `SELECT id, username, email, password_hash, avatar_url, bio, role, status, created_at, updated_at FROM users`

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var createdAt, updatedAt string
	if err := row.Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AvatarURL,
		&user.Bio, &user.Role, &user.Status, &createdAt, &updatedAt,
	); err != nil {
		return User{}, err
	}
	var err error
	if user.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return User{}, err
	}
	if user.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}

func insertUser(ctx context.Context, tx *sql.Tx, user User) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO users(id, username, email, password_hash, avatar_url, bio, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.AvatarURL, user.Bio,
		user.Role, user.Status, dbTime(user.CreatedAt), dbTime(user.UpdatedAt),
	)
	return err
}

func insertPersonalPage(ctx context.Context, tx *sql.Tx, pageID, categoryID string, user User, slug string) error {
	now := dbTime(user.CreatedAt)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO navigation_pages(id, kind, owner_id, title, description, settings_json, draft_updated_at, created_at, updated_at)
		VALUES (?, 'personal', ?, ?, '', ?, ?, ?, ?)`,
		pageID, user.ID, user.Username+" 的导航", navigation.DefaultSettingsJSON, now, now, now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO page_publications(page_id, visibility, slug, show_author, updated_at)
		VALUES (?, 'unlisted', ?, 1, ?)`, pageID, slug, now,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
		VALUES (?, ?, '未分类', '', 0, 1, ?, ?)`, categoryID, pageID, now, now,
	)
	return err
}

func insertSession(ctx context.Context, tx *sql.Tx, input SessionInput) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sessions(id, user_id, token_hash, device, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.UserID, input.TokenHash, input.Device, dbTime(input.Now), dbTime(input.Now), dbTime(input.ExpiresAt),
	)
	return err
}

func mapConflict(err error) error {
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

// ---- Email codes ----

func (s *SQLStore) CreateEmailCode(ctx context.Context, record EmailCodeRecord, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_email_codes(id, email, purpose, code_hash, payload_json, expires_at, attempts, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		record.ID, record.Email, string(record.Purpose), record.CodeHash, record.Payload,
		dbTime(record.ExpiresAt), dbTime(now),
	)
	return err
}

func (s *SQLStore) LatestEmailCode(ctx context.Context, email string, purpose EmailCodePurpose, now time.Time) (EmailCodeRecord, error) {
	var record EmailCodeRecord
	var purposeRaw, expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, purpose, code_hash, payload_json, expires_at, attempts
		FROM auth_email_codes
		WHERE email = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC LIMIT 1`, email, string(purpose), dbTime(now),
	).Scan(&record.ID, &record.Email, &purposeRaw, &record.CodeHash, &record.Payload, &expiresAt, &record.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return EmailCodeRecord{}, ErrEmailCodeInvalid
	}
	if err != nil {
		return EmailCodeRecord{}, err
	}
	record.Purpose = EmailCodePurpose(purposeRaw)
	record.ExpiresAt, err = parseDBTime(expiresAt)
	return record, err
}

func (s *SQLStore) ConsumeEmailCode(ctx context.Context, id string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE auth_email_codes SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`,
		dbTime(now), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrEmailCodeInvalid
	}
	return nil
}

func (s *SQLStore) BumpEmailCodeAttempt(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auth_email_codes SET attempts = attempts + 1 WHERE id = ?`, id)
	return err
}

// ---- OAuth ----

func (s *SQLStore) SaveOAuthState(ctx context.Context, state, provider, invitationToken string, expiresAt, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_states(state, provider, invitation_token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`, state, provider, invitationToken, dbTime(expiresAt), dbTime(now))
	return err
}

func (s *SQLStore) TakeOAuthState(ctx context.Context, state string, now time.Time) (provider, invitationToken string, err error) {
	err = database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var expiresAt string
		scanErr := tx.QueryRowContext(ctx, `
			SELECT provider, invitation_token, expires_at FROM oauth_states WHERE state = ?`, state,
		).Scan(&provider, &invitationToken, &expiresAt)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return ErrOAuthState
		}
		if scanErr != nil {
			return scanErr
		}
		exp, parseErr := parseDBTime(expiresAt)
		if parseErr != nil || now.After(exp) {
			_, _ = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = ?`, state)
			return ErrOAuthState
		}
		_, delErr := tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = ?`, state)
		return delErr
	})
	return provider, invitationToken, err
}

func (s *SQLStore) UserByOAuth(ctx context.Context, provider, subject string) (User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+`
		FROM users u JOIN oauth_identities o ON o.user_id = u.id
		WHERE o.provider = ? AND o.subject = ?`, provider, subject)
	return scanUser(row)
}

func (s *SQLStore) LinkOAuthIdentity(ctx context.Context, id, provider, subject, userID, email string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_identities(id, provider, subject, user_id, email, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, provider, subject, userID, email, dbTime(now))
	return mapConflict(err)
}

func (s *SQLStore) CreateOAuthUser(ctx context.Context, params RegistrationParams, provider, subject string, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if err := insertUser(ctx, tx, params.User); err != nil {
			return mapConflict(err)
		}
		if err := insertPersonalPage(ctx, tx, params.PageID, params.UncategorizedID, params.User, params.Slug); err != nil {
			return mapConflict(err)
		}
		if err := insertSession(ctx, tx, params.Session); err != nil {
			return err
		}
		linkID, err := identity.New("oai")
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO oauth_identities(id, provider, subject, user_id, email, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`, linkID, provider, subject, params.User.ID, params.User.Email, dbTime(now))
		return mapConflict(err)
	})
}
