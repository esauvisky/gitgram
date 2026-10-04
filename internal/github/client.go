package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal GitHub REST client authenticated with a token.
type Client struct {
	base  string
	token string
	hc    *http.Client
}

// New returns a client for the API at base (https://api.github.com or a
// GitHub Enterprise /api/v3 URL).
func New(base, token string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), token: token, hc: &http.Client{Timeout: 20 * time.Second}}
}

// Error is a non-2xx API answer.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string { return fmt.Sprintf("github: HTTP %d: %s", e.Status, e.Body) }

func (c *Client) do(ctx context.Context, method, path string, out any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, &Error{Status: resp.StatusCode, Body: msg}
	}
	if out != nil {
		return body, json.Unmarshal(body, out)
	}
	return body, nil
}

// CompareFile is one changed file of a comparison.
type CompareFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
}

// Compare is GET /repos/:repo/compare/:base...:head.
func (c *Client) Compare(ctx context.Context, repo, base, head string) ([]CompareFile, error) {
	var out struct {
		Files []CompareFile `json:"files"`
	}
	_, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/compare/%s...%s", repo, base, head), &out)
	return out.Files, err
}

// JobLog is GET /repos/:repo/actions/jobs/:id/logs: the job's plain log
// (GitHub redirects to storage; the client follows without the token).
func (c *Client) JobLog(ctx context.Context, repo string, jobID int64) (string, error) {
	body, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/actions/jobs/%d/logs", repo, jobID), nil)
	return string(body), err
}

// CancelRun is POST /repos/:repo/actions/runs/:id/cancel.
func (c *Client) CancelRun(ctx context.Context, repo string, runID int64) error {
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/actions/runs/%d/cancel", repo, runID), nil)
	return err
}

// RerunFailed is POST /repos/:repo/actions/runs/:id/rerun-failed-jobs.
func (c *Client) RerunFailed(ctx context.Context, repo string, runID int64) error {
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/actions/runs/%d/rerun-failed-jobs", repo, runID), nil)
	return err
}
