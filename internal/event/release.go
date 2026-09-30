package event

import "time"

// Release hook actions.
const (
	ReleaseActionCreate = "create"
	ReleaseActionUpdate = "update"
	ReleaseActionDelete = "delete"
)

// Release is a Release Hook. The payload carries no actor; Actor returns the
// zero User.
type Release struct {
	Meta
	Project Project

	// Action is one of the ReleaseAction* constants.
	Action string
	ID     int64
	Name   string
	Tag    string
	// Description is the raw markdown release notes.
	Description string
	URL         string
	Commit      Commit
	// Links are the release's asset links (assets.links[]).
	Links []Link

	CreatedAt  time.Time
	ReleasedAt time.Time
}

// EventKind implements Event.
func (*Release) EventKind() Kind { return KindRelease }

// Proj implements Event.
func (r *Release) Proj() Project { return r.Project }

// Actor implements Event.
func (*Release) Actor() User { return User{} }
