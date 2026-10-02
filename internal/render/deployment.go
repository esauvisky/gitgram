package render

import (
	"strconv"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Deployment renders a deployment status change: the title (`@ada
// deployed to production in demo (develop)`), the state in words, and the
// commit in small text.
func Deployment(d *event.Deployment, o Options) Message {
	var b htmlfmt.Builder
	env := htmlfmt.Esc(d.Environment)
	if d.EnvironmentURL != "" {
		env = htmlfmt.A(d.Environment, d.EnvironmentURL)
	}
	ref := ""
	if d.Ref != "" {
		ref = branchRef(d.Project, d.Ref)
	}
	headline(&b, o.who(d.User)+" deployed to "+env, "in", d.Project, ref)
	small(&b, deploymentText(d.Status))

	if d.Commit.SHA != "" {
		small(&b, commitLine(d.Commit, commitAuthor(d.Commit, d.User)))
	}
	if d.DeployableURL != "" {
		small(&b, htmlfmt.A("Deploy job", d.DeployableURL))
	}
	taglineFooter(&b, "deployment:"+strconv.FormatInt(d.Project.ID, 10)+":"+strconv.FormatInt(d.ID, 10)+":"+d.Status)

	more := d.DeployableURL
	if more == "" {
		more = d.Project.WebURL + "/-/environments"
	}
	return Message{HTML: b.Truncate(o.limit(), more)}
}

// deploymentText is the deployment's state in words.
func deploymentText(status string) string {
	switch status {
	case event.DeploymentRunning:
		return "Deploying..."
	case event.DeploymentSuccess:
		return "Deployed"
	case event.DeploymentFailed:
		return "Failed"
	case event.DeploymentCanceled:
		return "Canceled"
	case event.DeploymentBlocked:
		return "Waiting for approval"
	}
	return htmlfmt.Esc(sentence(humanize(status)))
}
