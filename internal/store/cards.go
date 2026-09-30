package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetCard loads one card_messages row, or nil, nil when absent.
func (q queries) GetCard(ctx context.Context, key Key) (*CardRow, error) {
	var c CardRow
	var threadID, messageID sql.NullInt64
	var hash sql.NullString
	var createdAt, updatedAt int64
	err := q.db.QueryRowContext(ctx,
		`SELECT kind, project_id, object_id, thread_id, message_id, rendered_hash, status, created_at, updated_at
		 FROM card_messages WHERE kind = ? AND project_id = ? AND object_id = ?`,
		key.Kind, key.ProjectID, key.ObjectID,
	).Scan(&c.Kind, &c.ProjectID, &c.ObjectID, &threadID, &messageID, &hash, &c.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get card %v: %w", key, err)
	}
	if threadID.Valid {
		c.ThreadID = &threadID.Int64
	}
	if messageID.Valid {
		c.MessageID = &messageID.Int64
	}
	c.RenderedHash = hash.String
	c.CreatedAt = time.Unix(createdAt, 0)
	c.UpdatedAt = time.Unix(updatedAt, 0)
	return &c, nil
}

// UpsertCard ensures a card_messages row exists for key with status live and
// no message yet. An existing row is left untouched: its thread is fixed
// once the card has been posted. The object_state row must already exist
// (foreign key).
func (q queries) UpsertCard(ctx context.Context, key Key, threadID *int64) error {
	now := time.Now().Unix()
	_, err := q.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO card_messages (kind, project_id, object_id, thread_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.Kind, key.ProjectID, key.ObjectID, nullInt(threadID), CardLive, now, now)
	if err != nil {
		return fmt.Errorf("upsert card %v: %w", key, err)
	}
	return nil
}

// SetCardMessage records the Telegram message a card was posted as: the
// thread it actually landed in (nil for General, which may differ from the
// queued thread after a "thread not found" fallback), the message id and the
// hash of the content sent. It also marks the card live.
func (q queries) SetCardMessage(ctx context.Context, key Key, threadID *int64, messageID int64, hash string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE card_messages SET thread_id = ?, message_id = ?, rendered_hash = ?, status = ?, updated_at = ?
		 WHERE kind = ? AND project_id = ? AND object_id = ?`,
		nullInt(threadID), messageID, hash, CardLive, time.Now().Unix(), key.Kind, key.ProjectID, key.ObjectID)
	if err != nil {
		return fmt.Errorf("set card message %v: %w", key, err)
	}
	return nil
}

// SetCardHash records the hash of the content currently shown in Telegram.
func (q queries) SetCardHash(ctx context.Context, key Key, hash string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE card_messages SET rendered_hash = ?, updated_at = ?
		 WHERE kind = ? AND project_id = ? AND object_id = ?`,
		hash, time.Now().Unix(), key.Kind, key.ProjectID, key.ObjectID)
	if err != nil {
		return fmt.Errorf("set card hash %v: %w", key, err)
	}
	return nil
}

// SetCardStatus sets the card status (CardLive, CardDeleted, CardUneditable).
func (q queries) SetCardStatus(ctx context.Context, key Key, status string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE card_messages SET status = ?, updated_at = ?
		 WHERE kind = ? AND project_id = ? AND object_id = ?`,
		status, time.Now().Unix(), key.Kind, key.ProjectID, key.ObjectID)
	if err != nil {
		return fmt.Errorf("set card status %v: %w", key, err)
	}
	return nil
}

// nullInt converts an optional id into a driver-friendly NULL-able value.
func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}
