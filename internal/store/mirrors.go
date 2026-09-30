package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetMirror loads the card's message in a secondary chat, or nil, nil when
// the card was never posted there.
func (q queries) GetMirror(ctx context.Context, key Key, chatID int64) (*CardRow, error) {
	var c CardRow
	var threadID, messageID sql.NullInt64
	var hash sql.NullString
	var createdAt, updatedAt int64
	err := q.db.QueryRowContext(ctx,
		`SELECT kind, project_id, object_id, thread_id, message_id, rendered_hash, status, created_at, updated_at
		 FROM card_mirrors WHERE kind = ? AND project_id = ? AND object_id = ? AND chat_id = ?`,
		key.Kind, key.ProjectID, key.ObjectID, chatID,
	).Scan(&c.Kind, &c.ProjectID, &c.ObjectID, &threadID, &messageID, &hash, &c.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get mirror %v in %d: %w", key, chatID, err)
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

// SetMirrorMessage records the card's first successful send in a secondary
// chat and marks it live.
func (q queries) SetMirrorMessage(ctx context.Context, key Key, chatID int64, threadID *int64, messageID int64, hash string) error {
	now := time.Now().Unix()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO card_mirrors (kind, project_id, object_id, chat_id, thread_id, message_id, rendered_hash, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (kind, project_id, object_id, chat_id) DO UPDATE SET
		   thread_id = excluded.thread_id, message_id = excluded.message_id,
		   rendered_hash = excluded.rendered_hash, status = excluded.status, updated_at = excluded.updated_at`,
		key.Kind, key.ProjectID, key.ObjectID, chatID, nullInt(threadID), messageID, hash, CardLive, now, now)
	if err != nil {
		return fmt.Errorf("set mirror message %v in %d: %w", key, chatID, err)
	}
	return nil
}

// SetMirrorHash records the hash shown in a secondary chat after an edit.
func (q queries) SetMirrorHash(ctx context.Context, key Key, chatID int64, hash string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE card_mirrors SET rendered_hash = ?, updated_at = ?
		 WHERE kind = ? AND project_id = ? AND object_id = ? AND chat_id = ?`,
		hash, time.Now().Unix(), key.Kind, key.ProjectID, key.ObjectID, chatID)
	if err != nil {
		return fmt.Errorf("set mirror hash %v in %d: %w", key, chatID, err)
	}
	return nil
}

// SetMirrorStatus sets the card's status in a secondary chat, creating the
// row when the card never reached that chat.
func (q queries) SetMirrorStatus(ctx context.Context, key Key, chatID int64, status string) error {
	now := time.Now().Unix()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO card_mirrors (kind, project_id, object_id, chat_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (kind, project_id, object_id, chat_id) DO UPDATE SET
		   status = excluded.status, updated_at = excluded.updated_at`,
		key.Kind, key.ProjectID, key.ObjectID, chatID, status, now, now)
	if err != nil {
		return fmt.Errorf("set mirror status %v in %d: %w", key, chatID, err)
	}
	return nil
}
