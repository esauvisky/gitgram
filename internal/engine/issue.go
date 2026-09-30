package engine

import (
	"context"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyIssue folds an Issue Hook into the issue state and enqueues the card.
// Open, reopen and close always produce or edit a card; an update only edits
// an existing card, so label churn on issues the bot never announced stays
// silent.
func (e *Engine) applyIssue(ctx context.Context, tx *store.Tx, eff config.EffectiveProject, ev *event.Issue) error {
	key := cards.Key{Kind: cards.KindIssue, ProjectID: ev.Project.ID, ObjectID: ev.IID}
	st, row, err := load[cards.IssueState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReduceIssue(st, ev)
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
	if card == nil && ev.Action == event.IssueActionUpdate {
		return nil
	}
	return e.enqueueCard(ctx, tx, key, eff.ThreadFor(config.EventIssue), card, st.Final)
}
