package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// eventTypeConfidentialIssue is the event_type GitLab sets on confidential
// issue hooks (object_kind stays "issue").
const eventTypeConfidentialIssue = "confidential_issue"

// rawIssue covers Issue Hook, Confidential Issue Hook and work item hooks
// (object_kind "work_item"), which share the object_attributes layout.
type rawIssue struct {
	User             rawUser    `json:"user"`
	Project          rawProject `json:"project"`
	ObjectAttributes struct {
		ID           int64      `json:"id"`
		IID          int64      `json:"iid"`
		Title        string     `json:"title"`
		Description  string     `json:"description"`
		URL          string     `json:"url"`
		State        string     `json:"state"`
		Action       string     `json:"action"`
		Confidential bool       `json:"confidential"`
		AuthorID     int64      `json:"author_id"`
		Labels       []rawLabel `json:"labels"`
		CreatedAt    ts         `json:"created_at"`
		UpdatedAt    ts         `json:"updated_at"`
		ClosedAt     ts         `json:"closed_at"`
	} `json:"object_attributes"`
	Labels    []rawLabel `json:"labels"`
	Assignees []rawUser  `json:"assignees"`
	Changes   rawChanges `json:"changes"`
}

func parseIssue(body []byte, eventType string, meta event.Meta) (event.Event, error) {
	var raw rawIssue
	if err := decode(body, kindIssue, &raw); err != nil {
		return nil, err
	}
	oa := raw.ObjectAttributes
	user := raw.User.event()

	is := &event.Issue{
		Meta:         meta,
		Project:      raw.Project.event(),
		User:         user,
		Action:       oa.Action,
		ID:           oa.ID,
		IID:          oa.IID,
		Title:        oa.Title,
		Description:  oa.Description,
		URL:          oa.URL,
		State:        oa.State,
		Confidential: oa.Confidential || eventType == eventTypeConfidentialIssue,
		Author:       authorFromActor(user, oa.AuthorID),
		Assignees:    users(raw.Assignees),
		Labels:       labels(oa.Labels),
		CreatedAt:    oa.CreatedAt.Time,
		UpdatedAt:    oa.UpdatedAt.Time,
		ClosedAt:     oa.ClosedAt.ptr(),
		Changes:      raw.Changes.event(),
	}
	if is.Labels == nil {
		is.Labels = labels(raw.Labels)
	}
	return is, nil
}
