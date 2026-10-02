package event

import "strings"

// Push is a Push Hook: commits pushed to a branch, or a branch created or
// deleted.
type Push struct {
	Meta
	Project Project
	User    User

	// Ref is the full ref, e.g. "refs/heads/main".
	Ref string
	// Before and After are the old and new head SHAs; ZeroSHA marks creation
	// or deletion respectively.
	Before string
	After  string
	// CheckoutSHA is empty on deletion.
	CheckoutSHA string
	// Commits holds at most 20 commits, newest last as GitLab sends them.
	Commits []Commit
	// TotalCommitsCount is the real count, which may exceed len(Commits).
	TotalCommitsCount int
	// Forced is not part of the payload: the engine sets it after the
	// force-push compare enrichment. False when enrichment is unavailable.
	Forced bool
}

// EventKind implements Event.
func (*Push) EventKind() Kind { return KindPush }

// Proj implements Event.
func (p *Push) Proj() Project { return p.Project }

// Actor implements Event.
func (p *Push) Actor() User { return p.User }

// Branch returns Ref without the "refs/heads/" prefix.
func (p *Push) Branch() string { return strings.TrimPrefix(p.Ref, "refs/heads/") }

// IsCreate reports whether the push created the branch.
func (p *Push) IsCreate() bool { return p.Before == ZeroSHA }

// IsDelete reports whether the push deleted the branch.
func (p *Push) IsDelete() bool { return p.After == ZeroSHA }
