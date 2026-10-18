package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// latestTag is the tag images move on every release; it is re-read each
// poll so a retag to a new digest is noticed.
const latestTag = "latest"

// registryWatch is what the registry watcher has already seen of a group.
type registryWatch struct {
	// PackageID is the newest package version id seen.
	PackageID int64
	// Images maps a container repository id to its tags and the digest each
	// pointed at when it was read ("" for tags present at the first poll,
	// which are never read again unless they are latestTag).
	Images map[int64]map[string]string
}

// RunRegistryWatch polls the GitLab group's package and container
// registries, which send no webhooks, and posts what was published since
// the previous poll as package cards. The first poll only records what
// exists. It runs once immediately, then every tick, until ctx is
// cancelled. Without an API reader or a GitLab group it returns at once.
func (e *Engine) RunRegistryWatch(ctx context.Context, every time.Duration) {
	if e.cfg.GitLab.Group == "" {
		return
	}
	if e.api == nil {
		e.log.Info("registry watch disabled: no GITGRAM_GITLAB_TOKEN")
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := e.watchRegistry(ctx); err != nil {
			e.log.Error("registry watch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// watchRegistry is one poll. State is saved only after every new
// publication was handled; a failed poll is repeated whole next tick and
// delivery dedupe drops what was already posted.
func (e *Engine) watchRegistry(ctx context.Context) error {
	group := e.cfg.GitLab.Group
	scope := "gitlab:" + group
	raw, err := e.st.GetWatch(ctx, scope)
	if err != nil {
		return err
	}
	first := raw == nil
	w := registryWatch{Images: map[int64]map[string]string{}}
	if !first {
		if err := json.Unmarshal(raw, &w); err != nil {
			return fmt.Errorf("decode registry watch %q: %w", scope, err)
		}
	}

	pkgs, err := e.api.GroupPackagesSince(ctx, group, w.PackageID)
	if err != nil {
		return fmt.Errorf("list packages: %w", err)
	}
	repos, err := e.api.GroupRegistryRepositories(ctx, group)
	if err != nil {
		return fmt.Errorf("list container repositories: %w", err)
	}
	type pushedTag struct {
		repo        api.RegistryRepository
		tag, digest string
	}
	var tags []pushedTag
	images := map[int64]map[string]string{}
	for _, repo := range repos {
		list, err := e.api.RegistryTags(ctx, repo.ProjectID, repo.ID)
		if err != nil {
			return fmt.Errorf("list tags of %s: %w", repo.Location, err)
		}
		seen := w.Images[repo.ID]
		cur := map[string]string{}
		for _, t := range list {
			old, was := seen[t.Name]
			digest := old
			if t.Name == latestTag || (!first && !was) {
				d, err := e.api.RegistryTagDetail(ctx, repo.ProjectID, repo.ID, t.Name)
				if err != nil {
					return fmt.Errorf("read tag %s:%s: %w", repo.Location, t.Name, err)
				}
				digest = d.Digest
				if !first && (!was || digest != old) {
					tags = append(tags, pushedTag{repo, t.Name, digest})
				}
			}
			cur[t.Name] = digest
		}
		images[repo.ID] = cur
	}
	w.Images = images
	for _, p := range pkgs {
		w.PackageID = max(w.PackageID, p.ID)
	}

	if !first && (len(pkgs) > 0 || len(tags) > 0) {
		list, err := e.api.GroupProjects(ctx, group)
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		projects := make(map[int64]event.Project, len(list))
		for _, p := range list {
			projects[p.ID] = event.Project{ID: p.ID, Path: p.PathWithNamespace, Name: p.Name, WebURL: p.WebURL, DefaultBranch: p.DefaultBranch}
		}
		project := func(id int64) (event.Project, error) {
			p, ok := projects[id]
			if !ok {
				return p, fmt.Errorf("project %d is not an active project of %s", id, group)
			}
			return p, nil
		}
		for _, t := range tags {
			proj, err := project(t.repo.ProjectID)
			if err != nil {
				return err
			}
			ev := &event.Package{Meta: event.Meta{Received: time.Now()}, Project: proj, Type: event.PackageContainer,
				Name: t.repo.Location, VersionID: imageVersionID(t.repo.ID, t.digest), Version: t.tag, Image: t.repo.Location,
				URL: proj.WebURL + "/container_registry/" + strconv.FormatInt(t.repo.ID, 10)}
			if err := e.Handle(ctx, "gl-img:"+strconv.FormatInt(t.repo.ID, 10)+":"+t.tag+":"+t.digest, ev); err != nil {
				return err
			}
		}
		for i := len(pkgs) - 1; i >= 0; i-- {
			p := pkgs[i]
			proj, err := project(p.ProjectID)
			if err != nil {
				return err
			}
			ev := &event.Package{Meta: event.Meta{Received: time.Now()}, Project: proj, Type: p.PackageType,
				Name: p.Name, VersionID: p.ID, Version: p.Version, URL: e.cfg.GitLab.BaseURL + p.Links.WebPath}
			// GitLab records no creator for most CI job token uploads.
			if p.CreatorID != nil {
				u, err := e.api.User(ctx, *p.CreatorID)
				if err != nil {
					return fmt.Errorf("read creator of package %d: %w", p.ID, err)
				}
				ev.User = event.User{ID: u.ID, Username: u.Username, Name: u.Name}
			}
			if err := e.Handle(ctx, "gl-pkg:"+strconv.FormatInt(p.ID, 10), ev); err != nil {
				return err
			}
		}
	}

	b, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("encode registry watch %q: %w", scope, err)
	}
	return e.st.PutWatch(ctx, scope, b)
}

// imageVersionID is the card object id of one image digest: GitLab's
// registry has no version id, and every tag of a digest shares its card.
func imageVersionID(repoID int64, digest string) int64 {
	h := fnv.New64a()
	h.Write([]byte(strconv.FormatInt(repoID, 10)))
	h.Write([]byte{0})
	h.Write([]byte(digest))
	return int64(h.Sum64())
}
