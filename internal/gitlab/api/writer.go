package api

import (
	"context"
	"fmt"
	"net/http"
)

// Writer is the write surface behind the card buttons. It needs an api
// scope token (gitlab.hooks_token).
type Writer interface {
	// CancelPipeline is POST /projects/:id/pipelines/:pipeline_id/cancel.
	CancelPipeline(ctx context.Context, projectID, id int64) (*Pipeline, error)
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
