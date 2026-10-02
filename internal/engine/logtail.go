package engine

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/store"
)

// traceTimeout bounds one job trace fetch.
const traceTimeout = 10 * time.Second

// maxTailLineLen caps one rendered log line.
const maxTailLineLen = 120

// ansi matches CSI escape sequences (colours, cursor moves, erase-line).
var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

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

// Decorate writes log tails and artifacts into a pipeline's stored state
// and queues its card, exactly as the failed-job and terminal-pipeline
// prefetch would after reading GitLab. The preview subcommand uses it to
// show tails and artifacts without a GitLab behind them.
func (e *Engine) Decorate(ctx context.Context, key cards.Key, tails map[int64]*cards.JobTail, artifacts []cards.Artifact) error {
	err := e.st.WithTx(ctx, func(tx *store.Tx) error {
		st, row, err := load[cards.PipelineState](ctx, tx, key, e.log)
		if err != nil || row == nil {
			return err
		}
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
		return e.publishPipeline(ctx, tx, key, st, row.LastEventAt)
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
