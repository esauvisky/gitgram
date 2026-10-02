package event

import (
	"time"
)

// ZeroSHA is the all-zero object id GitLab uses in push before/after to
// signal ref creation or deletion.
const ZeroSHA = "0000000000000000000000000000000000000000"

// Project identifies the GitLab project an event belongs to.
type Project struct {
	ID int64
	// Path is path_with_namespace, e.g. "my-org/backend". Project filtering
	// and config overrides match on it.
	Path string
	// Name is the human project name.
	Name string
	// WebURL is the project's browser URL without trailing slash.
	WebURL string
	// DefaultBranch is the project's default branch name.
	DefaultBranch string
}

// User is a GitLab user as it appears in webhook payloads.
type User struct {
	ID        int64
	Username  string
	Name      string
	AvatarURL string
}

// IsZero reports whether the user carries no identity at all.
func (u User) IsZero() bool { return u.ID == 0 && u.Username == "" && u.Name == "" }

// Commit is a commit as it appears in push, pipeline and MR payloads.
type Commit struct {
	SHA string
	// Title is the first line of Message.
	Title   string
	Message string
	URL     string
	// Author is the commit author. Only Name (and sometimes an email in
	// Username) is available from webhook payloads; ID is normally 0.
	Author    User
	Timestamp time.Time
}

// PipelineRef points to the parent pipeline of a child pipeline
// (source_pipeline in pipeline and job payloads).
type PipelineRef struct {
	ProjectID     int64
	ProjectPath   string
	ProjectWebURL string
	PipelineID    int64
	// JobID is the trigger (bridge) job in the parent pipeline.
	JobID int64
}
