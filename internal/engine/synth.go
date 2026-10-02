package engine

import (
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// synthPipeline builds a Pipeline Hook equivalent from the REST pipeline and
// its latest job runs. Fields the API does not carry (project, commit,
// merge request, parent, stage order) are taken from the stored state so
// the reducer keeps them.
func synthPipeline(st *cards.PipelineState, p *api.Pipeline, jobs []api.Job, now time.Time) *event.Pipeline {
	ev := &event.Pipeline{
		Meta:       event.Meta{Received: now},
		Project:    st.Project,
		User:       apiUserPtr(p.User),
		ID:         p.ID,
		IID:        p.IID,
		URL:        p.WebURL,
		Ref:        p.Ref,
		Tag:        p.Tag,
		SHA:        p.SHA,
		Source:     p.Source,
		Status:     p.Status,
		CreatedAt:  p.CreatedAt,
		FinishedAt: p.FinishedAt,
		Duration:   p.Duration,
		Commit:     st.Commit,
		Parent:     st.Parent,
	}
	if ev.Commit.SHA == "" && len(jobs) > 0 && jobs[0].Commit != nil {
		ev.Commit = apiCommit(*jobs[0].Commit)
	}
	ev.Jobs = make([]event.Job, len(jobs))
	for i, j := range jobs {
		ev.Jobs[i] = event.Job{
			Meta:           ev.Meta,
			Project:        ev.Project,
			User:           apiUserPtr(j.User),
			ID:             j.ID,
			Name:           j.Name,
			Stage:          j.Stage,
			Status:         j.Status,
			AllowFailure:   j.AllowFailure,
			Manual:         j.Status == event.StatusManual,
			CreatedAt:      j.CreatedAt,
			StartedAt:      j.StartedAt,
			FinishedAt:     j.FinishedAt,
			Duration:       j.Duration,
			QueuedDuration: j.QueuedDuration,
			URL:            j.WebURL,
			Ref:            p.Ref,
			Tag:            p.Tag,
			SHA:            p.SHA,
			PipelineID:     p.ID,
			Commit:         ev.Commit,
			Parent:         st.Parent,
		}
		if j.Status == event.StatusFailed {
			ev.Jobs[i].FailureReason = j.FailureReason
		}
		if ev.Jobs[i].User.IsZero() {
			ev.Jobs[i].User = ev.User
		}
	}
	return ev
}

func apiUser(u api.User) event.User {
	return event.User{ID: u.ID, Username: u.Username, Name: u.Name, AvatarURL: u.AvatarURL}
}

func apiUserPtr(u *api.User) event.User {
	if u == nil {
		return event.User{}
	}
	return apiUser(*u)
}

func apiCommit(c api.Commit) event.Commit {
	return event.Commit{
		SHA: c.ID, Title: c.Title, Message: c.Message, URL: c.WebURL,
		Author: event.User{Name: c.AuthorName}, Timestamp: c.CreatedAt,
	}
}
