package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// maxLogButtons caps the per-failed-job log buttons on a pipeline card.
const maxLogButtons = 3

// Pipeline renders a pipeline card.
func Pipeline(s *cards.PipelineState, o Options) Message {
	status := s.EffectiveStatus()
	jobs := s.SortedJobs()
	var b htmlfmt.Builder
	b.Line(pipelineHeader(s, status))
	if meta := pipelineMeta(s, status, o); meta != "" {
		b.Line(meta)
	}
	switch o.Verbosity {
	case "quiet":
		b.Line("")
		b.Line(pipelineSummary(jobs))
	case "verbose":
		for _, st := range groupStages(jobs) {
			b.Line("")
			b.Line(stageEmoji(st.jobs) + " " + htmlfmt.B(st.name))
			for _, j := range st.jobs {
				b.Line("  " + verboseJob(j))
			}
		}
	default:
		if len(jobs) > 0 {
			b.Line("")
		}
		for _, st := range groupStages(jobs) {
			b.Line(stageRow(st))
		}
	}
	if len(s.Children) > 0 {
		b.Line("")
		if o.Verbosity == "verbose" {
			b.Line(EmojiChild + " " + htmlfmt.B("Downstream"))
			for _, c := range s.Children {
				b.Line("  " + childLine(c) + " · " + plural(c.Jobs, "job") + failedSuffix(c.Failed))
			}
		} else {
			parts := make([]string, len(s.Children))
			for i, c := range s.Children {
				parts[i] = childLine(c)
			}
			b.Line(EmojiChild + " Downstream: " + strings.Join(parts, " · "))
		}
	}
	failed := hardFailures(s.FailedJobs())
	if event.IsTerminal(status) && len(failed) > 0 {
		b.Line("")
		b.Line(htmlfmt.B("Failed:"))
		for _, j := range failed {
			l := EmojiFailed + " " + jobLink(j)
			if j.FailureReason != "" {
				l += " (" + htmlfmt.Esc(humanize(j.FailureReason)) + ")"
			}
			b.Line(l)
		}
	}
	if manual := s.ManualJobs(); event.IsBlocked(status) && len(manual) > 0 {
		b.Line("")
		word := "job"
		if len(manual) > 1 {
			word = "jobs"
		}
		b.Line(EmojiManual + " waiting for manual " + word + ": " + htmlfmt.Esc(strings.Join(manual, ", ")))
	}
	return Message{HTML: b.Truncate(o.limit(), s.URL), Keyboard: pipelineKeyboard(s, status, failed, o)}
}

func pipelineHeader(s *cards.PipelineState, status string) string {
	num := s.IID
	if num == 0 {
		num = s.ID
	}
	h := statusEmoji(status, false) + " " + htmlfmt.B("Pipeline #"+strconv.FormatInt(num, 10))
	if s.Ref != "" {
		ref := s.Ref
		if s.Tag {
			ref = EmojiTag + " " + ref
		}
		h += " · " + htmlfmt.Code(ref)
	}
	if s.Commit.SHA != "" {
		sha := htmlfmt.ShortSHA(s.Commit.SHA)
		if s.Commit.URL != "" {
			h += " · " + htmlfmt.A(sha, s.Commit.URL)
		} else {
			h += " · " + htmlfmt.Code(sha)
		}
		if s.Commit.Title != "" {
			h += " " + htmlfmt.Esc(s.Commit.Title)
		}
	}
	return h
}

func pipelineMeta(s *cards.PipelineState, status string, o Options) string {
	var parts []string
	if !s.Triggerer.IsZero() {
		parts = append(parts, EmojiUser+" "+o.user(s.Triggerer))
	}
	switch {
	case event.IsTerminal(status) && s.Duration != nil:
		parts = append(parts, "took "+htmlfmt.Dur(float64(*s.Duration)))
	case event.IsTerminal(status) && s.CreatedAt != nil && s.FinishedAt != nil:
		parts = append(parts, "took "+htmlfmt.Dur(s.FinishedAt.Sub(*s.CreatedAt).Seconds()))
	case s.CreatedAt != nil:
		parts = append(parts, "started "+o.clock(*s.CreatedAt))
	}
	if s.MR != nil {
		parts = append(parts, "for "+htmlfmt.A("!"+strconv.FormatInt(s.MR.IID, 10), s.MR.URL))
	}
	if s.Parent != nil {
		id := strconv.FormatInt(s.Parent.PipelineID, 10)
		parts = append(parts, EmojiParent+" child of "+htmlfmt.A("#"+id, s.Parent.ProjectWebURL+"/-/pipelines/"+id))
	}
	return strings.Join(parts, " · ")
}

type stageGroup struct {
	name string
	jobs []cards.JobState
}

// groupStages splits SortedJobs output into consecutive stage groups,
// preserving the sort order.
func groupStages(jobs []cards.JobState) []stageGroup {
	var out []stageGroup
	for _, j := range jobs {
		if n := len(out); n > 0 && out[n-1].name == j.Stage {
			out[n-1].jobs = append(out[n-1].jobs, j)
			continue
		}
		out = append(out, stageGroup{name: j.Stage, jobs: []cards.JobState{j}})
	}
	return out
}

