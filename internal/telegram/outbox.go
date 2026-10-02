package telegram

import (
	"context"
	"time"
)

// Outbox ops, matching the outbox.op column.
const (
	OpSend = "send"
	OpCard = "card"
)

// Card statuses, matching card_messages.status.
const (
	CardLive       = "live"
	CardDeleted    = "deleted"
	CardUneditable = "uneditable"
)

// OutboxItem is one outbox row as the sender needs it.
type OutboxItem struct {
	ID       int64
	ThreadID *int64
	// Op is OpSend or OpCard.
	Op string
	// CardKind/CardProjectID/CardObjectID are the card target for OpCard and
	// empty for OpSend.
	CardKind      string
	CardProjectID int64
	CardObjectID  int64
	// Payload is Message JSON for OpSend; nil for OpCard.
	Payload   []byte
	NotBefore time.Time
	Attempts  int
	// Generation is the row's coalesce counter at read time; DeleteOutbox
	// only removes the row if it is still the same.
	Generation int64
	// SentChats lists the chats an OpSend row already reached.
	SentChats []int64
}

// Card is a card's message in one chat.
type Card struct {
	ThreadID *int64
	// MessageID is nil until the first send succeeded.
	MessageID    *int
	RenderedHash string
	// Status is CardLive, CardDeleted or CardUneditable.
	Status string
}

// Outbox is the store surface the Sender needs. The store package satisfies
// it structurally (or via a thin adapter); this package never imports it.
// Lookups return nil, nil when the row does not exist.
type Outbox interface {
	// OutboxHead returns the oldest outbox row regardless of not_before.
	OutboxHead(ctx context.Context) (*OutboxItem, error)
	// DeferOutbox reschedules a row and records the attempt.
	DeferOutbox(ctx context.Context, id int64, notBefore time.Time, attempts int, lastError string) error
	// DeleteOutbox removes a processed or abandoned row, unless its
	// generation changed since it was read (the row then stays queued).
	DeleteOutbox(ctx context.Context, id, generation int64) error
	// MarkOutboxSent records that an OpSend row is done in chatID.
	MarkOutboxSent(ctx context.Context, id, chatID int64) error

	// GetCard returns the card's message in chatID. In the primary chat nil
	// means the engine never created the card; in any other chat a card
	// never posted there comes back live with no message.
	GetCard(ctx context.Context, chatID int64, kind string, projectID, objectID int64) (*Card, error)
	// SetCardMessage records the first successful send in chatID: the
	// thread actually used, the message id and the hash of what was sent.
	SetCardMessage(ctx context.Context, chatID int64, kind string, projectID, objectID int64, threadID *int64, messageID int, hash string) error
	// SetCardHash records the hash after a successful edit in chatID.
	SetCardHash(ctx context.Context, chatID int64, kind string, projectID, objectID int64, hash string) error
	// SetCardStatus marks the card CardDeleted or CardUneditable in chatID.
	SetCardStatus(ctx context.Context, chatID int64, kind string, projectID, objectID int64, status string) error

	// GetObject returns object_state.state_json for a card key.
	GetObject(ctx context.Context, kind string, projectID, objectID int64) ([]byte, error)
}

// Renderer turns stored object state into a Message. The engine implements
// it: it knows the state type behind kind and the project's render options.
type Renderer interface {
	Render(kind string, stateJSON []byte, threadID *int64) (Message, error)
}
