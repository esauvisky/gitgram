package store

import (
	"context"
	"fmt"
	"time"
)

// Retention limits applied by Prune.
const (
	DeliveryRetention    = 7 * 24 * time.Hour
	FinalObjectRetention = 30 * 24 * time.Hour
	MaxOutboxAttempts    = 20
)

// PruneResult counts the rows Prune removed.
type PruneResult struct {
	Deliveries int64
	Objects    int64
	Links      int64
	Outbox     int64
}

// Prune is the janitor: it removes delivery keys older than
// DeliveryRetention, final objects untouched for FinalObjectRetention (their
// card_messages rows cascade), links whose either end no longer exists, and
// outbox rows that have failed MaxOutboxAttempts times or more.
func (q queries) Prune(ctx context.Context, now time.Time) (PruneResult, error) {
	var r PruneResult
	var err error
	r.Deliveries, err = q.exec(ctx,
		`DELETE FROM webhook_deliveries WHERE received_at < ?`, now.Add(-DeliveryRetention).Unix())
	if err != nil {
		return r, fmt.Errorf("prune deliveries: %w", err)
	}
	r.Objects, err = q.exec(ctx,
		`DELETE FROM object_state WHERE final = 1 AND updated_at < ?`, now.Add(-FinalObjectRetention).Unix())
	if err != nil {
		return r, fmt.Errorf("prune objects: %w", err)
	}
	r.Links, err = q.exec(ctx,
		`DELETE FROM object_links WHERE NOT EXISTS (
		   SELECT 1 FROM object_state o
		   WHERE o.kind = object_links.kind AND o.project_id = object_links.project_id AND o.object_id = object_links.object_id)
		 OR NOT EXISTS (
		   SELECT 1 FROM object_state o
		   WHERE o.kind = object_links.to_kind AND o.project_id = object_links.to_project_id AND o.object_id = object_links.to_object_id)`)
	if err != nil {
		return r, fmt.Errorf("prune links: %w", err)
	}
	r.Outbox, err = q.exec(ctx, `DELETE FROM outbox WHERE attempts >= ?`, MaxOutboxAttempts)
	if err != nil {
		return r, fmt.Errorf("prune outbox: %w", err)
	}
	return r, nil
}

func (q queries) exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := q.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
