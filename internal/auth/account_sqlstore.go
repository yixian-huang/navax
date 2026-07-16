package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

var _ AccountStore = (*SQLStore)(nil)

func (s *SQLStore) UserByID(ctx context.Context, userID string) (User, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, userSelect+" WHERE id = ?", userID))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *SQLStore) UpdateProfile(ctx context.Context, userID string, patch ProfilePatch, now time.Time) (User, error) {
	assignments := make([]string, 0, 4)
	arguments := make([]any, 0, 5)
	if patch.Username != nil {
		assignments = append(assignments, "username = ?")
		arguments = append(arguments, *patch.Username)
	}
	if patch.Bio != nil {
		assignments = append(assignments, "bio = ?")
		arguments = append(arguments, *patch.Bio)
	}
	if patch.AvatarURL != nil {
		assignments = append(assignments, "avatar_url = ?")
		arguments = append(arguments, *patch.AvatarURL)
	}
	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, dbTime(now), userID)
	result, err := s.db.ExecContext(ctx, "UPDATE users SET "+strings.Join(assignments, ", ")+" WHERE id = ?", arguments...)
	if err != nil {
		return User{}, mapConflict(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if changed != 1 {
		return User{}, ErrNotFound
	}
	return s.UserByID(ctx, userID)
}

func (s *SQLStore) UpdatePassword(ctx context.Context, userID, currentSessionID, passwordHash string, revokeOthers bool, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			"UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?",
			passwordHash, dbTime(now), userID,
		)
		if err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		if revokeOthers {
			_, err = tx.ExecContext(ctx, `
				UPDATE sessions SET revoked_at = ?
				WHERE user_id = ? AND id <> ? AND revoked_at IS NULL`,
				dbTime(now), userID, currentSessionID,
			)
			if err != nil {
				return fmt.Errorf("revoke other sessions: %w", err)
			}
		}
		return nil
	})
}

func (s *SQLStore) SessionsByUser(ctx context.Context, userID, currentSessionID string, now time.Time) ([]SessionInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, device, approximate_location, created_at, last_seen_at, expires_at
		FROM sessions
		WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ?
		ORDER BY last_seen_at DESC, id`, userID, dbTime(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]SessionInfo, 0)
	for rows.Next() {
		var item SessionInfo
		var createdAt, lastSeenAt, expiresAt string
		if err := rows.Scan(&item.ID, &item.Device, &item.ApproximateLocation, &createdAt, &lastSeenAt, &expiresAt); err != nil {
			return nil, err
		}
		item.Current = item.ID == currentSessionID
		if item.CreatedAt, err = parseDBTime(createdAt); err != nil {
			return nil, err
		}
		if item.LastSeenAt, err = parseDBTime(lastSeenAt); err != nil {
			return nil, err
		}
		if item.ExpiresAt, err = parseDBTime(expiresAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, item)
	}
	return sessions, rows.Err()
}

func (s *SQLStore) CreatePasswordResetToken(ctx context.Context, insert PasswordResetInsert) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Invalidate any earlier unused token so only the most recent link works.
		if _, err := tx.ExecContext(ctx,
			"UPDATE password_reset_tokens SET used_at = ? WHERE user_id = ? AND used_at IS NULL",
			dbTime(insert.CreatedAt), insert.UserID,
		); err != nil {
			return fmt.Errorf("invalidate prior reset tokens: %w", err)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO password_reset_tokens(id, user_id, token_hash, expires_at, used_at, created_at)
			VALUES (?, ?, ?, ?, NULL, ?)`,
			insert.ID, insert.UserID, insert.TokenHash, dbTime(insert.ExpiresAt), dbTime(insert.CreatedAt),
		)
		return err
	})
}

func (s *SQLStore) ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var id, userID string
		err := tx.QueryRowContext(ctx, `
			SELECT id, user_id FROM password_reset_tokens
			WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
			tokenHash, dbTime(now),
		).Scan(&id, &userID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidResetToken
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE password_reset_tokens SET used_at = ? WHERE id = ?", dbTime(now), id,
		); err != nil {
			return fmt.Errorf("consume reset token: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?", passwordHash, dbTime(now), userID,
		); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		// A completed reset revokes every existing session for the account.
		if _, err := tx.ExecContext(ctx,
			"UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", dbTime(now), userID,
		); err != nil {
			return fmt.Errorf("revoke sessions after reset: %w", err)
		}
		return nil
	})
}

func (s *SQLStore) RevokeOwnedSession(ctx context.Context, userID, sessionID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, dbTime(now), sessionID, userID)
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
	return nil
}
