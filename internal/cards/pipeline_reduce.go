package cards

import (
	"slices"
	"strconv"

	"github.com/esauvisky/gitgram/internal/event"
)

// ReducePipeline folds a Pipeline Hook into s. s may be zero-valued; it is
// initialised on first use.
//
// Rules: the pipeline fields are overwritten and StageOrder taken from
// stages[]; builds[] is upserted by id, except that a terminal job is never
// regressed to a non-terminal status (the snapshot may predate an applied job
// hook); runs of the same (stage, name) are collapsed so the highest id wins
// and older ones get SupersededBy; live jobs missing from builds[] that are
// still active and older than the newest job in the snapshot are dropped as
// phantoms. A snapshot received before the last applied one is ignored.
// Final becomes true when the reported status is terminal and no live job is
// active.
func ReducePipeline(s *PipelineState, ev *event.Pipeline) (changed bool) {
	if s.SchemaVer == 0 {
		s.init(ev.Project, ev.ID)
	}
	if s.SeenPipelineEvent && ev.Received.Before(s.LastPipelineEventAt) {
		return false
	}
	s.LastPipelineEventAt = ev.Received
	before := snapshot(s)

	s.Project = ev.Project
	s.ID = ev.ID
	s.IID = ev.IID
	if ev.URL != "" {
		s.URL = ev.URL
	}
	s.Ref = ev.Ref
	s.Tag = ev.Tag
	s.SHA = ev.SHA
	s.Source = ev.Source
	s.Status = ev.Status
	s.SeenPipelineEvent = true
	s.CreatedAt = timePtr(ev.CreatedAt)
	s.FinishedAt = ev.FinishedAt
	s.Duration = ev.Duration
	if !ev.User.IsZero() {
		s.Triggerer = ev.User
	}
	if ev.Commit.SHA != "" {
		s.Commit = ev.Commit
	}
	if ev.Parent != nil {
		p := *ev.Parent
		s.Parent = &p
	}
	if len(ev.Stages) > 0 {
		s.StageOrder = slices.Clone(ev.Stages)
	}

	inSnapshot := make(map[int64]bool, len(ev.Jobs))
	var newest int64
	for i := range ev.Jobs {
		b := &ev.Jobs[i]
		inSnapshot[b.ID] = true
		newest = max(newest, b.ID)
		js := jobState(b)
		if cur, ok := s.Jobs[b.ID]; ok && event.IsTerminal(cur.Status) && !event.IsTerminal(js.Status) {
			continue
		}
		s.Jobs[b.ID] = js
		s.addStage(js.Stage)
	}
	s.collapseRetries()
	for id, j := range s.Jobs {
		if !inSnapshot[id] && j.SupersededBy == 0 && id < newest && event.IsActive(j.Status) {
			delete(s.Jobs, id)
		}
	}
	s.recomputeFinal()
	return differs(before, s)
}

// ReduceJob folds a Job Hook into s. s may be zero-valued: a skeleton is then
// built from the job payload with URL derived as Project.WebURL +
// "/-/pipelines/" + id; callers should check Started before creating a card.
//
// Rules: a job whose id is already terminal is ignored (terminal to terminal
// or a stale non-terminal delivery), returning false. A new id with the same
// (stage, name) as a live job is a retry: the lower id gets SupersededBy and
// the pipeline stops being Final while the new run is active. Job hooks never
// change Status, which is GitLab's pipeline status; EffectiveStatus derives
// the displayed status from the jobs when needed.
func ReduceJob(s *PipelineState, ev *event.Job) (changed bool) {
	if s.SchemaVer == 0 {
		s.init(ev.Project, ev.PipelineID)
		s.URL = ev.Project.WebURL + "/-/pipelines/" + strconv.FormatInt(ev.PipelineID, 10)
	}
	before := snapshot(s)

	if s.Ref == "" {
		s.Ref = ev.Ref
		s.Tag = ev.Tag
	}
	if s.SHA == "" {
		s.SHA = ev.SHA
	}
	if s.Triggerer.IsZero() {
		s.Triggerer = ev.User
	}
	if s.Commit.SHA == "" && ev.Commit.SHA != "" {
		s.Commit = ev.Commit
	}
	if s.Parent == nil && ev.Parent != nil {
		p := *ev.Parent
		s.Parent = &p
	}

	js := jobState(ev)
	if cur, ok := s.Jobs[ev.ID]; ok {
		if event.IsTerminal(cur.Status) {
			return false
		}
		if event.IsActive(cur.Status) && event.IsActive(js.Status) && event.StatusRank(js.Status) < event.StatusRank(cur.Status) {
			return false
		}
		js.SupersededBy = cur.SupersededBy
		s.Jobs[ev.ID] = js
	} else {
		s.Jobs[ev.ID] = js
		s.collapseRetries()
	}
	s.addStage(js.Stage)
	s.recomputeFinal()
	return differs(before, s)
}

func (s *PipelineState) init(p event.Project, id int64) {
	s.SchemaVer = SchemaVer
	s.Project = p
	s.ID = id
	s.Jobs = map[int64]JobState{}
}

func (s *PipelineState) addStage(stage string) {
	if stage != "" && !slices.Contains(s.StageOrder, stage) {
		s.StageOrder = append(s.StageOrder, stage)
	}
}

// collapseRetries marks every live run of a (stage, name) other than the one
// with the highest id as superseded by that id.
func (s *PipelineState) collapseRetries() {
	type key struct{ stage, name string }
	newest := map[key]int64{}
	for id, j := range s.Jobs {
		if j.SupersededBy == 0 {
			k := key{j.Stage, j.Name}
			newest[k] = max(newest[k], id)
		}
	}
	for id, j := range s.Jobs {
		if j.SupersededBy != 0 {
			continue
		}
		if n := newest[key{j.Stage, j.Name}]; n != id {
			j.SupersededBy = n
			s.Jobs[id] = j
		}
	}
}

func (s *PipelineState) recomputeFinal() {
	s.Final = s.SeenPipelineEvent && event.IsTerminal(s.Status) && !s.hasActiveJobs()
}

func jobState(j *event.Job) JobState {
	return JobState{
		ID: j.ID, Name: j.Name, Stage: j.Stage, Status: j.Status,
		Manual: j.Manual || j.Status == event.StatusManual, AllowFailure: j.AllowFailure,
		FailureReason: j.FailureReason, Duration: j.Duration, QueuedDuration: j.QueuedDuration,
		StartedAt: j.StartedAt, FinishedAt: j.FinishedAt, URL: j.URL, Retries: j.Retries,
	}
}
