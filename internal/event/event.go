// Package event defines the normalized, transport-independent representation
// of GitLab webhook events. The webhook parser produces these values, the
// cards reducers consume them, and render reads them for one-shot messages.
//
// The package has no dependencies beyond the standard library and is frozen:
// every other package codes against these types.
package event

import "time"

// Event is implemented by every normalized event type in this package
// (*Push, *TagPush, *Pipeline, *Job, *MergeRequest, *Note, *Issue, *Release,
// *Deployment).
type Event interface {
	// EventKind identifies the concrete event type.
	EventKind() Kind
	// Proj returns the project the event belongs to.
	Proj() Project
	// Actor returns the user who caused the event. It is the zero User when
	// the payload carries no actor (releases).
	Actor() User
	// ReceivedAt is the instant the delivery was accepted by this bot. It is
	// the clock reducers use to order deliveries; it is never GitLab's time.
	ReceivedAt() time.Time
}

// Meta is embedded in every event and carries bot-side bookkeeping.
type Meta struct {
	// Received is when the delivery was accepted. Set by the webhook handler
	// (or by the reconciler for synthetic events); reducers compare it to
	// order deliveries and never call time.Now themselves.
	Received time.Time
}

// ReceivedAt implements Event.
func (m Meta) ReceivedAt() time.Time { return m.Received }
