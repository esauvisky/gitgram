package event

import "time"

// Merge request actions (object_attributes.action). Approval actions arrive
// too and are folded like an update.
const (
	MRActionOpen   = "open"
	MRActionClose  = "close"
	MRActionReopen = "reopen"
	MRActionUpdate = "update"
	MRActionMerge  = "merge"
)

// Merge request states (object_attributes.state).
const (
	MRStateOpened = "opened"
	MRStateClosed = "closed"
	MRStateMerged = "merged"
)

// MergeStatusMergeable and MergeStatusConflict are the
// detailed_merge_status values the card acts on.
const (
	MergeStatusMergeable = "mergeable"
	MergeStatusConflict  = "conflict"
)

// MergeRequest is a Merge Request Hook.
type MergeRequest struct {
	Meta
	Project Project
	// User is who acted (opened, merged, closed, edited).
	User User

	Action      string
	ID          int64
	IID         int64
	Title       string
	Description string
	URL         string
	State       string
	Draft       bool

	SourceBranch string
	TargetBranch string

	// Author is the MR's author when the payload identifies them (the
	// acting user is the author); zero otherwise.
	Author User

	DetailedMergeStatus string
	// Additions, Deletions and ChangedFiles are the line counts GitHub puts
	// in pull request payloads; zero for GitLab, whose counts come from
	// the API.
	Additions, Deletions, ChangedFiles int
	// HeadPipelineID is head_pipeline_id, nil when the payload has none.
	HeadPipelineID *int64
	MergedAt       *time.Time
}

// EventKind implements Event.
func (*MergeRequest) EventKind() Kind { return KindMergeRequest }

// Proj implements Event.
func (m *MergeRequest) Proj() Project { return m.Project }

// Actor implements Event.
func (m *MergeRequest) Actor() User { return m.User }

// MRRef points at a merge request from a pipeline payload.
type MRRef struct {
	IID   int64
	Title string
	URL   string
}
