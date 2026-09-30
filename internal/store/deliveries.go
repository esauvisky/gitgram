package store

import (
	"context"
	"fmt"
	"time"
)

// ClaimDelivery records a webhook delivery key. fresh is true when the key
// was not seen before; false means the delivery is a duplicate and must be
// ignored.
func (q queries) ClaimDelivery(ctx context.Context, key, kind string) (fresh bool, err error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO webhook_deliveries (key, event_kind, received_at) VALUES (?, ?, ?)`,
		key, kind, time.Now().Unix())
	if err != nil {
		return false, fmt.Errorf("claim delivery %q: %w", key, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim delivery %q: %w", key, err)
	}
	return n == 1, nil
}
