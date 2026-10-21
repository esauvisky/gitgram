package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyPush folds a branch push into the push card keyed by (project,
// branch, after), applies the diff enrichment, marks the branch's previous
// push card superseded and enqueues both cards.
func (e *Engine) applyPush(ctx context.Context, tx *store.Tx, ev *event.Push, en enrichment) error {
	key := cards.PushKey(ev.Project.ID, ev.Branch(), ev.After)
	st, row, err := load[cards.PushState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReducePush(st, ev)
	if en.diff != nil && st.SetDiff(*en.diff) {
		changed = true
	}
	// A pipeline that arrived first and is still waiting without a card of
	// its own joins this card now.
	if st.Pipeline != nil && !st.Absorbs && !st.Superseded {
		own, err := tx.GetCard(ctx, skey(cards.Key{Kind: cards.KindPipeline, ProjectID: ev.Project.ID, ObjectID: st.Pipeline.ID}))
		if err != nil {
			return err
		}
		if own == nil {
			st.Absorbs, changed = true, true
		}
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.Received)); err != nil {
		return err
	}
	if !ev.IsCreate() && ev.Before != ev.After {
		if err := e.supersedePush(ctx, tx, cards.PushKey(ev.Project.ID, ev.Branch(), ev.Before), ev.After); err != nil {
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
	return e.enqueueCard(ctx, tx, key, card, st.Final)
}

// applyBranchDeleted posts the deletion as a standalone message and freezes
// the branch's last push card.
func (e *Engine) applyBranchDeleted(ctx context.Context, tx *store.Tx, ev *event.Push) error {
	if err := e.supersedePush(ctx, tx, cards.PushKey(ev.Project.ID, ev.Branch(), ev.Before), ""); err != nil {
		return err
	}
	if marked, err := e.markSourceBranchDeleted(ctx, tx, ev); err != nil || marked {
		return err
	}
	payload, err := json.Marshal(render.BranchDeleted(ev, e.options(ev.Project)))
	if err != nil {
		return fmt.Errorf("encode branch deletion: %w", err)
	}
	return tx.EnqueueSend(ctx, nil, payload)
}

// markSourceBranchDeleted notes a branch deletion on the card of the merge
// request whose source branch it was (the newest one that is open, or merged
// within the hour), and reports whether there was one. GitLab's merge event can land after
// the deletion, so the MR need not show as merged yet.
func (e *Engine) markSourceBranchDeleted(ctx context.Context, tx *store.Tx, ev *event.Push) (bool, error) {
	rows, err := tx.ListProject(ctx, string(cards.KindMR), ev.Project.ID)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		var st cards.MRState
		if err := unmarshalState(row, &st); err != nil || st.SourceBranch != ev.Branch() || st.State == event.MRStateClosed {
			continue
		}
		// A branch name can come back long after an old MR merged it.
		if st.State == event.MRStateMerged && ev.Received.Sub(row.LastEventAt) > time.Hour {
			continue
		}
		if st.SourceBranchDeleted {
			return true, nil
		}
		st.SourceBranchDeleted = true
		key := st.Key()
		if err := put(ctx, tx, key, &st, st.Final, row.LastEventAt); err != nil {
			return false, err
		}
		card, err := tx.GetCard(ctx, skey(key))
		if err != nil {
			return false, err
		}
		return true, e.enqueueCard(ctx, tx, key, card, st.Final)
	}
	return false, nil
}

// supersedePush freezes the previous push card when it exists and is not
// already superseded, and flushes its final edit. Skeletons have no card
// to edit.
func (e *Engine) supersedePush(ctx context.Context, tx *store.Tx, prevKey cards.Key, by string) error {
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
	return e.enqueueCard(ctx, tx, prevKey, card, true)
}

// pushWait is how long a pipeline triggered by a push waits, without a card
// of its own, for the Push Hook to arrive and absorb it.
const pushWait = 2 * time.Minute

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
// that no card should be posted yet. Child and non-branch refs are
// skipped, as are projects without push cards.
//
// A pipeline with no live push card under its own ref (a tag, or a branch
// whose Push Hook has not arrived) joins the live push card of its commit
// on any other ref instead, when that card shows no other pipeline: a tag
// pushed together with the branch it points at ends up on the branch's
// card. GitHub never says whether a run's ref is a tag, so this is also
// how its tag runs find the push.
func (e *Engine) syncPipelinePush(ctx context.Context, tx *store.Tx, key cards.Key, st *cards.PipelineState, received time.Time) (absorbed, hold bool, err error) {
	if st.Parent != nil || st.SHA == "" || st.Ref == "" || strings.HasPrefix(st.Ref, "refs/") {
		return false, false, nil
	}
	pk := cards.PushKey(st.Project.ID, st.Ref, st.SHA)
	push, row, err := load[cards.PushState](ctx, tx, pk, e.log)
	if err != nil {
		return false, false, err
	}
	if row == nil || !push.SeenPush {
		other, otherRow, err := e.pushOfCommit(ctx, tx, st)
		if err != nil {
			return false, false, err
		}
		if other != nil {
			// A skeleton under the pipeline's own ref must not absorb it
			// too, should its Push Hook arrive after all.
			if row != nil && push.Pipeline != nil && push.Pipeline.ID == st.ID {
				push.ReleasePipeline()
				if err := put(ctx, tx, pk, push, push.Final, row.LastEventAt); err != nil {
					return false, false, err
				}
			}
			pk, push, row = other.Key(), other, otherRow
		}
	}
	confirmed := st.SeenPipelineEvent && st.Source == "push"
	if row == nil {
		if !confirmed {
			// Job Hooks often beat both the Pipeline Hook and a Push Hook
			// still in its diff fetch: a young pipeline of unknown source
			// with no card yet waits for them, up to pushWait.
			if st.SeenPipelineEvent || st.CreatedAt == nil || received.Sub(*st.CreatedAt) >= pushWait {
				return false, false, nil
			}
			ownCard, err := tx.GetCard(ctx, skey(key))
			return false, ownCard == nil, err
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
		return false, false, e.enqueueCard(ctx, tx, pk, card, push.Final)
	}

	if push.Superseded && !push.Absorbs {
		return false, false, nil
	}
	ownCard, err := tx.GetCard(ctx, skey(key))
	if err != nil {
		return false, false, err
	}
	if !mine || !push.Absorbs {
		push.Absorbs = push.SeenPush && ownCard == nil
	}
	// The Push Hook usually lands seconds after the pipeline's (its diff
	// fetch is slow): a pipeline with no card yet waits for it, up to
	// pushWait after the pipeline was created, so the push card can absorb
	// it.
	waiting := !push.SeenPush && ownCard == nil && st.CreatedAt != nil && received.Sub(*st.CreatedAt) < pushWait
	if !push.SetPipeline(st) {
		return push.Absorbs, waiting, nil
	}
	if err := put(ctx, tx, pk, push, push.Final, latest(row, received)); err != nil {
		return false, false, err
	}
	if err := tx.AddLink(ctx, store.Link{From: skey(pk), Rel: relPushPipeline, To: skey(key)}); err != nil {
		return false, false, err
	}
	if !push.SeenPush {
		return false, waiting, nil
	}
	card, err := tx.GetCard(ctx, skey(pk))
	if err != nil {
		return false, false, err
	}
	return push.Absorbs, false, e.enqueueCard(ctx, tx, pk, card, push.Final)
}

// pushOfCommit finds the live push card of st's commit on any ref that can
// take st: posted, not superseded and showing no pipeline but st. Pushes
// are scanned newest first, so the latest such push wins.
func (e *Engine) pushOfCommit(ctx context.Context, tx *store.Tx, st *cards.PipelineState) (*cards.PushState, *store.ObjectRow, error) {
	rows, err := tx.ListProject(ctx, string(cards.KindPush), st.Project.ID)
	if err != nil {
		return nil, nil, err
	}
	for i := range rows {
		var push cards.PushState
		if err := unmarshalState(rows[i], &push); err != nil {
			return nil, nil, fmt.Errorf("decode push %v: %w", rows[i].Key, err)
		}
		if push.After != st.SHA || !push.SeenPush || push.Superseded || (push.Pipeline != nil && push.Pipeline.ID != st.ID) {
			continue
		}
		return &push, &rows[i], nil
	}
	return nil, nil, nil
}

// publishPipeline pushes a changed pipeline state to every card that shows
// it: the push card that absorbs it, the MR cards it belongs to, and its
// own card unless a push or MR card absorbs it. The own card waits while
// the source is unknown and a push or MR card might claim it; one that was
// already posted keeps updating.
func (e *Engine) publishPipeline(ctx context.Context, tx *store.Tx, key cards.Key, st *cards.PipelineState, received time.Time) error {
	absorbed, hold, err := e.syncPipelinePush(ctx, tx, key, st, received)
	if err != nil || hold {
		return err
	}
	// A merge request pipeline waits for the Pipeline Hook that names its
	// MR before posting a card of its own (Job Hooks usually arrive first).
	if !st.SeenPipelineEvent && isMRPipelineRef(st.Ref) {
		return nil
	}
	byMR, err := e.syncPipelineMRs(ctx, tx, key, st)
	if err != nil || absorbed {
		return err
	}
	if byMR {
		own, err := tx.GetCard(ctx, skey(key))
		if err != nil || own == nil {
			return err
		}
	}
	return e.enqueuePipelineCard(ctx, tx, st)
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
		return e.enqueueCard(ctx, tx, key, card, st.Final)
	})
	if err != nil {
		return err
	}
	e.notify()
	return nil
}

// DecorateMR writes an MR's unresolved thread count and diff stats into its
// stored state and queues its card, as the MR prefetch would. The preview
// subcommand uses it.
func (e *Engine) DecorateMR(ctx context.Context, key cards.Key, unresolved int, diff cards.DiffStats) error {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.MRState](ctx, tx, key, e.log)
		if err != nil || row == nil {
			return err
		}
		threads, diffs := st.SetThreads(unresolved), st.SetDiff(diff)
		if !threads && !diffs {
			return nil
		}
		if err := put(ctx, tx, key, st, st.Final, row.LastEventAt); err != nil {
			return err
		}
		card, err := tx.GetCard(ctx, skey(key))
		if err != nil {
			return err
		}
		return e.enqueueCard(ctx, tx, key, card, st.Final)
	})
	if err != nil {
		return err
	}
	e.notify()
	return nil
}
