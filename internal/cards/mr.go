package cards

import (
	"slices"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// MaxNotes caps MRState.Notes; older notes are dropped first.
const MaxNotes = 10

// MRState is the state behind one merge request card.
type MRState struct {
	SchemaVer int
	Project   event.Project
	IID       int64
	ID        int64
	URL       string
	Title     string
	// Description is the raw markdown body.
	Description string
	Draft       bool
	// State is one of the event.MRState* constants.
	State        string
	Author       event.User
	Assignees    []event.User
	Reviewers    []event.User
	Labels       []string
	SourceBranch string
	TargetBranch string

	DetailedMergeStatus         string
	BlockingDiscussionsResolved bool

	Approvals ApprovalState
	Threads   ThreadState

	// HeadPipelineID is head_pipeline_id from the last MR hook, 0 when
	// unknown.
	HeadPipelineID int64
	// HeadPipeline is maintained by the engine through SetHeadPipeline; it
	// is cleared when an MR event reports a different head_pipeline_id.
	HeadPipeline *PipelineSummary
	// Notes holds the last MaxNotes comments when the project folds notes
	// into the card (mr.collapse_notes); see AppendNote.
	Notes []NoteSummary

	// LastChange is the last notable change, for the card footer.
	LastChange Change
	// LastEventAt is the receive time of the last applied MR event; older
	// deliveries are ignored.
	LastEventAt time.Time

	MergedBy *event.User
	MergedAt *time.Time
	// Final is true once merged or closed.
	Final bool
}

// ApprovalState tracks approvals. By is maintained from approval events;
// Required and Left come from the approvals API and are only meaningful when
// Enriched is true.
type ApprovalState struct {
	By       []event.User
	Required int
	Left     int
	Enriched bool
}

// ThreadState tracks unresolved discussion count from the discussions API;
// Unresolved is only meaningful when Enriched is true.
type ThreadState struct {
	Unresolved int
	Enriched   bool
}

// NoteSummary is one comment folded into an MR card.
type NoteSummary struct {
	ID     int64
	Author event.User
	// Body is the raw markdown body.
	Body string
	URL  string
	// At is the note's created_at.
	At           time.Time
	DiscussionID string
	// IsDiff, FilePath and Line describe a diff comment.
	IsDiff   bool
	FilePath string
	Line     int
}

// NewNoteSummary builds a NoteSummary from a Note Hook.
func NewNoteSummary(ev *event.Note) NoteSummary {
	return NoteSummary{
		ID: ev.ID, Author: ev.User, Body: ev.Body, URL: ev.URL, At: ev.CreatedAt,
		DiscussionID: ev.DiscussionID, IsDiff: ev.IsDiff, FilePath: ev.FilePath, Line: ev.Line,
	}
}

// Key returns the object_state key of this merge request.
func (s *MRState) Key() Key { return Key{Kind: KindMR, ProjectID: s.Project.ID, ObjectID: s.IID} }

// IsFinal reports whether the card needs no further edits from events.
func (s *MRState) IsFinal() bool { return s.Final }

// ReduceMR folds a Merge Request Hook into s. s may be zero-valued.
//
// Every action overwrites the object attributes from the payload. open,
// reopen, close and merge set State, Final, MergedBy/MergedAt and LastChange;
// approval/approved add the actor to Approvals.By and unapproval/unapproved
// remove them (adjusting Left when Enriched); update records LastChange from
// the first known key in Changes (draft, title, description, labels,
// assignees, reviewers, blocking_discussions_resolved, target_branch,
// milestone) and leaves LastChange alone for updates with no known key. A
// delivery received before the last applied one is ignored.
func ReduceMR(s *MRState, ev *event.MergeRequest) (changed bool) {
	if s.SchemaVer == 0 {
		s.SchemaVer = SchemaVer
	}
	if ev.Received.Before(s.LastEventAt) {
		return false
	}
	s.LastEventAt = ev.Received
	before := snapshot(s)

	s.Project = ev.Project
	s.IID = ev.IID
	s.ID = ev.ID
	s.URL = ev.URL
	s.Title = ev.Title
	s.Description = ev.Description
	s.Draft = ev.Draft
	if ev.State != "" {
		s.State = ev.State
	}
	if !ev.Author.IsZero() {
		s.Author = ev.Author
	}
	s.Assignees = slices.Clone(ev.Assignees)
	s.Reviewers = make([]event.User, len(ev.Reviewers))
	for i, r := range ev.Reviewers {
		s.Reviewers[i] = r.User
	}
	s.Labels = slices.Clone(ev.Labels)
	s.SourceBranch = ev.SourceBranch
	s.TargetBranch = ev.TargetBranch
	s.DetailedMergeStatus = ev.DetailedMergeStatus
	s.BlockingDiscussionsResolved = ev.BlockingDiscussionsResolved
	if ev.HeadPipelineID != nil {
		s.HeadPipelineID = *ev.HeadPipelineID
		if s.HeadPipeline != nil && s.HeadPipeline.ID != *ev.HeadPipelineID {
			s.HeadPipeline = nil
		}
	}

	switch ev.Action {
	case event.MRActionOpen:
		s.State = event.MRStateOpened
		s.setChange(ChangeOpened, ev)
	case event.MRActionReopen:
		s.State = event.MRStateOpened
		s.setChange(ChangeReopened, ev)
	case event.MRActionClose:
		s.State = event.MRStateClosed
		s.setChange(ChangeClosed, ev)
	case event.MRActionMerge:
		s.State = event.MRStateMerged
		by := ev.User
		if ev.MergedBy != nil {
			by = *ev.MergedBy
		}
		s.MergedBy = &by
		at := ev.Received
		if ev.MergedAt != nil {
			at = *ev.MergedAt
		}
		s.MergedAt = &at
		s.setChange(ChangeMerged, ev)
	case event.MRActionApproval, event.MRActionApproved:
		if !slices.ContainsFunc(s.Approvals.By, func(u event.User) bool { return sameUser(u, ev.User) }) {
			s.Approvals.By = append(s.Approvals.By, ev.User)
			if s.Approvals.Enriched && s.Approvals.Left > 0 {
				s.Approvals.Left--
			}
		}
		s.setChange(ChangeApproved, ev)
	case event.MRActionUnapproval, event.MRActionUnapproved:
		n := len(s.Approvals.By)
		s.Approvals.By = slices.DeleteFunc(s.Approvals.By, func(u event.User) bool { return sameUser(u, ev.User) })
		if len(s.Approvals.By) < n && s.Approvals.Enriched && s.Approvals.Left < s.Approvals.Required {
			s.Approvals.Left++
		}
		s.setChange(ChangeUnapproved, ev)
	case event.MRActionUpdate:
		if k := mrChangeKind(ev.Changes, s.Draft); k != "" {
			s.setChange(k, ev)
		}
	}
	s.Final = s.State == event.MRStateMerged || s.State == event.MRStateClosed
	return differs(before, s)
}

// SetHeadPipeline replaces the head pipeline summary and reports whether it
// changed.
func (s *MRState) SetHeadPipeline(p PipelineSummary) (changed bool) {
	if s.HeadPipeline != nil && *s.HeadPipeline == p {
		return false
	}
	s.HeadPipeline = &p
	return true
}

// AppendNote adds n to Notes, replacing an existing note with the same ID
// (note updates) and dropping the oldest entries beyond MaxNotes. It reports
// whether Notes changed.
func (s *MRState) AppendNote(n NoteSummary) (changed bool) {
	for i, cur := range s.Notes {
		if cur.ID == n.ID {
			if cur == n {
				return false
			}
			s.Notes[i] = n
			return true
		}
	}
	s.Notes = append(s.Notes, n)
	if len(s.Notes) > MaxNotes {
		s.Notes = slices.Clone(s.Notes[len(s.Notes)-MaxNotes:])
	}
	return true
}

func (s *MRState) setChange(k ChangeKind, ev *event.MergeRequest) {
	s.LastChange = Change{Kind: k, At: ev.Received, By: ev.User}
}

// mrChangeKind picks the change kind for an update action from the changes
// map, in priority order.
func mrChangeKind(changes map[string]event.Change, draft bool) ChangeKind {
	if _, ok := changes[event.ChangeDraft]; ok {
		if draft {
			return ChangeDraft
		}
		return ChangeReady
	}
	ordered := []struct {
		key  string
		kind ChangeKind
	}{
		{event.ChangeTitle, ChangeTitle},
		{event.ChangeDescription, ChangeDescription},
		{event.ChangeLabels, ChangeLabels},
		{event.ChangeAssignees, ChangeAssignees},
		{event.ChangeReviewers, ChangeReviewers},
		{event.ChangeBlockingDiscussionsResolved, ChangeThreadsResolved},
		{event.ChangeTargetBranch, ChangeTargetBranch},
		{event.ChangeMilestone, ChangeMilestone},
	}
	for _, o := range ordered {
		if _, ok := changes[o.key]; ok {
			return o.kind
		}
	}
	return ""
}
