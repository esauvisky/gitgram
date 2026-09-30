package render

import (
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Deployment renders a deployment status change.
func Deployment(d *event.Deployment, o Options) Message {
	var b htmlfmt.Builder
	env := d.Environment
	if d.EnvironmentURL != "" {
		env = htmlfmt.A(d.Environment, d.EnvironmentURL)
	} else {
		env = htmlfmt.Esc(env)
	}
	b.Line(EmojiDeploy + " <b>" + env + "</b> · " + deploymentEmoji(d.Status) + " " + deploymentText(d.Status) +
		" in " + htmlfmt.A(d.Project.Path, d.Project.WebURL))

	var parts []string
	if d.Ref != "" {
		parts = append(parts, htmlfmt.Code(d.Ref))
	}
	if d.Commit.SHA != "" {
		sha := htmlfmt.ShortSHA(d.Commit.SHA)
		l := htmlfmt.Code(sha)
		if d.Commit.URL != "" {
			l = htmlfmt.A(sha, d.Commit.URL)
		}
		if d.Commit.Title != "" {
			l += " " + htmlfmt.Esc(d.Commit.Title)
		}
		parts = append(parts, l)
	}
	if !d.User.IsZero() {
		parts = append(parts, EmojiUser+" "+o.user(d.User))
	}
	if len(parts) > 0 {
		b.Line(strings.Join(parts, " · "))
	}

	more := d.DeployableURL
	if more == "" {
		more = d.Project.WebURL + "/-/environments"
	}
	return Message{HTML: b.Truncate(o.limit(), more)}
}

func deploymentText(status string) string {
	switch status {
	case event.DeploymentRunning:
		return "deploying"
	case event.DeploymentSuccess:
		return "deployed"
	case event.DeploymentFailed:
		return "deployment failed"
	case event.DeploymentCanceled:
		return "deployment canceled"
	case event.DeploymentBlocked:
		return "deployment waiting for approval"
	}
	return htmlfmt.Esc(humanize(status))
}
