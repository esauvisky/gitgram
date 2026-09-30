package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Note renders a comment. When anchorKnown is true the message will be sent
// as a reply to the MR or issue card, so the header omits the target.
func Note(n *event.Note, anchorKnown bool, o Options) Message {
	var b htmlfmt.Builder
	verb := "commented"
	if n.Action == event.NoteActionUpdate {
		verb = "edited a comment"
	}
	h := EmojiNote + " " + htmlfmt.B(displayName(n.User)) + " " + verb
	if !anchorKnown {
		if target := noteTarget(n); target != "" {
			h += " on " + target
		}
	}
	if n.IsDiff && n.FilePath != "" {
		h += " · " + htmlfmt.Code(fileRef(n.FilePath, n.Line))
	}
	b.Line(h)
	if body := strings.TrimSpace(n.Body); body != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(body), o.Mentions), true)
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
		return "commit " + htmlfmt.A(htmlfmt.ShortSHA(n.CommitSHA), n.Project.WebURL+"/-/commit/"+n.CommitSHA)
	case n.NoteableType == event.NoteableSnippet:
		return "a snippet"
	}
	return ""
}
