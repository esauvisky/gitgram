package engine

import (
	"context"
	"errors"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/store"
)

// prefetchTimeout bounds the REST calls made before the transaction so a
// slow GitLab API never pushes the webhook response past GitLab's 10 s.
const prefetchTimeout = 5 * time.Second

// enrichment carries REST data fetched before the transaction for push,
// pipeline and job events. Zero when the API is unavailable or a call
// failed.
type enrichment struct {
	// tails holds final log tails for hard-failed jobs named by a pipeline
	// or job event, fetched here because a pipeline that fails in the same
	// delivery is final at once and the tail loop never visits it.
	tails map[int64]*cards.JobTail
	// artifacts lists the archives a terminal pipeline's jobs produced;
	// nil when not fetched.
	artifacts []cards.Artifact
	// diff holds a push's diff stats from the compare call; nil when not
	// fetched.
	diff *cards.DiffStats
}

// prefetch performs the read-only enrichment the plan allows before BEGIN:
// force-push detection for pushes (setting Push.Forced in place) and
// approvals plus unresolved thread count for merge requests. Failures are
// logged and degrade to no enrichment.
func (e *Engine) prefetch(ctx context.Context, ev event.Event) enrichment {
	if e.api == nil {
		return enrichment{}
	}
	switch v := ev.(type) {
	case *event.Push:
		if v.IsCreate() || v.IsDelete() {
			return enrichment{}
		}
		ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
		defer cancel()
		cmp, err := e.api.Compare(ctx, v.Project.ID, v.After, v.Before, true)
		if err != nil {
			e.logEnrich("compare", v.Project.Path, err)
			return enrichment{}
		}
		v.Forced = len(cmp.Commits) > 0
		return enrichment{diff: diffStatsReversed(cmp)}
	case *event.Pipeline:
		en := e.enrichFailedTails(ctx, v.Project, v.ID, v.Jobs)
		if event.IsTerminal(v.Status) {
			en.artifacts = e.enrichArtifacts(ctx, v.Project, v.ID)
		}
		return en
	case *event.Job:
		return e.enrichFailedTails(ctx, v.Project, v.PipelineID, []event.Job{*v})
	}
	return enrichment{}
}

// enrichFailedTails fetches the final log tail of every hard-failed job in
// jobs that the stored pipeline state does not already hold, within
// prefetchTimeout in total.
func (e *Engine) enrichFailedTails(ctx context.Context, project event.Project, pipelineID int64, jobs []event.Job) enrichment {
	eff := e.cfg.Resolve(project.Path)
	if eff.Pipelines.LogTailLines == 0 {
		return enrichment{}
	}
	var failed []event.Job
	for _, j := range jobs {
		if j.Status == event.StatusFailed && !j.AllowFailure {
			failed = append(failed, j)
		}
	}
	if len(failed) == 0 {
		return enrichment{}
	}
	var st cards.PipelineState
	row, err := e.st.GetObject(ctx, store.Key{Kind: string(cards.KindPipeline), ProjectID: project.ID, ObjectID: pipelineID})
	if err == nil && row != nil {
		_ = unmarshalState(*row, &st)
	}
	ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
	defer cancel()
	now := time.Now()
	en := enrichment{}
	for _, j := range failed {
		if cur, ok := st.Tails[j.ID]; ok && cur.Final {
			continue
		}
		if tail, ok := e.fetchTail(ctx, &cards.PipelineState{Project: project}, cards.JobState{ID: j.ID}, eff.Pipelines.LogTailLines, now, true); ok {
			if en.tails == nil {
				en.tails = map[int64]*cards.JobTail{}
			}
			en.tails[j.ID] = tail
		}
	}
	return en
}

// diffStatsReversed derives after-versus-before stats from the reversed
// compare (from=after, to=before, straight) the force-push check already
// makes: git diff B A is the exact inverse of git diff A B, so the files are
// the same, added and removed swap, old_path is the after-side path, and
// new_file and deleted_file swap.
func diffStatsReversed(cmp *api.Compare) *cards.DiffStats {
	d := &cards.DiffStats{FilesChanged: len(cmp.Diffs), Partial: cmp.CompareTimeout}
	for i, f := range cmp.Diffs {
		a, r := f.LineCounts()
		d.Added += r
		d.Removed += a
		if i < cards.MaxDiffFiles {
			df := cards.DiffFile{Path: f.OldPath, New: f.DeletedFile, Deleted: f.NewFile, Added: r, Removed: a}
			if f.RenamedFile {
				df.RenamedFrom = f.NewPath
			}
			d.Files = append(d.Files, df)
		}
	}
	return d
}

// enrichArtifacts lists the artifact archives of a pipeline's jobs.
func (e *Engine) enrichArtifacts(ctx context.Context, project event.Project, pipelineID int64) []cards.Artifact {
	ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
	defer cancel()
	jobs, err := e.api.PipelineJobs(ctx, project.ID, pipelineID, false)
	if err != nil {
		e.logEnrich("artifacts", project.Path, err)
		return nil
	}
	out := []cards.Artifact{}
	for _, j := range jobs {
		for _, a := range j.Artifacts {
			if a.FileType == "archive" {
				out = append(out, cards.Artifact{JobID: j.ID, JobName: j.Name, Size: a.Size})
			}
		}
	}
	return out
}

// logEnrich logs a failed enrichment call; 403 is expected on tiers without
// the feature and only logged at debug.
func (e *Engine) logEnrich(what, project string, err error) {
	if errors.Is(err, api.ErrForbidden) {
		e.log.Debug("enrichment unavailable", "call", what, "project", project, "err", err)
		return
	}
	e.log.Warn("enrichment failed", "call", what, "project", project, "err", err)
}
