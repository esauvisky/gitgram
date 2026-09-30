package cards

import (
	"hash/fnv"
	"slices"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// MaxDiffFiles caps DiffStats.Files kept in state; the counts stay exact.
const MaxDiffFiles = 30

// PushState is the state behind one push card: one card per push event to
// a branch, keyed by (project, branch, after SHA), edited as its diff stats
// and the pipeline for that SHA arrive.
type PushState struct {
	SchemaVer int
	Project   event.Project
	// Branch is the short name, without refs/heads/.
	Branch string
	// Before is ZeroSHA when the push created the branch.
	Before string
	After  string

	// SeenPush is false while the state is a skeleton created by a pipeline
	// event that arrived before the Push Hook; skeletons get no card.
	SeenPush        bool
	LastPushEventAt time.Time

	Pusher       event.User
	Commits      []event.Commit
	TotalCommits int
	Created      bool
	Forced       bool

	// Diff is nil until the compare enrichment (or the preview) supplies it.
	Diff *DiffStats

	// Pipeline is the newest pipeline seen for After, a copy of its full
	// state maintained by the engine through SetPipeline; nil until one is
	// seen. Absorbs is true when this card is that pipeline's only card:
	// the pipeline was triggered by this push and had no card of its own
	// when it arrived, so its stages, logs and buttons render here.
	Pipeline *PipelineState
	Absorbs  bool

	// Superseded is set when a newer push to Branch arrived; SupersededBy is
	// its After SHA, empty when the branch was deleted instead.
	Superseded   bool
	SupersededBy string

	// Final is true once the card will not change: superseded, or its
	// pipeline finished.
	Final bool
}

// DiffStats is what changed between Before and After.
type DiffStats struct {
	FilesChanged   int
	Added, Removed int
	// Files holds at most MaxDiffFiles entries in GitLab order.
	Files []DiffFile
	// Partial is true when GitLab timed out computing the compare, so the
	// counts may be low.
	Partial bool
}

// DiffFile is one changed file with its own line counts.
type DiffFile struct {
	// Path is the path on the After side.
	Path           string
	RenamedFrom    string
	New            bool
	Deleted        bool
	Added, Removed int
}

// PushObjectID derives the object id of a push card from what both a Push
// Hook and a pipeline carry: the branch and the head SHA.
func PushObjectID(branch, sha string) int64 {
	h := fnv.New64a()
	h.Write([]byte(branch))
	h.Write([]byte{0})
	h.Write([]byte(sha))
	return int64(h.Sum64())
}

// PushKey returns the object_state key of the push of sha to branch.
func PushKey(projectID int64, branch, sha string) Key {
	return Key{Kind: KindPush, ProjectID: projectID, ObjectID: PushObjectID(branch, sha)}
}

// NewPushSkeleton is the state for a pipeline that arrived before its push.
func NewPushSkeleton(p event.Project, branch, sha string) *PushState {
	return &PushState{SchemaVer: SchemaVer, Project: p, Branch: branch, After: sha}
}

// Key returns the object_state key of this push.
func (s *PushState) Key() Key { return PushKey(s.Project.ID, s.Branch, s.After) }

// IsFinal reports whether the card will not change again.
func (s *PushState) IsFinal() bool { return s.Final }

// ReducePush folds a branch push (never a deletion) into s. A delivery
// received before the last applied one is ignored. Diff, Pipeline and the
// superseded flags are kept: a skeleton may already hold a pipeline.
func ReducePush(s *PushState, ev *event.Push) (changed bool) {
	if s.SchemaVer == 0 {
		s.SchemaVer = SchemaVer
	}
	if s.SeenPush && ev.Received.Before(s.LastPushEventAt) {
		return false
	}
	s.LastPushEventAt = ev.Received
	before := snapshot(s)
	s.Project = ev.Project
	s.Branch = ev.Branch()
	s.Before = ev.Before
	s.After = ev.After
	s.SeenPush = true
	s.Pusher = ev.User
	s.Commits = slices.Clone(ev.Commits)
	s.TotalCommits = max(ev.TotalCommitsCount, len(ev.Commits))
	s.Created = ev.IsCreate()
	s.Forced = ev.Forced
	s.recomputeFinal()
	return differs(before, s)
}

// SetPipeline replaces the pipeline copy unless it names an older
// pipeline; the same pipeline always applies (retries, status, tails).
func (s *PushState) SetPipeline(p *PipelineState) (changed bool) {
	if s.Pipeline != nil && p.ID < s.Pipeline.ID {
		return false
	}
	before := snapshot(s)
	cp := *p
	s.Pipeline = &cp
	s.recomputeFinal()
	return differs(before, s)
}

// SetDiff records the diff stats and reports whether they changed.
func (s *PushState) SetDiff(d DiffStats) (changed bool) {
	if s.Diff != nil && s.Diff.FilesChanged == d.FilesChanged && s.Diff.Added == d.Added && s.Diff.Removed == d.Removed && s.Diff.Partial == d.Partial && slices.Equal(s.Diff.Files, d.Files) {
		return false
	}
	s.Diff = &d
	return true
}

// Supersede freezes the card: by is the newer push's After SHA, or empty
// when the branch was deleted. Idempotent.
func (s *PushState) Supersede(by string) (changed bool) {
	if s.Superseded {
		return false
	}
	s.Superseded, s.SupersededBy = true, by
	s.recomputeFinal()
	return true
}

func (s *PushState) recomputeFinal() {
	s.Final = s.Superseded || (s.Pipeline != nil && s.Pipeline.Final)
}
