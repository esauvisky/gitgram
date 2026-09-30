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
// under gitlab.group (or one group hook with --group-hook) so deliveries
// reach server.public_base_url + server.gitlab_webhook_path.
func runSyncHooks(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sync-hooks", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath, "path to config.yaml")
	dryRun := fs.Bool("dry-run", false, "print the planned hook changes without applying them")
	groupHook := fs.Bool("group-hook", false, "register one group webhook (Premium) instead of one hook per project")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	logger := ops.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)
	if cfg.GitLab.HooksToken == "" {
		return errors.New("sync-hooks: gitlab.hooks_token is required (api scope)")
	}
	if cfg.Server.PublicBaseURL == "" {
		return errors.New("sync-hooks: server.public_base_url is required")
	}

	// Subscribe to every class any project may relay; per-project filtering
	// happens in the bot.
	events := make(map[string]bool, len(config.AllEventClasses))
	for _, class := range cfg.Defaults.Events {
		events[string(class)] = true
	}
	for _, p := range cfg.Projects {
		for _, class := range p.Events {
			events[string(class)] = true
		}
	}

	return synchooks.Run(ctx, api.New(cfg.GitLab.BaseURL, cfg.GitLab.HooksToken, logger), synchooks.Options{
		Group:      cfg.GitLab.Group,
		WebhookURL: cfg.Server.PublicBaseURL + cfg.Server.GitLabWebhookPath,
		Secret:     cfg.GitLab.WebhookSecret,
		Events:     events,
		DryRun:     *dryRun,
		GroupHook:  *groupHook,
	}, os.Stdout)
}
