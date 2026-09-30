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

// Pipeline renders a pipeline card: who ran it in which project, the
// status line (lamp, anchor, outcome, ref), the commit in small text, a
// table of stages with their
// jobs and clock times, one log fold (open on failure, closed while
// running), and a footer with the artifacts and the last update. A pipeline that a push card absorbs is rendered there instead
// with the same status line and body.
func Pipeline(s *cards.PipelineState, o Options) Message {
	v := viewPipeline(s)
	var d htmlfmt.Doc
	o.lead(&d, s.Triggerer, "ran a pipeline in", s.Project)
	d.Block(htmlfmt.Heading(pipelineStatusLine(s, v), 6))
	if s.Commit.SHA != "" || s.Commit.Title != "" {
		d.Block(htmlfmt.Footer(commitFooter(s.Commit, s.Triggerer, o)))
	}
	pipelineBody(s, v, o, &d)

	var footer []string
	if a := artifactsLine(s); a != "" {
		footer = append(footer, a)
	}
	if u := o.updated(pipelineUpdatedAt(s)); u != "" {
		footer = append(footer, u)
	}
	if len(footer) > 0 {
		d.Block(htmlfmt.Footer(strings.Join(footer, " · ")))
	}
	taglineFooter(&d, "pipeline:"+strconv.FormatInt(s.Project.ID, 10)+":"+strconv.FormatInt(s.ID, 10))
	return Message{Rich: d.String(), Keyboard: pipelineKeyboard(s, v, o)}
}

// commitFooter is the small commit line under a pipeline heading: the
// short sha linked, the title clipped, and who ran it.
func commitFooter(c event.Commit, who event.User, o Options) string {
	short := c.SHA
	if len(short) > 7 {
		short = short[:7]
	}
	sha := htmlfmt.Esc(short)
	if c.URL != "" {
		sha = htmlfmt.A(short, c.URL)
	}
	l := sha + " " + htmlfmt.Esc(clip(c.Title, maxCommitTitle))
	if !who.IsZero() {
		l += " • " + o.user(who)
	}
	return l
}

// pipelineStatusLine is `lamp Pipeline #n <outcome> on <ref>`; cards draw
// it as a level-6 heading.
func pipelineStatusLine(s *cards.PipelineState, v *pipelineView) string {
	num := s.IID
	if num == 0 {
		num = s.ID
	}
	emoji := statusEmoji(v.status, false)
	if v.status == event.StatusSuccess && v.warned {
		emoji = EmojiWarning
	}
	line := emoji + " Pipeline " + anchorText("#", num, s.URL) + " " + htmlfmt.Esc(pipelinePhrase(s, v.status, v.warned, v.jobs))
	if s.Ref != "" {
		ref := htmlfmt.Code(s.Ref)
		if s.Tag {
			ref = EmojiTag + " " + ref
		}
		line += " on " + ref
	}
	return line
}

// pipelineBody writes the stage table (or the quiet summary), the
// downstream line and the log fold.
func pipelineBody(s *cards.PipelineState, v *pipelineView, o Options, d *htmlfmt.Doc) {
	jobs := v.jobs
	switch o.Verbosity {
	case "quiet":
		if len(jobs) > 0 {
			d.P(pipelineSummary(jobs))
		}
	default:
		var rows [][]string
		for _, st := range groupStages(jobs) {
			rows = append(rows, stageRows(st)...)
		}
		if len(rows) > 0 {
			d.Block(htmlfmt.Table(rows))
		}
		if o.Verbosity == "verbose" && len(jobs) > 0 {
			var detail []string
			for _, st := range groupStages(jobs) {
				detail = append(detail, htmlfmt.B(st.name))
				for _, j := range st.jobs {
					detail = append(detail, verboseJob(j))
				}
			}
			d.Block(htmlfmt.Details("Jobs", "<p>"+strings.Join(detail, "<br/>")+"</p>", false))
		}
	}
	if len(s.Children) > 0 {
		parts := make([]string, len(s.Children))
		for i, c := range s.Children {
			parts[i] = childLine(c)
		}
		d.P(EmojiChild + " Downstream: " + strings.Join(parts, " · "))
	}
	if manual := s.ManualJobs(); o.Verbosity == "quiet" && event.IsBlocked(v.status) && len(manual) > 0 {
		d.P(EmojiManual + " Waiting for manual: " + htmlfmt.Esc(strings.Join(manual, ", ")))
	}
	logFold(s, v.status, v.failed, jobs, d)
}

