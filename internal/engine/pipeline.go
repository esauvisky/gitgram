package engine

import (
	"context"
	"slices"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyPipeline folds a Pipeline or Job Hook into the pipeline state, then
// maintains links and dependent cards: the merge request(s) whose head
// pipeline this is, and the parent pipeline when this is a child. The
// pipeline's own card follows the verbosity and child_cards policy.
func (e *Engine) applyPipeline(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev event.Event) error {
	var key cards.Key
	switch v := ev.(type) {
	case *event.Pipeline:
		key = cards.Key{Kind: cards.KindPipeline, ProjectID: v.Project.ID, ObjectID: v.ID}
	case *event.Job:
		key = cards.Key{Kind: cards.KindPipeline, ProjectID: v.Project.ID, ObjectID: v.PipelineID}
	}
	st, row, err := load[cards.PipelineState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	var changed bool
	switch v := ev.(type) {
	case *event.Pipeline:
		changed = cards.ReducePipeline(st, v)
	case *event.Job:
		changed = cards.ReduceJob(st, v)
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.ReceivedAt())); err != nil {
		return err
	}
	if err := e.linkPipelineMRs(ctx, tx, key, st); err != nil {
		return err
	}

	ownCard := true
	if parentKey, ok := st.ChildKey(); ok {
		if err := tx.AddLink(ctx, store.Link{From: skey(parentKey), Rel: relChild, To: skey(key)}); err != nil {
			return err
		}
		if eff.Pipelines.ChildCards != "own" {
			inlined, err := e.updateParent(ctx, tx, parentKey, st)
			if err != nil {
				return err
			}
			// inline: the parent card shows the child; fall back to an own
			// card only when the parent is unknown so nothing is lost. A child
			// that already got its own card keeps it up to date.
			if inlined && eff.Pipelines.ChildCards == "inline" {
				card, err := tx.GetCard(ctx, skey(key))
				if err != nil {
					return err
				}
				ownCard = card != nil
			}
		}
	}
	if !ownCard || !changed {
		return nil
	}
	return e.enqueuePipelineCard(ctx, tx, eff, st)
}

// enqueuePipelineCard applies the card policy: nothing while the pipeline
// has only created jobs; quiet projects get a card on final only, and none
// for a success when quiet_success is set; once a card exists it is always
// kept up to date.
func (e *Engine) enqueuePipelineCard(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, st *cards.PipelineState) error {
	status := st.EffectiveStatus()
	if !st.Started() || status == event.StatusCreated {
		return nil
	}
	key := st.Key()
	card, err := tx.GetCard(ctx, skey(key))
	if err != nil {
		return err
	}
	if card == nil && eff.Verbosity == "quiet" {
		if !st.Final || (eff.Pipelines.QuietSuccess && status == event.StatusSuccess) {
			return nil
		}
	}
	return e.enqueueCard(ctx, tx, key, eff.ThreadFor(config.EventPipeline), card, st.Final)
}

// linkPipelineMRs records the mr → head_pipeline link when the payload names
// a merge request, then refreshes the head pipeline summary on every merge
// request linked to this pipeline (from the payload or from earlier MR
// events) and enqueues their cards. A merge request already showing or
// expecting a newer pipeline is left alone. Child pipelines are never an
// MR's head pipeline; their status reaches the MR card through the parent.
func (e *Engine) linkPipelineMRs(ctx context.Context, tx *store.Tx, key cards.Key, st *cards.PipelineState) error {
	if st.Parent != nil {
		return nil
	}
	var mrKeys []cards.Key
	if st.MR != nil {
		mk := cards.Key{Kind: cards.KindMR, ProjectID: st.Project.ID, ObjectID: st.MR.IID}
		if err := tx.AddLink(ctx, store.Link{From: skey(mk), Rel: relHeadPipeline, To: skey(key)}); err != nil {
			return err
		}
		mrKeys = append(mrKeys, mk)
	}
	links, err := tx.LinksTo(ctx, skey(key), relHeadPipeline)
	if err != nil {
		return err
	}
	for _, l := range links {
		if mk := ckey(l.From); !slices.Contains(mrKeys, mk) {
			mrKeys = append(mrKeys, mk)
		}
	}
	summary := st.PipelineSummary()
	for _, mk := range mrKeys {
		mr, row, err := load[cards.MRState](ctx, tx, mk, e.log)
		if err != nil {
			return err
		}
		if row == nil || mr.HeadPipelineID > st.ID || (mr.HeadPipeline != nil && mr.HeadPipeline.ID > st.ID) {
			continue
		}
		if !mr.SetHeadPipeline(summary) {
			continue
		}
		if err := put(ctx, tx, mk, mr, mr.Final, row.LastEventAt); err != nil {
			return err
		}
		card, err := tx.GetCard(ctx, skey(mk))
		if err != nil {
			return err
		}
		meff := e.cfg.Resolve(mr.Project.Path)
		if err := e.enqueueCard(ctx, tx, mk, meff.ThreadFor(config.EventMR), card, mr.Final); err != nil {
			return err
		}
	}
	return nil
}

// updateParent upserts the child's summary into its parent pipeline and
// enqueues the parent card. It reports whether the parent is known.
func (e *Engine) updateParent(ctx context.Context, tx *store.Tx, parentKey cards.Key, child *cards.PipelineState) (bool, error) {
	parent, row, err := load[cards.PipelineState](ctx, tx, parentKey, e.log)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}
	if !parent.UpsertChild(child.ChildSummary()) {
		return true, nil
	}
	if err := put(ctx, tx, parentKey, parent, parent.Final, row.LastEventAt); err != nil {
		return false, err
	}
	return true, e.enqueuePipelineCard(ctx, tx, e.cfg.Resolve(parent.Project.Path), parent)
}
