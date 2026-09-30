package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// noteSnippetLen caps the body excerpt of a note folded into an MR card.
const noteSnippetLen = 120

// MergeRequest renders a merge request card.
func MergeRequest(s *cards.MRState, o Options) Message {
	var b htmlfmt.Builder
	h := mrStateEmoji(s.State) + " "
	if s.Draft && s.State == event.MRStateOpened {
		h += EmojiMRDraft + " "
	}
	b.Line(h + htmlfmt.B("!"+strconv.FormatInt(s.IID, 10)+" "+s.Title))
	b.Line(htmlfmt.Code(s.SourceBranch) + " → " + htmlfmt.Code(s.TargetBranch) + " · by " + o.user(s.Author))
	if o.ShowDescription && strings.TrimSpace(s.Description) != "" {
		b.Quote(htmlfmt.RewriteMentions(htmlfmt.Esc(strings.TrimSpace(s.Description)), o.Mentions), true)
	}

	var people []string
	if len(s.Assignees) > 0 {
		people = append(people, "Assignees: "+o.users(s.Assignees))
	}
	if len(s.Reviewers) > 0 {
		rs := make([]string, len(s.Reviewers))
		for i, r := range s.Reviewers {
			mark := EmojiPending
			if approvedBy(s.Approvals.By, r) {
				mark = EmojiSuccess
			}
			rs[i] = o.user(r) + " " + mark
		}
		people = append(people, "Reviewers: "+strings.Join(rs, ", "))
	}
	if len(people) > 0 {
		b.Line(EmojiPeople + " " + strings.Join(people, " · "))
	}
	if len(s.Labels) > 0 {
		b.Line(EmojiLabel + " " + htmlfmt.Esc(strings.Join(s.Labels, ", ")))
	}
	if n := len(s.Approvals.By); n > 0 || (s.Approvals.Enriched && s.Approvals.Required > 0) {
		l := EmojiApprove + " Approvals " + strconv.Itoa(n)
		if s.Approvals.Enriched && s.Approvals.Required > 0 {
			l += "/" + strconv.Itoa(s.Approvals.Required)
		}
		if n > 0 {
			l += " (" + o.users(s.Approvals.By) + ")"
		}
		b.Line(l)
	}
	if s.Threads.Enriched && s.Threads.Unresolved > 0 {
		b.Line(EmojiNote + " " + plural(s.Threads.Unresolved, "unresolved thread"))
	}
	if p := s.HeadPipeline; p != nil {
		l := statusEmoji(p.Status, false) + " " + htmlfmt.A("Pipeline #"+strconv.FormatInt(p.ID, 10), p.URL) + " " + htmlfmt.Esc(humanize(p.Status))
		if p.Failed > 0 {
			l += " · " + strconv.Itoa(p.Failed) + " failed"
		}
		if p.Manual > 0 {
			l += " · " + strconv.Itoa(p.Manual) + " manual"
		}
		b.Line(l)
	}
	if s.DetailedMergeStatus == "conflict" {
		b.Line(EmojiConflict + " conflicts with " + htmlfmt.Code(s.TargetBranch))
	}
	if len(s.Notes) > 0 {
		lines := make([]string, len(s.Notes))
		for i, n := range s.Notes {
			lines[i] = noteLine(n, o)
		}
		b.Line("")
		b.Quote(strings.Join(lines, "\n"), true)
	}

	switch s.State {
	case event.MRStateMerged:
		l := EmojiMRMerged + " Merged"
		if s.MergedBy != nil && !s.MergedBy.IsZero() {
			l += " by " + o.user(*s.MergedBy)
		}
		b.Line("")
		b.Line(l)
	case event.MRStateClosed:
		l := EmojiMRClosed + " Closed"
		if s.LastChange.Kind == cards.ChangeClosed && !s.LastChange.By.IsZero() {
			l += " by " + o.user(s.LastChange.By)
		}
		b.Line("")
		b.Line(l)
	}
	if f := changeFooter(s.LastChange, o); f != "" {
		b.Line(f)
	}
	return Message{HTML: b.Truncate(o.limit(), s.URL), Keyboard: mrKeyboard(s, o)}
}

func approvedBy(by []event.User, u event.User) bool {
	for _, a := range by {
		if a.ID != 0 || u.ID != 0 {
			if a.ID == u.ID {
				return true
			}
			continue
		}
		if a.Username == u.Username {
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
	l := EmojiNote + " " + htmlfmt.B(displayName(n.Author)) + ": "
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

// changeFooter renders "✏️ <what> by <who>" for changes that no other card
// line already conveys.
func changeFooter(c cards.Change, o Options) string {
	switch c.Kind {
	case "", cards.ChangeOpened, cards.ChangeMerged, cards.ChangeClosed:
		return ""
	}
	l := EmojiEdit + " " + changeText(c.Kind)
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

func mrKeyboard(s *cards.MRState, o Options) [][]Button {
	row := []Button{{Text: "MR", URL: s.URL}}
	if s.HeadPipeline != nil && s.HeadPipeline.URL != "" {
		row = append(row, Button{Text: "Pipeline", URL: s.HeadPipeline.URL})
	}
	row = append(row, Button{Text: "Changes", URL: s.URL + "/diffs"})
	kb := [][]Button{row}
	if s.State != event.MRStateOpened {
		return kb
	}
	var acts []Button
	base := actions.Callback{Kind: actions.KindMergeRequest, ProjectID: s.Project.ID, ObjectID: s.IID}
	if o.can(actions.KindMergeRequest, actions.ActionApprove) {
		if b, ok := actionButton("Approve", withAction(base, actions.ActionApprove)); ok {
			acts = append(acts, b)
		}
	}
	if o.can(actions.KindMergeRequest, actions.ActionMerge) {
		if b, ok := actionButton("Merge", withAction(base, actions.ActionMerge)); ok {
			acts = append(acts, b)
		}
	}
	if len(acts) > 0 {
		kb = append(kb, acts)
	}
	return kb
}
