package preview

import (
	"context"
	"fmt"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
)

// Mock people. None maps to a Telegram user unless the config says so.
var (
	ada   = event.User{ID: 990101, Username: "ada", Name: "Ada Lovelace"}
	linus = event.User{ID: 990102, Username: "linus", Name: "Linus"}
	grace = event.User{ID: 990103, Username: "grace", Name: "Grace Hopper"}
)

const (
	pipelineID = 991001
	mrIID      = 42
	issueIID   = 7
	sha        = "8f6ded00c0ffee1234567890abcdef1234567890"
)

var buildLog = []string{
	"$ ./gradlew assembleRelease --no-daemon",
	"> Task :app:compileReleaseKotlin",
	"w: Injector.kt:88:5 Variable 'grant' is never used",
	"> Task :app:packageRelease",
	"BUILD SUCCESSFUL in 1m 2s",
	"Uploading artifacts for successful job",
	"Uploading artifacts as \"archive\" to coordinator... 201 Created",
	"Cleaning up project directory and file based variables",
	"Job succeeded",
}

var failLog = []string{
	"$ ./gradlew testRelease --no-daemon",
	"> Task :app:testReleaseUnitTest",
	"SignalsTest > sends SSAID as grant subject PASSED",
	"CatchTest > tappable when not busy FAILED",
	"    java.lang.AssertionError: expected tappable=true but was false",
	"        at CatchTest.tappable(CatchTest.kt:42)",
	"4 tests completed, 1 failed",
	"> Task :app:testReleaseUnitTest FAILED",
	"FAILURE: Build failed with an exception.",
	"ERROR: Job failed: exit code 1",
}

func ptrI(i int) *int             { return &i }
func ptrF(f float64) *float64     { return &f }
func ptrT(t time.Time) *time.Time { return &t }

func meta() event.Meta { return event.Meta{Received: time.Now()} }

func (r *Runner) commit(title string) event.Commit {
	return event.Commit{SHA: sha, Title: title, Message: title, URL: r.proj.WebURL + "/-/commit/" + sha[:8], Author: event.User{Name: ada.Name}}
}

// commits builds one commit per title with distinct SHAs, the last one at
// sha, alternating authors so the list shows both a handle and a name.
func (r *Runner) commits(titles ...string) []event.Commit {
	out := make([]event.Commit, len(titles))
	for i, t := range titles {
		c := r.commit(t)
		if i < len(titles)-1 {
			c.SHA = fmt.Sprintf("%07x", 0xa3f1c00+i*0x9e37) + sha[7:]
			c.URL = r.proj.WebURL + "/-/commit/" + c.SHA[:8]
		}
		if i%3 == 2 {
			c.Author = event.User{Name: linus.Name}
		}
		out[i] = c
	}
	return out
}

func (r *Runner) job(id int64, name, stage, status string, dur float64) event.Job {
	j := event.Job{Meta: meta(), Project: r.proj, User: ada, ID: id, Name: name, Stage: stage, Status: status,
		URL: r.proj.WebURL + "/-/jobs/" + itoa(id), Ref: "feat/ssaid-grant", SHA: sha, PipelineID: pipelineID}
	now := time.Now()
	switch status {
	case event.StatusRunning:
		j.StartedAt = ptrT(now.Add(-30 * time.Second))
	case event.StatusSuccess, event.StatusFailed:
		j.Duration = ptrF(dur)
		j.StartedAt = ptrT(now.Add(-time.Duration(dur) * time.Second))
		j.FinishedAt = ptrT(now)
		if status == event.StatusFailed {
			j.FailureReason = "script_failure"
		}
	}
	return j
}

func (r *Runner) pipeline(status string, jobs ...event.Job) *event.Pipeline {
	p := &event.Pipeline{Meta: meta(), Project: r.proj, User: ada, ID: pipelineID, IID: 318, URL: r.proj.WebURL + "/-/pipelines/" + itoa(pipelineID),
		Ref: "feat/ssaid-grant", SHA: sha, Source: "push", Status: status, Stages: []string{"build", "test", "deploy"},
		CreatedAt: time.Now().Add(-4 * time.Minute), Commit: r.commit("signals: send the SSAID as the grant subject"),
		MR:   &event.MRRef{IID: mrIID, URL: r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID), SourceBranch: "feat/ssaid-grant", TargetBranch: "develop"},
		Jobs: jobs}
	if event.IsTerminal(status) {
		p.FinishedAt = ptrT(time.Now())
		p.Duration = ptrI(234)
	}
	return p
}

