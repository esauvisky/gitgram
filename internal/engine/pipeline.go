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
func (e *Engine) applyPipeline(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev event.Event, en enrichment) error {
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
	if mergeTails(st, en.tails) {
		changed = true
	}
	if en.artifacts != nil && !slices.Equal(st.Artifacts, en.artifacts) {
		st.Artifacts = en.artifacts
		changed = true
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.ReceivedAt())); err != nil {
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
	return e.publishPipeline(ctx, tx, eff, key, st, ev.ReceivedAt())
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
	return true, e.publishPipeline(ctx, tx, e.cfg.Resolve(parent.Project.Path), parentKey, parent, row.LastEventAt)
}
