package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/store"
)

const (
	// staleAfter is how long a non-final object may go without an event
	// before the reconciler re-reads it from the API.
	staleAfter = 10 * time.Minute
	// reconcileTimeout bounds the API calls for one object.
	reconcileTimeout = 30 * time.Second
)

// RunReconciler re-reads non-final pipelines and merge requests that have
// not seen an event for staleAfter and feeds them through the normal reduce
// path as synthetic events, covering deliveries GitLab dropped. It runs once
// immediately, then every tick, until ctx is cancelled. Without an API
// reader it returns at once.
func (e *Engine) RunReconciler(ctx context.Context, every time.Duration) {
	if e.api == nil {
		e.log.Info("reconciler disabled: no GITGRAM_GITLAB_TOKEN")
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e.reconcile(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (e *Engine) reconcile(ctx context.Context) {
	stale := time.Now().Add(-staleAfter)
	rows, err := e.st.ListNonFinal(ctx, string(cards.KindPipeline), stale)
	if err != nil {
		e.log.Error("reconcile: list pipelines", "err", err)
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		e.reconcilePipeline(ctx, row)
	}
}

func (e *Engine) reconcilePipeline(ctx context.Context, row store.ObjectRow) {
	var st cards.PipelineState
	if err := json.Unmarshal(row.StateJSON, &st); err != nil {
		e.log.Error("reconcile: corrupt pipeline state", "key", row.Key, "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()
	p, err := e.api.Pipeline(ctx, row.ProjectID, row.ObjectID)
	if err != nil {
		e.reconcileFetchError(ctx, row, err)
		return
	}
	jobs, err := e.api.PipelineJobs(ctx, row.ProjectID, row.ObjectID, false)
	if err != nil {
		e.reconcileFetchError(ctx, row, err)
		return
	}
	e.applySynthetic(ctx, synthPipeline(&st, p, jobs, time.Now()), enrichment{})
}

// reconcileFetchError logs a failed fetch. An object GitLab no longer knows
// is marked final so it is not polled forever.
func (e *Engine) reconcileFetchError(ctx context.Context, row store.ObjectRow, err error) {
	if !errors.Is(err, api.ErrNotFound) {
		e.log.Warn("reconcile: fetch failed", "key", row.Key, "err", err)
		return
	}
	e.log.Info("reconcile: object gone, marking final", "key", row.Key)
	row.Final = true
	if err := e.st.PutObject(ctx, row); err != nil {
		e.log.Error("reconcile: mark final", "key", row.Key, "err", err)
	}
}

// applySynthetic runs a reconciler-built event through apply, bypassing
// delivery dedupe, and wakes the sender on success.
func (e *Engine) applySynthetic(ctx context.Context, ev event.Event, en enrichment) {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		return e.apply(ctx, tx, ev, en)
	})
	if err != nil {
		e.log.Error("reconcile: apply failed", "kind", ev.EventKind(), "project", ev.Proj().Path, "err", err)
		return
	}
	e.notify()
}
