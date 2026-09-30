package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Issue renders an issue card.
func Issue(s *cards.IssueState, o Options) Message {
	var b htmlfmt.Builder
	emoji := EmojiIssueOpen
	if s.State == event.IssueStateClosed {
		emoji = EmojiIssueClosed
	}
	h := emoji + " "
	if s.Confidential {
		h += EmojiLock + " "
	}
	b.Line(h + htmlfmt.B("#"+strconv.FormatInt(s.IID, 10)+" "+s.Title))
	b.Line("by " + o.user(s.Author) + " in " + htmlfmt.A(s.Project.Path, s.Project.WebURL))
	if o.ShowDescription && strings.TrimSpace(s.Description) != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(strings.TrimSpace(s.Description)), o.Mentions), true)
	}
	if len(s.Assignees) > 0 {
		b.Line(EmojiPeople + " Assignees: " + o.users(s.Assignees))
	}
	if len(s.Labels) > 0 {
		b.Line(EmojiLabel + " " + htmlfmt.Esc(strings.Join(s.Labels, ", ")))
	}
	if s.State == event.IssueStateClosed {
		l := EmojiIssueClosed + " Closed"
		if s.LastChange.Kind == cards.ChangeClosed && !s.LastChange.By.IsZero() {
			l += " by " + o.user(s.LastChange.By)
		}
		b.Line("")
		b.Line(l)
	}
	if f := changeFooter(s.LastChange, o); f != "" {
		b.Line(f)
	}
	return Message{
		HTML:     b.Truncate(o.limit(), s.URL),
		Keyboard: [][]Button{{{Text: "Issue", URL: s.URL}}},
	}
}
