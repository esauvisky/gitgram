package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Issue renders an issue card: the title (`@grace opened issue #7 in
// demo`, fixed once posted), the issue title in small text, who closed
// it, the description as a fold, and small lines for the people and the
// last change.
func Issue(s *cards.IssueState, o Options) Message {
	var b htmlfmt.Builder
	headline(&b, o.who(s.Author)+" opened issue "+anchorText("#", s.IID, s.URL), "in", s.Project, "")

	title := htmlfmt.Esc(clip(s.Title, maxTitleLen))
	if s.URL != "" {
		title = htmlfmt.A(clip(s.Title, maxTitleLen), s.URL)
	}
	if s.Confidential {
		title += " (confidential)"
	}
	small(&b, title)
	if s.State == event.IssueStateClosed {
		l := "Closed"
		if s.LastChange.Kind == cards.ChangeClosed && !s.LastChange.By.IsZero() {
			l += " by " + o.user(s.LastChange.By)
		}
		small(&b, l)
	}
	if desc := strings.TrimSpace(s.Description); o.ShowDescription && desc != "" {
		fold(&b, "Description", htmlfmt.RewriteMentions(htmlfmt.Esc(desc)))
	}
	var people []string
	if len(s.Assignees) > 0 {
		people = append(people, "Assignees "+o.users(s.Assignees))
	}
	if len(s.Labels) > 0 {
		people = append(people, "Labels "+htmlfmt.Esc(strings.Join(s.Labels, ", ")))
	}
	small(&b, strings.Join(people, " · "))
	small(&b, changeFooter(s.LastChange, o))
	taglineFooter(&b, "issue:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.IID, 10))
	return Message{HTML: b.Truncate(o.limit(), s.URL)}
}
