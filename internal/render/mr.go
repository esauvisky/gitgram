package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// MergeRequest renders a merge request card: the title (`@ada opened MR !42
// in demo (feat/x → develop)`, fixed once posted), the MR title in bold
// and linked (struck through once closed), the description and line counts
// in a quote, a small line for unresolved threads, the head pipeline's stage
// lines after a blank line, and last, in bold after a blank line, where it
// stands: `Merged into develop by @linus`, `Closed by @ada`, or while open
// `Draft` and `⚠️ Conflicts with develop`, followed by a small line once
// the source branch was deleted. The buttons are Merge (with a confirmation) while
// GitLab reports the MR as mergeable, and the pipeline's Stop, Retry and
// Run.
func MergeRequest(s *cards.MRState, o Options) Message {
	var b htmlfmt.Builder
	headline(&b, o.who(s.Author)+" opened MR !"+strconv.FormatInt(s.IID, 10), "in", s.Project,
		branchRef(s.Project, s.SourceBranch)+" → "+branchRef(s.Project, s.TargetBranch))

	title := htmlfmt.Esc(s.Title)
	if s.URL != "" {
		title = htmlfmt.A(s.Title, s.URL)
	}
	if s.State == event.MRStateClosed {
		title = "<s>" + title + "</s>"
	}
	b.Line("<b>" + title + "</b>")

	desc := htmlfmt.Esc(strings.TrimSpace(s.Description))
	if d := diffSummary(s.Diff); d != "" {
		if desc != "" {
			desc += "\n\n"
		}
		desc += "<i>" + d + "</i>"
	}
	if desc != "" {
		b.Quote(desc)
	}
	if s.State == event.MRStateOpened && s.Threads && s.UnresolvedThreads > 0 {
		small(&b, plural(s.UnresolvedThreads, "unresolved thread"))
	}

	var kb [][]Button
	if row := mergeButtons(s, o); row != nil {
		kb = append(kb, row)
	}
	if s.Pipeline != nil {
		v := viewPipeline(s.Pipeline)
		pipelineBody(s.Pipeline, v, &b)
		kb = append(kb, pipelineKeyboard(s.Pipeline, v, o)...)
	}

	var standing []string
	switch {
	case s.State == event.MRStateMerged:
		l := "Merged into " + htmlfmt.Code(s.TargetBranch)
		if s.MergedBy != nil {
			l += " by " + o.handle(*s.MergedBy)
		}
		standing = append(standing, l)
	case s.State == event.MRStateClosed:
		l := "Closed"
		if s.ClosedBy != nil {
			l += " by " + o.handle(*s.ClosedBy)
		}
		standing = append(standing, l)
	default:
		if s.Draft {
			standing = append(standing, "Draft")
		}
		if s.DetailedMergeStatus == event.MergeStatusConflict {
			standing = append(standing, "⚠️ Conflicts with "+htmlfmt.Code(s.TargetBranch))
		}
	}
	if len(standing) > 0 || s.SourceBranchDeleted {
		b.Line("")
	}
	if len(standing) > 0 {
		b.Line("<b>" + strings.Join(standing, " · ") + "</b>")
	}
	if s.SourceBranchDeleted {
		small(&b, "The branch "+htmlfmt.Code(s.SourceBranch)+" was deleted.")
	}
	taglineFooter(&b, "mr:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.IID, 10))
	return Message{HTML: b.Truncate(o.limit(), s.URL), Keyboard: kb}
}

// mergeButtons is `Merge` while the MR is open, ready and mergeable,
// swapped for `Yes, merge it` and `Keep open` once someone pressed it.
func mergeButtons(s *cards.MRState, o Options) []Button {
	if s.State != event.MRStateOpened || s.Draft || s.DetailedMergeStatus != event.MergeStatusMergeable || !o.can(actions.KindMergeRequest, actions.ActionMerge) {
		return nil
	}
	button := func(text string, a actions.Action) []Button {
		cb := actions.Callback{Kind: actions.KindMergeRequest, ProjectID: s.Project.ID, ObjectID: s.IID, Action: a}
		if b, ok := actionButton(text, cb); ok {
			return []Button{b}
		}
		return nil
	}
	if s.ConfirmMerge {
		return append(button("Yes, merge it", actions.ActionMergeYes), button("Keep open", actions.ActionMergeNo)...)
	}
	return button("Merge", actions.ActionMerge)
}
