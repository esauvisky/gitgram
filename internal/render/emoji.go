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

	EmojiPush          = "📤"
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
	EmojiApprove  = "👍"
	EmojiConflict = "⚔️"
	EmojiEdit     = "✏️"
	EmojiLink     = "🔗"
	EmojiLock     = "🔒"
	EmojiChild    = "↳"
	EmojiParent   = "↰"
)

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
