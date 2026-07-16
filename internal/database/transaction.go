package database

import (
	"context"
	"database/sql"
	"errors"
)

// WithinTx runs fn in a transaction and commits only when fn succeeds. A
// failed commit is returned to the caller; rollback is always attempted after
// any failure.
func WithinTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) error {
	if db == nil {
		return errors.New("transaction: database is nil")
	}
	if fn == nil {
		return errors.New("transaction: callback is nil")
	}

	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
