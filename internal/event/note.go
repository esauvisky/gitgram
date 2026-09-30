package event

import "time"

// Note hook noteable types.
const (
	NoteableCommit       = "Commit"
	NoteableMergeRequest = "MergeRequest"
	NoteableIssue        = "Issue"
	NoteableSnippet      = "Snippet"
)

// Note hook actions.
const (
	NoteActionCreate = "create"
	NoteActionUpdate = "update"
)

// Note is a Note Hook: a comment on a merge request, issue, commit or
// snippet.
type Note struct {
	Meta
	Project Project
	// User is the comment author.
	User User

	ID int64
	// NoteableType is one of the Noteable* constants.
	NoteableType string
	// Body is the raw markdown body.
	Body string
	URL  string
	// DiscussionID groups replies of one thread.
	DiscussionID string
	// IsDiff is true for DiffNote (a comment on a diff line). FilePath and
	// Line are then set from position{}.
	IsDiff   bool
	FilePath string
	Line     int
	// System is true for GitLab-generated notes (e.g. "added 1 commit").
	System bool
	// Action is one of the NoteAction* constants.
	Action string

	// Exactly one of MR, Issue or CommitSHA is set according to
	// NoteableType; snippets set none.
	MR        *MRRef
	Issue     *IssueRef
	CommitSHA string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// EventKind implements Event.
func (*Note) EventKind() Kind { return KindNote }

// Proj implements Event.
func (n *Note) Proj() Project { return n.Project }

// Actor implements Event.
func (n *Note) Actor() User { return n.User }
