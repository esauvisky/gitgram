package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Release renders a release created, updated or deleted: the headline
// (release payloads name no person), the commit in small text, the notes
// as a fold, and the asset links.
func Release(r *event.Release, o Options) Message {
	var b htmlfmt.Builder
	name := r.Name
	if name == "" {
		name = r.Tag
	}
	title := htmlfmt.Esc(name)
	if r.URL != "" {
		title = htmlfmt.A(name, r.URL)
	}
	switch r.Action {
	case event.ReleaseActionDelete:
		headline(&b, "Release "+htmlfmt.Esc(name)+" was deleted", "in", r.Project, tagRef(r.Project, r.Tag))
		return Message{HTML: b.String()}
	case event.ReleaseActionUpdate:
		headline(&b, "Release "+title+" was updated", "in", r.Project, tagRef(r.Project, r.Tag))
	default:
		headline(&b, "Release "+title+" was published", "in", r.Project, tagRef(r.Project, r.Tag))
	}
	if r.Commit.SHA != "" {
		small(&b, commitLine(r.Commit, r.Commit.Author.Name))
	}
	if desc := strings.TrimSpace(r.Description); desc != "" {
		fold(&b, "Release notes", htmlfmt.RewriteMentions(htmlfmt.Esc(desc)))
	}
	if len(r.Links) > 0 {
		parts := make([]string, len(r.Links))
		for i, l := range r.Links {
			parts[i] = htmlfmt.A(l.Name, l.URL)
		}
		small(&b, "Assets: "+strings.Join(parts, " · "))
	}
	taglineFooter(&b, "release:"+strconv.FormatInt(r.Project.ID, 10)+":"+r.Tag)
	return Message{HTML: b.Truncate(o.limit(), r.URL)}
}