// stageEmoji aggregates one stage's job statuses into a single emoji.
func stageEmoji(jobs []cards.JobState) string {
	var running, queued, failed, warned, canceled, manual, scheduled, skipped, success bool
	for _, j := range jobs {
		switch st := j.Status; {
		case st == event.StatusRunning || st == event.StatusCanceling:
			running = true
		case event.IsActive(st):
			queued = true
		case st == event.StatusFailed && !j.AllowFailure:
			failed = true
		case st == event.StatusFailed:
			warned = true
		case st == event.StatusCanceled:
			canceled = true
		case st == event.StatusManual:
			manual = true
		case st == event.StatusScheduled:
			scheduled = true
		case st == event.StatusSkipped:
			skipped = true
		default:
			success = true
		}
	}
	switch {
	case running:
		return EmojiRunning
	case queued:
		return EmojiPending
	case failed:
		return EmojiFailed
	case canceled:
		return EmojiCanceled
	case manual:
		return EmojiManual
	case scheduled:
		return EmojiScheduled
	case skipped && !success && !warned:
		return EmojiSkipped
	case warned && !success:
		return EmojiWarning
	}
	return EmojiSuccess
}

func stageRow(st stageGroup) string {
	parts := make([]string, len(st.jobs))
	for i, j := range st.jobs {
		parts[i] = statusEmoji(j.Status, j.AllowFailure) + " " + htmlfmt.Esc(j.Name)
	}
	return stageEmoji(st.jobs) + " " + htmlfmt.B(st.name) + "  " + strings.Join(parts, " · ")
}

func verboseJob(j cards.JobState) string {
	l := statusEmoji(j.Status, j.AllowFailure) + " " + jobLink(j)
	if j.Duration != nil {
		l += " · " + htmlfmt.Dur(*j.Duration)
	}
	if j.QueuedDuration != nil {
		l += " · queued " + htmlfmt.Dur(*j.QueuedDuration)
	}
	if j.Status == event.StatusFailed && j.FailureReason != "" {
		l += " · " + htmlfmt.Esc(humanize(j.FailureReason))
	}
	if j.Retries > 0 {
		l += " · retry " + strconv.Itoa(j.Retries)
	}
	return l
}

func pipelineSummary(jobs []cards.JobState) string {
	var passed, failed, warned, skipped, canceled, manual, active int
	for _, j := range jobs {
		switch st := j.Status; {
		case st == event.StatusSuccess:
			passed++
		case st == event.StatusFailed && j.AllowFailure:
			warned++
		case st == event.StatusFailed:
			failed++
		case st == event.StatusSkipped:
			skipped++
		case st == event.StatusCanceled:
			canceled++
		case event.IsBlocked(st):
			manual++
		case event.IsActive(st):
			active++
		}
	}
	parts := []string{plural(len(jobs), "job")}
	add := func(n int, emoji, label string) {
		if n > 0 {
			parts = append(parts, emoji+" "+strconv.Itoa(n)+" "+label)
		}
	}
	add(passed, EmojiSuccess, "passed")
	add(failed, EmojiFailed, "failed")
	add(warned, EmojiWarning, "allowed to fail")
	add(skipped, EmojiSkipped, "skipped")
	add(canceled, EmojiCanceled, "canceled")
	add(manual, EmojiManual, "manual")
	add(active, EmojiRunning, "in progress")
	return strings.Join(parts, " · ")
}

func childLine(c cards.ChildSummary) string {
	label := c.ProjectPath + " #" + strconv.FormatInt(c.ID, 10)
	if c.ProjectPath == "" {
		label = "#" + strconv.FormatInt(c.ID, 10)
	}
	return statusEmoji(c.Status, false) + " " + htmlfmt.A(label, c.URL)
}

func failedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return " · " + strconv.Itoa(n) + " failed"
}

// hardFailures drops allow_failure jobs from a failed job list.
func hardFailures(jobs []cards.JobState) []cards.JobState {
	var out []cards.JobState
	for _, j := range jobs {
		if !j.AllowFailure {
			out = append(out, j)
		}
	}
	return out
}

func jobLink(j cards.JobState) string {
	if j.URL == "" {
		return htmlfmt.Esc(j.Name)
	}
	return htmlfmt.A(j.Name, j.URL)
}

func pipelineKeyboard(s *cards.PipelineState, status string, failed []cards.JobState, o Options) [][]Button {
	var kb [][]Button
	row := []Button{{Text: "Pipeline", URL: s.URL}}
	if s.MR != nil && s.MR.URL != "" {
		row = append(row, Button{Text: "MR !" + strconv.FormatInt(s.MR.IID, 10), URL: s.MR.URL})
	}
	kb = append(kb, row)
	var logs []Button
	for _, j := range failed {
		if len(logs) == maxLogButtons {
			break
		}
		if j.URL != "" {
			logs = append(logs, Button{Text: j.Name + " log", URL: j.URL})
		}
	}
	if len(logs) > 0 {
		kb = append(kb, logs)
	}
	var acts []Button
	base := actions.Callback{Kind: actions.KindPipeline, ProjectID: s.Project.ID, ObjectID: s.ID}
	if event.IsActive(status) && o.can(actions.KindPipeline, actions.ActionCancel) {
		if b, ok := actionButton("Cancel", withAction(base, actions.ActionCancel)); ok {
			acts = append(acts, b)
		}
	}
	if event.IsTerminal(status) && status != event.StatusSuccess && o.can(actions.KindPipeline, actions.ActionRetry) {
		if b, ok := actionButton("Retry", withAction(base, actions.ActionRetry)); ok {
			acts = append(acts, b)
		}
	}
	if event.IsBlocked(status) && o.can(actions.KindJob, actions.ActionPlay) {
		for _, j := range s.SortedJobs() {
			if j.Status != event.StatusManual {
				continue
			}
			cb := actions.Callback{Kind: actions.KindJob, ProjectID: s.Project.ID, ObjectID: j.ID, Action: actions.ActionPlay}
			if b, ok := actionButton("▶ "+j.Name, cb); ok {
				acts = append(acts, b)
			}
		}
	}
	if len(acts) > 0 {
		kb = append(kb, acts)
	}
	return kb
}

func withAction(cb actions.Callback, a actions.Action) actions.Callback {
	cb.Action = a
	return cb
}
