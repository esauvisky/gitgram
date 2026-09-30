package render

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// Emoji used across every card, in one place so cards stay consistent.
const (
	EmojiSuccess   = "✅"
	EmojiFailed    = "❌"
	EmojiWarning   = "⚠️"
	EmojiRunning   = "🔄"
	EmojiPending   = "⏳"
	EmojiCanceled  = "🚫"
	EmojiSkipped   = "⏭️"
	EmojiManual    = "⏸️"
	EmojiScheduled = "🕒"

	EmojiMROpened = "🟢"
	EmojiMRMerged = "🟣"
	EmojiMRClosed = "🔴"
	EmojiMRDraft  = "📝"

	EmojiPush          = "↗\ufe0e"
	EmojiForce         = "⚠️"
	EmojiTag           = "🏷️"
	EmojiNewBranch     = "🌱"
	EmojiDeletedBranch = "🗑️"

	EmojiIssueOpen   = "🐛"
	EmojiIssueClosed = "✅"
	EmojiNote        = "💬"
	EmojiRelease     = "🚀"
	EmojiDeploy      = "🛫"

	EmojiUser     = "👤"
	EmojiPeople   = "👥"
	EmojiLabel    = "🏷"
	EmojiConflict = "⚔️"
	EmojiLink     = "🔗"
	EmojiLock     = "🔒"
	EmojiChild    = "↳"

	EmojiArtifacts = "📦"
)

// statusWord spells out a pipeline or job status so no lamp ever stands
// alone; a failed job that is allowed to fail says so.
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

// lamp is a status emoji followed by its word.
func lamp(status string, allowFailure bool) string {
	return statusEmoji(status, allowFailure) + " " + statusWord(status, allowFailure)
}

// statusEmoji maps a pipeline or job status to its emoji; a failed job that
// is allowed to fail is a warning.
func statusEmoji(status string, allowFailure bool) string {
	switch status {
	case event.StatusSuccess:
		return EmojiSuccess
	case event.StatusFailed:
		if allowFailure {
			return EmojiWarning
		}
		return EmojiFailed
	case event.StatusRunning, event.StatusCanceling:
		return EmojiRunning
	case event.StatusCanceled:
		return EmojiCanceled
	case event.StatusSkipped:
		return EmojiSkipped
	case event.StatusManual:
		return EmojiManual
	case event.StatusScheduled:
		return EmojiScheduled
	}
	return EmojiPending
}

// mrStateEmoji maps an MR state to its emoji.
func mrStateEmoji(state string) string {
	switch state {
	case event.MRStateMerged:
		return EmojiMRMerged
	case event.MRStateClosed:
		return EmojiMRClosed
	}
	return EmojiMROpened
}

// deploymentEmoji maps a deployment status to its emoji.
func deploymentEmoji(status string) string {
	switch status {
	case event.DeploymentSuccess:
		return EmojiSuccess
	case event.DeploymentFailed:
		return EmojiFailed
	case event.DeploymentCanceled:
		return EmojiCanceled
	case event.DeploymentBlocked:
		return EmojiManual
	}
	return EmojiRunning
}
