package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Compare is the body of GET /projects/:id/repository/compare.
type Compare struct {
	// Commits are the commits reachable from `to` but not from `from`.
	Commits        []Commit `json:"commits"`
	CompareTimeout bool     `json:"compare_timeout"`
	CompareSameRef bool     `json:"compare_same_ref"`
	WebURL         string   `json:"web_url"`
}

// Commit is a commit object as returned by the REST API (compare, jobs).
type Commit struct {
	ID          string    `json:"id"`
	ShortID     string    `json:"short_id"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   time.Time `json:"created_at"`
	WebURL      string    `json:"web_url"`
}

// Compare implements Reader.
func (c *Client) Compare(ctx context.Context, projectID int64, from, to string, straight bool) (*Compare, error) {
	q := url.Values{
		"from":     {from},
		"to":       {to},
		"straight": {strconv.FormatBool(straight)},
	}
	var out Compare
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/repository/compare", projectID), q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
