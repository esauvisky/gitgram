package cards

import (
	"slices"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// IssueState is the state behind one issue card.
type IssueState struct {
	SchemaVer int
	Project   event.Project
	ID        int64
	IID       int64
	Title     string
	// Description is the raw markdown body.
	Description string
	// State is one of the event.IssueState* constants.
	State        string
	Confidential bool
	Author       event.User
	Assignees    []event.User
	Labels       []string
	URL          string

	// LastChange is the last notable change, for the card footer.
	LastChange Change
	// LastEventAt is the receive time of the last applied issue event;
	// older deliveries are ignored.
	LastEventAt time.Time
	// Final is true once closed.
	Final bool
}

// Key returns the object_state key of this issue.
func (s *IssueState) Key() Key { return Key{Kind: KindIssue, ProjectID: s.Project.ID, ObjectID: s.IID} }

// IsFinal reports whether the card needs no further edits from events.
func (s *IssueState) IsFinal() bool { return s.Final }

// ReduceIssue folds an Issue Hook into s. s may be zero-valued.
//
// Every action overwrites the object attributes; open, reopen and close set
// State, Final and LastChange; update records LastChange from the first known
// key in Changes (title, description, labels, assignees, confidential,
// milestone, due_date) and leaves it alone otherwise. A delivery received
// before the last applied one is ignored.
func ReduceIssue(s *IssueState, ev *event.Issue) (changed bool) {
	if s.SchemaVer == 0 {
		s.SchemaVer = SchemaVer
	}
	if ev.Received.Before(s.LastEventAt) {
		return false
	}
	s.LastEventAt = ev.Received
	before := snapshot(s)

	s.Project = ev.Project
	s.ID = ev.ID
	s.IID = ev.IID
	s.Title = ev.Title
	s.Description = ev.Description
	if ev.State != "" {
		s.State = ev.State
	}
	s.Confidential = ev.Confidential
	if !ev.Author.IsZero() {
		s.Author = ev.Author
	}
	s.Assignees = slices.Clone(ev.Assignees)
	s.Labels = slices.Clone(ev.Labels)
	s.URL = ev.URL

	switch ev.Action {
	case event.IssueActionOpen:
		s.State = event.IssueStateOpened
		s.setChange(ChangeOpened, ev)
	case event.IssueActionReopen:
		s.State = event.IssueStateOpened
		s.setChange(ChangeReopened, ev)
	case event.IssueActionClose:
		s.State = event.IssueStateClosed
		s.setChange(ChangeClosed, ev)
	case event.IssueActionUpdate:
		if k := issueChangeKind(ev.Changes); k != "" {
			s.setChange(k, ev)
		}
	}
	s.Final = s.State == event.IssueStateClosed
	return differs(before, s)
}

func (s *IssueState) setChange(k ChangeKind, ev *event.Issue) {
	s.LastChange = Change{Kind: k, At: ev.Received, By: ev.User}
}

func issueChangeKind(changes map[string]event.Change) ChangeKind {
	ordered := []struct {
		key  string
		kind ChangeKind
	}{
		{event.ChangeTitle, ChangeTitle},
		{event.ChangeDescription, ChangeDescription},
		{event.ChangeLabels, ChangeLabels},
		{event.ChangeAssignees, ChangeAssignees},
		{event.ChangeConfidential, ChangeConfidential},
		{event.ChangeMilestone, ChangeMilestone},
		{event.ChangeDueDate, ChangeDueDate},
	}
	for _, o := range ordered {
		if _, ok := changes[o.key]; ok {
			return o.kind
		}
	}
	return ""
}
