package engine

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// traceTimeout bounds one job trace fetch.
const traceTimeout = 10 * time.Second

// maxTailLineLen caps one rendered log line.
const maxTailLineLen = 120

// ansi matches CSI escape sequences (colours, cursor moves, erase-line).
var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// RunLogTail keeps job log tails on non-final pipeline cards: the running
// job's last lines refreshed every project interval, each failed job's
// final lines once, and finished-successfully jobs cleared. It runs once
// immediately, then every tick, until ctx is cancelled. Without an API
// reader it returns at once.
func (e *Engine) RunLogTail(ctx context.Context, every time.Duration) {
	if e.api == nil {
		e.log.Info("log tail disabled: no gitlab.read_token")
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e.tailPipelines(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (e *Engine) tailPipelines(ctx context.Context) {
	rows, err := e.st.ListNonFinal(ctx, string(cards.KindPipeline), time.Now().Add(time.Hour))
	if err != nil {
		e.log.Error("log tail: list pipelines", "err", err)
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		var st cards.PipelineState
		if err := unmarshalState(row, &st); err != nil {
			e.log.Error("log tail: corrupt pipeline state", "key", row.Key, "err", err)
			continue
		}
		eff := e.cfg.Resolve(st.Project.Path)
		updates := e.refreshTails(ctx, &st, eff, time.Now())
		if len(updates) == 0 {
			continue
		}
		e.applyTails(ctx, st.Key(), eff, updates)
	}
}

// refreshTails fetches what the card is missing: a fresh live tail for the
// first running job of each stage whose tail is older than the interval, a
// final tail for each hard-failed job without one, and a nil entry for jobs
// whose tail should go (finished without failing, or no longer the stage's
// shown job). Nothing is fetched when the project has tails disabled.
func (e *Engine) refreshTails(ctx context.Context, st *cards.PipelineState, eff config.EffectiveProject, now time.Time) map[int64]*cards.JobTail {
	if eff.Pipelines.LogTailLines == 0 {
		return nil
	}
	updates := map[int64]*cards.JobTail{}
	shown := map[string]bool{}
	for _, j := range st.SortedJobs() {
		cur, has := st.Tails[j.ID]
		switch {
		case j.Status == event.StatusRunning:
			if shown[j.Stage] {
				if has {
					updates[j.ID] = nil
				}
				continue
			}
			shown[j.Stage] = true
			if has && now.Sub(cur.FetchedAt) < eff.Pipelines.LogTailInterval {
				continue
			}
			if tail, ok := e.fetchTail(ctx, st, j, eff.Pipelines.LogTailLiveLines, now, false); ok && (!has || !equalLines(cur.Lines, tail.Lines)) {
				updates[j.ID] = tail
			}
		case j.Status == event.StatusFailed && !j.AllowFailure:
			if has && cur.Final {
				continue
			}
			if tail, ok := e.fetchTail(ctx, st, j, eff.Pipelines.LogTailLines, now, true); ok {
				updates[j.ID] = tail
			}
		default:
			if has {
				updates[j.ID] = nil
			}
		}
	}
	return updates
}

// fetchTail reads one job's trace and keeps its last n lines.
func (e *Engine) fetchTail(ctx context.Context, st *cards.PipelineState, j cards.JobState, n int, now time.Time, final bool) (*cards.JobTail, bool) {
	ctx, cancel := context.WithTimeout(ctx, traceTimeout)
	defer cancel()
	trace, err := e.api.JobTrace(ctx, st.Project.ID, j.ID)
	if err != nil {
		e.logEnrich("trace", st.Project.Path, err)
		return nil, false
	}
	return &cards.JobTail{Lines: tailLines(trace, n), FetchedAt: now, Final: final}, true
}

// applyTails writes tail updates into the stored state and queues the
// card. The state is re-read inside the transaction so a delivery that
// landed meanwhile is not overwritten.
func (e *Engine) applyTails(ctx context.Context, key cards.Key, eff config.EffectiveProject, updates map[int64]*cards.JobTail) {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.PipelineState](ctx, tx, key, e.log)
		if err != nil || row == nil {
			return err
		}
		if !mergeTails(st, updates) {
			return nil
		}
		if err := put(ctx, tx, key, st, st.Final, row.LastEventAt); err != nil {
			return err
		}
		return e.publishPipeline(ctx, tx, eff, key, st, row.LastEventAt)
	})
	if err != nil {
		e.log.Error("log tail: apply failed", "key", key, "err", err)
		return
	}
	e.notify()
}

// Decorate writes log tails and artifacts into a pipeline's stored state
// and queues its card, exactly as the tail loop and the terminal-pipeline
// prefetch would after reading GitLab. The preview subcommand uses it to
// show tails and artifacts without a GitLab behind them.
func (e *Engine) Decorate(ctx context.Context, key cards.Key, tails map[int64]*cards.JobTail, artifacts []cards.Artifact) error {
	eff := e.cfg.Resolve("")
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.PipelineState](ctx, tx, key, e.log)
		if err != nil || row == nil {
			return err
		}
		eff = e.cfg.Resolve(st.Project.Path)
		changed := mergeTails(st, tails)
		if artifacts != nil {
			st.Artifacts = artifacts
			changed = true
		}
		if !changed {
			return nil
		}
		if err := put(ctx, tx, key, st, st.Final, row.LastEventAt); err != nil {
			return err
		}
		return e.publishPipeline(ctx, tx, eff, key, st, row.LastEventAt)
	})
	if err != nil {
		return err
	}
	e.notify()
	return nil
}

// mergeTails applies updates to st.Tails; a nil update removes the tail.
// It reports whether anything changed.
func mergeTails(st *cards.PipelineState, updates map[int64]*cards.JobTail) bool {
	changed := false
	for id, t := range updates {
		if t == nil {
			if _, ok := st.Tails[id]; ok {
				delete(st.Tails, id)
				changed = true
			}
			continue
		}
		if st.Tails == nil {
			st.Tails = map[int64]cards.JobTail{}
		}
		if cur, ok := st.Tails[id]; !ok || cur.Final != t.Final || !equalLines(cur.Lines, t.Lines) {
			changed = true
		}
		st.Tails[id] = *t
	}
	return changed
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// tailLines returns the last n meaningful lines of a GitLab job trace:
// carriage-return progress lines keep only their final state, ANSI colour
// and erase sequences are stripped, section markers and blank lines are
// dropped, and each line is capped at maxTailLineLen runes.
func tailLines(trace string, n int) []string {
	raw := strings.Split(trace, "\n")
	var out []string
	for _, l := range raw {
		if i := strings.LastIndexByte(l, '\r'); i >= 0 {
			l = l[i+1:]
		}
		l = strings.TrimRight(ansi.ReplaceAllString(l, ""), " \t")
		if l == "" || strings.HasPrefix(l, "section_start:") || strings.HasPrefix(l, "section_end:") {
			continue
		}
		if r := []rune(l); len(r) > maxTailLineLen {
			l = string(r[:maxTailLineLen-1]) + "…"
		}
		out = append(out, l)
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}
