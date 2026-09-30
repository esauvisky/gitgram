package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// HookSpec is the writable part of a webhook, shared by project and group
// hooks (POST/PUT /projects/:id/hooks, /groups/:id/hooks).
type HookSpec struct {
	URL string `json:"url"`
	// Token is the X-Gitlab-Token secret. GitLab never returns it, so it is
	// empty on hooks read back from the API.
	Token                    string `json:"token,omitempty"`
	PushEvents               bool   `json:"push_events"`
	TagPushEvents            bool   `json:"tag_push_events"`
	IssuesEvents             bool   `json:"issues_events"`
	ConfidentialIssuesEvents bool   `json:"confidential_issues_events"`
	MergeRequestsEvents      bool   `json:"merge_requests_events"`
	NoteEvents               bool   `json:"note_events"`
	ConfidentialNoteEvents   bool   `json:"confidential_note_events"`
	JobEvents                bool   `json:"job_events"`
	PipelineEvents           bool   `json:"pipeline_events"`
	DeploymentEvents         bool   `json:"deployment_events"`
	ReleasesEvents           bool   `json:"releases_events"`
	// PushEventsBranchFilter is sent empty on purpose: branch filtering is
	// done by Gitgram's config, so any filter set by hand is cleared.
	PushEventsBranchFilter string `json:"push_events_branch_filter"`
	EnableSSLVerification  bool   `json:"enable_ssl_verification"`
}

// Hook is a webhook as read back from GitLab.
type Hook struct {
	ID int64 `json:"id"`
	HookSpec
	// ProjectID is set on project hooks, GroupID on group hooks.
	ProjectID *int64 `json:"project_id"`
	GroupID   *int64 `json:"group_id"`
	// AlertStatus is executable, disabled or temporarily_disabled.
	AlertStatus   string     `json:"alert_status"`
	DisabledUntil *time.Time `json:"disabled_until"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ProjectHooks is GET /projects/:id/hooks.
func (c *Client) ProjectHooks(ctx context.Context, projectID int64) ([]Hook, error) {
	return getPages[Hook](ctx, c, fmt.Sprintf("/projects/%d/hooks", projectID), nil)
}

// CreateProjectHook is POST /projects/:id/hooks.
func (c *Client) CreateProjectHook(ctx context.Context, projectID int64, spec HookSpec) (*Hook, error) {
	var out Hook
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/hooks", projectID), nil, spec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProjectHook is PUT /projects/:id/hooks/:hook_id.
func (c *Client) UpdateProjectHook(ctx context.Context, projectID, hookID int64, spec HookSpec) (*Hook, error) {
	var out Hook
	if _, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/projects/%d/hooks/%d", projectID, hookID), nil, spec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GroupHooks is GET /groups/:id/hooks (group is a path or numeric id).
func (c *Client) GroupHooks(ctx context.Context, group string) ([]Hook, error) {
	return getPages[Hook](ctx, c, "/groups/"+url.PathEscape(group)+"/hooks", nil)
}

// CreateGroupHook is POST /groups/:id/hooks.
func (c *Client) CreateGroupHook(ctx context.Context, group string, spec HookSpec) (*Hook, error) {
	var out Hook
	if _, err := c.do(ctx, http.MethodPost, "/groups/"+url.PathEscape(group)+"/hooks", nil, spec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGroupHook is PUT /groups/:id/hooks/:hook_id.
func (c *Client) UpdateGroupHook(ctx context.Context, group string, hookID int64, spec HookSpec) (*Hook, error) {
	var out Hook
	if _, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/groups/%s/hooks/%d", url.PathEscape(group), hookID), nil, spec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
