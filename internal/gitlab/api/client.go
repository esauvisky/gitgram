// Package api is a hand-written client for the subset of the GitLab REST API
// (v4) that Gitgram uses: enrichment reads (compare, approvals, discussions,
// merge request and pipeline detail, group projects) and webhook CRUD for
// the sync-hooks subcommand.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiPrefix      = "/api/v4"
	perPage        = "100"
	requestTimeout = 10 * time.Second
	maxErrorBody   = 512
	maxRetryAfter  = 60 * time.Second
)

// Sentinel errors matched by Error.Is so callers can errors.Is against them
// without inspecting status codes.
var (
	ErrNotFound  = errors.New("gitlab: not found")
	ErrForbidden = errors.New("gitlab: forbidden")
)

// Error is a non-2xx response from GitLab.
type Error struct {
	Method string
	URL    string
	Status int
	// Body is the response body, truncated to maxErrorBody bytes.
	Body string
	// RetryAfter is the parsed Retry-After header of a 429 response.
	RetryAfter time.Duration
}

// Error implements error.
func (e *Error) Error() string {
	return fmt.Sprintf("gitlab: %s %s: %d %s", e.Method, e.URL, e.Status, strings.TrimSpace(e.Body))
}

// Is maps 404 to ErrNotFound and 403 to ErrForbidden.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrForbidden:
		return e.Status == http.StatusForbidden
	}
	return false
}

// Client talks to one GitLab instance with one personal/group access token.
// It is safe for concurrent use.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
}

// New returns a client for the GitLab instance at baseURL (e.g.
// "https://gitlab.com") authenticating with token via the PRIVATE-TOKEN
// header. Requests time out after 10 s and a 429 is retried once after
// Retry-After. A nil logger falls back to slog.Default().
func New(baseURL, token string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/") + apiPrefix,
		token:   token,
		http:    &http.Client{Timeout: requestTimeout},
		log:     logger,
	}
}

// do performs one API call, retrying once when GitLab answers 429. body, when
// non-nil, is JSON-encoded; out, when non-nil, receives the decoded 2xx body.
// The response headers are returned so list calls can paginate.
func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) (http.Header, error) {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("gitlab: encode %s %s: %w", method, path, err)
		}
	}
	hdr, err := c.once(ctx, method, u, payload, out)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests {
		return hdr, err
	}
	c.log.Warn("gitlab api rate limited, retrying once", "method", method, "url", u, "retry_after", apiErr.RetryAfter)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(apiErr.RetryAfter):
	}
	return c.once(ctx, method, u, payload, out)
}

func (c *Client) once(ctx context.Context, method, u string, payload []byte, out any) (http.Header, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %s %s: %w", method, u, err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()
	c.log.Debug("gitlab api", "method", method, "url", u, "status", resp.StatusCode, "took", time.Since(start))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		e := &Error{Method: method, URL: u, Status: resp.StatusCode, Body: string(b)}
		if resp.StatusCode == http.StatusTooManyRequests {
			e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		}
		return resp.Header, e
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return nil, fmt.Errorf("gitlab: decode %s %s: %w", method, u, err)
		}
	}
	return resp.Header, nil
}

// parseRetryAfter accepts delay-seconds or an HTTP-date; anything else, or a
// value in the past, yields one second. The result is capped at maxRetryAfter.
func parseRetryAfter(v string) time.Duration {
	d := time.Second
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil && time.Until(t) > 0 {
		d = time.Until(t)
	}
	return min(d, maxRetryAfter)
}

// getPages fetches every page of a list endpoint, following X-Next-Page with
// per_page=100.
func getPages[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("per_page", perPage)
	var all []T
	for page := "1"; page != ""; {
		q.Set("page", page)
		var items []T
		hdr, err := c.do(ctx, http.MethodGet, path, q, nil, &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		page = hdr.Get("X-Next-Page")
	}
	return all, nil
}
