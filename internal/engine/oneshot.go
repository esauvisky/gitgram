package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyOneShot renders tag, release and deployment events immediately and
// queues them as standalone messages.
func (e *Engine) applyOneShot(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev event.Event) error {
	opts := e.options(eff)
	var msg render.Message
	var class config.EventClass
	switch v := ev.(type) {
	case *event.TagPush:
		msg, class = render.TagPush(v, opts), config.EventTag
	case *event.Release:
		msg, class = render.Release(v, opts), config.EventRelease
	case *event.Deployment:
		msg, class = render.Deployment(v, opts), config.EventDeployment
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode %s message: %w", ev.EventKind(), err)
	}
	return tx.EnqueueSend(ctx, threadPtr(eff.ThreadFor(class)), payload)
}
