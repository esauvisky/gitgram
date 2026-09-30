package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/engine"
	"github.com/esauvisky/gitgram/internal/gitlab/api"
	"github.com/esauvisky/gitgram/internal/gitlab/webhook"
	"github.com/esauvisky/gitgram/internal/httpserver"
	"github.com/esauvisky/gitgram/internal/ops"
	"github.com/esauvisky/gitgram/internal/store"
	"github.com/esauvisky/gitgram/internal/telegram"
)

const (
	maxWebhookBody    = 8 << 20
	reconcileInterval = 5 * time.Minute
	janitorInterval   = time.Hour
	shutdownTimeout   = 30 * time.Second
	healthTimeout     = 2 * time.Second
)

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath, "path to config.yaml")
	poll := fs.Bool("poll", false, "use Telegram long polling instead of the webhook (dev)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// The mode override runs before validation so a webhook-mode config
	// without telegram.webhook_secret still loads under --poll.
	cfg, err := config.Load(*configPath, func(c *config.Config) {
		if *poll {
			c.Telegram.Mode = telegram.ModePolling
		}
	})
	if err != nil {
		return err
	}
	logger := ops.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("starting gitgram",
		"version", ops.Version,
		"listen", cfg.Server.Listen,
		"telegram_mode", cfg.Telegram.Mode,
		"gitlab_group", cfg.GitLab.Group,
		"projects", len(cfg.Projects),
		"enrichment", cfg.GitLab.ReadToken != "",
	)

	st, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return err
	}
	defer st.Close()
	logger.Info("store ready", "path", cfg.Storage.Path)

	var reader api.Reader
	if cfg.GitLab.ReadToken != "" {
		reader = api.New(cfg.GitLab.BaseURL, cfg.GitLab.ReadToken, logger)
	}

	client, err := telegram.New(cfg.Telegram.Token, telegram.Options{
		ChatID:        cfg.Telegram.ChatID,
		Mode:          cfg.Telegram.Mode,
		PublicURL:     cfg.Server.PublicBaseURL,
		WebhookPath:   cfg.Server.TelegramWebhookPath,
		WebhookSecret: cfg.Telegram.WebhookSecret,
		Logger:        logger,
	})
	if err != nil {
		return err
	}
	username, err := client.Me(ctx)
	if err != nil {
		return err
	}
	logger.Info("telegram bot verified", "username", username, "chat_id", cfg.Telegram.ChatID)

	// The engine notifies the sender and the sender renders through the
	// engine; the closure breaks the construction cycle.
	var sender *telegram.Sender
	eng := engine.New(cfg, st, reader, func() { sender.Notify() }, logger)
	sender = telegram.NewSender(client, outboxAdapter{st: st}, eng, logger)

	srv := httpserver.New(httpserver.Options{
		Addr:   cfg.Server.Listen,
		Logger: logger,
		Health: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
			defer cancel()
			return st.Ping(ctx)
		},
	})
	srv.Handle("POST "+cfg.Server.GitLabWebhookPath,
		webhook.NewHandler(cfg.GitLab.WebhookSecret, maxWebhookBody, eng.Handle, logger))
	if cfg.Telegram.Mode == telegram.ModeWebhook {
		srv.Handle("POST "+cfg.Server.TelegramWebhookPath+"/"+cfg.Telegram.WebhookSecret, client.Handler())
	}

	// Background workers outlive the signal context: they stop only after
	// the HTTP server has drained so no accepted webhook is left unprocessed.
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	if err := client.Start(runCtx); err != nil {
		return err
	}
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		sender.Run(runCtx)
	}()
	go eng.RunReconciler(runCtx, reconcileInterval)
	go eng.RunJanitor(runCtx, janitorInterval)

	errc := make(chan error, 1)
	go func() {
		logger.Info("http listening", "addr", cfg.Server.Listen)
		errc <- srv.ListenAndServe()
	}()

	var serveErr error
	select {
	case err := <-errc:
		serveErr = fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("http shutdown: %w", err)
		}
	}

	cancelRun()
	<-senderDone
	logger.Info("stopped")
	return serveErr
}
