package api

import (
	"context"
	"fmt"
	"net/http"
)

// discussion is one entry of the discussions list; only the first note's
// resolvable/resolved flags matter for counting unresolved threads.
type discussion struct {
	Notes []struct {
		Resolvable bool `json:"resolvable"`
		Resolved   bool `json:"resolved"`
	} `json:"notes"`
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

// MergeMR implements Writer.
func (c *Client) MergeMR(ctx context.Context, projectID, iid int64) error {
	_, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/projects/%d/merge_requests/%d/merge", projectID, iid), nil, nil, nil)
	return err
}
