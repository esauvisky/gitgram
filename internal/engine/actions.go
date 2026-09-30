package engine

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// actionTimeout bounds one write call behind a button press.
const actionTimeout = 15 * time.Second

var (
	_ actions.Capabilities = (*Engine)(nil)
	_ actions.Dispatcher   = (*Engine)(nil)
)

// Can implements actions.Capabilities: with a writer, pipelines can be
// canceled. Anyone in the group may press.
func (e *Engine) Can(k actions.Kind, a actions.Action) bool {
	return e.writer != nil && k == actions.KindPipeline && a == actions.ActionCancel
}

// Dispatch implements actions.Dispatcher: it performs the GitLab call
// behind a pressed button and answers with a toast. The resulting webhook
// updates the card; nothing is edited here.
func (e *Engine) Dispatch(ctx context.Context, req actions.Request) (actions.Result, error) {
	if e.writer == nil {
		return actions.Result{Toast: "Actions need gitlab.hooks_token.", Alert: true}, nil
	}
	if !e.Can(req.Kind, req.Action) {
		return actions.Result{Toast: "This button is no longer valid.", Alert: true}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	_, err := e.writer.CancelPipeline(ctx, req.ProjectID, req.ObjectID)
	if err != nil {
		e.log.Warn("action failed", "action", req.Action, "kind", string(req.Kind), "project", req.ProjectID, "object", req.ObjectID, "user", req.TelegramUserID, "err", err)
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.Status == 400 || errors.Is(err, api.ErrForbidden) {
			return actions.Result{Toast: "GitLab refused: " + shortError(err), Alert: true}, nil
		}
		return actions.Result{Toast: "GitLab call failed: " + shortError(err), Alert: true}, nil
	}
	e.log.Info("action done", "action", req.Action, "kind", string(req.Kind), "project", req.ProjectID, "object", req.ObjectID, "user", req.TelegramUserID)
	return actions.Result{Toast: "Canceling pipeline " + strconv.FormatInt(req.ObjectID, 10)}, nil
}

// shortError keeps a toast within Telegram's 200-character answer limit.
func shortError(err error) string {
	s := err.Error()
	if len(s) > 150 {
		s = s[:150] + "…"
	}
	return s
}
