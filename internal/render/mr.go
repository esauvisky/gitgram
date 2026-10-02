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

// MergeRequest renders a merge request card: the title (`@ada opened MR
// !42 in demo (feat/x → develop)`, fixed once posted), the MR title in
// small text, who merged or closed it, the diff stats, one small line per
// fact (pipeline, conflicts, threads, approvals while open), the
// description and the comments as folds, and small lines for the people
// and the last change.
func MergeRequest(s *cards.MRState, o Options) Message {
	var b htmlfmt.Builder
	kind := " opened MR "
	if s.Draft {
		kind = " opened draft MR "
	}
	headline(&b, o.who(s.Author)+kind+anchorText("!", s.IID, s.URL), "in", s.Project,
		branchRef(s.Project, s.SourceBranch)+" → "+branchRef(s.Project, s.TargetBranch))

	title := htmlfmt.Esc(clip(s.Title, maxTitleLen))
	if s.URL != "" {
		title = htmlfmt.A(clip(s.Title, maxTitleLen), s.URL)
	}
	if s.State == event.MRStateClosed {
		title = "<s>" + title + "</s>"
	}
	small(&b, title)
	switch s.State {
	case event.MRStateMerged:
		l := "Merged"
		if s.MergedBy != nil && !s.MergedBy.IsZero() {
			l += " by " + o.user(*s.MergedBy)
		}
		small(&b, l+" into "+htmlfmt.Code(s.TargetBranch))
	case event.MRStateClosed:
		l := "Closed"
		if s.LastChange.Kind == cards.ChangeClosed && !s.LastChange.By.IsZero() {
			l += " by " + o.user(s.LastChange.By)
		}
		small(&b, l)
	}
	if d := diffSummary(s.Diff); d != "" {
		small(&b, d)
	}
	if p := s.HeadPipeline; p != nil {
		small(&b, pipelineLine(p))
	}
	if s.DetailedMergeStatus == "conflict" {
		small(&b, "Conflicts with "+htmlfmt.Code(s.TargetBranch))
	}
	if s.Threads.Enriched {
		if s.Threads.Unresolved > 0 {
			small(&b, plural(s.Threads.Unresolved, "unresolved thread"))
		} else {
			small(&b, "Discussions resolved")
		}
	}
	approvals := ""
	if n := len(s.Approvals.By); n > 0 || (s.Approvals.Enriched && s.Approvals.Required > 0) {
		approvals = "Approvals " + strconv.Itoa(n)
		if s.Approvals.Enriched && s.Approvals.Required > 0 {
			approvals += "/" + strconv.Itoa(s.Approvals.Required)
		}
		if n > 0 && s.State == event.MRStateOpened {
			approvals += " · " + o.users(s.Approvals.By)
		}
	}
	if approvals != "" && s.State == event.MRStateOpened {
		small(&b, approvals)
	}

	if desc := strings.TrimSpace(s.Description); o.ShowDescription && desc != "" {
		fold(&b, "Description", htmlfmt.RewriteMentions(htmlfmt.Esc(desc)))
	}
	if len(s.Notes) > 0 {
		noteLines := make([]string, len(s.Notes))
		for i, n := range s.Notes {
			noteLines[i] = noteLine(n, o)
		}
		fold(&b, plural(len(s.Notes), "comment"), strings.Join(noteLines, "\n"))
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
	small(&b, strings.Join(people, " · "))
	small(&b, changeFooter(s.LastChange, o))
	taglineFooter(&b, "mr:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.IID, 10))
	return Message{HTML: b.Truncate(o.limit(), s.URL)}
}

// pipelineLine is the pipeline fact line shared by MR and push cards:
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
	l := o.user(n.Author) + ": "
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
