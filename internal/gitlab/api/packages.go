package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Package is one entry of GET /groups/:id/packages: one version of a
// package in the package registry.
type Package struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	PackageType string    `json:"package_type"`
	ProjectID   int64     `json:"project_id"`
	CreatedAt   time.Time `json:"created_at"`
	Links       struct {
		WebPath string `json:"web_path"`
	} `json:"_links"`
	// CreatorID is the user who uploaded the version; nil when GitLab
	// recorded none (most CI job token uploads).
	CreatorID *int64 `json:"creator_id"`
}

// GroupPackagesSince pages through GET /groups/:id/packages newest first and
// returns the versions whose id is above afterID, subgroups included.
func (c *Client) GroupPackagesSince(ctx context.Context, group string, afterID int64) ([]Package, error) {
	q := url.Values{"order_by": {"created_at"}, "sort": {"desc"}, "per_page": {perPage}}
	var out []Package
	for page := "1"; page != ""; {
		q.Set("page", page)
		var items []Package
		hdr, err := c.do(ctx, http.MethodGet, "/groups/"+url.PathEscape(group)+"/packages", q, nil, &items)
		if err != nil {
			return nil, err
		}
		for _, p := range items {
			if p.ID <= afterID {
				return out, nil
			}
			out = append(out, p)
		}
		page = hdr.Get("X-Next-Page")
	}
	return out, nil
}

// RegistryRepository is one entry of GET /groups/:id/registry/repositories:
// one image in a project's container registry.
type RegistryRepository struct {
	ID        int64 `json:"id"`
	ProjectID int64 `json:"project_id"`
	// Location is the image reference without tag, e.g.
	// "registry.gitlab.com/group/project".
	Location string `json:"location"`
}

// GroupRegistryRepositories lists every container image under group.
func (c *Client) GroupRegistryRepositories(ctx context.Context, group string) ([]RegistryRepository, error) {
	return getPages[RegistryRepository](ctx, c, "/groups/"+url.PathEscape(group)+"/registry/repositories", nil)
}

// RegistryTag is a tag of a container image. The list endpoint fills only
// Name; RegistryTagDetail fills the rest.
type RegistryTag struct {
	Name      string    `json:"name"`
	Digest    string    `json:"digest"`
	CreatedAt time.Time `json:"created_at"`
}

// RegistryTags is GET /projects/:id/registry/repositories/:repo_id/tags.
func (c *Client) RegistryTags(ctx context.Context, projectID, repoID int64) ([]RegistryTag, error) {
	return getPages[RegistryTag](ctx, c, fmt.Sprintf("/projects/%d/registry/repositories/%d/tags", projectID, repoID), nil)
}

// RegistryTagDetail is GET .../registry/repositories/:repo_id/tags/:tag.
func (c *Client) RegistryTagDetail(ctx context.Context, projectID, repoID int64, tag string) (*RegistryTag, error) {
	var out RegistryTag
	path := fmt.Sprintf("/projects/%d/registry/repositories/%d/tags/%s", projectID, repoID, url.PathEscape(tag))
	if _, err := c.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// User is GET /users/:id.
func (c *Client) User(ctx context.Context, id int64) (*User, error) {
	var out User
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/users/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
