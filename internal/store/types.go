package store

import "time"

// Key identifies one object_state row and its card. Kind is the card kind
// string ("pipeline", "mr", "issue"); ObjectID is the pipeline id or the
// MR/issue iid.
type Key struct {
	Kind      string
	ProjectID int64
	ObjectID  int64
}

// ObjectRow is one object_state row. StateJSON is the opaque card state
// blob; the store never decodes it.
type ObjectRow struct {
	Key
	StateJSON   []byte
	SchemaVer   int
	Final       bool
	LastEventAt time.Time
	UpdatedAt   time.Time
}

// Card message statuses.
const (
	CardLive       = "live"
	CardDeleted    = "deleted"
	CardUneditable = "uneditable"
)

// CardRow is one card_messages row: where (if anywhere) the card currently
// lives in Telegram and the hash of what was last rendered there.
type CardRow struct {
	Key
	ThreadID     *int64
	MessageID    *int64
	RenderedHash string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Outbox operations.
const (
	OpSend  = "send"
	OpReply = "reply"
	OpCard  = "card"
)

// OutboxRow is one pending sender job. For OpCard, Card is the card to
// render and send/edit and Payload is nil; for OpReply, Card is the anchor
// whose message the reply targets; for OpSend, Card is nil. Payload holds
// the pre-rendered message JSON for send and reply.
type OutboxRow struct {
	ID        int64
	ThreadID  *int64
	Op        string
	Card      *Key
	Payload   []byte
	NotBefore time.Time
	Attempts  int
	LastError string
	// Generation counts coalesced card enqueues that hit this row; a delete
	// must name the generation that was read.
	Generation int64
}

// Link is a directed, labelled edge between two objects
// (mr → head_pipeline, parent → child pipeline).
type Link struct {
	From Key
	Rel  string
	To   Key
}
