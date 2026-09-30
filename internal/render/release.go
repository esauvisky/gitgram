package render

import (
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Release renders a release created, updated or deleted.
func Release(r *event.Release, o Options) Message {
	var b htmlfmt.Builder
	name := r.Name
	if name == "" {
		name = r.Tag
	}
	title := htmlfmt.B("Release "+name) + " (" + htmlfmt.Code(r.Tag) + ")"
	project := htmlfmt.A(r.Project.Path, r.Project.WebURL)
	switch r.Action {
	case event.ReleaseActionDelete:
		b.Line(EmojiDeletedBranch + " " + title + " deleted in " + project)
		return Message{HTML: b.Truncate(o.limit(), r.Project.WebURL+"/-/releases")}
	case event.ReleaseActionUpdate:
		b.Line(EmojiRelease + " " + title + " updated in " + project)
	default:
		b.Line(EmojiRelease + " " + title + " in " + project)
	}
	if r.Commit.SHA != "" {
		sha := htmlfmt.ShortSHA(r.Commit.SHA)
		l := htmlfmt.Code(sha)
		if r.Commit.URL != "" {
			l = htmlfmt.A(sha, r.Commit.URL)
		}
		if r.Commit.Title != "" {
			l += " " + htmlfmt.Esc(r.Commit.Title)
		}
		b.Line(l)
	}
	if desc := strings.TrimSpace(r.Description); desc != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(desc), o.Mentions), true)
	}
	if len(r.Links) > 0 {
		parts := make([]string, len(r.Links))
		for i, l := range r.Links {
			parts[i] = htmlfmt.A(l.Name, l.URL)
		}
		b.Line(EmojiLink + " " + strings.Join(parts, " · "))
	}
	return Message{HTML: b.Truncate(o.limit(), r.URL)}
}
