package event

import "time"

// Issue hook actions.
const (
	IssueActionOpen   = "open"
	IssueActionClose  = "close"
	IssueActionReopen = "reopen"
	IssueActionUpdate = "update"
)

// Issue states.
const (
	IssueStateOpened = "opened"
	IssueStateClosed = "closed"
)

// Issue is an Issue Hook (also confidential issue hooks).
type Issue struct {
	Meta
	Project Project
	// User is the actor who performed Action.
	User User

	// Action is one of the IssueAction* constants.
	Action string
	ID     int64
	IID    int64
	Title  string
	// Description is the raw markdown body.
	Description string
	URL         string
	// State is one of the IssueState* constants.
	State        string
	Confidential bool

	Author    User
	Assignees []User
	Labels    []string

	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  *time.Time

	// Changes is the changes{} map of the payload, keyed by attribute name.
	Changes map[string]Change
}

// EventKind implements Event.
func (*Issue) EventKind() Kind { return KindIssue }

// Proj implements Event.
func (i *Issue) Proj() Project { return i.Project }

// Actor implements Event.
func (i *Issue) Actor() User { return i.User }

// Ref returns an IssueRef for this issue.
func (i *Issue) Ref() IssueRef {
	return IssueRef{ID: i.ID, IID: i.IID, Title: i.Title, URL: i.URL, State: i.State}
}
