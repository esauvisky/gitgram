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

// TagPush is a Tag Push Hook: a tag created or deleted.
type TagPush struct {
	Meta
	Project Project
	User    User

	// Ref is the full ref, e.g. "refs/tags/v1.2.0".
	Ref    string
	Before string
	After  string
	// CheckoutSHA is empty on deletion.
	CheckoutSHA string
	// Message is the annotated tag message, empty for lightweight tags.
	Message string
	// Commits holds the commits GitLab attached to the tag push (may be
	// empty).
	Commits           []Commit
	TotalCommitsCount int
}

// EventKind implements Event.
func (*TagPush) EventKind() Kind { return KindTagPush }

// Proj implements Event.
func (t *TagPush) Proj() Project { return t.Project }

// Actor implements Event.
func (t *TagPush) Actor() User { return t.User }

// Tag returns Ref without the "refs/tags/" prefix.
func (t *TagPush) Tag() string { return strings.TrimPrefix(t.Ref, "refs/tags/") }

// IsCreate reports whether the push created the tag.
func (t *TagPush) IsCreate() bool { return t.Before == ZeroSHA }

// IsDelete reports whether the push deleted the tag.
func (t *TagPush) IsDelete() bool { return t.After == ZeroSHA }
