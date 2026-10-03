package engine

import (
	"context"
	"slices"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyMR folds a Merge Request Hook into the MR state, applies the
// prefetched thread count and diff stats, links the head pipeline named by
// the payload (adopting its state when the pipeline is already known) and
// enqueues the card.
func (e *Engine) applyMR(ctx context.Context, tx *store.Tx, ev *event.MergeRequest, en enrichment) error {
	key := cards.Key{Kind: cards.KindMR, ProjectID: ev.Project.ID, ObjectID: ev.IID}
	st, row, err := load[cards.MRState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReduceMR(st, ev)
	if en.threads && st.SetThreads(en.unresolved) {
		changed = true
	}
	if en.diff != nil && st.SetDiff(*en.diff) {
		changed = true
	}
	if st.HeadPipelineID != 0 {
		pk := cards.Key{Kind: cards.KindPipeline, ProjectID: ev.Project.ID, ObjectID: st.HeadPipelineID}
		if err := tx.AddLink(ctx, store.Link{From: skey(key), Rel: relHeadPipeline, To: skey(pk)}); err != nil {
			return err
		}
		if st.Pipeline == nil || st.Pipeline.ID != st.HeadPipelineID {
			ps, prow, err := load[cards.PipelineState](ctx, tx, pk, e.log)
			if err != nil {
				return err
			}
			if prow != nil && st.SetPipeline(ps) {
				changed = true
			}
		}
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.Received)); err != nil {
		return err
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

// syncPipelineMRs copies a pipeline's state into the MR cards that show it:
// every MR whose head pipeline it is, and the MR a merge request pipeline
// was run for (whose head moves to it when it is newer). It reports whether
// an MR card absorbs the pipeline: a merge request pipeline shown on its
// MR's card gets no card of its own.
func (e *Engine) syncPipelineMRs(ctx context.Context, tx *store.Tx, key cards.Key, st *cards.PipelineState) (bool, error) {
	if st.Parent != nil {
		return false, nil
	}
	var mrKeys []cards.Key
	if st.MR != nil {
		mk := cards.Key{Kind: cards.KindMR, ProjectID: st.Project.ID, ObjectID: st.MR.IID}
		if err := tx.AddLink(ctx, store.Link{From: skey(mk), Rel: relHeadPipeline, To: skey(key)}); err != nil {
			return false, err
		}
		mrKeys = append(mrKeys, mk)
	}
	links, err := tx.LinksTo(ctx, skey(key), relHeadPipeline)
	if err != nil {
		return false, err
	}
	for _, l := range links {
		mk := cards.Key{Kind: cards.Kind(l.From.Kind), ProjectID: l.From.ProjectID, ObjectID: l.From.ObjectID}
		if !slices.Contains(mrKeys, mk) {
			mrKeys = append(mrKeys, mk)
		}
	}
	absorbed := false
	for _, mk := range mrKeys {
		mr, row, err := load[cards.MRState](ctx, tx, mk, e.log)
		if err != nil {
			return false, err
		}
		if row == nil {
			continue
		}
		own := st.MR != nil && st.MR.IID == mk.ObjectID
		if own && st.ID > mr.HeadPipelineID {
			mr.HeadPipelineID = st.ID
		}
		if mr.SetPipeline(st) {
			if err := put(ctx, tx, mk, mr, mr.Final, row.LastEventAt); err != nil {
				return false, err
			}
			card, err := tx.GetCard(ctx, skey(mk))
			if err != nil {
				return false, err
			}
			if err := e.enqueueCard(ctx, tx, mk, card, mr.Final); err != nil {
				return false, err
			}
		}
		if own && mr.Pipeline != nil && mr.Pipeline.ID == st.ID {
			absorbed = true
		}
	}
	return absorbed, nil
}

// isMRPipelineRef reports whether ref is a merge request pipeline's ref
// (refs/merge-requests/<iid>/head or /merge).
func isMRPipelineRef(ref string) bool { return strings.HasPrefix(ref, "refs/merge-requests/") }
