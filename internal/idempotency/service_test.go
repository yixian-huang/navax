package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yixian-huang/navax/internal/database"
)

func TestReservationReplayAndConflict(t *testing.T) {
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ('user_idempotency', 'idem-user', 'idem@example.com', 'unused', 'user', 'active', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	request := struct {
		Revision int `json:"revision"`
	}{Revision: 3}
	reservation, replay, err := service.Begin(context.Background(), "publish:page", "0123456789abcdef", "user_idempotency", request)
	if err != nil || reservation == nil || replay != nil {
		t.Fatalf("Begin() = %+v, %+v, %v", reservation, replay, err)
	}
	if _, _, err := service.Begin(context.Background(), "publish:page", "0123456789abcdef", "user_idempotency", request); !errors.Is(err, ErrInProgress) {
		t.Fatalf("in-progress error = %v", err)
	}
	if err := reservation.Complete(context.Background(), 200, map[string]any{"published": true}); err != nil {
		t.Fatal(err)
	}
	_, replay, err = service.Begin(context.Background(), "publish:page", "0123456789abcdef", "user_idempotency", request)
	if err != nil || replay == nil || replay.Status != 200 || !json.Valid(replay.Data) {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	request.Revision = 4
	if _, _, err := service.Begin(context.Background(), "publish:page", "0123456789abcdef", "user_idempotency", request); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting request error = %v", err)
	}
}
