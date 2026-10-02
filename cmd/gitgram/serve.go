package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/esauvisky/gitgram/internal/actions"
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
		"actions", cfg.GitLab.HooksToken != "",
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
	var writer api.Writer
	if cfg.GitLab.HooksToken != "" {
		writer = api.New(cfg.GitLab.BaseURL, cfg.GitLab.HooksToken, logger)
	}

	// The engine notifies the sender, the sender renders through the
	// engine, and button presses dispatch into the engine; the closures
	// break the construction cycle.
	var sender *telegram.Sender
	var eng *engine.Engine
	client, err := telegram.New(cfg.Telegram.Token, telegram.Options{
		ChatIDs:       cfg.Telegram.ChatIDs,
		Mode:          cfg.Telegram.Mode,
		PublicURL:     cfg.Server.PublicBaseURL,
		WebhookPath:   cfg.Server.TelegramWebhookPath,
		WebhookSecret: cfg.Telegram.WebhookSecret,
		Logger:        logger,
		OnCallback: func(ctx context.Context, req actions.Request) actions.Result {
			res, err := eng.Dispatch(ctx, req)
			if err != nil {
				logger.Error("action dispatch", "err", err)
				return actions.Result{Toast: "Something went wrong.", Alert: true}
			}
			return res
		},
		Commands: []telegram.Command{{Name: "preview", Description: "Send mock cards of every kind, or of the scenarios named"}},
	})
	if err != nil {
		return err
	}
	username, err := client.Me(ctx)
	if err != nil {
		return err
	}
	logger.Info("telegram bot verified", "username", username, "chat_ids", cfg.Telegram.ChatIDs)

	eng = engine.New(cfg, st, reader, writer, func() { sender.Notify() }, logger)
	chats := cfg.Telegram.ChatIDs
	sender = telegram.NewSender(client, chats, outboxAdapter{st: st, primary: chats[0]}, eng, logger)

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

	// /preview runs the mock scenarios from a throwaway store beside the
	// live one, one run at a time, into the chat that asked.
	var previewBusy atomic.Bool
	client.SetOnCommand(func(ctx context.Context, call telegram.CommandCall) {
		if call.Name != "preview" {
			return
		}
		if !previewBusy.CompareAndSwap(false, true) {
			_ = client.Reply(ctx, call.ChatID, call.MessageID, "A preview is already running.")
			return
		}
		scenarios := []string{"all"}
		if call.Args != "" {
			scenarios = strings.Split(strings.ReplaceAll(call.Args, " ", ","), ",")
		}
		logger.Info("preview: requested from chat", "chat", call.ChatID, "user", call.UserID, "scenarios", scenarios)
		go func() {
			defer previewBusy.Store(false)
			if err := runPreviewWith(runCtx, cfg, client, []int64{call.ChatID}, logger, "", scenarios, 4*time.Second); err != nil {
				logger.Warn("preview failed", "err", err)
				_ = client.Reply(runCtx, call.ChatID, call.MessageID, "Preview failed: "+err.Error())
			}
		}()
	})

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
