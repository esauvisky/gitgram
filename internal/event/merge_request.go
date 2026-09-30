package event

import "time"

// Merge request hook actions.
const (
	MRActionOpen       = "open"
	MRActionClose      = "close"
	MRActionReopen     = "reopen"
	MRActionUpdate     = "update"
	MRActionApproval   = "approval"
	MRActionApproved   = "approved"
	MRActionUnapproval = "unapproval"
	MRActionUnapproved = "unapproved"
	MRActionMerge      = "merge"
)

// Merge request states.
const (
	MRStateOpened = "opened"
	MRStateClosed = "closed"
	MRStateMerged = "merged"
	MRStateLocked = "locked"
)

// Keys of MergeRequest.Changes and Issue.Changes that the reducers read.
const (
	ChangeTitle                       = "title"
	ChangeDescription                 = "description"
	ChangeDraft                       = "draft"
	ChangeLabels                      = "labels"
	ChangeAssignees                   = "assignees"
	ChangeReviewers                   = "reviewers"
	ChangeBlockingDiscussionsResolved = "blocking_discussions_resolved"
	ChangeTargetBranch                = "target_branch"
	ChangeMilestone                   = "milestone_id"
	ChangeConfidential                = "confidential"
	ChangeDueDate                     = "due_date"
)

// Reviewer is a requested reviewer together with their review state
// (unreviewed, reviewed, requested_changes, approved, ...).
type Reviewer struct {
	User
	State string
}

// MergeRequest is a Merge Request Hook.
type MergeRequest struct {
	Meta
	Project Project
	// User is the actor who performed Action.
	User User

	// Action is one of the MRAction* constants.
	Action string
	ID     int64
	IID    int64
	Title  string
	// Description is the raw markdown body.
	Description string
	URL         string
	// State is one of the MRState* constants.
	State string
	Draft bool

	SourceBranch string
	TargetBranch string
	// SourceProjectID differs from Project.ID for fork MRs.
	SourceProjectID int64

	Author    User
	Assignees []User
	Reviewers []Reviewer
	Labels    []string

	// DetailedMergeStatus is detailed_merge_status: mergeable, conflict,
	// ci_still_running, discussions_not_resolved, draft_status, need_rebase,
	// not_approved, requested_changes, ...
	DetailedMergeStatus         string
	BlockingDiscussionsResolved bool
	// HeadPipelineID is head_pipeline_id, nil when no pipeline ran yet.
	HeadPipelineID *int64
	LastCommit     Commit

	// MergedBy and MergedAt are set on merge.
	MergedBy *User
	MergedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	// Changes is the changes{} map of the payload, keyed by attribute name.
	Changes map[string]Change
}

// EventKind implements Event.
func (*MergeRequest) EventKind() Kind { return KindMergeRequest }

// Proj implements Event.
func (m *MergeRequest) Proj() Project { return m.Project }

// Actor implements Event.
func (m *MergeRequest) Actor() User { return m.User }

// Ref returns an MRRef for this merge request.
func (m *MergeRequest) Ref() MRRef {
	return MRRef{
		ID: m.ID, IID: m.IID, Title: m.Title, URL: m.URL,
		SourceBranch: m.SourceBranch, TargetBranch: m.TargetBranch,
		State: m.State, Draft: m.Draft,
	}
}
