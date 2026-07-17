// Package idempotency coordinates safe retries for mutating operations.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

var (
	ErrInvalidKey = errors.New("invalid idempotency key")
	ErrConflict   = errors.New("idempotency key was used for a different request")
	ErrInProgress = errors.New("idempotent request is still in progress")
)

type Replay struct {
	Status int
	Data   json.RawMessage
}

type Reservation struct {
	service *Service
	scope   string
	key     string
	actorID string
	hash    []byte
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service { return &Service{db: db, now: time.Now} }

func (s *Service) Begin(ctx context.Context, scope, key, actorID string, request any) (*Reservation, *Replay, error) {
	if s == nil || s.db == nil || len(key) < 16 || len(key) > 128 || scope == "" || len(scope) > 100 || actorID == "" {
		return nil, nil, ErrInvalidKey
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(encoded)
	now := s.now().UTC()
	reservation := &Reservation{service: s, scope: scope, key: key, actorID: actorID, hash: digest[:]}
	var replay *Replay
	err = database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM idempotency_records WHERE scope = ? AND idempotency_key = ? AND expires_at <= ?", scope, key, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		var storedHash []byte
		var storedActor string
		var status sql.NullInt64
		var response sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT request_hash, COALESCE(actor_id, ''), response_status, response_json
			FROM idempotency_records WHERE scope = ? AND idempotency_key = ?`, scope, key,
		).Scan(&storedHash, &storedActor, &status, &response)
		if err == nil {
			if storedActor != actorID || !bytes.Equal(storedHash, digest[:]) {
				return ErrConflict
			}
			if !status.Valid || !response.Valid {
				return ErrInProgress
			}
			replay = &Replay{Status: int(status.Int64), Data: json.RawMessage(response.String)}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO idempotency_records(scope, idempotency_key, actor_id, request_hash, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?)`, scope, key, actorID, digest[:],
			now.Format(time.RFC3339Nano), now.Add(24*time.Hour).Format(time.RFC3339Nano))
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	if replay != nil {
		return nil, replay, nil
	}
	return reservation, nil, nil
}

func (r *Reservation) Complete(ctx context.Context, status int, data any) error {
	if r == nil || r.service == nil || status < 200 || status > 599 {
		return errors.New("invalid idempotency completion")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	result, err := r.service.db.ExecContext(ctx, `
		UPDATE idempotency_records SET response_status = ?, response_json = ?
		WHERE scope = ? AND idempotency_key = ? AND actor_id = ? AND request_hash = ? AND response_status IS NULL`,
		status, string(encoded), r.scope, r.key, r.actorID, r.hash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrConflict
	}
	return nil
}

func (r *Reservation) Abort(ctx context.Context) {
	if r == nil || r.service == nil {
		return
	}
	if _, err := r.service.db.ExecContext(ctx, `
		DELETE FROM idempotency_records
		WHERE scope = ? AND idempotency_key = ? AND actor_id = ? AND request_hash = ? AND response_status IS NULL`,
		r.scope, r.key, r.actorID, r.hash); err != nil {
		slog.Warn("abort idempotency reservation", "error", err, "scope", r.scope)
	}
}
