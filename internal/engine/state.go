package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/store"
)

// Object link relations stored in object_links.
const (
	relChild        = "child"    // parent pipeline → child pipeline
	relPushPipeline = "pipeline" // push → pipeline for its (branch, sha)
)

func skey(k cards.Key) store.Key {
	return store.Key{Kind: string(k.Kind), ProjectID: k.ProjectID, ObjectID: k.ObjectID}
}

// load reads the state behind key into a fresh T. row is nil when the object
// does not exist yet; T is then zero-valued for the reducer to initialise.
// A blob that no longer decodes is logged and treated as absent state.
func load[T any](ctx context.Context, tx *store.Tx, key cards.Key, log *slog.Logger) (*T, *store.ObjectRow, error) {
	row, err := tx.GetObject(ctx, skey(key))
	if err != nil {
		return nil, nil, err
	}
	st := new(T)
	if row != nil {
		if err := json.Unmarshal(row.StateJSON, st); err != nil {
			log.Error("corrupt object state, starting fresh", "key", key, "err", err)
			st = new(T)
		}
	}
	return st, row, nil
}

// unmarshalState decodes a stored row into st.
func unmarshalState(row store.ObjectRow, st any) error {
	return json.Unmarshal(row.StateJSON, st)
}

// put persists st as the state behind key.
func put(ctx context.Context, tx *store.Tx, key cards.Key, st any, final bool, lastEventAt time.Time) error {
	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("encode %v state: %w", key, err)
	}
	return tx.PutObject(ctx, store.ObjectRow{
		Key: skey(key), StateJSON: b, SchemaVer: cards.SchemaVer, Final: final, LastEventAt: lastEventAt,
	})
}

// latest returns the later of the stored last_event_at and t, so an ignored
// stale delivery never moves the reconciler clock backwards.
func latest(row *store.ObjectRow, t time.Time) time.Time {
	if row != nil && row.LastEventAt.After(t) {
		return row.LastEventAt
	}
	return t
}

// enqueueCard ensures a card_messages row exists and queues a render of the
// card. Bursts coalesce on the outbox's unique card index; the first send
// and final states are flushed to now instead of waiting coalesceDelay.
// card is the current card_messages row, nil when none exists; cards that
// Telegram reported deleted or uneditable are left alone.
func (e *Engine) enqueueCard(ctx context.Context, tx *store.Tx, key cards.Key, thread int64, card *store.CardRow, final bool) error {
	sk, t := skey(key), threadPtr(thread)
	if card == nil {
		if err := tx.UpsertCard(ctx, sk, t); err != nil {
			return err
		}
	} else if card.Status != store.CardLive {
		return nil
	}
	if err := tx.EnqueueCard(ctx, sk, t, time.Now().Add(coalesceDelay)); err != nil {
		return err
	}
	if final || card == nil || card.MessageID == nil {
		return tx.FlushCard(ctx, sk)
	}
	return nil
}
