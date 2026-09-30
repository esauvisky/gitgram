package engine

import (
	"context"
	"errors"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// prefetchTimeout bounds the REST calls made before the transaction so a
// slow GitLab API never pushes the webhook response past GitLab's 10 s.
const prefetchTimeout = 5 * time.Second

// enrichment carries REST data fetched before the transaction for merge
// request events. Zero when the API is unavailable or a call failed.
type enrichment struct {
	approvals  *api.Approvals
	unresolved int
	// threads is true when unresolved holds a fetched value.
	threads bool
}

// prefetch performs the read-only enrichment the plan allows before BEGIN:
// force-push detection for pushes (setting Push.Forced in place) and
// approvals plus unresolved thread count for merge requests. Failures are
// logged and degrade to no enrichment.
func (e *Engine) prefetch(ctx context.Context, ev event.Event) enrichment {
	if e.api == nil {
		return enrichment{}
	}
	switch v := ev.(type) {
	case *event.Push:
		if v.IsCreate() || v.IsDelete() {
			return enrichment{}
		}
		ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
		defer cancel()
		cmp, err := e.api.Compare(ctx, v.Project.ID, v.After, v.Before, true)
		if err != nil {
			e.logEnrich("compare", v.Project.Path, err)
			return enrichment{}
		}
		v.Forced = len(cmp.Commits) > 0
	case *event.MergeRequest:
		return e.enrichMR(ctx, v.Project.Path, v.Project.ID, v.IID)
	}
	return enrichment{}
}

// enrichMR fetches approvals and the unresolved discussion count for one
// merge request within prefetchTimeout.
func (e *Engine) enrichMR(ctx context.Context, projectPath string, projectID, iid int64) enrichment {
	ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
	defer cancel()
	var en enrichment
	if a, err := e.api.MRApprovals(ctx, projectID, iid); err != nil {
		e.logEnrich("approvals", projectPath, err)
	} else {
		en.approvals = a
	}
	if n, err := e.api.MRDiscussions(ctx, projectID, iid); err != nil {
		e.logEnrich("discussions", projectPath, err)
	} else {
		en.unresolved, en.threads = n, true
	}
	return en
}

// logEnrich logs a failed enrichment call; 403 is expected on tiers without
// the feature and only logged at debug.
func (e *Engine) logEnrich(what, project string, err error) {
	if errors.Is(err, api.ErrForbidden) {
		e.log.Debug("enrichment unavailable", "call", what, "project", project, "err", err)
		return
	}
	e.log.Warn("enrichment failed", "call", what, "project", project, "err", err)
}
