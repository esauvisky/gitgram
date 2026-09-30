package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Push renders a push summary. forced marks a history rewrite detected by
// the engine's compare enrichment; maxCommits caps the listed commits.
func Push(p *event.Push, forced bool, maxCommits int, o Options) Message {
	var b htmlfmt.Builder
	branch := htmlfmt.Code(p.Branch())
	project := htmlfmt.A(p.Project.Path, p.Project.WebURL)
	who := htmlfmt.B(displayName(p.User))
	total := p.TotalCommitsCount
	if total < len(p.Commits) {
		total = len(p.Commits)
	}
	var h string
	switch {
	case p.IsDelete():
		h = EmojiDeletedBranch + " " + who + " deleted branch " + branch + " in " + project
	case p.IsCreate():
		h = EmojiNewBranch + " " + who + " created branch " + branch + " in " + project
		if total > 0 {
			h += " with " + plural(total, "commit")
		}
	case total == 0:
		h = EmojiPush + " " + who + " pushed to " + branch + " in " + project
	default:
		h = EmojiPush + " " + who + " pushed " + plural(total, "commit") + " to " + branch + " in " + project
	}
	if forced {
		h += " " + EmojiForce + " force-pushed"
	}
	b.Line(h)
	if len(p.Commits) > 0 && !p.IsDelete() {
		b.Quote(commitList(p.Commits, total, maxCommits), true)
	}

	var kb [][]Button
	more := p.Project.WebURL + "/-/commits/" + p.Branch()
	if p.Before != event.ZeroSHA && p.After != event.ZeroSHA && p.Before != "" && p.After != "" {
		compare := p.Project.WebURL + "/-/compare/" + p.Before + "..." + p.After
		kb = [][]Button{{{Text: "Compare", URL: compare}}}
		more = compare
	}
	return Message{HTML: b.Truncate(o.limit(), more), Keyboard: kb}
}

// commitList renders up to max commits as "<sha> title — author" lines,
// followed by "+N more" when total exceeds what is shown.
func commitList(commits []event.Commit, total, max int) string {
	if max <= 0 {
		max = len(commits)
	}
	shown := commits
	if len(shown) > max {
		shown = shown[:max]
	}
	lines := make([]string, 0, len(shown)+1)
	for _, c := range shown {
		sha := htmlfmt.ShortSHA(c.SHA)
		l := htmlfmt.Code(sha)
		if c.URL != "" {
			l = htmlfmt.A(sha, c.URL)
		}
		l += " " + htmlfmt.Esc(c.Title)
		if c.Author.Name != "" {
			l += " — " + htmlfmt.Esc(c.Author.Name)
		}
		lines = append(lines, l)
	}
	if rest := total - len(shown); rest > 0 {
		lines = append(lines, "+"+strconv.Itoa(rest)+" more")
	}
	return strings.Join(lines, "\n")
}

// TagPush renders a tag push.
func TagPush(t *event.TagPush, o Options) Message {
	var b htmlfmt.Builder
	tag := htmlfmt.Code(t.Tag())
	project := htmlfmt.A(t.Project.Path, t.Project.WebURL)
	who := htmlfmt.B(displayName(t.User))
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
	url := t.Project.WebURL + "/-/tags/" + t.Tag()
	return Message{
		HTML:     b.Truncate(o.limit(), url),
		Keyboard: [][]Button{{{Text: "Tag", URL: url}}},
	}
}
