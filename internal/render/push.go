package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Push renders a push card: who pushed to which project, what happened to
// which branch, the commits quoted in a closed fold, the git --stat block
// in a closed fold titled with the diff stats (the title struck through
// once the branch is deleted), then the pipeline for the pushed head: in full
// (status, stages, logs, buttons) when this card absorbs the pipeline the
// push triggered, as one small line when the pipeline has a card of its
// own. maxCommits caps the
// listed commits.
func Push(s *cards.PushState, maxCommits int, o Options) Message {
	var d htmlfmt.Doc
	branch := htmlfmt.Code(s.Branch)
	if s.Project.WebURL != "" {
		branch = `<a href="` + htmlfmt.Esc(s.Project.WebURL+"/-/tree/"+s.Branch) + `">` + branch + "</a>"
	}
	var lamp, verb, title string
	switch {
	case s.Created:
		lamp, verb, title = EmojiNewBranch, "created a branch in", branch+" was created"
		if s.TotalCommits > 0 {
			title += " with " + plural(s.TotalCommits, "commit")
		}
	case s.TotalCommits == 0:
		lamp, verb, title = EmojiPush, "pushed to", branch+" was pushed"
	default:
		lamp, verb, title = EmojiPush, "pushed to", plural(s.TotalCommits, "commit")+" pushed to "+branch
	}
	if s.Forced {
		verb = "force-pushed to"
	}
	o.lead(&d, s.Pusher, verb, s.Project)
	if s.Superseded && s.SupersededBy == "" {
		title = "<s>" + title + "</s>"
	}
	d.Block(htmlfmt.Heading(lamp+" "+title, 6))
	if s.Forced {
		d.Block(htmlfmt.Footer(EmojiForce + " Force push: the branch history was rewritten"))
	}
	if len(s.Commits) > 0 {
		quote := htmlfmt.Blockquote(commitList(s.Commits, s.TotalCommits, maxCommits, s.Pusher, o), false)
		d.Block(htmlfmt.Details(plural(s.TotalCommits, "commit"), quote, false))
	}
	if s.Diff != nil {
		d.Block(diffBlock(s.Diff))
	}
	var kb [][]Button
	switch {
	case s.Pipeline != nil && s.Absorbs:
		v := viewPipeline(s.Pipeline)
		d.Block(htmlfmt.Divider())
		d.Block(htmlfmt.Heading(pipelineStatusLine(s.Pipeline, v), 6))
		pipelineBody(s.Pipeline, v, o, &d)
		if a := artifactsLine(s.Pipeline); a != "" {
			d.Block(htmlfmt.Footer(a))
		}
		kb = pipelineKeyboard(s.Pipeline, v, o)
	case s.Pipeline != nil:
		sum := s.Pipeline.PipelineSummary()
		d.Block(htmlfmt.Footer(pipelineLine(&sum)))
	}
	taglineFooter(&d, "push:"+strconv.FormatInt(s.Project.ID, 10)+":"+s.Branch+":"+s.After)
	return Message{Rich: d.String(), Keyboard: kb}
}

// diffBlock is a closed fold titled `N files changed · +A −D` holding the
// git --stat block of the files, diff-highlighted.
func diffBlock(df *cards.DiffStats) string {
	if df.FilesChanged == 0 {
		return htmlfmt.Footer("No file changes")
	}
	summary := plural(df.FilesChanged, "file") + " changed · +" + strconv.Itoa(df.Added) + " −" + strconv.Itoa(df.Removed)
	if df.Partial {
		summary += " (partial)"
	}
	return htmlfmt.Details(summary, htmlfmt.Pre(diffStat(df), "diff"), false)
}

// diffStat renders the files the way git --stat does: the path clipped
// from the left and padded to one column, the change count, and a bar of
// pluses and minuses scaled to barWidth on the busiest file, then git's
// closing summary line. Lines carry no leading space: Telegram trims the
// first line of a code block, which would shift the rest.
func diffStat(df *cards.DiffStats) string {
	const barWidth, pathWidth = 10, 26
	width, busiest := 0, 0
	for _, f := range df.Files {
		if n := len([]rune(diffPath(f))); n > width {
			width = n
		}
		if n := f.Added + f.Removed; n > busiest {
			busiest = n
		}
	}
	width = min(width, pathWidth)
	var lines []string
	for _, f := range df.Files {
		total := f.Added + f.Removed
		plus, minus := f.Added, f.Removed
		if busiest > barWidth {
			plus = max(f.Added*barWidth/busiest, min(f.Added, 1))
			minus = max(f.Removed*barWidth/busiest, min(f.Removed, 1))
		}
		path := diffPath(f)
		if r := []rune(path); len(r) > pathWidth {
			path = "…" + string(r[len(r)-pathWidth+1:])
		}
		lines = append(lines, path+strings.Repeat(" ", width-len([]rune(path)))+" | "+strconv.Itoa(total)+" "+strings.Repeat("+", plus)+strings.Repeat("-", minus))
	}
	if rest := df.FilesChanged - len(df.Files); rest > 0 {
		lines = append(lines, "... "+plural(rest, "more file"))
	}
	lines = append(lines, plural(df.FilesChanged, "file")+" changed, "+plural(df.Added, "insertion")+"(+), "+plural(df.Removed, "deletion")+"(-)")
	return strings.Join(lines, "\n")
}

