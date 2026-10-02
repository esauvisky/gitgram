package cards

import (
	"slices"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// PipelineState is the state behind one pipeline card.
//
// Status is the last status GitLab reported for the pipeline itself and is
// empty while the state is a job-built skeleton; renderers must use
// EffectiveStatus, which also reflects jobs retried after the pipeline
// finished.
type PipelineState struct {
	SchemaVer int
	Project   event.Project
	ID        int64
	IID       int64
	URL       string

	Ref    string
	Tag    bool
	SHA    string
	Source string

	// Status is the pipeline status from the last Pipeline Hook, empty for
	// a skeleton built from job hooks only.
	Status string
	// SeenPipelineEvent is false while the state is a skeleton built from
	// job hooks only.
	SeenPipelineEvent bool
	// LastPipelineEventAt is the receive time of the last applied Pipeline
	// Hook; older snapshots are ignored.
	LastPipelineEventAt time.Time

	CreatedAt  *time.Time
	FinishedAt *time.Time
	// Duration is seconds, set on the final pipeline event.
	Duration *int

	Triggerer event.User
	Commit    event.Commit

	// ConfirmStop is set while someone pressed Stop and the card is asking
	// to confirm; cleared by the answer or a retry.
	ConfirmStop bool

	// Parent is set for child pipelines.
	Parent *event.PipelineRef
	// Children summarises child pipelines, sorted by ID. Maintained by the
	// engine through UpsertChild.
	Children []ChildSummary

	// StageOrder is stages[] from the pipeline payload, extended with stages
	// first seen through job hooks.
	StageOrder []string
	// Jobs is every run seen, keyed by job id; superseded runs stay for
	// history and are excluded by SortedJobs.
	Jobs map[int64]JobState

	// Artifacts lists the downloadable archives jobs produced, filled from
	// the API once the pipeline is terminal.
	Artifacts []Artifact

	// Tails holds the last lines of job logs keyed by job id: the running
	// job's live tail and each failed job's final tail. The engine's log
	// tail loop maintains it; snapshot reduces leave it alone.
	Tails map[int64]JobTail

	// Final is true once a Pipeline Hook reported a terminal status and no
	// live job is active. Blocked pipelines (manual, scheduled) are never
	// final.
	Final bool
}

// Artifact is one job's downloadable artifact archive.
type Artifact struct {
	JobID   int64
	JobName string
	// Size is bytes.
	Size int64
}

// JobTail is the last lines of one job's log.
type JobTail struct {
	Lines []string
	// FetchedAt is when Lines were read.
	FetchedAt time.Time
	// Final is true once the job finished and Lines will not change.
	Final bool
}

// JobState is one job run within a pipeline card.
type JobState struct {
	ID     int64
	Name   string
	Stage  string
	Status string
	// Manual is true for when: manual jobs regardless of Status.
	Manual       bool
	AllowFailure bool
	// FailureReason is set when Status is failed.
	FailureReason string
	// Duration and QueuedDuration are seconds.
	Duration       *float64
	QueuedDuration *float64
	StartedAt      *time.Time
	FinishedAt     *time.Time
	URL            string
	// Retries is retries_count from the payload.
	Retries int
	// SupersededBy is the id of the newer run of the same (Stage, Name)
	// when this run was retried, 0 otherwise.
	SupersededBy int64
}

// ChildSummary is what a parent pipeline card knows about one child.
type ChildSummary struct {
	ProjectID   int64
	ProjectPath string
	ID          int64
	Ref         string
	// Status is the child's EffectiveStatus at the time of the summary.
	Status string
	URL    string
	// TriggerJobID is the bridge job in the parent that spawned the child.
	TriggerJobID int64
	// Jobs and Failed count live jobs and failed live jobs.
	Jobs   int
	Failed int
	Final  bool
}

// PipelineSummary is what an MR card knows about its head pipeline.
type PipelineSummary struct {
	ProjectID int64
	ID        int64
	// IID is the project-scoped number shown as the anchor; 0 for
	// job-hook skeletons that have not seen a Pipeline Hook.
	IID int64
	// Status is the pipeline's EffectiveStatus at the time of the summary.
	Status string
	URL    string
	Ref    string
	SHA    string
	// Jobs, Failed and Manual count live jobs, failed live jobs and jobs
	// waiting for manual action.
	Jobs   int
	Failed int
	Manual int
	Final  bool
}

// Key returns the object_state key of this pipeline.
func (s *PipelineState) Key() Key {
	return Key{Kind: KindPipeline, ProjectID: s.Project.ID, ObjectID: s.ID}
}

// IsFinal reports whether the card needs no further edits from events.
func (s *PipelineState) IsFinal() bool { return s.Final }

// Started reports whether the pipeline has anything worth a card yet: a
// Pipeline Hook was seen or at least one job left the created state. A
// skeleton built from created-only job hooks should not get a card.
func (s *PipelineState) Started() bool {
	if s.SeenPipelineEvent {
		return true
	}
	for _, j := range s.Jobs {
		if j.SupersededBy == 0 && j.Status != event.StatusCreated {
			return true
		}
	}
	return false
}

// ChildKey reports whether this pipeline is a child pipeline and, if so,
// the key of the parent pipeline whose Children entry it belongs to.
func (s *PipelineState) ChildKey() (Key, bool) {
	if s.Parent == nil {
		return Key{}, false
	}
	return Key{Kind: KindPipeline, ProjectID: s.Parent.ProjectID, ObjectID: s.Parent.PipelineID}, true
}

// EffectiveStatus is the status the card should display. For a skeleton it
// is derived from the jobs (pending when there are none). Once a Pipeline
// Hook was seen it is the reported status, except that a terminal or blocked
// pipeline whose jobs are active again (retry, played manual job) shows the
// job-derived status until the next Pipeline Hook.
func (s *PipelineState) EffectiveStatus() string {
	derived := s.derivedStatus()
	if !s.SeenPipelineEvent {
		if derived == "" {
			return event.StatusPending
		}
		return derived
	}
	if (event.IsTerminal(s.Status) || event.IsBlocked(s.Status)) && live(derived) {
		return derived
	}
	return s.Status
}

// live reports whether a job is progressing on its own; created jobs are
// queued behind a stage or a manual job and do not un-finish a pipeline.
func live(status string) bool { return event.IsActive(status) && status != event.StatusCreated }

// ManualJobs returns the names of live jobs waiting for manual action, in
// SortedJobs order.
func (s *PipelineState) ManualJobs() []string {
	var names []string
	for _, j := range s.SortedJobs() {
		if j.Status == event.StatusManual {
			names = append(names, j.Name)
		}
	}
	return names
}

// FailedJobs returns live jobs with status failed (including allow_failure
// ones; check AllowFailure to distinguish), in SortedJobs order.
func (s *PipelineState) FailedJobs() []JobState {
	var out []JobState
	for _, j := range s.SortedJobs() {
		if j.Status == event.StatusFailed {
			out = append(out, j)
		}
	}
	return out
}

// SortedJobs returns live (non-superseded) jobs ordered by stage (per
// StageOrder, unknown stages last alphabetically), then start time (unstarted
// last), then name, then id.
func (s *PipelineState) SortedJobs() []JobState {
	out := make([]JobState, 0, len(s.Jobs))
	for _, j := range s.Jobs {
		if j.SupersededBy == 0 {
			out = append(out, j)
		}
	}
	slices.SortFunc(out, func(a, b JobState) int {
		if c := s.compareStage(a.Stage, b.Stage); c != 0 {
			return c
		}
		if c := compareTimePtr(a.StartedAt, b.StartedAt); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmpInt64(a.ID, b.ID)
	})
	return out
}

// PipelineSummary condenses the state for an MR card.
func (s *PipelineState) PipelineSummary() PipelineSummary {
	live, failed, manual := s.counts()
	return PipelineSummary{
		ProjectID: s.Project.ID, ID: s.ID, IID: s.IID, Status: s.EffectiveStatus(), URL: s.URL,
		Ref: s.Ref, SHA: s.SHA, Jobs: live, Failed: failed, Manual: manual, Final: s.Final,
	}
}

// ChildSummary condenses the state for its parent pipeline's card.
func (s *PipelineState) ChildSummary() ChildSummary {
	live, failed, _ := s.counts()
	c := ChildSummary{
		ProjectID: s.Project.ID, ProjectPath: s.Project.Path, ID: s.ID, Ref: s.Ref,
		Status: s.EffectiveStatus(), URL: s.URL, Jobs: live, Failed: failed, Final: s.Final,
	}
	if s.Parent != nil {
		c.TriggerJobID = s.Parent.JobID
	}
	return c
}

// UpsertChild inserts or replaces the child with the same (ProjectID, ID)
// in Children, keeping the slice sorted by ID. It reports whether Children
// changed.
func (s *PipelineState) UpsertChild(c ChildSummary) bool {
	for i, cur := range s.Children {
		if cur.ProjectID == c.ProjectID && cur.ID == c.ID {
			if cur == c {
				return false
			}
			s.Children[i] = c
			return true
		}
	}
	s.Children = append(s.Children, c)
	slices.SortFunc(s.Children, func(a, b ChildSummary) int { return cmpInt64(a.ID, b.ID) })
	return true
}

func (s *PipelineState) counts() (live, failed, manual int) {
	for _, j := range s.Jobs {
		if j.SupersededBy != 0 {
			continue
		}
		live++
		switch j.Status {
		case event.StatusFailed:
			failed++
		case event.StatusManual:
			manual++
		}
	}
	return live, failed, manual
}

func (s *PipelineState) hasActiveJobs() bool {
	for _, j := range s.Jobs {
		if j.SupersededBy == 0 && live(j.Status) {
			return true
		}
	}
	return false
}

// derivedStatus aggregates live job statuses the way GitLab does for a
// pipeline; empty when there are no live jobs.
func (s *PipelineState) derivedStatus() string {
	var (
		any, running, canceling, failed, canceled, skipped, success bool
		queued, blocked                                             string
	)
	for _, j := range s.Jobs {
		if j.SupersededBy != 0 {
			continue
		}
		any = true
		switch st := j.Status; {
		case st == event.StatusRunning:
			running = true
		case st == event.StatusCanceling:
			canceling = true
		case event.IsActive(st):
			if event.StatusRank(st) > event.StatusRank(queued) {
				queued = st
			}
		case st == event.StatusFailed:
			if !j.AllowFailure {
				failed = true
			} else {
				success = true
			}
		case st == event.StatusCanceled:
			canceled = true
		case st == event.StatusManual:
			blocked = st
		case st == event.StatusScheduled:
			if blocked == "" {
				blocked = st
			}
		case st == event.StatusSkipped:
			skipped = true
		case st == event.StatusSuccess:
			success = true
		}
	}
	switch {
	case !any:
		return ""
	case running:
		return event.StatusRunning
	case canceling:
		return event.StatusCanceling
	case queued != "":
		return queued
	case failed:
		return event.StatusFailed
	case canceled:
		return event.StatusCanceled
	case blocked != "":
		return blocked
	case skipped && !success:
		return event.StatusSkipped
	}
	return event.StatusSuccess
}

func (s *PipelineState) compareStage(a, b string) int {
	ia, ib := slices.Index(s.StageOrder, a), slices.Index(s.StageOrder, b)
	switch {
	case ia >= 0 && ib >= 0:
		return ia - ib
	case ia >= 0:
		return -1
	case ib >= 0:
		return 1
	}
	return strings.Compare(a, b)
}

func compareTimePtr(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return a.Compare(*b)
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
