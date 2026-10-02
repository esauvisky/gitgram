package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Note renders a comment: the headline (who commented, on what unless the
// message replies to that card, in which project), the file and line of a
// diff comment in small text, and the body as a quote. When anchorKnown is
// true the message is sent as a reply to the MR or issue card.
func Note(n *event.Note, anchorKnown bool, o Options) Message {
	var b htmlfmt.Builder
	verb := "commented"
	if n.Action == event.NoteActionUpdate {
		verb = "edited a comment"
	}
	line := o.who(n.User) + " " + verb
	if !anchorKnown {
		if target := noteTarget(n); target != "" {
			line += " on " + target
		}
	}
	headline(&b, line, "in", n.Project, "")
	if n.IsDiff && n.FilePath != "" {
		small(&b, htmlfmt.Code(fileRef(n.FilePath, n.Line)))
	}
	if body := strings.TrimSpace(n.Body); body != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(body)), true)
	}
	return Message{HTML: b.Truncate(o.limit(), n.URL)}
}

// noteTarget names what the note is attached to, linked when possible.
func noteTarget(n *event.Note) string {
	switch {
	case n.MR != nil:
		label := "!" + strconv.FormatInt(n.MR.IID, 10)
		if n.MR.Title != "" {
			label += " " + n.MR.Title
		}
		return htmlfmt.A(label, n.MR.URL)
	case n.Issue != nil:
		label := "#" + strconv.FormatInt(n.Issue.IID, 10)
		if n.Issue.Title != "" {
			label += " " + n.Issue.Title
		}
		return htmlfmt.A(label, n.Issue.URL)
	case n.CommitSHA != "":
		return htmlfmt.A("a commit", n.Project.WebURL+"/-/commit/"+n.CommitSHA)
	case n.NoteableType == event.NoteableSnippet:
		return "a snippet"
	}
	return ""
}
