package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Pipeline is GET /projects/:id/pipelines/:pipeline_id. The same shape, with
// fewer fields filled, appears as MR.HeadPipeline and Job.Pipeline.
type Pipeline struct {
	ID        int64 `json:"id"`
	IID       int64 `json:"iid"`
	ProjectID int64 `json:"project_id"`
	// Status is one of the GitLab pipeline statuses (created, pending,
	// running, success, failed, canceled, skipped, manual, scheduled, ...).
	Status string `json:"status"`
	// Source is push, web, trigger, schedule, api, merge_request_event,
	// parent_pipeline, ...
	Source string `json:"source"`
	Ref    string `json:"ref"`
	SHA    string `json:"sha"`
	Tag    bool   `json:"tag"`
	WebURL string `json:"web_url"`
	User   *User  `json:"user"`
	// Duration is seconds; nil until the pipeline finishes.
	Duration       *int       `json:"duration"`
	QueuedDuration *int       `json:"queued_duration"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

// Job is one entry of GET /projects/:id/pipelines/:pipeline_id/jobs.
type Job struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	// AllowFailure is true for jobs whose failure does not fail the pipeline.
	AllowFailure  bool   `json:"allow_failure"`
	FailureReason string `json:"failure_reason"`
	// Duration and QueuedDuration are fractional seconds; nil until set.
	Duration       *float64   `json:"duration"`
	QueuedDuration *float64   `json:"queued_duration"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	WebURL         string     `json:"web_url"`
	Ref            string     `json:"ref"`
	Tag            bool       `json:"tag"`
	// Commit.ID is the real commit SHA here (unlike Job Hook payloads).
	Commit   *Commit  `json:"commit"`
	Pipeline Pipeline `json:"pipeline"`
	User     *User    `json:"user"`
}

// Pipeline implements Reader.
func (c *Client) Pipeline(ctx context.Context, projectID, id int64) (*Pipeline, error) {
	var out Pipeline
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/pipelines/%d", projectID, id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PipelineJobs implements Reader.
func (c *Client) PipelineJobs(ctx context.Context, projectID, id int64, includeRetried bool) ([]Job, error) {
	q := url.Values{}
	if includeRetried {
		q.Set("include_retried", "true")
	}
	return getPages[Job](ctx, c, fmt.Sprintf("/projects/%d/pipelines/%d/jobs", projectID, id), q)
}
