package actions

import "context"

// Request is a decoded callback query together with who pressed the button
// and where.
type Request struct {
	Callback
	TelegramUserID int64
	ChatID         int64
	MessageID      int
}

// Result tells the callback route how to answer the query.
type Result struct {
	// Toast is the answerCallbackQuery text.
	Toast string
	// Alert shows Toast as a modal alert instead of a transient toast.
	Alert bool
	// Refresh asks the caller to re-render the card the button belongs to.
	Refresh bool
}

// Dispatcher performs the action behind a pressed button. The callback route
// always answers the query: with Result on success, with a generic failure
// toast when err is non-nil.
type Dispatcher interface {
	Dispatch(context.Context, Request) (Result, error)
}

// UnsupportedToast is the answer Unsupported gives to every button.
const UnsupportedToast = "Actions aren't enabled on this bot yet."

// Unsupported is the v1 Dispatcher: every button answers with an alert
// explaining that actions are not enabled.
type Unsupported struct{}

// Dispatch implements Dispatcher.
func (Unsupported) Dispatch(context.Context, Request) (Result, error) {
	return Result{Toast: UnsupportedToast, Alert: true}, nil
}
