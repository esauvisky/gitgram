package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Compare is the body of GET /projects/:id/repository/compare.
type Compare struct {
	// Commits are the commits reachable from `to` but not from `from`.
	Commits []Commit `json:"commits"`
	// Diffs holds one entry per changed file.
	Diffs          []Diff `json:"diffs"`
	CompareTimeout bool   `json:"compare_timeout"`
	CompareSameRef bool   `json:"compare_same_ref"`
	WebURL         string `json:"web_url"`
}

// Diff is one entry of compare.diffs: the unified hunks of one file. Diff
// starts at the first @@ line; GitLab omits the ---/+++ file headers.
type Diff struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
	Diff        string `json:"diff"`
}

// LineCounts counts the added and removed lines of the hunks: lines that
// start with + or -, excluding the +++/--- file headers should GitLab ever
// include them. Binary diffs count 0 and 0.
func (d Diff) LineCounts() (added, removed int) {
	for _, l := range strings.Split(d.Diff, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ ") || strings.HasPrefix(l, "--- "):
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			removed++
		}
	}
	return added, removed
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
