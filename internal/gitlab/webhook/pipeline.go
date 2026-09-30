package webhook

import (
	"strconv"

	"github.com/esauvisky/gitgram/internal/event"
)

// rawBuild is one builds[] entry of a Pipeline Hook.
type rawBuild struct {
	ID             int64    `json:"id"`
	Stage          string   `json:"stage"`
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	CreatedAt      ts       `json:"created_at"`
	StartedAt      ts       `json:"started_at"`
	FinishedAt     ts       `json:"finished_at"`
	Duration       *float64 `json:"duration"`
	QueuedDuration *float64 `json:"queued_duration"`
	FailureReason  string   `json:"failure_reason"`
	When           string   `json:"when"`
	Manual         bool     `json:"manual"`
	AllowFailure   bool     `json:"allow_failure"`
	User           rawUser  `json:"user"`
}

type rawPipeline struct {
	ObjectAttributes struct {
		ID             int64    `json:"id"`
		IID            int64    `json:"iid"`
		Ref            string   `json:"ref"`
		Tag            bool     `json:"tag"`
		SHA            string   `json:"sha"`
		Source         string   `json:"source"`
		Status         string   `json:"status"`
		DetailedStatus string   `json:"detailed_status"`
		Stages         []string `json:"stages"`
		CreatedAt      ts       `json:"created_at"`
		FinishedAt     ts       `json:"finished_at"`
		Duration       *int     `json:"duration"`
		QueuedDuration *int     `json:"queued_duration"`
		URL            string   `json:"url"`
	} `json:"object_attributes"`
	MergeRequest   *rawMRRef          `json:"merge_request"`
	User           rawUser            `json:"user"`
	Project        rawProject         `json:"project"`
	Commit         rawCommit          `json:"commit"`
	SourcePipeline *rawSourcePipeline `json:"source_pipeline"`
	Builds         []rawBuild         `json:"builds"`
}

func parsePipeline(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawPipeline
	if err := decode(body, kindPipeline, &raw); err != nil {
		return nil, err
	}
	oa := raw.ObjectAttributes
	project := raw.Project.event()
	user := raw.User.event()
	commit := raw.Commit.event()
	parent := raw.SourcePipeline.event()

	p := &event.Pipeline{
		Meta:           meta,
		Project:        project,
		User:           user,
		ID:             oa.ID,
		IID:            oa.IID,
		URL:            oa.URL,
		Ref:            oa.Ref,
		Tag:            oa.Tag,
		SHA:            oa.SHA,
		Source:         oa.Source,
		Status:         oa.Status,
		DetailedStatus: oa.DetailedStatus,
		Stages:         oa.Stages,
		CreatedAt:      oa.CreatedAt.Time,
		FinishedAt:     oa.FinishedAt.ptr(),
		Duration:       oa.Duration,
		QueuedDuration: oa.QueuedDuration,
		Commit:         commit,
		MR:             raw.MergeRequest.event(project),
		Parent:         parent,
	}
	if p.URL == "" {
		p.URL = project.WebURL + "/-/pipelines/" + strconv.FormatInt(oa.ID, 10)
	}
	if len(raw.Builds) > 0 {
		p.Jobs = make([]event.Job, len(raw.Builds))
	}
	for i, b := range raw.Builds {
		jobUser := b.User.event()
		if jobUser.IsZero() {
			jobUser = user
		}
		p.Jobs[i] = event.Job{
			Meta:           meta,
			Project:        project,
			User:           jobUser,
			ID:             b.ID,
			Name:           b.Name,
			Stage:          b.Stage,
			Status:         b.Status,
			AllowFailure:   b.AllowFailure,
			Manual:         b.Manual || b.When == "manual",
			FailureReason:  failureReason(b.Status, b.FailureReason),
			CreatedAt:      b.CreatedAt.Time,
			StartedAt:      b.StartedAt.ptr(),
			FinishedAt:     b.FinishedAt.ptr(),
			Duration:       b.Duration,
			QueuedDuration: b.QueuedDuration,
			URL:            jobURL(project, b.ID),
			Ref:            oa.Ref,
			Tag:            oa.Tag,
			SHA:            oa.SHA,
			PipelineID:     oa.ID,
			Commit:         commit,
			Parent:         parent,
		}
	}
	return p, nil
}

// jobURL derives the job page URL; neither builds[] nor Job Hooks carry one.
func jobURL(project event.Project, id int64) string {
	return project.WebURL + "/-/jobs/" + strconv.FormatInt(id, 10)
}

// failureReason keeps the reason only for failed jobs: GitLab fills the
// field with a placeholder on other statuses.
func failureReason(status, reason string) string {
	if status != event.StatusFailed {
		return ""
	}
	return reason
}
