package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const objectColumns = `kind, project_id, object_id, state_json, schema_ver, final, last_event_at, updated_at`

// GetObject loads one object_state row, or nil, nil when absent.
func (q queries) GetObject(ctx context.Context, key Key) (*ObjectRow, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT `+objectColumns+` FROM object_state WHERE kind = ? AND project_id = ? AND object_id = ?`,
		key.Kind, key.ProjectID, key.ObjectID)
	o, err := scanObject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get object %v: %w", key, err)
	}
	return o, nil
}

// PutObject inserts or replaces an object_state row. UpdatedAt is set to
// now; the field on the argument is ignored.
func (q queries) PutObject(ctx context.Context, o ObjectRow) error {
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO object_state (`+objectColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (kind, project_id, object_id) DO UPDATE SET
		   state_json = excluded.state_json, schema_ver = excluded.schema_ver, final = excluded.final,
		   last_event_at = excluded.last_event_at, updated_at = excluded.updated_at`,
		o.Kind, o.ProjectID, o.ObjectID, o.StateJSON, o.SchemaVer, o.Final, unixOrZero(o.LastEventAt), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("put object %v: %w", o.Key, err)
	}
	return nil
}

// ListNonFinal returns objects of the given kind with final = 0 whose last
// event is older than olderThan, oldest first. The reconciler uses it to
// find cards that may have missed a delivery.
func (q queries) ListNonFinal(ctx context.Context, kind string, olderThan time.Time) ([]ObjectRow, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT `+objectColumns+` FROM object_state
		 WHERE kind = ? AND final = 0 AND last_event_at < ? ORDER BY last_event_at`,
		kind, olderThan.Unix())
	if err != nil {
		return nil, fmt.Errorf("list non-final %q: %w", kind, err)
	}
	defer rows.Close()
	var out []ObjectRow
	for rows.Next() {
		o, err := scanObject(rows)
		if err != nil {
			return nil, fmt.Errorf("list non-final %q: %w", kind, err)
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list non-final %q: %w", kind, err)
	}
	return out, nil
}

// ListProject returns every object of kind in one project, newest event
// first.
func (q queries) ListProject(ctx context.Context, kind string, projectID int64) ([]ObjectRow, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT `+objectColumns+` FROM object_state
		 WHERE kind = ? AND project_id = ? ORDER BY last_event_at DESC`,
		kind, projectID)
	if err != nil {
		return nil, fmt.Errorf("list %q in project %d: %w", kind, projectID, err)
	}
	defer rows.Close()
	var out []ObjectRow
	for rows.Next() {
		o, err := scanObject(rows)
		if err != nil {
			return nil, fmt.Errorf("list %q in project %d: %w", kind, projectID, err)
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list %q in project %d: %w", kind, projectID, err)
	}
	return out, nil
}

// ExpireNonFinal marks non-final objects of kind whose last event precedes
// olderThan as final, so Prune can retire them later. It returns the
// number of rows changed.
func (q queries) ExpireNonFinal(ctx context.Context, kind string, olderThan time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`UPDATE object_state SET final = 1, updated_at = ? WHERE kind = ? AND final = 0 AND last_event_at < ?`,
		time.Now().Unix(), kind, olderThan.Unix())
	if err != nil {
		return 0, fmt.Errorf("expire non-final %q: %w", kind, err)
	}
	return res.RowsAffected()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanObject(s scanner) (*ObjectRow, error) {
	var o ObjectRow
	var lastEventAt, updatedAt int64
	if err := s.Scan(&o.Kind, &o.ProjectID, &o.ObjectID, &o.StateJSON, &o.SchemaVer, &o.Final, &lastEventAt, &updatedAt); err != nil {
		return nil, err
	}
	o.LastEventAt = time.Unix(lastEventAt, 0)
	o.UpdatedAt = time.Unix(updatedAt, 0)
	return &o, nil
}

// unixOrZero stores the zero time as 0 instead of a negative year-1 value.
func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
