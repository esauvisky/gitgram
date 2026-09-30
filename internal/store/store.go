// Package store is the SQLite persistence layer: webhook dedupe keys, card
// object state (opaque JSON blobs, never decoded here), Telegram card message
// bookkeeping, object links and the outbox the sender drains.
//
// Every method takes a context and exists on both *Store and *Tx, so the
// engine can run dedupe → reduce → enqueue inside one transaction while the
// sender uses the plain Store. The pool is limited to a single connection:
// while a WithTx callback runs, calls on the *Store block until it commits,
// so never call Store methods from inside the callback.
//
// Times are stored as unix seconds, except outbox.not_before which is unix
// milliseconds.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed store. Create one with Open.
type Store struct {
	queries
	db *sql.DB
}

// Tx is a transaction handed to WithTx callbacks. It offers the same
// methods as Store.
type Tx struct {
	queries
}

// dbtx is the subset of *sql.DB and *sql.Tx the queries use.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// queries implements every repository method against either a *sql.DB or a
// *sql.Tx.
type queries struct {
	db dbtx
}

// Open opens (creating if needed) the SQLite database at path, applies the
// pragmas the design requires and runs pending migrations.
func Open(path string) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %q: %w", path, err)
	}
	return &Store{queries: queries{db: db}, db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// WithTx runs fn inside a transaction, committing when fn returns nil and
// rolling back otherwise. The error from fn is returned unwrapped so callers
// can errors.Is against their own sentinels.
func (s *Store) WithTx(ctx context.Context, fn func(tx *Tx) error) error {
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(&Tx{queries: queries{db: sqlTx}}); err != nil {
		sqlTx.Rollback()
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
