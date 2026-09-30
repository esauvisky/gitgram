package api

import (
	"context"
	"net/url"
)

// Project is one entry of GET /groups/:id/projects?simple=true.
type Project struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	DefaultBranch     string `json:"default_branch"`
	// Archived is not part of the simple representation; GroupProjects
	// filters with archived=false so it is always false there.
	Archived bool `json:"archived"`
}

// GroupProjects implements Reader.
func (c *Client) GroupProjects(ctx context.Context, group string) ([]Project, error) {
	q := url.Values{
		"include_subgroups": {"true"},
		"archived":          {"false"},
		"simple":            {"true"},
		"with_shared":       {"false"},
	}
	return getPages[Project](ctx, c, "/groups/"+url.PathEscape(group)+"/projects", q)
}
