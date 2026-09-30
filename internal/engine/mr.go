package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyMR folds a Merge Request Hook into the MR state, applies prefetched
// enrichment, links the head pipeline named by the payload (adopting its
// summary when the pipeline is already known) and enqueues the card.
func (e *Engine) applyMR(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev *event.MergeRequest, en enrichment) error {
	key := cards.Key{Kind: cards.KindMR, ProjectID: ev.Project.ID, ObjectID: ev.IID}
	st, row, err := load[cards.MRState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReduceMR(st, ev)
	if applyEnrichment(st, en) {
		changed = true
	}
	if ev.HeadPipelineID != nil {
		pk := cards.Key{Kind: cards.KindPipeline, ProjectID: ev.Project.ID, ObjectID: *ev.HeadPipelineID}
		if err := tx.AddLink(ctx, store.Link{From: skey(key), Rel: relHeadPipeline, To: skey(pk)}); err != nil {
			return err
		}
		if st.HeadPipeline == nil || st.HeadPipeline.ID != *ev.HeadPipelineID {
			ps, prow, err := load[cards.PipelineState](ctx, tx, pk, e.log)
			if err != nil {
				return err
			}
			if prow != nil && st.SetHeadPipeline(ps.PipelineSummary()) {
				changed = true
			}
		}
	}
	if err := put(ctx, tx, key, st, st.Final, latest(row, ev.Received)); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	card, err := tx.GetCard(ctx, skey(key))
	if err != nil {
		return err
	}
	return e.enqueueCard(ctx, tx, key, eff.ThreadFor(config.EventMR), card, st.Final)
}

// applyEnrichment overwrites the approval and thread state with fetched
// values and reports whether anything changed.
func applyEnrichment(st *cards.MRState, en enrichment) bool {
	changed := false
	if en.approvals != nil {
		a := cards.ApprovalState{
			By:       apiUsers(en.approvals.ApprovedBy),
			Required: en.approvals.ApprovalsRequired,
			Left:     en.approvals.ApprovalsLeft,
			Enriched: true,
		}
		if a.Required != st.Approvals.Required || a.Left != st.Approvals.Left ||
			a.Enriched != st.Approvals.Enriched || !slices.Equal(a.By, st.Approvals.By) {
			st.Approvals = a
			changed = true
		}
	}
	if en.threads {
		t := cards.ThreadState{Unresolved: en.unresolved, Enriched: true}
		if st.Threads != t {
			st.Threads = t
			changed = true
		}
	}
	return changed
}

// applyNote relays a comment on a merge request or issue. With
// mr.collapse_notes the note is folded into a known MR card; otherwise it is
// sent as a reply to the anchor card, or standalone when the bot never
// posted one.
func (e *Engine) applyNote(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev *event.Note) error {
	var anchor cards.Key
	var thread int64
	switch ev.NoteableType {
	case event.NoteableMergeRequest:
		anchor = cards.Key{Kind: cards.KindMR, ProjectID: ev.Project.ID, ObjectID: ev.MR.IID}
		thread = eff.ThreadFor(config.EventMRNote)
	case event.NoteableIssue:
		anchor = cards.Key{Kind: cards.KindIssue, ProjectID: ev.Project.ID, ObjectID: ev.Issue.IID}
		thread = eff.ThreadFor(config.EventIssueNote)
	}
	card, err := tx.GetCard(ctx, skey(anchor))
	if err != nil {
		return err
	}
	anchored := card != nil && card.Status == store.CardLive

	if anchor.Kind == cards.KindMR && eff.MR.CollapseNotes && (card == nil || anchored) {
		st, row, err := load[cards.MRState](ctx, tx, anchor, e.log)
		if err != nil {
			return err
		}
		if row != nil {
			if !st.AppendNote(cards.NewNoteSummary(ev)) {
				return nil
			}
			if err := put(ctx, tx, anchor, st, st.Final, row.LastEventAt); err != nil {
				return err
			}
			return e.enqueueCard(ctx, tx, anchor, eff.ThreadFor(config.EventMR), card, st.Final)
		}
	}

	payload, err := json.Marshal(render.Note(ev, anchored, e.options(eff)))
	if err != nil {
		return fmt.Errorf("encode note: %w", err)
	}
	if anchored {
		// The sender posts the reply in the anchor's topic; the class thread
		// only applies if the anchor never got a message.
		return tx.EnqueueReply(ctx, skey(anchor), threadPtr(thread), payload)
	}
	return tx.EnqueueSend(ctx, threadPtr(thread), payload)
}