func (r *Runner) mr(action, state string, actor event.User, mut func(*event.MergeRequest)) *event.MergeRequest {
	m := &event.MergeRequest{Meta: meta(), Project: r.proj, User: actor, Action: action, ID: 9900042, IID: mrIID,
		Title: "signals: send the SSAID as the grant subject when it's known", Description: "Fixes the grant subject and bumps the injectors to 17.19.0.\n\nAlso: @grace please check the tappable fix.",
		URL: r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID), State: state, SourceBranch: "feat/ssaid-grant", TargetBranch: "develop",
		Author: ada, Reviewers: []event.Reviewer{{User: grace}, {User: linus}}, Labels: []string{"client", "release"},
		DetailedMergeStatus: "mergeable", LastCommit: r.commit("signals: send the SSAID as the grant subject"), CreatedAt: time.Now()}
	if mut != nil {
		mut(m)
	}
	return m
}

func (r *Runner) push(ref, before, after string, forced bool, commits ...event.Commit) *event.Push {
	return &event.Push{Meta: meta(), Project: r.proj, User: ada, Ref: "refs/heads/" + ref, Before: before, After: after,
		Commits: commits, TotalCommitsCount: len(commits), Forced: forced}
}

var scenarios = []Scenario{
	{Name: "push", About: "a push to a feature branch with diff stats, then a force push that supersedes it", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			if err := r.handle(ctx, r.push("feat/ssaid-grant", "1111111111111111111111111111111111111111", sha, false, r.commits(
				"signals: send the SSAID as the grant subject when it's known",
				"signals: retry the grant call once on 502",
				"injector: drop the legacy SSAID lookup path",
				"tests: cover the grant subject fallback",
				"ci: cache gradle between stages",
				"deps: bump okhttp to 4.12.0",
				"settings: expose the grant toggle in debug builds",
				"docs: describe the SSAID grant flow",
				"fix: null pusher on first sync after login",
				"refactor: split SignalClient into transport and codec",
				"chore: remove unused strings",
				"bump version to 13.5.0-001")...)); err != nil {
				return err
			}
			return r.eng.DecoratePush(ctx, r.pushKey("feat/ssaid-grant", sha), cards.DiffStats{FilesChanged: 14, Added: 412, Removed: 187, Files: []cards.DiffFile{
				{Path: "app/src/main/kotlin/SignalTest.kt", Added: 30, Removed: 12},
				{Path: "app/src/main/kotlin/Injector.kt", Added: 11, Removed: 6},
				{Path: "app/src/main/kotlin/signals/SignalClient.kt", Added: 96, Removed: 74},
				{Path: "app/src/main/kotlin/signals/SignalTransport.kt", Added: 88, New: true},
				{Path: "app/src/main/kotlin/signals/SignalCodec.kt", Added: 61, New: true},
				{Path: "app/src/main/kotlin/signals/GrantSubject.kt", Added: 24, Removed: 3},
				{Path: "app/src/main/kotlin/settings/DebugSettings.kt", Added: 18, Removed: 2},
				{Path: "app/src/main/kotlin/legacy/SsaidLookup.kt", Removed: 57, Deleted: true},
				{Path: "app/src/test/kotlin/GrantSubjectTest.kt", Added: 44, New: true},
				{Path: "app/src/main/res/values/strings.xml", Added: 2, Removed: 9},
				{Path: "docs/signals.md", RenamedFrom: "docs/ssaid.md", Added: 31, Removed: 14},
				{Path: ".gitlab-ci.yml", Added: 5, Removed: 1},
				{Path: "gradle/libs.versions.toml", Added: 1, Removed: 1},
				{Path: "gradle.properties", Added: 1}}})
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.push("feat/ssaid-grant", sha, "2222222222222222222222222222222222222222", true, r.commit("signals: squash fixups")))
		},
	}},
	{Name: "push-then-pipeline", About: "a push whose pipeline arrives afterwards and updates the push card", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.push("feat/tappable-fix", "1111111111111111111111111111111111111111", sha, false, r.commit("TOM: fix catch pokemon tappable")))
		},
		func(ctx context.Context, r *Runner) error {
			p := r.pipeline(event.StatusPending, r.job(31, "assemble", "build", event.StatusCreated, 0), r.job(32, "unit", "test", event.StatusCreated, 0))
			p.ID, p.IID, p.Ref, p.MR = pipelineID+3, 321, "feat/tappable-fix", nil
			for i := range p.Jobs {
				p.Jobs[i].PipelineID, p.Jobs[i].Ref = p.ID, "feat/tappable-fix"
			}
			return r.handle(ctx, p)
		},
		func(ctx context.Context, r *Runner) error {
			p := r.pipeline(event.StatusSuccess, r.job(31, "assemble", "build", event.StatusSuccess, 62), r.job(32, "unit", "test", event.StatusSuccess, 100))
			p.ID, p.IID, p.Ref, p.MR = pipelineID+3, 321, "feat/tappable-fix", nil
			for i := range p.Jobs {
				p.Jobs[i].PipelineID, p.Jobs[i].Ref = p.ID, "feat/tappable-fix"
			}
			return r.handle(ctx, p)
		},
	}},
	{Name: "branch", About: "a branch created, then deleted", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.push("feat/cleanup", event.ZeroSHA, sha, false, r.commit("chore: remove dead injector paths")))
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.push("feat/cleanup", sha, event.ZeroSHA, false))
		},
	}},
	{Name: "pipeline", About: "pending, running with a live tail, then failed with the failed tail and artifacts", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.pipeline(event.StatusPending,
				r.job(1, "assemble", "build", event.StatusCreated, 0), r.job(2, "unit", "test", event.StatusCreated, 0),
				r.job(3, "assemble:public", "test", event.StatusCreated, 0), r.job(4, "deploy:prod", "deploy", event.StatusCreated, 0)))
		},
		func(ctx context.Context, r *Runner) error {
			if err := r.handle(ctx, r.pipeline(event.StatusRunning,
				r.job(1, "assemble", "build", event.StatusSuccess, 62), r.job(2, "unit", "test", event.StatusRunning, 0),
				r.job(3, "assemble:public", "test", event.StatusPending, 0), r.job(4, "deploy:prod", "deploy", event.StatusCreated, 0))); err != nil {
				return err
			}
			return r.eng.Decorate(ctx, r.pipelineKey(pipelineID), map[int64]*cards.JobTail{2: {Lines: buildLog[:5], FetchedAt: time.Now()}}, nil)
		},
		func(ctx context.Context, r *Runner) error {
			if err := r.handle(ctx, r.pipeline(event.StatusFailed,
				r.job(1, "assemble", "build", event.StatusSuccess, 62), r.job(2, "unit", "test", event.StatusFailed, 100),
				r.job(3, "assemble:public", "test", event.StatusSuccess, 160), r.job(4, "deploy:prod", "deploy", event.StatusSkipped, 0))); err != nil {
				return err
			}
			return r.eng.Decorate(ctx, r.pipelineKey(pipelineID),
				map[int64]*cards.JobTail{2: {Lines: failLog, FetchedAt: time.Now(), Final: true}},
				[]cards.Artifact{{JobID: 1, JobName: "assemble", Size: 53100000}})
		},
	}},
	{Name: "pipeline-pass", About: "a pipeline that passes with an allowed failure and artifacts", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			lint := r.job(12, "lint", "build", event.StatusFailed, 20)
			lint.AllowFailure = true
			p := r.pipeline(event.StatusSuccess, r.job(11, "assemble", "build", event.StatusSuccess, 62), lint,
				r.job(13, "unit", "test", event.StatusSuccess, 100), r.job(14, "deploy:prod", "deploy", event.StatusSuccess, 30))
			p.ID, p.IID, p.MR = pipelineID+1, 319, nil
			p.Ref, p.Tag = "v13.5.0", true
			for i := range p.Jobs {
				p.Jobs[i].PipelineID = p.ID
			}
			if err := r.handle(ctx, p); err != nil {
				return err
			}
			return r.eng.Decorate(ctx, r.pipelineKey(p.ID), nil, []cards.Artifact{{JobID: 11, JobName: "assemble", Size: 53100000}, {JobID: 14, JobName: "deploy:prod", Size: 1200}})
		},
	}},
	{Name: "pipeline-manual", About: "a pipeline waiting for manual jobs", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			p := r.pipeline(event.StatusManual, r.job(21, "assemble", "build", event.StatusSuccess, 62), r.job(22, "unit", "test", event.StatusSuccess, 100),
				r.job(23, "deploy:prod", "deploy", event.StatusManual, 0), r.job(24, "deploy:cdn", "deploy", event.StatusManual, 0))
			p.ID, p.IID, p.MR = pipelineID+2, 320, nil
			for i := range p.Jobs {
				p.Jobs[i].PipelineID = p.ID
				p.Jobs[i].Manual = p.Jobs[i].Status == event.StatusManual
			}
			return r.handle(ctx, p)
		},
	}},
	{Name: "mr", About: "a merge request opened, approved by a reviewer, then merged by someone else", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.mr(event.MRActionOpen, event.MRStateOpened, ada, func(m *event.MergeRequest) {
				id := int64(pipelineID)
				m.HeadPipelineID = &id
			}))
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.mr(event.MRActionApproved, event.MRStateOpened, grace, nil))
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.mr(event.MRActionMerge, event.MRStateMerged, linus, func(m *event.MergeRequest) {
				m.MergedBy, m.MergedAt = &linus, ptrT(time.Now())
			}))
		},
	}},
	{Name: "mr-note", About: "a comment replying to the merge request card", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, &event.Note{Meta: meta(), Project: r.proj, User: grace, ID: 9900501, NoteableType: event.NoteableMergeRequest,
				Body: "Tappable fix looks right, but please keep the SSAID fallback.", URL: r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID) + "#note_9900501",
				Action: event.NoteActionCreate, IsDiff: true, FilePath: "src/catch.kt", Line: 42,
				MR: &event.MRRef{IID: mrIID, Title: "signals: send the SSAID as the grant subject when it's known", URL: r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID), TargetBranch: "develop"}})
		},
	}},
	{Name: "mr-draft", About: "a draft merge request opened, then closed", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.mr(event.MRActionOpen, event.MRStateOpened, linus, func(m *event.MergeRequest) {
				m.ID, m.IID, m.Draft, m.Title, m.Reviewers, m.Labels, m.Description = 9900043, mrIID+1, true, "prod update 0.431.0", nil, nil, "supported versions: 0.431.0"
				m.URL = r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID+1)
				m.Author = linus
			}))
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.mr(event.MRActionClose, event.MRStateClosed, ada, func(m *event.MergeRequest) {
				m.ID, m.IID, m.Draft, m.Title, m.Reviewers, m.Labels, m.Description = 9900043, mrIID+1, true, "prod update 0.431.0", nil, nil, "supported versions: 0.431.0"
				m.URL = r.proj.WebURL + "/-/merge_requests/" + itoa(mrIID+1)
				m.Author = linus
			}))
		},
	}},
	{Name: "issue", About: "an issue opened, then closed", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, &event.Issue{Meta: meta(), Project: r.proj, User: grace, Action: event.IssueActionOpen, ID: 9900700, IID: issueIID,
				Title: "Catch screen not tappable after injector restart", Description: "Repro: restart the injector while a catch is open.", URL: r.proj.WebURL + "/-/issues/" + itoa(issueIID),
				State: event.IssueStateOpened, Author: grace, Assignees: []event.User{ada}, Labels: []string{"bug"}, CreatedAt: time.Now()})
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, &event.Issue{Meta: meta(), Project: r.proj, User: ada, Action: event.IssueActionClose, ID: 9900700, IID: issueIID,
				Title: "Catch screen not tappable after injector restart", URL: r.proj.WebURL + "/-/issues/" + itoa(issueIID),
				State: event.IssueStateClosed, Author: grace, Assignees: []event.User{ada}, Labels: []string{"bug"}, ClosedAt: ptrT(time.Now())})
		},
	}},
	{Name: "tag", About: "a tag pushed", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, &event.TagPush{Meta: meta(), Project: r.proj, User: ada, Ref: "refs/tags/v13.5.0", Before: event.ZeroSHA, After: sha, CheckoutSHA: sha, Message: "13.5.0: SSAID grant subject"})
		},
	}},
	{Name: "release", About: "a release created", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, &event.Release{Meta: meta(), Project: r.proj, Action: event.ReleaseActionCreate, ID: 9900900, Name: "13.5.0", Tag: "v13.5.0",
				Description: "SSAID grant subject, injectors 17.19.0.", URL: r.proj.WebURL + "/-/releases/v13.5.0", Commit: r.commit("bump version to 13.5.0-001"),
				Links: []event.Link{{Name: "apk", URL: r.proj.WebURL + "/-/releases/v13.5.0/downloads/app.apk"}}, CreatedAt: time.Now(), ReleasedAt: time.Now()})
		},
	}},
	{Name: "deployment", About: "a deployment running, then succeeded", Steps: []Step{
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.deployment(event.DeploymentRunning))
		},
		func(ctx context.Context, r *Runner) error {
			return r.handle(ctx, r.deployment(event.DeploymentSuccess))
		},
	}},
}

func (r *Runner) deployment(status string) *event.Deployment {
	return &event.Deployment{Meta: meta(), Project: r.proj, User: ada, ID: 9901100, Status: status, StatusChangedAt: time.Now(),
		DeployableID: 4, DeployableURL: r.proj.WebURL + "/-/jobs/4", Environment: "production", EnvironmentURL: "https://app.example.com", EnvironmentTier: "production",
		Ref: "develop", Commit: r.commit("signals: send the SSAID as the grant subject")}
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
