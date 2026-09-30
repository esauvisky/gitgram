package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// maxTrace caps how much of a job log is read; only its tail is shown.
const maxTrace = 4 << 20

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
	// Artifacts lists every artifact file of the job; the downloadable
	// bundle is the one with FileType "archive".
	Artifacts []JobArtifact `json:"artifacts"`
}

// JobArtifact is one artifacts[] entry of a job.
type JobArtifact struct {
	FileType string `json:"file_type"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
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

// JobTrace implements Reader. The endpoint answers text/plain, so it
// bypasses the JSON decoder; at most maxTrace bytes are read.
func (c *Client) JobTrace(ctx context.Context, projectID, jobID int64) (string, error) {
	u := c.baseURL + fmt.Sprintf("/projects/%d/jobs/%d/trace", projectID, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("gitlab: GET %s: %w", u, err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab: GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return "", &Error{Method: http.MethodGet, URL: u, Status: resp.StatusCode, Body: string(b)}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxTrace))
	if err != nil {
		return "", fmt.Errorf("gitlab: read %s: %w", u, err)
	}
	return string(b), nil
}
