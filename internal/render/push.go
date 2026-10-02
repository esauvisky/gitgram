package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Push renders a push card: the title (`@ada pushed to demo (feat/x)`),
// the force-push warning, the commits as a fold of `author: title` rows
// closed by the line counts in italics, then the pipeline for the pushed
// head: in full (one line per stage with its time, the failed log, Stop
// and Retry) when this card absorbs the pipeline the push triggered, as
// one small line when the pipeline has a card of its own. maxCommits caps
// the listed commits.
func Push(s *cards.PushState, maxCommits int, o Options) Message {
	var b htmlfmt.Builder
	lead, prep := o.who(s.Pusher)+" pushed", "to"
	switch {
	case s.Created:
		lead, prep = o.who(s.Pusher)+" created a branch", "in"
	case s.Forced:
		lead = o.who(s.Pusher) + " force-pushed"
	}
	headline(&b, lead, prep, s.Project, branchRef(s.Project, s.Branch))
	if s.Forced {
		small(&b, "⚠️ Force push: the branch history was rewritten")
	}
	if len(s.Commits) > 0 {
		rows := commitList(s.Commits, s.TotalCommits, maxCommits, s.Pusher, pushMoreURL(s))
		if d := diffSummary(s.Diff); d != "" {
			rows += "\n\n<i>" + d + "</i>"
		}
		b.Quote(rows, true)
	}
	var kb [][]Button
	switch {
	case s.Pipeline != nil && s.Absorbs:
		v := viewPipeline(s.Pipeline)
		pipelineBody(s.Pipeline, v, o, &b)
		if a := artifactsLine(s.Pipeline); a != "" {
			b.Line("")
			small(&b, a)
		}
		kb = pipelineKeyboard(s.Pipeline, v, o)
	case s.Pipeline != nil:
		sum := s.Pipeline.PipelineSummary()
		small(&b, pipelineLine(&sum))
	}
	taglineFooter(&b, "push:"+strconv.FormatInt(s.Project.ID, 10)+":"+s.Branch+":"+s.After)
	return Message{HTML: b.Truncate(o.limit(), pushMoreURL(s)), Keyboard: kb}
}

// pushMoreURL is where the commits a card cannot list live: the compare
// view of the push, or the branch history when the push created it.
func pushMoreURL(s *cards.PushState) string {
	if s.Project.WebURL == "" {
		return ""
	}
	if s.Before != event.ZeroSHA && s.Before != "" && s.After != "" {
		return s.Project.WebURL + "/-/compare/" + s.Before + "..." + s.After
	}
	return s.Project.WebURL + "/-/commits/" + s.Branch
}

// diffSummary is the push's line counts: `+23, -46 lines on 4 files`.
// Empty when no stats were fetched.
func diffSummary(df *cards.DiffStats) string {
	if df == nil {
		return ""
	}
	if df.FilesChanged == 0 {
		return "No file changes"
	}
	l := "+" + strconv.Itoa(df.Added) + ", -" + strconv.Itoa(df.Removed) + " lines on " + plural(df.FilesChanged, "file")
	if df.Partial {
		l += " (partial)"
	}
	return l
}

// BranchDeleted renders the one-shot branch deletion message.
func BranchDeleted(p *event.Push, o Options) Message {
	var b htmlfmt.Builder
	headline(&b, o.who(p.User)+" deleted a branch", "in", p.Project, htmlfmt.Code(p.Branch()))
	taglineFooter(&b, "deleted:"+strconv.FormatInt(p.Project.ID, 10)+":"+p.Branch()+":"+p.Before)
	return Message{HTML: b.String()}
}

// commitList renders up to max commits as `author: title` lines (see
// commitLine), followed by "+N more" when total exceeds what is shown,
// linked to moreURL when set.
func commitList(commits []event.Commit, total, max int, pusher event.User, moreURL string) string {
	if max <= 0 {
		max = len(commits)
	}
	shown := commits
	if len(shown) > max {
		shown = shown[:max]
	}
	lines := make([]string, 0, len(shown)+1)
	for _, c := range shown {
		lines = append(lines, commitLine(c, commitAuthor(c, pusher)))
	}
	if rest := total - len(shown); rest > 0 {
		more := "+" + strconv.Itoa(rest) + " more"
		if moreURL != "" {
			lines = append(lines, htmlfmt.A(more, moreURL))
		} else {
			lines = append(lines, more)
		}
	}
	return strings.Join(lines, "\n")
}

// commitLine is a commit as `author: title`, the author bold and italic,
// the whole title linked to the commit and never clipped (it wraps).
// Cards never show SHAs.
func commitLine(c event.Commit, author string) string {
	title := htmlfmt.Esc(c.Title)
	if c.URL != "" {
		title = htmlfmt.A(c.Title, c.URL)
	}
	if author == "" {
		return title
	}
	return "<b><i>" + htmlfmt.Esc(author) + "</i></b>: " + title
}

// commitAuthor names a commit's author: the actor's username when the
// author name matches theirs (webhooks carry only names), else the name.
func commitAuthor(c event.Commit, actor event.User) string {
	if c.Author.Name != "" && c.Author.Name == actor.Name && actor.Username != "" {
		return actor.Username
	}
	return c.Author.Name
}

// TagPush renders a tag pushed or deleted: the headline naming the tag and
// the commit it points at, and the annotation as a fold.
func TagPush(t *event.TagPush, o Options) Message {
	var b htmlfmt.Builder
	tag := htmlfmt.Code(t.Tag())
	if t.IsDelete() {
		headline(&b, o.who(t.User)+" deleted a tag", "in", t.Project, tag)
		return Message{HTML: b.String()}
	}
	url := t.Project.WebURL + "/-/tags/" + t.Tag()
	if t.Project.WebURL != "" {
		tag = `<a href="` + htmlfmt.Esc(url) + `">` + tag + "</a>"
	}
	headline(&b, o.who(t.User)+" pushed a tag", "to", t.Project, tag)
	if msg := strings.TrimSpace(t.Message); msg != "" {
		fold(&b, "Message", htmlfmt.RewriteMentions(htmlfmt.Esc(msg)))
	}
	taglineFooter(&b, "tag:"+strconv.FormatInt(t.Project.ID, 10)+":"+t.Tag()+":"+t.CheckoutSHA)
	return Message{HTML: b.Truncate(o.limit(), url)}
}
