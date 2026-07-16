package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	projectmigrations "github.com/yixian-huang/navax/migrations"
)

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    name TEXT NOT NULL UNIQUE,
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    applied_at TEXT NOT NULL
)`

type migration struct {
	version  int64
	name     string
	checksum string
	sql      string
}

// Migrate applies the migrations embedded in the application binary.
func Migrate(ctx context.Context, db *sql.DB) error {
	return MigrateFS(ctx, db, projectmigrations.Files)
}

// MigrateFS applies ordered *.sql files from migrationFS. It is exported so
// tests and maintenance tools can verify migrations without rebuilding assets.
func MigrateFS(ctx context.Context, db *sql.DB, migrationFS fs.FS) error {
	if db == nil {
		return errors.New("migrate: database is nil")
	}
	if migrationFS == nil {
		return errors.New("migrate: filesystem is nil")
	}

	migrations, err := loadMigrations(migrationFS)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return errors.New("migrate: no SQL migrations found")
	}
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	for _, item := range migrations {
		if err := applyMigration(ctx, db, item); err != nil {
			return err
		}
	}
	return nil
}

func loadMigrations(migrationFS fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	items := make([]migration, 0, len(entries))
	versions := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator < 1 {
			return nil, fmt.Errorf("migration %q must start with a numeric version and underscore", entry.Name())
		}
		version, err := strconv.ParseInt(entry.Name()[:separator], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("migration %q has an invalid version", entry.Name())
		}
		if previous, exists := versions[version]; exists {
			return nil, fmt.Errorf("migrations %q and %q use duplicate version %d", previous, entry.Name(), version)
		}
		body, err := fs.ReadFile(migrationFS, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			return nil, fmt.Errorf("migration %q is empty", entry.Name())
		}
		digest := sha256.Sum256(body)
		items = append(items, migration{
			version:  version,
			name:     entry.Name(),
			checksum: hex.EncodeToString(digest[:]),
			sql:      string(body),
		})
		versions[version] = entry.Name()
	}

	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	return items, nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration) (returnErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration %q: %w", item.name, err)
	}
	defer func() {
		if err := conn.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close migration connection: %w", err)
		}
	}()

	// BEGIN IMMEDIATE serializes migration writers across application processes.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin migration %q: %w", item.name, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	var storedName, storedChecksum string
	err = conn.QueryRowContext(ctx,
		"SELECT name, checksum FROM schema_migrations WHERE version = ?",
		item.version,
	).Scan(&storedName, &storedChecksum)
	switch {
	case err == nil:
		if storedName != item.name || storedChecksum != item.checksum {
			return fmt.Errorf("migration %d drift detected: database has %q (%s), binary has %q (%s)",
				item.version, storedName, storedChecksum, item.name, item.checksum)
		}
	case errors.Is(err, sql.ErrNoRows):
		if _, err := conn.ExecContext(ctx, item.sql); err != nil {
			return fmt.Errorf("apply migration %q: %w", item.name, err)
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))",
			item.version, item.name, item.checksum,
		); err != nil {
			return fmt.Errorf("record migration %q: %w", item.name, err)
		}
	default:
		return fmt.Errorf("inspect migration %q: %w", item.name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit migration %q: %w", item.name, err)
	}
	committed = true
	return nil
}
