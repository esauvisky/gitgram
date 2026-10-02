package engine

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/store"
)

// actionTimeout bounds one write call behind a button press.
const actionTimeout = 15 * time.Second

var (
	_ actions.Capabilities = (*Engine)(nil)
	_ actions.Dispatcher   = (*Engine)(nil)
)

// Can implements actions.Capabilities: with a writer, pipelines can be
// stopped (after a confirmation) and retried, and manual jobs run. Anyone in the group may
// press.
func (e *Engine) Can(k actions.Kind, a actions.Action) bool {
	if e.writer == nil {
		return false
	}
	if k == actions.KindJob {
		return a == actions.ActionPlay
	}
	if k != actions.KindPipeline {
		return false
	}
	switch a {
	case actions.ActionCancel, actions.ActionCancelYes, actions.ActionCancelNo, actions.ActionRetry:
		return true
	}
	return false
}

// Dispatch implements actions.Dispatcher. Stop and its "keep running"
// answer only flip the card's confirmation; "yes, stop it" and Retry make
// the GitLab call and answer with a toast, and the resulting webhooks
// update the card.
func (e *Engine) Dispatch(ctx context.Context, req actions.Request) (actions.Result, error) {
	if e.writer == nil {
		return actions.Result{Toast: "Actions need GITGRAM_GITLAB_HOOKS_TOKEN.", Alert: true}, nil
	}
	if !e.Can(req.Kind, req.Action) {
		return actions.Result{Toast: "This button is no longer valid.", Alert: true}, nil
	}
	key := cards.Key{Kind: cards.KindPipeline, ProjectID: req.ProjectID, ObjectID: req.ObjectID}
	id := strconv.FormatInt(req.ObjectID, 10)
	switch req.Action {
	case actions.ActionCancel:
		if err := e.setConfirmStop(ctx, key, true); err != nil {
			return actions.Result{}, err
		}
		return actions.Result{Toast: "Stop this pipeline? Confirm on the card."}, nil
	case actions.ActionCancelNo:
		if err := e.setConfirmStop(ctx, key, false); err != nil {
			return actions.Result{}, err
		}
		return actions.Result{Toast: "Pipeline keeps running."}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	var err error
	toast := "Stopping pipeline " + id
	switch req.Action {
	case actions.ActionCancelYes:
		if err := e.setConfirmStop(ctx, key, false); err != nil {
			return actions.Result{}, err
		}
		_, err = e.writer.CancelPipeline(ctx, req.ProjectID, req.ObjectID)
	case actions.ActionRetry:
		if err := e.setConfirmStop(ctx, key, false); err != nil {
			return actions.Result{}, err
		}
		_, err = e.writer.RetryPipeline(ctx, req.ProjectID, req.ObjectID)
		toast = "Retrying pipeline " + id
	case actions.ActionPlay:
		_, err = e.writer.PlayJob(ctx, req.ProjectID, req.ObjectID)
		toast = "Starting job " + id
	}
	if err != nil {
		e.log.Warn("action failed", "action", req.Action, "kind", string(req.Kind), "project", req.ProjectID, "object", req.ObjectID, "user", req.TelegramUserID, "err", err)
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.Status == 400 || errors.Is(err, api.ErrForbidden) {
			return actions.Result{Toast: "GitLab refused: " + shortError(err), Alert: true}, nil
		}
		return actions.Result{Toast: "GitLab call failed: " + shortError(err), Alert: true}, nil
	}
	e.log.Info("action done", "action", req.Action, "kind", string(req.Kind), "project", req.ProjectID, "object", req.ObjectID, "user", req.TelegramUserID)
	return actions.Result{Toast: toast}, nil
}

// setConfirmStop records whether the card is asking to confirm a stop and
// re-renders it.
func (e *Engine) setConfirmStop(ctx context.Context, key cards.Key, on bool) error {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.PipelineState](ctx, tx, key, e.log)
		if err != nil || row == nil || st.ConfirmStop == on {
			return err
		}
		st.ConfirmStop = on
		if err := put(ctx, tx, key, st, st.Final, row.LastEventAt); err != nil {
			return err
		}
		return e.publishPipeline(ctx, tx, key, st, row.LastEventAt)
	})
	if err != nil {
		return err
	}
	e.notify()
	return nil
}

// shortError keeps a toast within Telegram's 200-character answer limit.
func shortError(err error) string {
	s := err.Error()
	if len(s) > 150 {
		s = s[:150] + "…"
	}
	return s
}
