package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	projectmigrations "github.com/yixian-huang/navax/migrations"
)

func TestOpenAndMigrate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(ctx, Config{Path: filepath.Join(t.TempDir(), "navax.db")})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var migrationCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	if migrationCount != 4 {
		t.Fatalf("migration count = %d, want 4", migrationCount)
	}

	var defaultTheme string
	if err := db.QueryRowContext(ctx, "SELECT id FROM themes WHERE is_default = 1").Scan(&defaultTheme); err != nil {
		t.Fatalf("query default theme: %v", err)
	}
	if defaultTheme != "slate" {
		t.Fatalf("default theme = %q, want slate", defaultTheme)
	}

	var pageKind, settingsJSON string
	if err := db.QueryRowContext(ctx,
		"SELECT kind, settings_json FROM navigation_pages WHERE id = 'page_system_root'",
	).Scan(&pageKind, &settingsJSON); err != nil {
		t.Fatalf("query system page: %v", err)
	}
	if pageKind != "system" || settingsJSON == "" {
		t.Fatalf("system page seed = kind %q, settings length %d", pageKind, len(settingsJSON))
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestForeignKeyAndUniqueConstraints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(ctx, Config{Path: filepath.Join(t.TempDir(), "constraints.db")})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions
        (id, user_id, token_hash, created_at, last_seen_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?)`,
		"session_missing_user", "user_missing", make([]byte, 32), now, now, now,
	)
	if err == nil {
		t.Fatal("inserting a session for a missing user succeeded")
	}

	_, err = db.ExecContext(ctx, `INSERT INTO users
        (id, username, email, password_hash, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)`,
		"user_example_one", "Example", "User@example.com", "hash", now, now,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users
        (id, username, email, password_hash, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)`,
		"user_example_two", "Another", "user@EXAMPLE.com", "hash", now, now,
	)
	if err == nil {
		t.Fatal("case-insensitive duplicate email succeeded")
	}
}

func TestShortSubdomainMigrationPreservesExistingRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	initial, err := projectmigrations.Files.ReadFile("0001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MigrateFS(ctx, db, fstest.MapFS{
		"0001_initial.sql": &fstest.MapFile{Data: initial},
	}); err != nil {
		t.Fatalf("apply initial migration: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, user := range []struct{ id, username, email string }{
		{"user_old_subdomain", "existing", "existing@example.com"},
		{"user_short_label", "short", "short@example.com"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users(id, username, email, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, 'hash', ?, ?)`, user.id, user.username, user.email, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO subdomain_requests(id, user_id, label, full_domain, status, applied_at)
		VALUES ('subdomain_existing', 'user_old_subdomain', 'old', 'old.nav.ax', 'pending', ?)`, now); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("upgrade migrations: %v", err)
	}
	var preserved string
	if err := db.QueryRowContext(ctx, "SELECT full_domain FROM subdomain_requests WHERE id = 'subdomain_existing'").Scan(&preserved); err != nil || preserved != "old.nav.ax" {
		t.Fatalf("preserved domain = %q, %v", preserved, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO subdomain_requests(id, user_id, label, full_domain, status, applied_at)
		VALUES ('subdomain_one_char', 'user_short_label', 'x', 'x.nav.ax', 'pending', ?)`, now); err != nil {
		t.Fatalf("insert one-character label after migration: %v", err)
	}
}

func TestWithinTxCommitAndRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(ctx, Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, "CREATE TABLE values_test (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create test table: %v", err)
	}

	if err := WithinTx(ctx, db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO values_test(value) VALUES ('committed')")
		return err
	}); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	wantErr := errors.New("stop")
	err = WithinTx(ctx, db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO values_test(value) VALUES ('rolled-back')"); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("rollback error = %v, want %v", err, wantErr)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM values_test").Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}
