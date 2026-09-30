package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// rawReviewer is one reviewers[] entry; state is present on newer GitLab
// versions only.
type rawReviewer struct {
	rawUser
	State string `json:"state"`
}

type rawMergeRequest struct {
	User             rawUser    `json:"user"`
	Project          rawProject `json:"project"`
	ObjectAttributes struct {
		ID                          int64      `json:"id"`
		IID                         int64      `json:"iid"`
		Title                       string     `json:"title"`
		Description                 string     `json:"description"`
		URL                         string     `json:"url"`
		State                       string     `json:"state"`
		Action                      string     `json:"action"`
		Draft                       bool       `json:"draft"`
		WorkInProgress              bool       `json:"work_in_progress"`
		SourceBranch                string     `json:"source_branch"`
		TargetBranch                string     `json:"target_branch"`
		SourceProjectID             int64      `json:"source_project_id"`
		AuthorID                    int64      `json:"author_id"`
		DetailedMergeStatus         string     `json:"detailed_merge_status"`
		BlockingDiscussionsResolved bool       `json:"blocking_discussions_resolved"`
		HeadPipelineID              *int64     `json:"head_pipeline_id"`
		LastCommit                  rawCommit  `json:"last_commit"`
		Labels                      []rawLabel `json:"labels"`
		CreatedAt                   ts         `json:"created_at"`
		UpdatedAt                   ts         `json:"updated_at"`
		MergedAt                    ts         `json:"merged_at"`
	} `json:"object_attributes"`
	Labels    []rawLabel    `json:"labels"`
	Assignees []rawUser     `json:"assignees"`
	Reviewers []rawReviewer `json:"reviewers"`
	Changes   rawChanges    `json:"changes"`
}

func parseMergeRequest(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawMergeRequest
	if err := decode(body, kindMergeRequest, &raw); err != nil {
		return nil, err
	}
	oa := raw.ObjectAttributes
	user := raw.User.event()

	mr := &event.MergeRequest{
		Meta:                        meta,
		Project:                     raw.Project.event(),
		User:                        user,
		Action:                      oa.Action,
		ID:                          oa.ID,
		IID:                         oa.IID,
		Title:                       oa.Title,
		Description:                 oa.Description,
		URL:                         oa.URL,
		State:                       oa.State,
		Draft:                       oa.Draft || oa.WorkInProgress,
		SourceBranch:                oa.SourceBranch,
		TargetBranch:                oa.TargetBranch,
		SourceProjectID:             oa.SourceProjectID,
		Author:                      authorFromActor(user, oa.AuthorID),
		Assignees:                   users(raw.Assignees),
		Labels:                      labels(oa.Labels),
		DetailedMergeStatus:         oa.DetailedMergeStatus,
		BlockingDiscussionsResolved: oa.BlockingDiscussionsResolved,
		HeadPipelineID:              oa.HeadPipelineID,
		LastCommit:                  oa.LastCommit.event(),
		MergedAt:                    oa.MergedAt.ptr(),
		CreatedAt:                   oa.CreatedAt.Time,
		UpdatedAt:                   oa.UpdatedAt.Time,
		Changes:                     raw.Changes.event(),
	}
	if mr.Labels == nil {
		mr.Labels = labels(raw.Labels)
	}
	if len(raw.Reviewers) > 0 {
		mr.Reviewers = make([]event.Reviewer, len(raw.Reviewers))
		for i, r := range raw.Reviewers {
			mr.Reviewers[i] = event.Reviewer{User: r.rawUser.event(), State: r.State}
		}
	}
	return mr, nil
}

// authorFromActor returns the acting user when they are the author
// (author_id matches), otherwise the zero User. Payloads only carry
// author_id, and an ID-only User would clobber a fully populated author
// already held in state.
func authorFromActor(actor event.User, authorID int64) event.User {
	if authorID != 0 && actor.ID == authorID {
		return actor
	}
	return event.User{}
}
