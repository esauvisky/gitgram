// Package cards holds the persisted state behind every live-edited Telegram
// card (pipeline, merge request, issue) and the pure reducers that fold
// normalized events into that state.
//
// Reducers never call time.Now and never do I/O: ordering uses
// event.Meta.Received, which the webhook handler stamps. Every state type is
// JSON-serialisable; the store persists it as an opaque blob tagged with
// SchemaVer. Renderers are a function of state only: anything they need to
// show, including "what changed last", lives in the state.
//
// Every reducer returns changed, which reports whether a render-relevant
// field differs from before the call. Bookkeeping fields (Last*EventAt) are
// updated regardless, so callers should persist the state either way and use
// changed only to decide whether to enqueue a card edit.
package cards

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// SchemaVer is the current layout version of every state type. Stored
// alongside the blob so future migrations can detect old layouts.
const SchemaVer = 1

// Kind is the card kind stored in object_state.kind and card_messages.kind.
type Kind string

// Card kinds.
const (
	KindPipeline Kind = "pipeline"
	KindMR       Kind = "mr"
	KindIssue    Kind = "issue"
)

// Key identifies one object_state row and therefore one card. ObjectID is
// the pipeline id for pipelines and the iid for merge requests and issues.
type Key struct {
	Kind      Kind
	ProjectID int64
	ObjectID  int64
}

// ChangeKind names the last notable change recorded on an MR or issue card
// so the renderer can show "labels changed by X" without a side channel.
type ChangeKind string

// Change kinds recorded in LastChange.
const (
	ChangeOpened          ChangeKind = "opened"
	ChangeReopened        ChangeKind = "reopened"
	ChangeClosed          ChangeKind = "closed"
	ChangeMerged          ChangeKind = "merged"
	ChangeApproved        ChangeKind = "approved"
	ChangeUnapproved      ChangeKind = "unapproved"
	ChangeDraft           ChangeKind = "draft"
	ChangeReady           ChangeKind = "ready"
	ChangeTitle           ChangeKind = "title"
	ChangeDescription     ChangeKind = "description"
	ChangeLabels          ChangeKind = "labels"
	ChangeAssignees       ChangeKind = "assignees"
	ChangeReviewers       ChangeKind = "reviewers"
	ChangeThreadsResolved ChangeKind = "threads_resolved"
	ChangeTargetBranch    ChangeKind = "target_branch"
	ChangeMilestone       ChangeKind = "milestone"
	ChangeConfidential    ChangeKind = "confidential"
	ChangeDueDate         ChangeKind = "due_date"
)

// Change records the last notable change on an MR or issue.
type Change struct {
	Kind ChangeKind
	// At is the receive time of the event that carried the change.
	At time.Time
	By event.User
}

// snapshot serialises a state so reducers can detect whether they changed
// anything. All state types marshal without error by construction.
func snapshot(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func differs(before []byte, v any) bool { return !bytes.Equal(before, snapshot(v)) }

func sameUser(a, b event.User) bool {
	if a.ID != 0 || b.ID != 0 {
		return a.ID == b.ID
	}
	return a.Username == b.Username
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
