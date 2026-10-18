package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetWatch loads the registry watcher's state for scope, or nil, nil before
// the first poll.
func (q queries) GetWatch(ctx context.Context, scope string) ([]byte, error) {
	var state []byte
	err := q.db.QueryRowContext(ctx, `SELECT state FROM registry_watch WHERE scope = ?`, scope).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get watch %q: %w", scope, err)
	}
	return state, nil
}

// PutWatch stores the registry watcher's state for scope.
func (q queries) PutWatch(ctx context.Context, scope string, state []byte) error {
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO registry_watch (scope, state, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (scope) DO UPDATE SET state = excluded.state, updated_at = excluded.updated_at`,
		scope, state, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("put watch %q: %w", scope, err)
	}
	return nil
}