func diffPath(f cards.DiffFile) string {
	switch {
	case f.RenamedFrom != "":
		return f.RenamedFrom + " → " + f.Path
	case f.New:
		return f.Path + " (new)"
	}
	return f.Path
}

// BranchDeleted renders the one-shot branch deletion message.
func BranchDeleted(p *event.Push, o Options) Message {
	var d htmlfmt.Doc
	o.lead(&d, p.User, "deleted a branch in", p.Project)
	d.Block(htmlfmt.Heading(EmojiDeletedBranch+" "+htmlfmt.Code(p.Branch())+" was deleted", 6))
	taglineFooter(&d, "deleted:"+strconv.FormatInt(p.Project.ID, 10)+":"+p.Branch()+":"+p.Before)
	return Message{Rich: d.String()}
}

// maxCommitTitle keeps a commit line on one phone line with its author.
const maxCommitTitle = 48

// commitList renders up to max commits as "<sha> title - @author" lines,
// the title clipped to one line, followed by "+N more" when total exceeds
// what is shown. Push payloads carry only the author's name, so the handle
// is the pusher's when the names match and the name in italics otherwise.
func commitList(commits []event.Commit, total, max int, pusher event.User, o Options) string {
	if max <= 0 {
		max = len(commits)
	}
	shown := commits
	if len(shown) > max {
		shown = shown[:max]
	}
	lines := make([]string, 0, len(shown)+1)
	for _, c := range shown {
		l := commitLine(c, o)
		if c.Author.Name != "" {
			if c.Author.Name == pusher.Name && pusher.Username != "" {
				l += " - " + o.user(pusher)
			} else {
				l += " - " + htmlfmt.I(c.Author.Name)
			}
		}
		lines = append(lines, l)
	}
	if rest := total - len(shown); rest > 0 {
		lines = append(lines, "+"+strconv.Itoa(rest)+" more")
	}
	return strings.Join(lines, "<br/>")
}

// commitLine is the shared commit shape: a sha chip and the title clipped
// to one line, both linked to the commit.
func commitLine(c event.Commit, o Options) string {
	title := htmlfmt.Esc(clip(c.Title, maxCommitTitle))
	if c.URL != "" {
		title = htmlfmt.A(clip(c.Title, maxCommitTitle), c.URL)
	}
	return shaChip(c.SHA, c.URL) + " " + title
}

// shaChip renders a commit SHA as a 7-character code chip, linked when the
// commit URL is known.
func shaChip(sha, url string) string {
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	chip := htmlfmt.Code(short)
	if url != "" {
		return `<a href="` + htmlfmt.Esc(url) + `">` + chip + "</a>"
	}
	return chip
}

// TagPush renders a tag push.
func TagPush(t *event.TagPush, o Options) Message {
	var b htmlfmt.Builder
	tag := htmlfmt.Code(t.Tag())
	project := htmlfmt.A(t.Project.Path, t.Project.WebURL)
	who := o.user(t.User)
	if t.IsDelete() {
		b.Line(EmojiDeletedBranch + " " + who + " deleted tag " + tag + " in " + project)
		return Message{HTML: b.Truncate(o.limit(), t.Project.WebURL+"/-/tags")}
	}
	h := EmojiTag + " " + who + " pushed tag " + tag + " in " + project
	if t.CheckoutSHA != "" {
		h += " at " + htmlfmt.Code(htmlfmt.ShortSHA(t.CheckoutSHA))
	}
	b.Line(h)
	if msg := strings.TrimSpace(t.Message); msg != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(msg), o.Mentions), true)
	}
	return Message{HTML: b.Truncate(o.limit(), t.Project.WebURL+"/-/tags/"+t.Tag())}
}
