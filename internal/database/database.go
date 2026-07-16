// Package database owns SQLite connection setup, migrations, and transaction helpers.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultBusyTimeout = 5 * time.Second
	defaultMaxOpen     = 4
)

var memoryDatabaseID atomic.Uint64

// Config controls the SQLite connection pool. Path is required; use ":memory:"
// only in tests because production relies on WAL persistence.
type Config struct {
	Path         string
	BusyTimeout  time.Duration
	MaxOpenConns int
	MaxIdleConns int
}

// Open creates and verifies a SQLite database configured for the single-node
// nav.ax workload. Call Migrate before serving traffic.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, errors.New("database path is required")
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = defaultBusyTimeout
	}
	if cfg.BusyTimeout < 0 {
		return nil, errors.New("database busy timeout must not be negative")
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = defaultMaxOpen
	}
	if cfg.MaxOpenConns < 1 {
		return nil, errors.New("database max open connections must be positive")
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	if cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return nil, errors.New("database max idle connections must be between zero and max open connections")
	}

	dsn, persistent, err := dataSourceName(cfg.Path, cfg.BusyTimeout)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	closeOnError := func(openErr error) (*sql.DB, error) {
		_ = db.Close()
		return nil, openErr
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(fmt.Errorf("ping sqlite: %w", err))
	}
	if persistent {
		var mode string
		if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
			return closeOnError(fmt.Errorf("enable sqlite WAL: %w", err))
		}
		if !strings.EqualFold(mode, "wal") {
			return closeOnError(fmt.Errorf("enable sqlite WAL: sqlite returned journal mode %q", mode))
		}
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return closeOnError(fmt.Errorf("verify sqlite foreign keys: %w", err))
	}
	if foreignKeys != 1 {
		return closeOnError(errors.New("verify sqlite foreign keys: pragma is disabled"))
	}

	return db, nil
}

// OpenAndMigrate opens a database and applies all embedded migrations before
// returning it. The database is closed automatically when migration fails.
func OpenAndMigrate(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func dataSourceName(path string, busyTimeout time.Duration) (string, bool, error) {
	milliseconds := busyTimeout.Milliseconds()
	if busyTimeout > 0 && milliseconds == 0 {
		milliseconds = 1
	}

	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout("+strconv.FormatInt(milliseconds, 10)+")")
	query.Add("_pragma", "synchronous(NORMAL)")

	if path == ":memory:" {
		query.Set("mode", "memory")
		query.Set("cache", "shared")
		name := fmt.Sprintf("navax-memory-%d", memoryDatabaseID.Add(1))
		return "file:" + name + "?" + query.Encode(), false, nil
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o750); err != nil {
		return "", false, fmt.Errorf("create database directory: %w", err)
	}
	databaseFile, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", false, fmt.Errorf("create database file: %w", err)
	}
	if err := databaseFile.Close(); err != nil {
		return "", false, fmt.Errorf("close database file: %w", err)
	}
	if err := os.Chmod(absolutePath, 0o600); err != nil {
		return "", false, fmt.Errorf("secure database file permissions: %w", err)
	}

	dsnURL := &url.URL{Scheme: "file", Path: absolutePath, RawQuery: query.Encode()}
	return dsnURL.String(), true, nil
}
