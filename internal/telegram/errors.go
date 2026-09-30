package telegram

import (
	"errors"
	"strings"
	"time"

	"github.com/go-telegram/bot"
)

// outcome classifies a failed Telegram call.
type outcome int

const (
	// outcomeTransient: network error, 5xx, 401/403/404/409 → exponential backoff.
	outcomeTransient outcome = iota
	// outcomeRateLimited: 429 → wait retry_after.
	outcomeRateLimited
	// outcomeNotModified: 400 "message is not modified" → success.
	outcomeNotModified
	// outcomeMessageGone: 400 "message to edit not found" → card deleted.
	outcomeMessageGone
	// outcomeUneditable: 400 "message can't be edited" → card uneditable.
	outcomeUneditable
	// outcomeThreadNotFound: 400 "message thread not found", TOPIC_CLOSED or
	// TOPIC_DELETED → retry once without thread.
	outcomeThreadNotFound
	// outcomePermanent: any other 400 (bad HTML, bad chat id) or a chat
	// migration (chat_id changed): drop the row.
	outcomePermanent
)

// Backoff bounds for outcomeTransient: 1s doubling per attempt, capped at 60s.
const (
	backoffMin = time.Second
	backoffMax = time.Minute
)

func classify(err error) (outcome, time.Duration) {
	var tooMany *bot.TooManyRequestsError
	if errors.As(err, &tooMany) {
		d := time.Duration(tooMany.RetryAfter) * time.Second
		if d < backoffMin {
			d = backoffMin
		}
		return outcomeRateLimited, d
	}
	var mig *bot.MigrateError
	if errors.As(err, &mig) {
		return outcomePermanent, 0
	}
	if !errors.Is(err, bot.ErrorBadRequest) {
		return outcomeTransient, 0
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "message is not modified"):
		return outcomeNotModified, 0
	case strings.Contains(msg, "message to edit not found"):
		return outcomeMessageGone, 0
	case strings.Contains(msg, "message can't be edited"):
		return outcomeUneditable, 0
	case strings.Contains(msg, "message thread not found"),
		strings.Contains(msg, "topic_closed"),
		strings.Contains(msg, "topic_deleted"):
		return outcomeThreadNotFound, 0
	}
	return outcomePermanent, 0
}

// backoff returns the transient-failure delay for the given attempt count
// (attempts already made before this one).
func backoff(attempts int) time.Duration {
	if attempts >= 6 {
		return backoffMax
	}
	d := backoffMin << uint(attempts)
	if d > backoffMax {
		return backoffMax
	}
	return d
}
