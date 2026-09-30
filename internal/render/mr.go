package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// noteSnippetLen caps the body excerpt of a note folded into an MR card.
const noteSnippetLen = 120

// maxLabels caps the labels listed on a card; the rest are counted.
const maxLabels = 8

// MergeRequest renders a merge request card in the family's cadence: who
// opened, merged or closed it in which project, a heading with the state
// and the branches, the title and author in small text, one small line
// per fact (pipeline, conflicts, threads, approvals while open), the
// description, the diff stats and the comments folded, the people in
// small text, and a footer with the last change and update.
func MergeRequest(s *cards.MRState, o Options) Message {
	var d htmlfmt.Doc
	actor, verb := s.Author, "opened a merge request in"
	switch s.State {
	case event.MRStateMerged:
		verb = "merged a merge request in"
		if s.MergedBy != nil && !s.MergedBy.IsZero() {
			actor = *s.MergedBy
		}
	case event.MRStateClosed:
		verb = "closed a merge request in"
		if s.LastChange.Kind == cards.ChangeClosed && !s.LastChange.By.IsZero() {
			actor = s.LastChange.By
		}
	}
	o.lead(&d, actor, verb, s.Project)
	d.Block(htmlfmt.Heading(mrLamp(s)+" MR "+anchorText("!", s.IID, s.URL)+" "+mrHeadline(s), 6))

	title := htmlfmt.Esc(clip(s.Title, maxTitleLen))
	if s.URL != "" {
		title = htmlfmt.A(clip(s.Title, maxTitleLen), s.URL)
	}
	if s.State == event.MRStateClosed {
		title = "<s>" + title + "</s>"
	}
	if !s.Author.IsZero() {
		title += " • " + o.user(s.Author)
	}
	d.Block(htmlfmt.Footer(title))

	if p := s.HeadPipeline; p != nil {
		d.Block(htmlfmt.Footer(pipelineLine(p)))
	}
	if s.DetailedMergeStatus == "conflict" {
		d.Block(htmlfmt.Footer(EmojiConflict + " Conflicts with " + htmlfmt.Code(s.TargetBranch)))
	}
	if s.Threads.Enriched {
		if s.Threads.Unresolved > 0 {
			d.Block(htmlfmt.Footer(EmojiNote + " " + plural(s.Threads.Unresolved, "unresolved thread")))
		} else {
			d.Block(htmlfmt.Footer(EmojiNote + " Discussions resolved"))
		}
	}
	approvals := ""
	if n := len(s.Approvals.By); n > 0 || (s.Approvals.Enriched && s.Approvals.Required > 0) {
		approvals = "Approvals " + strconv.Itoa(n)
		if s.Approvals.Enriched && s.Approvals.Required > 0 {
			approvals += "/" + strconv.Itoa(s.Approvals.Required)
		}
		if n > 0 {
			approvals += " · " + o.users(s.Approvals.By)
		}
	}
	if approvals != "" && s.State == event.MRStateOpened {
		d.Block(htmlfmt.Footer(approvals))
	}

	if desc := strings.TrimSpace(s.Description); o.ShowDescription && desc != "" {
		quote := htmlfmt.Blockquote(strings.ReplaceAll(htmlfmt.RewriteMentions(htmlfmt.Esc(desc), o.Mentions), "\n", "<br/>"), false)
		d.Block(htmlfmt.Details("Description", quote, false))
	}
	if s.Diff != nil {
		d.Block(diffBlock(s.Diff))
	}
	if len(s.Notes) > 0 {
		noteLines := make([]string, len(s.Notes))
		for i, n := range s.Notes {
			noteLines[i] = noteLine(n, o)
		}
		d.Block(htmlfmt.Details(plural(len(s.Notes), "comment"), "<p>"+strings.Join(noteLines, "<br/>")+"</p>", false))
	}

	var people []string
	if approvals != "" && s.State != event.MRStateOpened {
		people = append(people, approvals)
	}
	if len(s.Reviewers) > 0 {
		rs := make([]string, len(s.Reviewers))
		for i, r := range s.Reviewers {
			rs[i] = o.user(r)
			if approvedBy(s.Approvals.By, r) {
				rs[i] += " (approved)"
			}
		}
		people = append(people, "Reviewers "+strings.Join(rs, ", "))
	}
	if len(s.Assignees) > 0 {
		people = append(people, "Assignees "+o.users(s.Assignees))
	}
	if len(s.Labels) > 0 {
		shown := s.Labels
		if len(shown) > maxLabels {
			shown = shown[:maxLabels]
		}
		l := "Labels " + htmlfmt.Esc(strings.Join(shown, ", "))
		if rest := len(s.Labels) - len(shown); rest > 0 {
			l += " +" + strconv.Itoa(rest)
		}
		people = append(people, l)
	}
	if len(people) > 0 {
		d.Block(htmlfmt.Footer(strings.Join(people, " · ")))
	}
	var footer []string
	if f := changeFooter(s.LastChange, o); f != "" {
		footer = append(footer, f)
	}
	if u := o.updated(s.LastEventAt); u != "" {
		footer = append(footer, u)
	}
	if len(footer) > 0 {
		d.Block(htmlfmt.Footer(strings.Join(footer, " · ")))
	}
	taglineFooter(&d, "mr:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.IID, 10))
	return Message{Rich: d.String()}
}

// mrHeadline is the state and branches after the anchor: `opened:
// <src> → <tgt>` (`draft:` for drafts), `merged into <tgt>`, `closed`.
func mrHeadline(s *cards.MRState) string {
	branches := htmlfmt.Code(s.SourceBranch) + " → " + htmlfmt.Code(s.TargetBranch)
	switch s.State {
	case event.MRStateMerged:
		return "merged into " + htmlfmt.Code(s.TargetBranch)
	case event.MRStateClosed:
		return "closed"
	}
	if s.Draft {
		return "draft: " + branches
	}
	return "opened: " + branches
}

// pipelineLine is the pipeline fact line shared by MR and push cards:
// `lamp Pipeline #n word [· N failed] [· N manual]`.
func pipelineLine(p *cards.PipelineSummary) string {
	num := p.IID
	if num == 0 {
		num = p.ID
	}
	l := statusEmoji(p.Status, false) + " Pipeline " + anchorText("#", num, p.URL) + " " + strings.ToLower(statusWord(p.Status, false))
	if event.IsActive(p.Status) && p.Status != event.StatusRunning {
		l = EmojiPending + " Pipeline " + anchorText("#", num, p.URL) + " queued"
	}
	if p.Failed > 0 && p.Status != event.StatusFailed {
		l += " · " + strconv.Itoa(p.Failed) + " failed"
	}
	if p.Manual > 0 && p.Status != event.StatusManual {
		l += " · " + strconv.Itoa(p.Manual) + " manual"
	}
	return l
}

// mrLamp is the state emoji; a draft shows the draft lamp instead of the
// open one.
func mrLamp(s *cards.MRState) string {
	if s.Draft && s.State == event.MRStateOpened {
		return EmojiMRDraft
	}
	return mrStateEmoji(s.State)
}

func sameUser(a, b event.User) bool {
	if a.ID != 0 || b.ID != 0 {
		return a.ID == b.ID
	}
	return a.Username == b.Username
}

func approvedBy(by []event.User, u event.User) bool {
	for _, a := range by {
		if sameUser(a, u) {
			return true
		}
	}
	return false
}

func noteLine(n cards.NoteSummary, o Options) string {
	body := strings.TrimSpace(n.Body)
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[:i] + " …"
	}
	if r := []rune(body); len(r) > noteSnippetLen {
		body = string(r[:noteSnippetLen]) + "…"
	}
	l := htmlfmt.B(displayName(n.Author)) + ": "
	if n.IsDiff && n.FilePath != "" {
		l += htmlfmt.Code(fileRef(n.FilePath, n.Line)) + " "
	}
	if n.URL != "" {
		return l + htmlfmt.A(body, n.URL)
	}
	return l + htmlfmt.Esc(body)
}