// artifactsLine lists the artifact archives with download links.
func artifactsLine(s *cards.PipelineState) string {
	if len(s.Artifacts) == 0 {
		return ""
	}
	parts := make([]string, len(s.Artifacts))
	for i, a := range s.Artifacts {
		parts[i] = htmlfmt.A(a.JobName, s.Project.WebURL+"/-/jobs/"+strconv.FormatInt(a.JobID, 10)+"/artifacts/download") + " " + htmlfmt.Size(a.Size)
	}
	return EmojiArtifacts + " Artifacts: " + strings.Join(parts, " · ")
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

// pipelineUpdatedAt is the newest time the state knows about.
func pipelineUpdatedAt(s *cards.PipelineState) time.Time {
	t := s.LastPipelineEventAt
	if s.FinishedAt != nil && s.FinishedAt.After(t) {
		t = *s.FinishedAt
	}
	for _, j := range s.Jobs {
		if j.FinishedAt != nil && j.FinishedAt.After(t) {
			t = *j.FinishedAt
		}
		if j.StartedAt != nil && j.StartedAt.After(t) {
			t = *j.StartedAt
		}
	}
	return t
}

// logFold adds the one log block a card carries: the first hard-failed
// job's tail, open, once the pipeline failed; the running job's tail,
// closed, while it runs.
func logFold(s *cards.PipelineState, status string, failed, jobs []cards.JobState, d *htmlfmt.Doc) {
	if event.IsTerminal(status) {
		for _, j := range failed {
			if t, ok := s.Tails[j.ID]; ok && len(t.Lines) > 0 {
				d.Block(htmlfmt.Details("Error · last lines", htmlfmt.Pre(strings.Join(t.Lines, "\n"), "log"), true))
				return
			}
		}
		return
	}
	for _, j := range jobs {
		if j.Status != event.StatusRunning {
			continue
		}
		if t, ok := s.Tails[j.ID]; ok && len(t.Lines) > 0 {
			d.Block(htmlfmt.Details("Logs · current stage", htmlfmt.Pre(strings.Join(t.Lines, "\n"), "log"), false))
			return
		}
	}
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

// stageEmoji aggregates one stage's job statuses into a single lamp.
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

// stageRows is one table row per job: the lamp and bold stage name in the
// first column of the stage's first row only, the job in the middle, and
// its clock time right-aligned. Failed jobs come first with their reason
// when it says something; a running job is italic; skipped and canceled
// jobs are struck through with a dash for a clock.
func stageRows(st stageGroup) [][]string {
	jobs := make([]cards.JobState, 0, len(st.jobs))
	for _, j := range st.jobs {
		if j.Status == event.StatusFailed && !j.AllowFailure {
			jobs = append(jobs, j)
		}
	}
	for _, j := range st.jobs {
		if !(j.Status == event.StatusFailed && !j.AllowFailure) {
			jobs = append(jobs, j)
		}
	}
	rows := make([][]string, 0, len(jobs))
	for i, j := range jobs {
		stage := ""
		if i == 0 {
			stage = stageEmoji(st.jobs) + " " + htmlfmt.B(sentence(st.name))
		}
		rows = append(rows, []string{stage, jobFragment(j), jobClock(j)})
	}
	return rows
}

// jobClock is a job's run time once it has one; a dash when it never ran.
func jobClock(j cards.JobState) string {
	switch {
	case j.Status == event.StatusSkipped || j.Status == event.StatusCanceled:
		return "—"
	case j.Duration != nil && *j.Duration > 0:
		return htmlfmt.Clock(*j.Duration)
	}
	return ""
}

// jobFragment is one job inside a mixed stage row.
func jobFragment(j cards.JobState) string {
	name := htmlfmt.Esc(j.Name)
	if j.URL != "" {
		name = htmlfmt.A(j.Name, j.URL)
	}
	switch st := j.Status; {
	case st == event.StatusRunning || st == event.StatusCanceling:
		return "<i>" + name + "</i>"
	case st == event.StatusSkipped || st == event.StatusCanceled:
		return "<s>" + name + "</s>"
	case st == event.StatusFailed && !j.AllowFailure:
		if r := failureReason(j); r != "" {
			return name + " (" + htmlfmt.Esc(r) + ")"
		}
		return name
	case st == event.StatusFailed:
		return name + " (allowed to fail)"
	case st == event.StatusManual:
		return name + " (manual)"
	}
	return name
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
	l := statusEmoji(j.Status, j.AllowFailure) + " " + jobLink(j)
	if j.Status == event.StatusFailed || j.Status == event.StatusCanceled || j.Status == event.StatusManual || j.Status == event.StatusScheduled || j.Status == event.StatusSkipped {
		l += " " + strings.ToLower(statusWord(j.Status, j.AllowFailure))
	}
	if j.Duration != nil {
		l += " · " + htmlfmt.Clock(*j.Duration)
	}
	if j.QueuedDuration != nil {
		l += " · queued " + htmlfmt.Clock(*j.QueuedDuration)
	}
	if r := failureReason(j); j.Status == event.StatusFailed && r != "" {
		l += " · " + htmlfmt.Esc(r)
	}
	if j.Retries > 0 {
		l += " · retry " + strconv.Itoa(j.Retries)
	}
	return l
}

// pipelineSummary is the quiet mode's one line: job counts by outcome,
// words only.
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
	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+label)
		}
	}
	add(passed, "passed")
	add(failed, "failed")
	add(warned, "allowed to fail")
	add(skipped, "skipped")
	add(canceled, "canceled")
	add(manual, "manual")
	add(active, "in progress")
	return strings.Join(parts, " · ")
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

// pipelineKeyboard is the one button a card ever carries: Cancel, while
// the pipeline is active and the bot can cancel it. It disappears with the
// edit that shows the outcome.
func pipelineKeyboard(s *cards.PipelineState, v *pipelineView, o Options) [][]Button {
	if !event.IsActive(v.status) || !o.can(actions.KindPipeline, actions.ActionCancel) {
		return nil
	}
	cb := actions.Callback{Kind: actions.KindPipeline, ProjectID: s.Project.ID, ObjectID: s.ID, Action: actions.ActionCancel}
	b, ok := actionButton("Cancel", cb)
	if !ok {
		return nil
	}
	return [][]Button{{b}}
}
