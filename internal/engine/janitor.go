package engine

import (
	"context"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/store"
)

// pushLinger is how long a push card without a finished pipeline stays
// live before the janitor finalises it: no pipeline ever came, or one is
// blocked on a manual job. A later pipeline event rewrites the row and the
// card resumes until the next expiry.
const pushLinger = 6 * time.Hour

// RunJanitor prunes expired deliveries, old final objects, dangling links and
// abandoned outbox rows, and retires expired Retry buttons, once
// immediately and then every tick, until ctx is cancelled.
func (e *Engine) RunJanitor(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e.prune(ctx)
		e.expireRetry(ctx, every)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (e *Engine) prune(ctx context.Context) {
	if n, err := e.st.ExpireNonFinal(ctx, string(cards.KindPush), time.Now().Add(-pushLinger)); err != nil {
		e.log.Error("janitor: expire pushes failed", "err", err)
	} else if n > 0 {
		e.log.Info("janitor: expired push cards", "count", n)
	}
	r, err := e.st.Prune(ctx, time.Now())
	if err != nil {
		e.log.Error("janitor: prune failed", "err", err)
		return
	}
	e.log.Info("janitor: pruned",
		"deliveries", r.Deliveries, "objects", r.Objects, "links", r.Links, "outbox", r.Outbox)
}

// expireRetry re-renders the cards whose pipeline stopped about
// render.RetryWindow ago, so the Retry button they drew goes away. Push and
// merge request cards draw it too when they show the pipeline. The range
// reaches back two ticks so a late pass misses none; a render that changes
// nothing is hash-skipped by the sender.
func (e *Engine) expireRetry(ctx context.Context, every time.Duration) {
	now := time.Now()
	from, to := now.Add(-render.RetryWindow-2*every), now.Add(-render.RetryWindow)
	queued := 0
	for _, kind := range []cards.Kind{cards.KindPipeline, cards.KindPush, cards.KindMR} {
		rows, err := e.st.ListTouched(ctx, string(kind), from, to)
		if err != nil {
			e.log.Error("janitor: list cards failed", "kind", kind, "err", err)
			return
		}
		for _, row := range rows {
			key := cards.Key{Kind: kind, ProjectID: row.ProjectID, ObjectID: row.ObjectID}
			err := e.st.WithTx(ctx, func(tx *store.Tx) error {
				card, err := tx.GetCard(ctx, row.Key)
				if err != nil || card == nil {
					return err
				}
				queued++
				return e.enqueueCard(ctx, tx, key, card, true)
			})
			if err != nil {
				e.log.Error("janitor: requeue card failed", "key", key, "err", err)
				return
			}
		}
	}
	if queued > 0 {
		e.log.Info("janitor: re-rendered cards past the retry window", "count", queued)
		e.notify()
	}
}
