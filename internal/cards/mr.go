package cards

import (
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

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
	SourceBranch string
	TargetBranch string
	// DetailedMergeStatus is GitLab's merge readiness (mergeable,
	// conflict, ...); the card offers Merge only when it is mergeable.
	DetailedMergeStatus string

	// UnresolvedThreads is the count from the discussions API; Threads is
	// false until it was fetched.
	UnresolvedThreads int
	Threads           bool
	// Diff is what the MR changes, from the diffs API; nil until fetched.
	Diff *DiffStats

	// HeadPipelineID is head_pipeline_id from the last MR hook, 0 when
	// unknown. Pipeline is a copy of that pipeline's state, kept in step by
	// the engine and rendered in full on the card.
	HeadPipelineID int64
	Pipeline       *PipelineState

	MergedBy *event.User
	ClosedBy *event.User
	// SourceBranchDeleted is set when the source branch was deleted (GitLab
	// usually does it right after the merge).
	SourceBranchDeleted bool
	// ConfirmMerge is set while someone pressed Merge and the card is
	// asking to confirm; cleared by the answer.
	ConfirmMerge bool

	// LastEventAt is the receive time of the last applied MR event; older
	// deliveries are ignored.
	LastEventAt time.Time
	// Final is true once merged or closed.
	Final bool
}

// Key returns the object_state key of this merge request.
func (s *MRState) Key() Key { return Key{Kind: KindMR, ProjectID: s.Project.ID, ObjectID: s.IID} }

// IsFinal reports whether the card needs no further edits from events.
func (s *MRState) IsFinal() bool { return s.Final }

// ReduceMR folds a Merge Request Hook into s, which may be zero-valued.
// Every action overwrites the attributes from the payload; open and reopen
// mark it opened, close records who closed it, merge who merged it. A
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
	s.SourceBranch = ev.SourceBranch
	s.TargetBranch = ev.TargetBranch
	s.DetailedMergeStatus = ev.DetailedMergeStatus
	if ev.HeadPipelineID != nil && *ev.HeadPipelineID != s.HeadPipelineID {
		s.HeadPipelineID = *ev.HeadPipelineID
		if s.Pipeline != nil && s.Pipeline.ID != s.HeadPipelineID {
			s.Pipeline = nil
		}
	}

	switch ev.Action {
	case event.MRActionOpen, event.MRActionReopen:
		s.State = event.MRStateOpened
		s.ClosedBy = nil
	case event.MRActionClose:
		s.State = event.MRStateClosed
		by := ev.User
		s.ClosedBy = &by
	case event.MRActionMerge:
		s.State = event.MRStateMerged
		by := ev.User
		s.MergedBy = &by
	}
	s.Final = s.State == event.MRStateMerged || s.State == event.MRStateClosed
	if s.Final {
		s.ConfirmMerge = false
	}
	return differs(before, s)
}

// SetPipeline replaces the pipeline copy when p is the MR's head pipeline
// (or the head is unknown and p is newer), and reports whether anything
// changed.
func (s *MRState) SetPipeline(p *PipelineState) (changed bool) {
	if s.HeadPipelineID != 0 && p.ID != s.HeadPipelineID {
		return false
	}
	if s.Pipeline != nil && p.ID < s.Pipeline.ID {
		return false
	}
	before := snapshot(s)
	cp := *p
	s.Pipeline = &cp
	return differs(before, s)
}

// SetThreads records the unresolved thread count and reports whether it
// changed.
func (s *MRState) SetThreads(unresolved int) (changed bool) {
	if s.Threads && s.UnresolvedThreads == unresolved {
		return false
	}
	s.Threads, s.UnresolvedThreads = true, unresolved
	return true
}

// SetDiff records the diff stats and reports whether they changed.
func (s *MRState) SetDiff(d DiffStats) (changed bool) {
	before := snapshot(s)
	s.Diff = &d
	return differs(before, s)
}
