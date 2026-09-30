package engine

import (
	"context"
	"time"
)

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
	r, err := e.st.Prune(ctx, time.Now())
	if err != nil {
		e.log.Error("janitor: prune failed", "err", err)
		return
	}
	e.log.Info("janitor: pruned",
		"deliveries", r.Deliveries, "objects", r.Objects, "links", r.Links, "outbox", r.Outbox)
}
