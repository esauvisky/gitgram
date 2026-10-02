package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

// rawPush covers Push Hook and Tag Push Hook, which share a flat layout with
// the actor spread over user_* fields.
type rawPush struct {
	Before            string      `json:"before"`
	After             string      `json:"after"`
	Ref               string      `json:"ref"`
	CheckoutSHA       string      `json:"checkout_sha"`
	Message           string      `json:"message"`
	UserID            int64       `json:"user_id"`
	UserName          string      `json:"user_name"`
	UserUsername      string      `json:"user_username"`
	UserAvatar        string      `json:"user_avatar"`
	ProjectID         int64       `json:"project_id"`
	Project           rawProject  `json:"project"`
	Commits           []rawCommit `json:"commits"`
	TotalCommitsCount int         `json:"total_commits_count"`
}

func (p rawPush) project() event.Project {
	proj := p.Project.event()
	if proj.ID == 0 {
		proj.ID = p.ProjectID
	}
	return proj
}

func (p rawPush) user() event.User {
	return event.User{ID: p.UserID, Username: p.UserUsername, Name: p.UserName, AvatarURL: p.UserAvatar}
}

func parsePush(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawPush
	if err := decode(body, kindPush, &raw); err != nil {
		return nil, err
	}
	return &event.Push{
		Meta:              meta,
		Project:           raw.project(),
		User:              raw.user(),
		Ref:               raw.Ref,
		Before:            raw.Before,
		After:             raw.After,
		CheckoutSHA:       raw.CheckoutSHA,
		Commits:           commits(raw.Commits),
		TotalCommitsCount: raw.TotalCommitsCount,
	}, nil
}
