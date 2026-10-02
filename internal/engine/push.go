package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyPush folds a branch push into the push card keyed by (project,
// branch, after), applies the diff enrichment, marks the branch's previous
// push card superseded and enqueues both cards.
func (e *Engine) applyPush(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev *event.Push, en enrichment) error {
	key := cards.PushKey(ev.Project.ID, ev.Branch(), ev.After)
	st, row, err := load[cards.PushState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReducePush(st, ev)
	if en.diff != nil && st.SetDiff(*en.diff) {
		changed = true
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.Received)); err != nil {
		return err
	}
	if !ev.IsCreate() && ev.Before != ev.After {
		if err := e.supersedePush(ctx, tx, eff, cards.PushKey(ev.Project.ID, ev.Branch(), ev.Before), ev.After); err != nil {
			return err
		}
	}
	card, err := tx.GetCard(ctx, skey(key))
	if err != nil {
		return err
	}
	if !changed && card != nil {
		return nil
	}
	return e.enqueueCard(ctx, tx, key, eff.ThreadFor(config.EventPush), card, st.Final)
}

// applyBranchDeleted posts the deletion as a standalone message and freezes
// the branch's last push card.
func (e *Engine) applyBranchDeleted(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev *event.Push) error {
	if err := e.supersedePush(ctx, tx, eff, cards.PushKey(ev.Project.ID, ev.Branch(), ev.Before), ""); err != nil {
		return err
	}
	payload, err := json.Marshal(render.BranchDeleted(ev, e.options(eff)))
	if err != nil {
		return fmt.Errorf("encode branch deletion: %w", err)
	}
	return tx.EnqueueSend(ctx, threadPtr(eff.ThreadFor(config.EventPush)), payload)
}

// supersedePush freezes the previous push card when it exists and is not
// already superseded, and flushes its final edit. Skeletons have no card
// to edit.
func (e *Engine) supersedePush(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, prevKey cards.Key, by string) error {
	prev, row, err := load[cards.PushState](ctx, tx, prevKey, e.log)
	if err != nil || row == nil {
		return err
	}
	if !prev.Supersede(by) {
		return nil
	}
	if err := put(ctx, tx, prevKey, prev, true, row.LastEventAt); err != nil {
		return err
	}
	if !prev.SeenPush {
		return nil
	}
	card, err := tx.GetCard(ctx, skey(prevKey))
	if err != nil {
		return err
	}
	return e.enqueueCard(ctx, tx, prevKey, eff.ThreadFor(config.EventPush), card, true)
}

// syncPipelinePush keeps the push card of a pipeline's (branch, sha) in
// step with it. Only a pipeline the Pipeline Hook confirms as triggered by
// a push (source "push") is attached: its state is copied into the push
// card, a skeleton is created when the Push Hook has not arrived yet, and
// push → pipeline is recorded. The push card absorbs it (renders it in full
// and the pipeline gets no card of its own) when the pipeline had no card
// when it first met a live push card.
//
// A pipeline from anywhere else (web, api, schedule, ...) never touches the
// push card, and one attached before its source was known is released.
// While the source is still unknown (Job Hooks usually arrive before the
// Pipeline Hook) and a live push card exists for the commit, hold reports
// that no card should be posted yet. Child, tag and non-branch refs are
// skipped, as are projects without push cards.
func (e *Engine) syncPipelinePush(ctx context.Context, tx *store.Tx, key cards.Key, st *cards.PipelineState, received time.Time) (absorbed, hold bool, err error) {
	if st.Parent != nil || st.Tag || st.SHA == "" || st.Ref == "" || strings.HasPrefix(st.Ref, "refs/") {
		return false, false, nil
	}
	eff := e.cfg.Resolve(st.Project.Path)
	if !eff.EventEnabled(config.EventPush) {
		return false, false, nil
	}
	pk := cards.PushKey(st.Project.ID, st.Ref, st.SHA)
	push, row, err := load[cards.PushState](ctx, tx, pk, e.log)
	if err != nil {
		return false, false, err
	}
	confirmed := st.SeenPipelineEvent && st.Source == "push"
	if row == nil {
		if !confirmed {
			return false, false, nil
		}
		push = cards.NewPushSkeleton(st.Project, st.Ref, st.SHA)
	}
	if push.Pipeline != nil && st.ID < push.Pipeline.ID {
		return false, false, nil
	}
	mine := push.Pipeline != nil && push.Pipeline.ID == st.ID

	if !confirmed {
		if !st.SeenPipelineEvent && push.SeenPush && !push.Superseded {
			ownCard, err := tx.GetCard(ctx, skey(key))
			if err != nil {
				return false, false, err
			}
			if ownCard == nil {
				return false, true, nil
			}
		}
		if !mine || !st.SeenPipelineEvent || !push.ReleasePipeline() {
			return false, false, nil
		}
		if err := put(ctx, tx, pk, push, push.Final, row.LastEventAt); err != nil {
			return false, false, err
		}
		if !push.SeenPush {
			return false, false, nil
		}
		card, err := tx.GetCard(ctx, skey(pk))
		if err != nil {
			return false, false, err
		}
		return false, false, e.enqueueCard(ctx, tx, pk, eff.ThreadFor(config.EventPush), card, push.Final)
	}

	if push.Superseded && !push.Absorbs {
		return false, false, nil
	}
	if !mine || !push.Absorbs {
		ownCard, err := tx.GetCard(ctx, skey(key))
		if err != nil {
			return false, false, err
		}
		push.Absorbs = push.SeenPush && ownCard == nil
	}
	if !push.SetPipeline(st) {
		return push.Absorbs, false, nil
	}
	if err := put(ctx, tx, pk, push, push.Final, latest(row, received)); err != nil {
		return false, false, err
	}
	if err := tx.AddLink(ctx, store.Link{From: skey(pk), Rel: relPushPipeline, To: skey(key)}); err != nil {
		return false, false, err
	}
	if !push.SeenPush {
		return false, false, nil
	}
	card, err := tx.GetCard(ctx, skey(pk))
	if err != nil {
		return false, false, err
	}
	return push.Absorbs, false, e.enqueueCard(ctx, tx, pk, eff.ThreadFor(config.EventPush), card, push.Final)
}

// publishPipeline pushes a changed pipeline state to whichever card shows
// it: the push card that absorbs it, or its own card, which waits while
// the pipeline's source is unknown and a push card might claim it.
func (e *Engine) publishPipeline(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, key cards.Key, st *cards.PipelineState, received time.Time) error {
	absorbed, hold, err := e.syncPipelinePush(ctx, tx, key, st, received)
	if err != nil || absorbed || hold {
		return err
	}
	return e.enqueuePipelineCard(ctx, tx, eff, st)
}

// DecoratePush writes diff stats into a push's stored state and queues its
// card, as the compare prefetch would. The preview subcommand uses it.
func (e *Engine) DecoratePush(ctx context.Context, key cards.Key, diff cards.DiffStats) error {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.PushState](ctx, tx, key, e.log)
		if err != nil || row == nil {
			return err
		}
		if !st.SetDiff(diff) {
			return nil
		}
		if err := put(ctx, tx, key, st, st.Final, row.LastEventAt); err != nil {
			return err
		}
		card, err := tx.GetCard(ctx, skey(key))
		if err != nil {
			return err
		}
		return e.enqueueCard(ctx, tx, key, e.cfg.Resolve(st.Project.Path).ThreadFor(config.EventPush), card, st.Final)
	})
	if err != nil {
		return err
	}
	e.notify()
	return nil
}
