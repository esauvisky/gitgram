package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/ops"
	"github.com/esauvisky/gitgram/internal/synchooks"
)

// runSyncHooks registers or updates the GitLab webhook on every project
// under GITGRAM_GITLAB_GROUP (or one group hook with --group-hook) so
// deliveries reach GITGRAM_PUBLIC_URL + /webhook/gitlab.
func runSyncHooks(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sync-hooks", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print the planned hook changes without applying them")
	groupHook := fs.Bool("group-hook", false, "register one group webhook (Premium) instead of one hook per project")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := ops.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)
	if cfg.GitLab.HooksToken == "" {
		return errors.New("sync-hooks: GITGRAM_GITLAB_HOOKS_TOKEN is required (api scope)")
	}
	if cfg.Server.PublicBaseURL == "" {
		return errors.New("sync-hooks: GITGRAM_PUBLIC_URL is required")
	}

	return synchooks.Run(ctx, api.New(cfg.GitLab.BaseURL, cfg.GitLab.HooksToken, logger), synchooks.Options{
		Group:      cfg.GitLab.Group,
		WebhookURL: cfg.Server.PublicBaseURL + cfg.Server.GitLabWebhookPath,
		Secret:     cfg.GitLab.WebhookSecret,
		DryRun:     *dryRun,
		GroupHook:  *groupHook,
	}, os.Stdout)
}
