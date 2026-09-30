package actions

// Capabilities tells render which action buttons to draw on a card. v1 uses
// None; v2 backs it with per-bot configuration and token availability.
type Capabilities interface {
	// Can reports whether buttons for action on kind should be offered.
	Can(Kind, Action) bool
}

// None offers no actions: cards carry URL buttons only.
type None struct{}

// Can implements Capabilities and always returns false.
func (None) Can(Kind, Action) bool { return false }
