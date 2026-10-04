package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Push renders a push card: the title (`@ada pushed to demo (feat/x)`),
// the force-push warning (the verb links to the compare view), the commits
// as a quote of `author: title` rows
// closed by the line counts in italics, then the pipeline for the pushed
// head: in full (one line per stage with its time, the failed log, Stop
// and Retry) when this card absorbs the pipeline the push triggered, as
// one small line when the pipeline has a card of its own.
func Push(s *cards.PushState, o Options) Message {
	var b htmlfmt.Builder
	verb, prep := "pushed", "to"
	switch {
	case s.Created:
		verb, prep = "created a branch", "in"
	case s.Forced:
		verb = "force-pushed"
	}
	what := htmlfmt.Esc(verb)
	if u := pushMoreURL(s); u != "" {
		what = htmlfmt.A(verb, u)
	}
	headline(&b, o.who(s.Pusher)+" "+what, prep, s.Project, branchRef(s.Project, s.Branch))
	if s.Forced {
		small(&b, "⚠️ Force push: the branch history was rewritten")
	}
	if len(s.Commits) > 0 {
		rows := commitList(s.Commits, s.TotalCommits, s.Pusher, pushMoreURL(s))
		if d := diffSummary(s.Diff); d != "" {
			rows = append(rows, "", "<i>"+d+"</i>")
		}
		b.Quote(strings.Join(rows, "\n"), len(rows) >= collapseCommitLines)
	}
	var kb [][]Button
	switch {
	case s.Pipeline != nil && s.Absorbs:
		v := viewPipeline(s.Pipeline)
		pipelineBody(s.Pipeline, v, &b)
		kb = pipelineKeyboard(s.Pipeline, v, o)
	case s.Pipeline != nil:
		sum := s.Pipeline.PipelineSummary()
		small(&b, pipelineLine(&sum))
	}
	taglineFooter(&b, "push:"+strconv.FormatInt(s.Project.ID, 10)+":"+s.Branch+":"+s.After)
	return Message{HTML: b.Truncate(o.limit(), pushMoreURL(s)), Keyboard: kb}
}

// pushMoreURL is the compare view of the push (previous head to new head),
// or for a new branch everything since it left the default branch; the
// branch history when neither is known. The headline's verb and "+N more"
// link to it.
func pushMoreURL(s *cards.PushState) string {
	if s.Project.WebURL == "" {
		return ""
	}
	if s.Before != event.ZeroSHA && s.Before != "" && s.After != "" {
		return s.Project.WebURL + "/-/compare/" + s.Before + "..." + s.After
	}
	if s.Created && s.After != "" && s.Project.DefaultBranch != "" && s.Project.DefaultBranch != s.Branch {
		return s.Project.WebURL + "/-/compare/" + s.Project.DefaultBranch + "..." + s.After
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

// collapseCommitLines is the size from which the commits quote starts
// collapsed: shorter quotes (commit rows, the blank line and the line
// counts) show in full.
const collapseCommitLines = 8

// commitList is one `author: title` row per commit the payload carries,
// then "+N more" (linked to moreURL when set) when the push held more.
func commitList(commits []event.Commit, total int, pusher event.User, moreURL string) []string {
	lines := make([]string, 0, len(commits)+1)
	for _, c := range commits {
		lines = append(lines, commitLine(c, commitAuthor(c, pusher)))
	}
	if rest := total - len(commits); rest > 0 {
		more := "+" + strconv.Itoa(rest) + " more"
		if moreURL != "" {
			lines = append(lines, htmlfmt.A(more, moreURL))
		} else {
			lines = append(lines, more)
		}
	}
	return lines
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

// pipelineLine is the small pipeline line on a push card whose pipeline
// has a card of its own:
// `Pipeline #n word [· N failed] [· N manual]`.
func pipelineLine(p *cards.PipelineSummary) string {
	num := p.IID
	if num == 0 {
		num = p.ID
	}
	l := "Pipeline " + anchorText("#", num, p.URL) + " " + strings.ToLower(statusWord(p.Status, false))
	if event.IsActive(p.Status) && p.Status != event.StatusRunning {
		l = "Pipeline " + anchorText("#", num, p.URL) + " queued"
	}
	if p.Failed > 0 && p.Status != event.StatusFailed {
		l += " · " + strconv.Itoa(p.Failed) + " failed"
	}
	if p.Manual > 0 && p.Status != event.StatusManual {
		l += " · " + strconv.Itoa(p.Manual) + " manual"
	}
	return l
}
