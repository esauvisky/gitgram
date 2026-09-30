package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EnqueueCard queues a render-and-send/edit of the card identified by key.
// The partial unique index keeps at most one pending card row per key, so
// bursts coalesce: when a row is already pending its not_before is left as
// it was (use FlushCard to pull it forward) and its generation is bumped so
// a sender that is mid-flight on the old state does not delete the row.
func (q queries) EnqueueCard(ctx context.Context, key Key, threadID *int64, notBefore time.Time) error {
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO outbox
		   (thread_id, op, card_kind, card_project_id, card_object_id, payload, not_before, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
		 ON CONFLICT(card_kind, card_project_id, card_object_id) WHERE op = 'card'
		 DO UPDATE SET generation = generation + 1`,
		nullInt(threadID), OpCard, key.Kind, key.ProjectID, key.ObjectID, notBefore.UnixMilli(), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("enqueue card %v: %w", key, err)
	}
	return nil
}

// FlushCard makes an already pending card row for key eligible now, if it is
// still in its coalesce delay. Rows the sender has already attempted keep
// the not_before it assigned (retry_after, backoff). No-op when nothing is
// pending.
func (q queries) FlushCard(ctx context.Context, key Key) error {
	now := time.Now().UnixMilli()
	_, err := q.db.ExecContext(ctx,
		`UPDATE outbox SET not_before = ?
		 WHERE op = ? AND card_kind = ? AND card_project_id = ? AND card_object_id = ? AND not_before > ? AND attempts = 0`,
		now, OpCard, key.Kind, key.ProjectID, key.ObjectID, now)
	if err != nil {
		return fmt.Errorf("flush card %v: %w", key, err)
	}
	return nil
}

// EnqueueSend queues a standalone message with a pre-rendered payload.
func (q queries) EnqueueSend(ctx context.Context, threadID *int64, payload []byte) error {
	now := time.Now()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO outbox (thread_id, op, payload, not_before, created_at) VALUES (?, ?, ?, ?, ?)`,
		nullInt(threadID), OpSend, payload, now.UnixMilli(), now.Unix())
	if err != nil {
		return fmt.Errorf("enqueue send: %w", err)
	}
	return nil
}

// EnqueueReply queues a pre-rendered message to be sent as a reply to the
// card identified by anchor.
func (q queries) EnqueueReply(ctx context.Context, anchor Key, threadID *int64, payload []byte) error {
	now := time.Now()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO outbox
		   (thread_id, op, card_kind, card_project_id, card_object_id, payload, not_before, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(threadID), OpReply, anchor.Kind, anchor.ProjectID, anchor.ObjectID, payload, now.UnixMilli(), now.Unix())
	if err != nil {
		return fmt.Errorf("enqueue reply to %v: %w", anchor, err)
	}
	return nil
}

// OutboxHead returns the oldest outbox row regardless of not_before (the
// sender waits until it is due), or nil, nil when the outbox is empty.
func (q queries) OutboxHead(ctx context.Context) (*OutboxRow, error) {
	var r OutboxRow
	var threadID, cardProjectID, cardObjectID sql.NullInt64
	var cardKind, lastError sql.NullString
	var notBefore int64
	var sent string
	err := q.db.QueryRowContext(ctx,
		`SELECT id, thread_id, op, card_kind, card_project_id, card_object_id, payload, not_before, attempts, last_error, generation, sent_chats
		 FROM outbox ORDER BY id LIMIT 1`,
	).Scan(&r.ID, &threadID, &r.Op, &cardKind, &cardProjectID, &cardObjectID, &r.Payload, &notBefore, &r.Attempts, &lastError, &r.Generation, &sent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("outbox head: %w", err)
	}
	if threadID.Valid {
		r.ThreadID = &threadID.Int64
	}
	if cardKind.Valid {
		r.Card = &Key{Kind: cardKind.String, ProjectID: cardProjectID.Int64, ObjectID: cardObjectID.Int64}
	}
	r.NotBefore = time.UnixMilli(notBefore)
	r.LastError = lastError.String
	for _, part := range strings.Split(sent, ",") {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			r.SentChats = append(r.SentChats, id)
		}
	}
	return &r, nil
}

// MarkOutboxSent records that a send or reply row was delivered to (or
// given up on in) chatID, so a retry of the row skips that chat.
func (q queries) MarkOutboxSent(ctx context.Context, id, chatID int64) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE outbox SET sent_chats = CASE sent_chats WHEN '' THEN ? ELSE sent_chats || ',' || ? END WHERE id = ?`,
		strconv.FormatInt(chatID, 10), strconv.FormatInt(chatID, 10), id)
	if err != nil {
		return fmt.Errorf("mark outbox %d sent to %d: %w", id, chatID, err)
	}
	return nil
}

// DeferOutbox reschedules a row after a failed or rate-limited attempt.
func (q queries) DeferOutbox(ctx context.Context, id int64, notBefore time.Time, lastError string, attempts int) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE outbox SET not_before = ?, last_error = ?, attempts = ? WHERE id = ?`,
		notBefore.UnixMilli(), lastError, attempts, id)
	if err != nil {
		return fmt.Errorf("defer outbox %d: %w", id, err)
	}
	return nil
}

// DeleteOutbox removes a row once it has been sent or given up on. The row
// is kept when its generation moved on since it was read: a card enqueue
// landed while the sender was processing it, and it must be rendered again.
func (q queries) DeleteOutbox(ctx context.Context, id, generation int64) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM outbox WHERE id = ? AND generation = ?`, id, generation); err != nil {
		return fmt.Errorf("delete outbox %d: %w", id, err)
	}
	return nil
}
