package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

type rawRelease struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Tag         string     `json:"tag"`
	Description string     `json:"description"`
	URL         string     `json:"url"`
	Action      string     `json:"action"`
	CreatedAt   ts         `json:"created_at"`
	ReleasedAt  ts         `json:"released_at"`
	Project     rawProject `json:"project"`
	Commit      rawCommit  `json:"commit"`
	Assets      struct {
		Links []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"links"`
	} `json:"assets"`
}

func parseRelease(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawRelease
	if err := decode(body, kindRelease, &raw); err != nil {
		return nil, err
	}
	r := &event.Release{
		Meta:        meta,
		Project:     raw.Project.event(),
		Action:      raw.Action,
		ID:          raw.ID,
		Name:        raw.Name,
		Tag:         raw.Tag,
		Description: raw.Description,
		URL:         raw.URL,
		Commit:      raw.Commit.event(),
		CreatedAt:   raw.CreatedAt.Time,
		ReleasedAt:  raw.ReleasedAt.Time,
	}
	if len(raw.Assets.Links) > 0 {
		r.Links = make([]event.Link, len(raw.Assets.Links))
		for i, l := range raw.Assets.Links {
			r.Links[i] = event.Link{Name: l.Name, URL: l.URL}
		}
	}
	return r, nil
}