// fileRef formats a diff position as path:line (path alone when line is 0).
func fileRef(path string, line int) string {
	if line == 0 {
		return path
	}
	return path + ":" + strconv.Itoa(line)
}

// changeFooter renders "<what> by <who>" for changes that no other card
// line already conveys.
func changeFooter(c cards.Change, o Options) string {
	switch c.Kind {
	case "", cards.ChangeOpened, cards.ChangeMerged, cards.ChangeClosed:
		return ""
	}
	l := changeText(c.Kind)
	if !c.By.IsZero() {
		l += " by " + o.user(c.By)
	}
	return l
}

func changeText(k cards.ChangeKind) string {
	switch k {
	case cards.ChangeReopened:
		return "reopened"
	case cards.ChangeApproved:
		return "approved"
	case cards.ChangeUnapproved:
		return "approval revoked"
	case cards.ChangeDraft:
		return "marked as draft"
	case cards.ChangeReady:
		return "marked as ready"
	case cards.ChangeTitle:
		return "title changed"
	case cards.ChangeDescription:
		return "description edited"
	case cards.ChangeLabels:
		return "labels changed"
	case cards.ChangeAssignees:
		return "assignees changed"
	case cards.ChangeReviewers:
		return "reviewers changed"
	case cards.ChangeThreadsResolved:
		return "all threads resolved"
	case cards.ChangeTargetBranch:
		return "target branch changed"
	case cards.ChangeMilestone:
		return "milestone changed"
	case cards.ChangeConfidential:
		return "confidentiality changed"
	case cards.ChangeDueDate:
		return "due date changed"
	}
	return htmlfmt.Esc(humanize(string(k)))
}
