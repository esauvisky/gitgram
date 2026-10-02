package config

// EventClass names a family of GitLab events that can be enabled, filtered
// and routed to a Telegram thread independently.
type EventClass string

// Event classes accepted in `events:` lists and `threads:` keys.
const (
	EventPush     EventClass = "push"
	EventPipeline EventClass = "pipeline"
)

// ThreadDefault is the `threads:` key used when no per-class thread is set.
const ThreadDefault = "default"

// AllEventClasses lists every event class in a stable order. It is the
// built-in value of `defaults.events`.
var AllEventClasses = []EventClass{EventPush, EventPipeline}

// Valid reports whether c is one of the known event classes.
func (c EventClass) Valid() bool {
	for _, k := range AllEventClasses {
		if k == c {
			return true
		}
	}
	return false
}
