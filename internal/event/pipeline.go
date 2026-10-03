package event

import "time"

// Pipeline is a Pipeline Hook. GitLab fires it on status transitions only
// (never on creation); Jobs is the snapshot of builds[] at that moment,
// holding the latest run of every job (retried runs are excluded).
type Pipeline struct {
	Meta
	Project Project
	// User is the pipeline triggerer.
	User User

	ID  int64
	IID int64
	// URL is object_attributes.url.
	URL string
	// Ref is the branch or tag name (not a full ref).
	Ref string
	Tag bool
	SHA string
	// Source is object_attributes.source: push, web, trigger, schedule, api,
	// merge_request_event, parent_pipeline, ...
	Source string
	// Status is one of the Status* constants.
	Status string
	// DetailedStatus is object_attributes.detailed_status (e.g. "passed",
	// "passed with warnings"); informational only.
	DetailedStatus string
	// Stages is stages[] in execution order.
	Stages []string

	CreatedAt  time.Time
	FinishedAt *time.Time
	// Duration is seconds; nil while the pipeline is running.
	Duration *int
	// QueuedDuration is seconds spent queued, when reported.
	QueuedDuration *int

	Commit Commit
	// MR is set for merge request pipelines (source merge_request_event).
	MR *MRRef
	// Parent is set for child pipelines (source_pipeline).
	Parent *PipelineRef
	// Jobs is builds[] normalized. Each entry's Meta, Project, User,
	// PipelineID, Ref, SHA and Parent are filled from the pipeline payload so
	// entries are self-contained.
	Jobs []Job
}

// EventKind implements Event.
func (*Pipeline) EventKind() Kind { return KindPipeline }

// Proj implements Event.
func (p *Pipeline) Proj() Project { return p.Project }

// Actor implements Event.
func (p *Pipeline) Actor() User { return p.User }

// Job is a Job Hook (object_kind "build"), or one builds[] entry of a
// Pipeline Hook. GitLab sends job hooks on created, pending, running,
// success, failed and canceled only; manual, skipped and scheduled jobs are
// only visible through the pipeline snapshot.
type Job struct {
	Meta
	Project Project
	// User is the job's triggerer.
	User User

	// ID is build_id. A retry gets a new ID; match runs of the same job by
	// (Stage, Name).
	ID    int64
	Name  string
	Stage string
	// Status is one of the Status* constants.
	Status       string
	AllowFailure bool
	// Manual is true for jobs with when: manual, regardless of Status.
	Manual bool
	// FailureReason is build_failure_reason (e.g. "script_failure"), empty
	// unless failed.
	FailureReason string
	// Retries is retries_count: how many earlier runs this job supersedes.
	Retries int

	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	// Duration is seconds; nil until the job started.
	Duration *float64
	// QueuedDuration is seconds spent waiting for a runner.
	QueuedDuration *float64

	// URL is the job page. The Job Hook payload has no job URL; the parser
	// derives Project.WebURL + "/-/jobs/" + ID.
	URL string
	// Ref is the branch or tag name (not a full ref).
	Ref string
	Tag bool
	// SHA is the top-level sha field. Never use commit.id from a Job Hook:
	// it is the pipeline id.
	SHA        string
	PipelineID int64
	// Commit carries what the Job Hook's commit{} object provides (message,
	// author name) with SHA copied from the top-level sha.
	Commit Commit
	// Parent is set for jobs of child pipelines (source_pipeline).
	Parent *PipelineRef
}

// EventKind implements Event.
func (*Job) EventKind() Kind { return KindJob }

// Proj implements Event.
func (j *Job) Proj() Project { return j.Project }

// Actor implements Event.
func (j *Job) Actor() User { return j.User }
