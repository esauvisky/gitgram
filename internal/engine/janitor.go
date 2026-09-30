package engine

import (
	"context"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
)

// pushLinger is how long a push card without a finished pipeline stays
// live before the janitor finalises it: no pipeline ever came, or one is
// blocked on a manual job. A later pipeline event rewrites the row and the
// card resumes until the next expiry.
const pushLinger = 6 * time.Hour

// RunJanitor prunes expired deliveries, old final objects, dangling links and
// abandoned outbox rows once immediately and then every tick, until ctx is
// cancelled.
func (e *Engine) RunJanitor(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e.prune(ctx)
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
