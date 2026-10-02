package render

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// statusWord spells out a pipeline or job status; a failed job that is
// allowed to fail says so.
func statusWord(status string, allowFailure bool) string {
	switch status {
	case event.StatusSuccess:
		return "Passed"
	case event.StatusFailed:
		if allowFailure {
			return "Failed (allowed)"
		}
		return "Failed"
	case event.StatusRunning:
		return "Running"
	case event.StatusCanceling:
		return "Canceling"
	case event.StatusCanceled:
		return "Canceled"
	case event.StatusSkipped:
		return "Skipped"
	case event.StatusManual:
		return "Waiting for manual"
	case event.StatusScheduled:
		return "Scheduled"
	case event.StatusCreated, event.StatusPending, event.StatusWaitingForResource, event.StatusPreparing, event.StatusWaitingForCallback:
		return "Pending"
	}
	return humanize(status)
}

// stageMarks are the stage lines' marks, one emoji per stage state; the
// only emoji a card carries. None is drawn as a square tile except ✅.
var stageMarks = map[string]string{
	"passed":    "✅",
	"warned":    "⚠️",
	"failed":    "❌",
	"running":   "🏃",
	"queued":    "⏳",
	"manual":    "✋",
	"scheduled": "🕒",
	"skipped":   "➖",
	"canceled":  "🚫",
}
