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

// Pipeline renders a pipeline card: the title (`@emi ran a pipeline in
// demo (main)`, fixed once posted), the commit quoted like a push's commit
// row and closed by its line counts, and one line per stage that carries the result (mark, state, time,
// artifacts, the failed job's log under its stage). A pipeline that a push card
// absorbs is rendered there instead with the same stage lines.
func Pipeline(s *cards.PipelineState, o Options) Message {
	v := viewPipeline(s)
	var b htmlfmt.Builder
	pipeline := projectPage("pipeline", s.Project, "/-/pipelines")
	lead := "A " + pipeline + " ran"
	if !s.Triggerer.IsZero() {
		lead = o.who(s.Triggerer) + " ran a " + pipeline
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
		rows := []string{commitLine(s.Commit, commitAuthor(s.Commit, s.Triggerer))}
		if d := diffSummary(s.Diff); d != "" {
			rows = append(rows, "", "<i>"+d+"</i>")
		}
		b.Quote(strings.Join(rows, "\n"), false)
	}
	pipelineBody(s, v, &b)
	return Message{HTML: b.Truncate(o.limit(), s.URL), Keyboard: pipelineKeyboard(s, v, o)}
}

// pipelineBody writes, after a blank line, one line per stage in every
// state of the pipeline (mark, linked name, time once finished) and, under
// each, one small line per job with a failed job's log right under its
// line. Downstream pipelines follow.
func pipelineBody(s *cards.PipelineState, v *pipelineView, b *htmlfmt.Builder) {
	stages := groupStages(v.jobs)
	if len(stages) > 0 {
		b.Line("")
	}
	for _, st := range stages {
		b.Line(stageLine(st) + stageArtifacts(s, st))
		timed := stageDuration(st.jobs) != ""
		for _, j := range st.jobs {
			small(b, jobIndent+jobLine(j, timed))
			if t, ok := s.Tails[j.ID]; ok && j.Status == event.StatusFailed && !j.AllowFailure && len(t.Lines) > 0 {
				b.Line(htmlfmt.Pre(strings.Join(t.Lines, "\n"), "log"))
			}
		}
	}
	if len(s.Children) > 0 {
		parts := make([]string, len(s.Children))
		for i, c := range s.Children {
			parts[i] = childLine(c)
		}
		b.Line("Downstream: " + strings.Join(parts, " · "))
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
// opening (running, else failed, else the first), and how long it took
// once every job in it finished. What the jobs are doing is on their own
// lines below.
func stageLine(st stageGroup) string {
	target := st.jobs[0]
	for _, j := range st.jobs {
		if j.Status == event.StatusRunning || j.Status == event.StatusCanceling {
			target = j
			break
		}
		if j.Status == event.StatusFailed && !j.AllowFailure && target.Status != event.StatusFailed {
			target = j
		}
	}
	name := htmlfmt.Esc(sentence(st.name))
	if target.URL != "" {
		name = htmlfmt.A(sentence(st.name), target.URL)
	}
	l := stageMark(stageState(st.jobs)) + " <b>" + name + "</b>"
	if d := stageDuration(st.jobs); d != "" {
		l += " · " + d
	}
	return l
}

// jobIndent sets a job line in from its stage.
const jobIndent = "   - "

// jobLine is one job under its stage, in the small voice: its name as a
// chip, struck through once the job is done, its state in a word
// (`running...`, `queued`, `manual`, `failed (stuck or timeout failure)`)
// and, once the whole stage finished, how long it ran.
func jobLine(j cards.JobState, timed bool) string {
	name := htmlfmt.Code(j.Name)
	if j.URL != "" {
		name = `<a href="` + htmlfmt.Esc(j.URL) + `">` + name + "</a>"
	}
	if event.IsTerminal(j.Status) {
		name = "<s>" + name + "</s>"
	}
	l := name + " " + jobWord(j)
	if d := jobDuration(j); timed && d != "" {
		l += " · " + d
	}
	return l
}

// jobWord is a job's state as one lowercase word, with the failure reason
// when GitLab gives one.
func jobWord(j cards.JobState) string {
	switch j.Status {
	case event.StatusSuccess:
		return "passed"
	case event.StatusFailed:
		var notes []string
		if j.AllowFailure {
			notes = append(notes, "allowed")
		}
		if r := failureReason(j); r != "" {
			notes = append(notes, r)
		}
		if len(notes) == 0 {
			return "failed"
		}
		return "failed (" + htmlfmt.Esc(strings.Join(notes, ", ")) + ")"
	case event.StatusRunning:
		return "running..."
	case event.StatusCanceling:
		return "canceling..."
	case event.StatusCanceled:
		return "canceled"
	case event.StatusSkipped:
		return "skipped"
	case event.StatusManual:
		return "manual"
	case event.StatusScheduled:
		return "scheduled"
	case event.StatusCreated, event.StatusPending, event.StatusWaitingForResource, event.StatusPreparing, event.StatusWaitingForCallback:
		return "queued"
	}
	return htmlfmt.Esc(humanize(j.Status))
}

// jobDuration is how long a job ran, from its reported duration or its
// start and finish; empty when it never ran.
func jobDuration(j cards.JobState) string {
	switch {
	case j.Duration != nil && *j.Duration > 0:
		return htmlfmt.Dur(*j.Duration)
	case j.StartedAt != nil && j.FinishedAt != nil:
		return htmlfmt.Dur(j.FinishedAt.Sub(*j.StartedAt).Seconds())
	}
	return ""
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

// failureReason is GitLab's reason when it says something; script_failure
// is what almost every failed job reports and is left out.
func failureReason(j cards.JobState) string {
	if j.FailureReason == "" || j.FailureReason == "script_failure" {
		return ""
	}
	return humanize(j.FailureReason)
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
