// Package synchooks implements `gitgram sync-hooks`: it registers or updates
// the Gitgram webhook on every project under the configured group (or one
// group hook on Premium groups) so that reruns are idempotent.
package synchooks

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// Options configures one sync run.
type Options struct {
	// Group is the top-level GitLab group path (or numeric id).
	Group string
	// WebhookURL is the public URL GitLab must deliver to
	// (GITGRAM_PUBLIC_URL + /webhook/gitlab).
	WebhookURL string
	// Secret is the X-Gitlab-Token value (GITGRAM_WEBHOOK_SECRET).
	Secret string
	// DryRun prints the planned changes without calling POST/PUT.
	DryRun bool
	// GroupHook registers a single hook on the group instead of one per
	// project (Premium/Ultimate only).
	GroupHook bool
}

type target struct {
	name   string
	list   func(context.Context) ([]api.Hook, error)
	create func(context.Context, api.HookSpec) (*api.Hook, error)
	update func(context.Context, int64, api.HookSpec) (*api.Hook, error)
}

// Run lists the targets (every project under opts.Group, or the group itself
// with GroupHook), finds the hook whose URL equals opts.WebhookURL on each,
// creates or updates it, and prints one table row per target to out with
// the outcome: created, updated, unchanged, error (prefixed with "would" in
// dry-run). Because GitLab never returns hook tokens, an existing hook is
// always PUT so the secret is re-applied even when reported unchanged.
// It returns an error if any target failed.
func Run(ctx context.Context, c *api.Client, opts Options, out io.Writer) error {
	spec := hookSpec(opts)

	var targets []target
	if opts.GroupHook {
		targets = []target{{
			name: opts.Group,
			list: func(ctx context.Context) ([]api.Hook, error) { return c.GroupHooks(ctx, opts.Group) },
			create: func(ctx context.Context, s api.HookSpec) (*api.Hook, error) {
				return c.CreateGroupHook(ctx, opts.Group, s)
			},
			update: func(ctx context.Context, id int64, s api.HookSpec) (*api.Hook, error) {
				return c.UpdateGroupHook(ctx, opts.Group, id, s)
			},
		}}
	} else {
		projects, err := c.GroupProjects(ctx, opts.Group)
		if err != nil {
			return fmt.Errorf("synchooks: list projects of %q: %w", opts.Group, err)
		}
		for _, p := range projects {
			targets = append(targets, target{
				name: p.PathWithNamespace,
				list: func(ctx context.Context) ([]api.Hook, error) { return c.ProjectHooks(ctx, p.ID) },
				create: func(ctx context.Context, s api.HookSpec) (*api.Hook, error) {
					return c.CreateProjectHook(ctx, p.ID, s)
				},
				update: func(ctx context.Context, id int64, s api.HookSpec) (*api.Hook, error) {
					return c.UpdateProjectHook(ctx, p.ID, id, s)
				},
			})
		}
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tACTION\tHOOK")
	failed := 0
	for _, t := range targets {
		action, detail, err := sync(ctx, t, spec, opts.DryRun)
		if err != nil {
			failed++
			action, detail = "error", err.Error()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", t.name, action, detail)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("synchooks: write table: %w", err)
	}
	if failed > 0 {
		return fmt.Errorf("synchooks: %d of %d hooks failed", failed, len(targets))
	}
	return nil
}

// sync reconciles one target and returns the action taken plus a detail
// column ("#id" and, when updating, the changed fields).
func sync(ctx context.Context, t target, spec api.HookSpec, dryRun bool) (action, detail string, err error) {
	hooks, err := t.list(ctx)
	if err != nil {
		return "", "", err
	}
	var existing *api.Hook
	for i := range hooks {
		if hooks[i].URL == spec.URL {
			existing = &hooks[i]
			break
		}
	}

	if existing == nil {
		if dryRun {
			return "would create", "", nil
		}
		h, err := t.create(ctx, spec)
		if err != nil {
			return "", "", err
		}
		return "created", fmt.Sprintf("#%d", h.ID), nil
	}

	diffs := diffSpec(existing.HookSpec, spec)
	detail = fmt.Sprintf("#%d", existing.ID)
	if len(diffs) > 0 {
		detail += " " + strings.Join(diffs, ", ")
	}
	switch {
	case len(diffs) == 0:
		action = "unchanged"
	case dryRun:
		action = "would update"
	default:
		action = "updated"
	}
	if !dryRun {
		if _, err := t.update(ctx, existing.ID, spec); err != nil {
			return "", "", err
		}
	}
	return action, detail, nil
}
