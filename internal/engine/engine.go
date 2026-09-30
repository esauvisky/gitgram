// Package engine turns normalized GitLab events into persisted card state
// and outbox rows: dedupe, project and branch filtering, optional REST
// enrichment, the cards reducers, object links and card enqueueing all
// happen here, inside one store transaction per delivery. It also implements
// the sender's Renderer and runs the reconciler and janitor loops.
package engine

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/store"
)

// coalesceDelay is how long a card edit waits in the outbox so bursts of
// job events collapse into one Telegram edit.
const coalesceDelay = 700 * time.Millisecond

// errDuplicate aborts the delivery transaction when the key was seen before.
var errDuplicate = errors.New("duplicate delivery")

// Engine processes events for one bot instance.
type Engine struct {
	cfg    *config.Config
	st     *store.Store
	api    api.Reader
	writer api.Writer
	notify func()
	log    *slog.Logger
}

// New returns an Engine. reader may be nil when no gitlab.read_token is
// configured: enrichment, log tails and the reconciler are then disabled.
// writer may be nil when no gitlab.hooks_token is configured: cards then
// carry no action buttons. notify is called after every committed
// transaction that added outbox rows (the sender's Notify).
func New(cfg *config.Config, st *store.Store, reader api.Reader, writer api.Writer, notify func(), logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{cfg: cfg, st: st, api: reader, writer: writer, notify: notify, log: logger}
}

// Handle processes one webhook delivery. It returns an error only when the
// store fails; filtered, duplicate and otherwise uninteresting deliveries
// return nil so the webhook handler answers 200.
func (e *Engine) Handle(ctx context.Context, deliveryKey string, ev event.Event) error {
	proj := ev.Proj()
	log := e.log.With("kind", ev.EventKind(), "project", proj.Path)
	if proj.Path != e.cfg.GitLab.Group && !e.cfg.Accepts(proj.Path) {
		log.Debug("event outside configured group")
		return nil
	}
	eff := e.cfg.Resolve(proj.Path)
	class, ok := classOf(ev)
	if !ok {
		log.Debug("event has no relayed class")
		return nil
	}
	if !eff.EventEnabled(class) {
		log.Debug("event class disabled", "class", class)
		return nil
	}
	if ref, ok := filterRef(ev); ok && !eff.BranchAllowed(ref) {
		log.Debug("branch filtered", "ref", ref)
		return nil
	}
	if n, ok := ev.(*event.Note); ok && n.System && !eff.MR.ShowSystemNotes {
		log.Debug("system note skipped")
		return nil
	}

	en := e.prefetch(ctx, ev)

	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		fresh, err := tx.ClaimDelivery(ctx, deliveryKey, ev.EventKind().String())
		if err != nil {
			return err
		}
		if !fresh {
			return errDuplicate
		}
		return e.apply(ctx, tx, eff, ev, en)
	})
	if errors.Is(err, errDuplicate) {
		log.Debug("duplicate delivery", "key", deliveryKey)
		return nil
	}
	if err != nil {
		return err
	}
	e.notify()
	return nil
}

// apply routes one event to its handler inside tx. It is shared by Handle
// and the reconciler, which bypasses dedupe.
func (e *Engine) apply(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev event.Event, en enrichment) error {
	switch v := ev.(type) {
	case *event.Pipeline, *event.Job:
		return e.applyPipeline(ctx, tx, eff, ev, en)
	case *event.MergeRequest:
		return e.applyMR(ctx, tx, eff, v, en)
	case *event.Note:
		return e.applyNote(ctx, tx, eff, v)
	case *event.Issue:
		return e.applyIssue(ctx, tx, eff, v)
	case *event.Push:
		if v.IsDelete() {
			return e.applyBranchDeleted(ctx, tx, eff, v)
		}
		return e.applyPush(ctx, tx, eff, v, en)
	case *event.TagPush, *event.Release, *event.Deployment:
		return e.applyOneShot(ctx, tx, eff, ev)
	}
	e.log.Warn("unhandled event type", "kind", ev.EventKind())
	return nil
}

// classOf maps an event to its config event class. Notes on commits and
// snippets have no class and are not relayed.
func classOf(ev event.Event) (config.EventClass, bool) {
	switch v := ev.(type) {
	case *event.Push:
		return config.EventPush, true
	case *event.TagPush:
		return config.EventTag, true
	case *event.Pipeline, *event.Job:
		return config.EventPipeline, true
	case *event.MergeRequest:
		return config.EventMR, true
	case *event.Note:
		switch v.NoteableType {
		case event.NoteableMergeRequest:
			return config.EventMRNote, v.MR != nil
		case event.NoteableIssue:
			return config.EventIssueNote, v.Issue != nil
		}
		return "", false
	case *event.Issue:
		return config.EventIssue, true
	case *event.Release:
		return config.EventRelease, true
	case *event.Deployment:
		return config.EventDeployment, true
	}
	return "", false
}

// filterRef returns the ref the branch allow/deny filter applies to, when
// the event has one: push and pipeline/job refs, and the target branch for
// merge requests and their notes.
func filterRef(ev event.Event) (string, bool) {
	switch v := ev.(type) {
	case *event.Push:
		return v.Ref, true
	case *event.Pipeline:
		return v.Ref, true
	case *event.Job:
		return v.Ref, true
	case *event.MergeRequest:
		return v.TargetBranch, true
	case *event.Note:
		if v.MR != nil && v.MR.TargetBranch != "" {
			return v.MR.TargetBranch, true
		}
	}
	return "", false
}

// threadPtr converts a config thread id into the store's optional form; 0
// (General) becomes nil.
func threadPtr(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}
