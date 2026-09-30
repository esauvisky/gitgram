package webhook

import (
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
)

type rawDeployment struct {
	Status                 string     `json:"status"`
	StatusChangedAt        ts         `json:"status_changed_at"`
	DeploymentID           int64      `json:"deployment_id"`
	DeployableID           int64      `json:"deployable_id"`
	DeployableURL          string     `json:"deployable_url"`
	Environment            string     `json:"environment"`
	EnvironmentTier        string     `json:"environment_tier"`
	EnvironmentExternalURL string     `json:"environment_external_url"`
	Ref                    string     `json:"ref"`
	ShortSHA               string     `json:"short_sha"`
	CommitURL              string     `json:"commit_url"`
	CommitTitle            string     `json:"commit_title"`
	User                   rawUser    `json:"user"`
	Project                rawProject `json:"project"`
}

func parseDeployment(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawDeployment
	if err := decode(body, kindDeployment, &raw); err != nil {
		return nil, err
	}
	sha := raw.ShortSHA
	if full := raw.CommitURL[strings.LastIndex(raw.CommitURL, "/")+1:]; len(full) == 40 {
		sha = full
	}
	return &event.Deployment{
		Meta:            meta,
		Project:         raw.Project.event(),
		User:            raw.User.event(),
		ID:              raw.DeploymentID,
		Status:          raw.Status,
		StatusChangedAt: raw.StatusChangedAt.Time,
		DeployableID:    raw.DeployableID,
		DeployableURL:   raw.DeployableURL,
		Environment:     raw.Environment,
		EnvironmentURL:  raw.EnvironmentExternalURL,
		EnvironmentTier: raw.EnvironmentTier,
		Ref:             raw.Ref,
		Commit: event.Commit{
			SHA:   sha,
			Title: raw.CommitTitle,
			URL:   raw.CommitURL,
		},
	}, nil
}
