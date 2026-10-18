package engine

import (
	"context"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// applyPackage folds a published image tag into its version's card. The
// card is final from the start: a later tag still edits it, nothing waits
// on it.
func (e *Engine) applyPackage(ctx context.Context, tx *store.Tx, ev *event.Package) error {
	key := cards.PackageKey(ev.Project.ID, ev.VersionID)
	st, row, err := load[cards.PackageState](ctx, tx, key, e.log)
	if err != nil {
		return err
	}
	changed := cards.ReducePackage(st, ev)
	if err := put(ctx, tx, key, st, true, latest(row, ev.Received)); err != nil {
		return err
	}
	card, err := tx.GetCard(ctx, skey(key))
	if err != nil {
		return err
	}
	if !changed && card != nil {
		return nil
	}
	return e.enqueueCard(ctx, tx, key, card, true)
}
