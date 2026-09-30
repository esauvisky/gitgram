package main

import (
	"context"
	"time"

	"github.com/esauvisky/gitgram/internal/store"
	"github.com/esauvisky/gitgram/internal/telegram"
)

// outboxAdapter presents *store.Store as the telegram.Outbox the sender
// consumes: it maps key triples onto store.Key and the store's row types
// onto the sender's. Cards in primary live in card_messages, the rows the
// engine owns; cards in any other chat live in card_mirrors.
type outboxAdapter struct {
	st      *store.Store
	primary int64
}

var _ telegram.Outbox = outboxAdapter{}

func (a outboxAdapter) OutboxHead(ctx context.Context) (*telegram.OutboxItem, error) {
	row, err := a.st.OutboxHead(ctx)
	if err != nil || row == nil {
		return nil, err
	}
	item := &telegram.OutboxItem{
		ID:         row.ID,
		ThreadID:   row.ThreadID,
		Op:         row.Op,
		Payload:    row.Payload,
		NotBefore:  row.NotBefore,
		Attempts:   row.Attempts,
		Generation: row.Generation,
		SentChats:  row.SentChats,
	}
	if row.Card != nil {
		item.CardKind = row.Card.Kind
		item.CardProjectID = row.Card.ProjectID
		item.CardObjectID = row.Card.ObjectID
	}
	return item, nil
}

func (a outboxAdapter) DeferOutbox(ctx context.Context, id int64, notBefore time.Time, attempts int, lastError string) error {
	return a.st.DeferOutbox(ctx, id, notBefore, lastError, attempts)
}

func (a outboxAdapter) DeleteOutbox(ctx context.Context, id, generation int64) error {
	return a.st.DeleteOutbox(ctx, id, generation)
}

func (a outboxAdapter) MarkOutboxSent(ctx context.Context, id, chatID int64) error {
	return a.st.MarkOutboxSent(ctx, id, chatID)
}

func (a outboxAdapter) GetCard(ctx context.Context, chatID int64, kind string, projectID, objectID int64) (*telegram.Card, error) {
	key := store.Key{Kind: kind, ProjectID: projectID, ObjectID: objectID}
	var row *store.CardRow
	var err error
	if chatID == a.primary {
		row, err = a.st.GetCard(ctx, key)
	} else {
		row, err = a.st.GetMirror(ctx, key, chatID)
		if err == nil && row == nil {
			return &telegram.Card{Status: telegram.CardLive}, nil
		}
	}
	if err != nil || row == nil {
		return nil, err
	}
	card := &telegram.Card{ThreadID: row.ThreadID, RenderedHash: row.RenderedHash, Status: row.Status}
	if row.MessageID != nil {
		id := int(*row.MessageID)
		card.MessageID = &id
	}
	return card, nil
}

func (a outboxAdapter) SetCardMessage(ctx context.Context, chatID int64, kind string, projectID, objectID int64, threadID *int64, messageID int, hash string) error {
	key := store.Key{Kind: kind, ProjectID: projectID, ObjectID: objectID}
	if chatID == a.primary {
		return a.st.SetCardMessage(ctx, key, threadID, int64(messageID), hash)
	}
	return a.st.SetMirrorMessage(ctx, key, chatID, threadID, int64(messageID), hash)
}

func (a outboxAdapter) SetCardHash(ctx context.Context, chatID int64, kind string, projectID, objectID int64, hash string) error {
	key := store.Key{Kind: kind, ProjectID: projectID, ObjectID: objectID}
	if chatID == a.primary {
		return a.st.SetCardHash(ctx, key, hash)
	}
	return a.st.SetMirrorHash(ctx, key, chatID, hash)
}

func (a outboxAdapter) SetCardStatus(ctx context.Context, chatID int64, kind string, projectID, objectID int64, status string) error {
	key := store.Key{Kind: kind, ProjectID: projectID, ObjectID: objectID}
	if chatID == a.primary {
		return a.st.SetCardStatus(ctx, key, status)
	}
	return a.st.SetMirrorStatus(ctx, key, chatID, status)
}

func (a outboxAdapter) GetObject(ctx context.Context, kind string, projectID, objectID int64) ([]byte, error) {
	row, err := a.st.GetObject(ctx, store.Key{Kind: kind, ProjectID: projectID, ObjectID: objectID})
	if err != nil || row == nil {
		return nil, err
	}
	return row.StateJSON, nil
}
