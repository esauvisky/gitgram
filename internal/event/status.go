package event

// Pipeline and job statuses as reported by GitLab (identical set for both).
const (
	StatusCreated            = "created"
	StatusWaitingForResource = "waiting_for_resource"
	StatusPreparing          = "preparing"
	StatusWaitingForCallback = "waiting_for_callback"
	StatusPending            = "pending"
	StatusRunning            = "running"
	StatusCanceling          = "canceling"
	StatusManual             = "manual"
	StatusScheduled          = "scheduled"
	StatusSkipped            = "skipped"
	StatusCanceled           = "canceled"
	StatusSuccess            = "success"
	StatusFailed             = "failed"
)

var statusRank = map[string]int{
	StatusCreated:            0,
	StatusWaitingForResource: 1,
	StatusPreparing:          2,
	StatusWaitingForCallback: 3,
	StatusPending:            4,
	StatusRunning:            5,
	StatusCanceling:          6,
	StatusManual:             7,
	StatusScheduled:          8,
	StatusSkipped:            9,
	StatusCanceled:           10,
	StatusSuccess:            11,
	StatusFailed:             12,
}

// StatusRank orders statuses along the lifecycle: queued states first, then
// running/canceling, then blocked (manual, scheduled), then terminal states
// with failed ranking highest. Unknown statuses return -1. It is meant for
// sorting and for picking the most advanced status among several.
func StatusRank(status string) int {
	if r, ok := statusRank[status]; ok {
		return r
	}
	return -1
}

// IsTerminal reports whether the status is one GitLab never leaves without a
// retry: success, failed, canceled, skipped.
func IsTerminal(status string) bool {
	switch status {
	case StatusSuccess, StatusFailed, StatusCanceled, StatusSkipped:
		return true
	}
	return false
}

// IsBlocked reports whether the status is waiting for a human or a timer:
// manual, scheduled. Blocked pipelines are never final.
func IsBlocked(status string) bool {
	return status == StatusManual || status == StatusScheduled
}

// IsActive reports whether the status is a known state in which the job or
// pipeline will still progress on its own (queued, running or canceling).
func IsActive(status string) bool {
	r, ok := statusRank[status]
	return ok && r <= statusRank[StatusCanceling]
}
