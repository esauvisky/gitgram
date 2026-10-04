// Package preview sends mock cards of every kind and scenario through the
// real engine and sender, so a change to the cards can be seen and felt in
// the actual Telegram client: first sends, in-place edits, folding, log
// tails and artifacts all behave as in production. Nothing
// reaches GitLab; the project and its objects are made up.
package preview

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/engine"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/store"
)

// drainTimeout bounds how long one step waits for the outbox to empty.
const drainTimeout = 90 * time.Second

// Options configures Run.
type Options struct {
	// Group is the configured top-level group; the mock project lives under
	// it so the engine accepts its events.
	Group string
	// Owners are the configured GitHub owners; the mock repository lives
	// under the first when no GitLab group is configured.
	Owners []string
	// Scenarios names what to send; empty or "all" runs everything in order.
	Scenarios []string
	// Delay is the pause after each step, so edits can be watched.
	Delay time.Duration
	Log   *slog.Logger
}

// Scenario is one named sequence of steps.
type Scenario struct {
	Name  string
	About string
	Steps []Step
}

// Step delivers events and decorations, then waits for the outbox to drain.
type Step func(ctx context.Context, r *Runner) error

// Runner hands steps the engine and store.
type Runner struct {
	eng  *engine.Engine
	st   *store.Store
	opts Options
	proj event.Project
	seq  int
}

// Names lists every scenario in run order.
func Names() []string {
	out := make([]string, len(scenarios))
	for i, s := range scenarios {
		out[i] = s.Name
	}
	return out
}

// Run sends the selected scenarios.
func Run(ctx context.Context, eng *engine.Engine, st *store.Store, opts Options) error {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	group := strings.Trim(opts.Group, "/")
	proj := event.Project{
		ID: 990001, Path: group + "/preview/demo", Name: "demo",
		WebURL: "https://gitlab.com/" + group + "/preview/demo", DefaultBranch: "develop",
	}
	if group == "" {
		owner := opts.Owners[0]
		proj = event.Project{
			ID: -990001, Path: owner + "/demo", Name: "demo",
			WebURL: "https://github.com/" + owner + "/demo", DefaultBranch: "develop",
		}
	}
	r := &Runner{eng: eng, st: st, opts: opts, proj: proj}
	want := opts.Scenarios
	if len(want) == 0 || slices.Contains(want, "all") {
		want = Names()
	}
	for _, name := range want {
		i := slices.IndexFunc(scenarios, func(s Scenario) bool { return s.Name == name })
		if i < 0 {
			return fmt.Errorf("preview: unknown scenario %q (have %s)", name, strings.Join(Names(), ", "))
		}
	}
	for _, name := range want {
		sc := scenarios[slices.IndexFunc(scenarios, func(s Scenario) bool { return s.Name == name })]
		opts.Log.Info("preview: scenario", "name", sc.Name, "about", sc.About, "steps", len(sc.Steps))
		for i, step := range sc.Steps {
			if err := step(ctx, r); err != nil {
				return fmt.Errorf("preview: %s step %d: %w", sc.Name, i+1, err)
			}
			if err := r.drain(ctx); err != nil {
				return err
			}
			if i < len(sc.Steps)-1 && !sleep(ctx, opts.Delay) {
				return ctx.Err()
			}
		}
	}
	return nil
}

// handle delivers one synthetic event with a fresh delivery key.
func (r *Runner) handle(ctx context.Context, ev event.Event) error {
	r.seq++
	return r.eng.Handle(ctx, fmt.Sprintf("preview:%d:%d", time.Now().UnixNano(), r.seq), ev)
}

// drain waits until the sender emptied the outbox.
func (r *Runner) drain(ctx context.Context) error {
	deadline := time.Now().Add(drainTimeout)
	for {
		head, err := r.st.OutboxHead(ctx)
		if err != nil {
			return err
		}
		if head == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("preview: outbox did not drain in %s (last error: %s)", drainTimeout, head.LastError)
		}
		if !sleep(ctx, 300*time.Millisecond) {
			return ctx.Err()
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (r *Runner) pipelineKey(id int64) cards.Key {
	return cards.Key{Kind: cards.KindPipeline, ProjectID: r.proj.ID, ObjectID: id}
}

func (r *Runner) pushKey(branch, sha string) cards.Key {
	return cards.PushKey(r.proj.ID, branch, sha)
}
