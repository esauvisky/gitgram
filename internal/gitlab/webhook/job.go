package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// rawJob is a Job Hook (object_kind "build"). Fields are flat and prefixed
// build_*; commit{} describes the pipeline (its id is the pipeline id), so
// the SHA comes from the top-level sha.
type rawJob struct {
	Ref                 string   `json:"ref"`
	Tag                 bool     `json:"tag"`
	SHA                 string   `json:"sha"`
	BuildID             int64    `json:"build_id"`
	BuildName           string   `json:"build_name"`
	BuildStage          string   `json:"build_stage"`
	BuildStatus         string   `json:"build_status"`
	BuildCreatedAt      ts       `json:"build_created_at"`
	BuildStartedAt      ts       `json:"build_started_at"`
	BuildFinishedAt     ts       `json:"build_finished_at"`
	BuildDuration       *float64 `json:"build_duration"`
	BuildQueuedDuration *float64 `json:"build_queued_duration"`
	BuildAllowFailure   bool     `json:"build_allow_failure"`
	BuildFailureReason  string   `json:"build_failure_reason"`
	RetriesCount        int      `json:"retries_count"`
	PipelineID          int64    `json:"pipeline_id"`
	ProjectID           int64    `json:"project_id"`
	ProjectName         string   `json:"project_name"`
	User                rawUser  `json:"user"`
	Commit              struct {
		Message     string `json:"message"`
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
	} `json:"commit"`
	Project        rawProject         `json:"project"`
	SourcePipeline *rawSourcePipeline `json:"source_pipeline"`
}

func parseJob(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawJob
	if err := decode(body, "job", &raw); err != nil {
		return nil, err
	}
	project := raw.Project.event()
	if project.ID == 0 {
		project.ID = raw.ProjectID
	}
	if project.Path == "" {
		project.Path = raw.ProjectName
	}
	commit := event.Commit{
		SHA:     raw.SHA,
		Title:   firstLine("", raw.Commit.Message),
		Message: raw.Commit.Message,
		Author:  event.User{Name: raw.Commit.AuthorName, Username: raw.Commit.AuthorEmail},
	}
	if raw.SHA != "" && project.WebURL != "" {
		commit.URL = project.WebURL + "/-/commit/" + raw.SHA
	}
	return &event.Job{
		Meta:           meta,
		Project:        project,
		User:           raw.User.event(),
		ID:             raw.BuildID,
		Name:           raw.BuildName,
		Stage:          raw.BuildStage,
		Status:         raw.BuildStatus,
		AllowFailure:   raw.BuildAllowFailure,
		Manual:         raw.BuildStatus == event.StatusManual,
		FailureReason:  failureReason(raw.BuildStatus, raw.BuildFailureReason),
		Retries:        raw.RetriesCount,
		CreatedAt:      raw.BuildCreatedAt.Time,
		StartedAt:      raw.BuildStartedAt.ptr(),
		FinishedAt:     raw.BuildFinishedAt.ptr(),
		Duration:       raw.BuildDuration,
		QueuedDuration: raw.BuildQueuedDuration,
		URL:            jobURL(project, raw.BuildID),
		Ref:            raw.Ref,
		Tag:            raw.Tag,
		SHA:            raw.SHA,
		PipelineID:     raw.PipelineID,
		Commit:         commit,
		Parent:         raw.SourcePipeline.event(),
	}, nil
}
