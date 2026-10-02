package render

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// maxTitleLen caps the commit title on the title line at the git subject
// convention so a long title never buries the card.
const maxTitleLen = 72

// pipelineView is what every pipeline block reads from the state once.
type pipelineView struct {
	status string
	jobs   []cards.JobState
	failed []cards.JobState
	warned bool
}

func viewPipeline(s *cards.PipelineState) *pipelineView {
	all := s.FailedJobs()
	failed := hardFailures(all)
	return &pipelineView{status: s.EffectiveStatus(), jobs: s.SortedJobs(), failed: failed, warned: len(all) > len(failed)}
}

// Pipeline renders a pipeline card: the title (`@emi ran pipeline #84 in
// demo (main)`, fixed once posted), the commit quoted like a push's commit
// row, and one line per stage that carries the result (mark, state, time,
// artifacts, the failed job's log under its stage). A pipeline that a push card
// absorbs is rendered there instead with the same stage lines.
func Pipeline(s *cards.PipelineState, o Options) Message {
	v := viewPipeline(s)
	var b htmlfmt.Builder
	lead := "Pipeline " + pipelineAnchor(s) + " ran"
	if !s.Triggerer.IsZero() {
		lead = o.who(s.Triggerer) + " ran pipeline " + pipelineAnchor(s)
	}
	ref := ""
	if s.Ref != "" {
		ref = branchRef(s.Project, s.Ref)
		if s.Tag {
			ref = tagRef(s.Project, s.Ref)
		}
	}
	headline(&b, lead, "in", s.Project, ref)
	if s.Commit.SHA != "" || s.Commit.Title != "" {
		b.Quote(commitLine(s.Commit, commitAuthor(s.Commit, s.Triggerer)), false)
	}
	pipelineBody(s, v, o, &b)
	taglineFooter(&b, "pipeline:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.ID, 10))
	return Message{HTML: b.Truncate(o.limit(), s.URL), Keyboard: pipelineKeyboard(s, v, o)}
}

// pipelineAnchor is `#n`, linked to the pipeline.
func pipelineAnchor(s *cards.PipelineState) string {
	num := s.IID
	if num == 0 {
		num = s.ID
	}
	return anchorText("#", num, s.URL)
}

// pipelineBody writes, after a blank line, one line per stage in every
// state of the pipeline:
// mark, linked name, state, time once finished, and the failed job's log
// right under a failed stage. Downstream pipelines and the verbose Jobs
// fold follow.
func pipelineBody(s *cards.PipelineState, v *pipelineView, o Options, b *htmlfmt.Builder) {
	stages := groupStages(v.jobs)
	if len(stages) > 0 {
		b.Line("")
	}
	for _, st := range stages {
		line, logJob := stageLine(st)
		b.Line(line + stageArtifacts(s, st))
		if t, ok := s.Tails[logJob]; ok && logJob != 0 && len(t.Lines) > 0 {
			b.Line(htmlfmt.Pre(strings.Join(t.Lines, "\n"), "log"))
		}
	}
	if len(s.Children) > 0 {
		parts := make([]string, len(s.Children))
		for i, c := range s.Children {
			parts[i] = childLine(c)
		}
		b.Line("Downstream: " + strings.Join(parts, " · "))
	}
	if o.Verbosity == "verbose" && len(v.jobs) > 0 {
		var detail []string
		for _, st := range stages {
			detail = append(detail, htmlfmt.B(st.name))
			for _, j := range st.jobs {
				detail = append(detail, verboseJob(j))
			}
		}
		fold(b, "Jobs", strings.Join(detail, "\n"))
	}
}

// stageArtifacts is the artifact archives of a stage's jobs, appended to
// its line in italics with download links: ` (artifacts, 88.7 MB)`, or each named by
// job when the stage has several (` (build artifacts, 88.7 MB · lint
// artifacts, 1 KB)`). Empty when the stage has none.
func stageArtifacts(s *cards.PipelineState, st stageGroup) string {
	var arts []cards.Artifact
	for _, a := range s.Artifacts {
		for _, j := range st.jobs {
			if j.ID == a.JobID {
				arts = append(arts, a)
			}
		}
	}
	if len(arts) == 0 {
		return ""
	}
	parts := make([]string, len(arts))
	for i, a := range arts {
		label := "artifacts"
		if len(arts) > 1 {
			label = a.JobName + " artifacts"
		}
		parts[i] = htmlfmt.A(label, s.Project.WebURL+"/-/jobs/"+strconv.FormatInt(a.JobID, 10)+"/artifacts/download") + ", " + htmlfmt.Size(a.Size)
	}
	return " <i>(" + strings.Join(parts, " · ") + ")</i>"
}

// pipelinePhrase is the outcome as a lowercase verb phrase: failed,
// passed, running, waiting for manual, queued, canceled, skipped.
func pipelinePhrase(s *cards.PipelineState, status string, warned bool, jobs []cards.JobState) string {
	switch {
	case status == event.StatusFailed:
		return "failed"
	case status == event.StatusSuccess && warned:
		return "passed with warnings"
	case status == event.StatusSuccess:
		return "passed"
	case status == event.StatusRunning || status == event.StatusCanceling:
		return "running"
	case event.IsBlocked(status):
		return "waiting for manual"
	case event.IsActive(status):
		return "queued"
	}
	return strings.ToLower(statusWord(status, false))
}

// sentence capitalises the first letter of a phrase.
func sentence(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// anchorText renders the card's own anchor, linked when the URL is known.
func anchorText(prefix string, num int64, url string) string {
	text := prefix + strconv.FormatInt(num, 10)
	if url == "" {
		return htmlfmt.Esc(text)
	}
	return htmlfmt.A(text, url)
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

// stageState aggregates one stage's job statuses into one state word:
// running, queued, failed, canceled, manual, scheduled, skipped, warned or
// passed.
func stageState(jobs []cards.JobState) string {
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
		return "running"
	case queued:
		return "queued"
	case failed:
		return "failed"
	case canceled:
		return "canceled"
	case manual:
		return "manual"
	case scheduled:
		return "scheduled"
	case skipped && !success && !warned:
		return "skipped"
	case warned && !success:
		return "warned"
	}
	return "passed"
}

// stageMark is the emoji before a stage for its state.
func stageMark(state string) string {
	return stageMarks[state]
}

// stageLine is one stage: its mark and bold name, linked to the job worth
// opening (running, else failed, else the first), then what it is doing:
// `running job x...`, `x failed`, `waiting for x`, `waiting...`, and how
// long it took once it finished. logJob is the hard-failed job whose log
// belongs under the line, 0 for none: logs show only on failure.
func stageLine(st stageGroup) (line string, logJob int64) {
	var running, failed, manual []cards.JobState
	for _, j := range st.jobs {
		switch {
		case j.Status == event.StatusRunning || j.Status == event.StatusCanceling:
			running = append(running, j)
		case j.Status == event.StatusFailed && !j.AllowFailure:
			failed = append(failed, j)
		case j.Status == event.StatusManual:
			manual = append(manual, j)
		}
	}
	target := st.jobs[0]
	switch {
	case len(running) > 0:
		target = running[0]
	case len(failed) > 0:
		target = failed[0]
	}
	name := htmlfmt.Esc(sentence(st.name))
	if target.URL != "" {
		name = htmlfmt.A(sentence(st.name), target.URL)
	}
	state := stageState(st.jobs)
	l := stageMark(state) + " <b>" + name + "</b>"
	switch {
	case len(running) > 0:
		word := "job"
		if len(running) > 1 {
			word = "jobs"
		}
		l += ": running " + word + " " + jobNames(running) + "..."
	case len(failed) > 0:
		l += ": " + jobNames(failed) + " failed"
		if r := failureReason(failed[0]); r != "" {
			l += " (" + htmlfmt.Esc(r) + ")"
		}
		logJob = failed[0].ID
	case len(manual) > 0:
		l += ": waiting for " + jobNames(manual)
	case state == "queued":
		l += ": waiting..."
	case state == "skipped", state == "canceled", state == "scheduled":
		l += ": " + state
	case state == "warned":
		l += ": passed with warnings"
	}
	if d := stageDuration(st.jobs); d != "" {
		l += " · " + d
	}
	return l, logJob
}

// stageDuration is the stage's wall time, first start to last finish, once
// every job in it has finished; empty while any job is still to run or
// when it never ran.
func stageDuration(jobs []cards.JobState) string {
	var first, last *time.Time
	for _, j := range jobs {
		if !event.IsTerminal(j.Status) {
			return ""
		}
		if j.StartedAt != nil && (first == nil || j.StartedAt.Before(*first)) {
			first = j.StartedAt
		}
		if j.FinishedAt != nil && (last == nil || j.FinishedAt.After(*last)) {
			last = j.FinishedAt
		}
	}
	if first == nil || last == nil {
		return ""
	}
	return htmlfmt.Dur(last.Sub(*first).Seconds())
}

// jobNames lists jobs as code chips.
func jobNames(jobs []cards.JobState) string {
	names := make([]string, len(jobs))
	for i, j := range jobs {
		names[i] = htmlfmt.Code(j.Name)
	}
	return strings.Join(names, ", ")
}

// failureReason is GitLab's reason when it says something; script_failure
// is what almost every failed job reports and is left out.
func failureReason(j cards.JobState) string {
	if j.FailureReason == "" || j.FailureReason == "script_failure" {
		return ""
	}
	return humanize(j.FailureReason)
}

func verboseJob(j cards.JobState) string {
	l := jobLink(j) + " " + strings.ToLower(statusWord(j.Status, j.AllowFailure))
	if j.Duration != nil {
		l += " · " + htmlfmt.Dur(*j.Duration)
	}
	if j.QueuedDuration != nil {
		l += " · queued " + htmlfmt.Dur(*j.QueuedDuration)
	}
	if r := failureReason(j); j.Status == event.StatusFailed && r != "" {
		l += " · " + htmlfmt.Esc(r)
	}
	if j.Retries > 0 {
		l += " · retry " + strconv.Itoa(j.Retries)
	}
	return l
}

// childLine names a downstream pipeline with its status as a word.
func childLine(c cards.ChildSummary) string {
	label := "#" + strconv.FormatInt(c.ID, 10)
	if c.ProjectPath != "" {
		label = c.ProjectPath[strings.LastIndexByte(c.ProjectPath, '/')+1:] + " " + label
	}
	l := htmlfmt.A(label, c.URL) + " " + strings.ToLower(statusWord(c.Status, false))
	if c.Failed > 0 {
		l += " (" + strconv.Itoa(c.Failed) + " failed)"
	}
	return l
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

// pipelineKeyboard is the card's buttons: `Run <job>` for each manual job
// waiting to be started (up to maxRunButtons), then `Stop pipeline` while
// the pipeline is active, swapped for `Yes, stop it` and `Keep running`
// once someone pressed it; `Retry` once it failed.
// Each needs the bot to be able to write to GitLab.
func pipelineKeyboard(s *cards.PipelineState, v *pipelineView, o Options) [][]Button {
	base := actions.Callback{Kind: actions.KindPipeline, ProjectID: s.Project.ID, ObjectID: s.ID}
	button := func(text string, a actions.Action) []Button {
		cb := base
		cb.Action = a
		if b, ok := actionButton(text, cb); ok {
			return []Button{b}
		}
		return nil
	}
	var kb [][]Button
	if o.can(actions.KindJob, actions.ActionPlay) {
		var runs []Button
		for _, j := range v.jobs {
			if j.Status != event.StatusManual || len(runs) == maxRunButtons {
				continue
			}
			cb := actions.Callback{Kind: actions.KindJob, ProjectID: s.Project.ID, ObjectID: j.ID, Action: actions.ActionPlay}
			if b, ok := actionButton("Run "+j.Name, cb); ok {
				runs = append(runs, b)
			}
		}
		if len(runs) > 0 {
			kb = append(kb, runs)
		}
	}
	var row []Button
	switch {
	case event.IsActive(v.status) && o.can(actions.KindPipeline, actions.ActionCancel):
		if s.ConfirmStop {
			row = append(button("Yes, stop it", actions.ActionCancelYes), button("Keep running", actions.ActionCancelNo)...)
		} else {
			row = button("Stop pipeline", actions.ActionCancel)
		}
	case v.status == event.StatusFailed && o.can(actions.KindPipeline, actions.ActionRetry):
		row = button("Retry", actions.ActionRetry)
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}
	return kb
}

// maxRunButtons caps the Run buttons for manual jobs on one card.
const maxRunButtons = 3
