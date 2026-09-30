package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Approvals is GET /projects/:id/merge_requests/:iid/approvals. On Free
// tiers ApprovalsRequired/ApprovalsLeft may be absent and stay zero.
type Approvals struct {
	ApprovalsRequired int
	ApprovalsLeft     int
	ApprovedBy        []User
}

// UnmarshalJSON flattens approved_by[].user into ApprovedBy.
func (a *Approvals) UnmarshalJSON(b []byte) error {
	var raw struct {
		ApprovalsRequired int `json:"approvals_required"`
		ApprovalsLeft     int `json:"approvals_left"`
		ApprovedBy        []struct {
			User User `json:"user"`
		} `json:"approved_by"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	a.ApprovalsRequired = raw.ApprovalsRequired
	a.ApprovalsLeft = raw.ApprovalsLeft
	a.ApprovedBy = a.ApprovedBy[:0]
	for _, e := range raw.ApprovedBy {
		a.ApprovedBy = append(a.ApprovedBy, e.User)
	}
	return nil
}

// MR is GET /projects/:id/merge_requests/:iid.
type MR struct {
	ID          int64  `json:"id"`
	IID         int64  `json:"iid"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// State is opened, closed, merged or locked.
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`

	Author    User     `json:"author"`
	Assignees []User   `json:"assignees"`
	Reviewers []User   `json:"reviewers"`
	Labels    []string `json:"labels"`

	DetailedMergeStatus         string `json:"detailed_merge_status"`
	HasConflicts                bool   `json:"has_conflicts"`
	BlockingDiscussionsResolved bool   `json:"blocking_discussions_resolved"`
	UserNotesCount              int    `json:"user_notes_count"`

	// HeadPipeline is the latest pipeline on the source branch head; nil
	// when none ran.
	HeadPipeline *Pipeline `json:"head_pipeline"`

	MergeUser *User      `json:"merge_user"`
	MergedAt  *time.Time `json:"merged_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type discussion struct {
	ID    string `json:"id"`
	Notes []struct {
		Resolvable bool `json:"resolvable"`
		Resolved   bool `json:"resolved"`
	} `json:"notes"`
}

// MRApprovals implements Reader.
func (c *Client) MRApprovals(ctx context.Context, projectID, iid int64) (*Approvals, error) {
	var out Approvals
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/merge_requests/%d/approvals", projectID, iid), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MRDiscussions implements Reader.
func (c *Client) MRDiscussions(ctx context.Context, projectID, iid int64) (int, error) {
	ds, err := getPages[discussion](ctx, c, fmt.Sprintf("/projects/%d/merge_requests/%d/discussions", projectID, iid), nil)
	if err != nil {
		return 0, err
	}
	unresolved := 0
	for _, d := range ds {
		if len(d.Notes) > 0 && d.Notes[0].Resolvable && !d.Notes[0].Resolved {
			unresolved++
		}
	}
	return unresolved, nil
}

// MRDiffs implements Reader.
func (c *Client) MRDiffs(ctx context.Context, projectID, iid int64) ([]Diff, error) {
	return getPages[Diff](ctx, c, fmt.Sprintf("/projects/%d/merge_requests/%d/diffs", projectID, iid), nil)
}

// MergeRequest implements Reader.
func (c *Client) MergeRequest(ctx context.Context, projectID, iid int64) (*MR, error) {
	var out MR
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/merge_requests/%d", projectID, iid), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
