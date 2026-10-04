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
// stopped (after a confirmation) and retried, manual jobs run, and merge
// requests merged (after a confirmation). Anyone in the group may
// press.
func (e *Engine) Can(k actions.Kind, a actions.Action) bool {
	if e.writer == nil {
		return false
	}
	if k == actions.KindJob {
		return a == actions.ActionPlay
	}
	if k == actions.KindMergeRequest {
		return a == actions.ActionMerge || a == actions.ActionMergeYes || a == actions.ActionMergeNo
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

// canGitHub is the capability set of GitHub repositories: with a client
// (which needs a token), workflow runs can be stopped (after a confirmation) and their failed
// jobs re-run.
func (e *Engine) canGitHub(k actions.Kind, a actions.Action) bool {
	if e.gh == nil || k != actions.KindPipeline {
		return false
	}
	switch a {
	case actions.ActionCancel, actions.ActionCancelYes, actions.ActionCancelNo, actions.ActionRetry:
		return true
	}
	return false
}

// githubCaps offers the GitHub capability set to the renderer.
type githubCaps struct{ e *Engine }

func (c githubCaps) Can(k actions.Kind, a actions.Action) bool { return c.e.canGitHub(k, a) }

// Dispatch implements actions.Dispatcher. Stop and its "keep running"
// answer only flip the card's confirmation; "yes, stop it" and Retry make
// the GitLab or GitHub call (by the project's host) and answer with a
// toast, and the resulting webhooks update the card.
func (e *Engine) Dispatch(ctx context.Context, req actions.Request) (actions.Result, error) {
	gh := req.ProjectID < 0
	switch {
	case gh && !e.canGitHub(req.Kind, req.Action):
		return actions.Result{Toast: "This button is no longer valid.", Alert: true}, nil
	case !gh && e.writer == nil:
		return actions.Result{Toast: "Actions need GITGRAM_GITLAB_HOOKS_TOKEN.", Alert: true}, nil
	case !gh && !e.Can(req.Kind, req.Action):
		return actions.Result{Toast: "This button is no longer valid.", Alert: true}, nil
	}
	if req.Kind == actions.KindMergeRequest {
		return e.dispatchMerge(ctx, req)
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
	if gh {
		return e.dispatchGitHub(ctx, req, key)
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

// dispatchMerge handles the Merge button: the first press and "keep open"
// only flip the card's confirmation; "yes, merge it" calls GitLab, and the
// resulting Merge Request Hook updates the card.
func (e *Engine) dispatchMerge(ctx context.Context, req actions.Request) (actions.Result, error) {
	key := cards.Key{Kind: cards.KindMR, ProjectID: req.ProjectID, ObjectID: req.ObjectID}
	iid := "!" + strconv.FormatInt(req.ObjectID, 10)
	switch req.Action {
	case actions.ActionMerge:
		if err := e.setConfirmMerge(ctx, key, true); err != nil {
			return actions.Result{}, err
		}
		return actions.Result{Toast: "Merge " + iid + "? Confirm on the card."}, nil
	case actions.ActionMergeNo:
		if err := e.setConfirmMerge(ctx, key, false); err != nil {
			return actions.Result{}, err
		}
		return actions.Result{Toast: iid + " stays open."}, nil
	}
	if err := e.setConfirmMerge(ctx, key, false); err != nil {
		return actions.Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	if err := e.writer.MergeMR(ctx, req.ProjectID, req.ObjectID); err != nil {
		e.log.Warn("action failed", "action", req.Action, "project", req.ProjectID, "mr", req.ObjectID, "user", req.TelegramUserID, "err", err)
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 || errors.Is(err, api.ErrForbidden) {
			return actions.Result{Toast: "GitLab refused: " + shortError(err), Alert: true}, nil
		}
		return actions.Result{Toast: "GitLab call failed: " + shortError(err), Alert: true}, nil
	}
	e.log.Info("action done", "action", req.Action, "project", req.ProjectID, "mr", req.ObjectID, "user", req.TelegramUserID)
	return actions.Result{Toast: "Merging " + iid}, nil
}

// setConfirmMerge records whether the MR card is asking to confirm a merge
// and re-renders it.
func (e *Engine) setConfirmMerge(ctx context.Context, key cards.Key, on bool) error {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.MRState](ctx, tx, key, e.log)
		if err != nil || row == nil || st.ConfirmMerge == on {
			return err
		}
		st.ConfirmMerge = on
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

// dispatchGitHub performs a confirmed stop or a retry on a GitHub workflow
// run.
func (e *Engine) dispatchGitHub(ctx context.Context, req actions.Request, key cards.Key) (actions.Result, error) {
	if err := e.setConfirmStop(ctx, key, false); err != nil {
		return actions.Result{}, err
	}
	row, err := e.st.GetObject(ctx, skey(key))
	if err != nil || row == nil {
		return actions.Result{Toast: "This workflow run is no longer known.", Alert: true}, err
	}
	var st cards.PipelineState
	if err := unmarshalState(*row, &st); err != nil {
		return actions.Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	toast := "Stopping workflow run"
	if req.Action == actions.ActionRetry {
		err, toast = e.gh.RerunFailed(ctx, st.Project.Path, req.ObjectID), "Re-running the failed jobs"
	} else {
		err = e.gh.CancelRun(ctx, st.Project.Path, req.ObjectID)
	}
	if err != nil {
		e.log.Warn("action failed", "action", req.Action, "repo", st.Project.Path, "run", req.ObjectID, "user", req.TelegramUserID, "err", err)
		return actions.Result{Toast: "GitHub call failed: " + shortError(err), Alert: true}, nil
	}
	e.log.Info("action done", "action", req.Action, "repo", st.Project.Path, "run", req.ObjectID, "user", req.TelegramUserID)
	return actions.Result{Toast: toast}, nil
}
