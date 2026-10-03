package api

import (
	"context"
	"fmt"
	"net/http"
)

// Writer is the write surface behind the card buttons. It needs an api
// scope token (GITGRAM_GITLAB_HOOKS_TOKEN).
type Writer interface {
	// CancelPipeline is POST /projects/:id/pipelines/:pipeline_id/cancel.
	CancelPipeline(ctx context.Context, projectID, id int64) (*Pipeline, error)
	// RetryPipeline is POST /projects/:id/pipelines/:pipeline_id/retry.
	RetryPipeline(ctx context.Context, projectID, id int64) (*Pipeline, error)
	// PlayJob is POST /projects/:id/jobs/:job_id/play.
	PlayJob(ctx context.Context, projectID, jobID int64) (*Job, error)
	// MergeMR is PUT /projects/:id/merge_requests/:iid/merge.
	MergeMR(ctx context.Context, projectID, iid int64) error
}

var _ Writer = (*Client)(nil)

// CancelPipeline implements Writer.
func (c *Client) CancelPipeline(ctx context.Context, projectID, id int64) (*Pipeline, error) {
	var out Pipeline
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/pipelines/%d/cancel", projectID, id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RetryPipeline implements Writer.
func (c *Client) RetryPipeline(ctx context.Context, projectID, id int64) (*Pipeline, error) {
	var out Pipeline
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/pipelines/%d/retry", projectID, id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PlayJob implements Writer.
func (c *Client) PlayJob(ctx context.Context, projectID, jobID int64) (*Job, error) {
	var out Job
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/jobs/%d/play", projectID, jobID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
