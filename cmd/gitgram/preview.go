package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/config"
	"github.com/esauvisky/gitgram/internal/engine"
	"github.com/esauvisky/gitgram/internal/ops"
	"github.com/esauvisky/gitgram/internal/preview"
	"github.com/esauvisky/gitgram/internal/store"
	"github.com/esauvisky/gitgram/internal/telegram"
)

// runPreview sends mock cards of every kind and scenario to the configured
// chat through the real engine and sender, from a throwaway database, so a
// card change can be seen in the actual client. It never talks to GitLab
// and never polls Telegram, so it runs beside a live `serve`.
func runPreview(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	configPath := fs.String("config", defaultConfigPath, "path to config.yaml")
	scenario := fs.String("scenario", "all", "comma-separated scenarios, or all: "+strings.Join(preview.Names(), ", "))
	delay := fs.Duration("delay", 4*time.Second, "pause between the steps of a scenario, so edits can be watched")
	dbPath := fs.String("db", "", "SQLite file for the preview state; default is a temporary file removed on exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath, func(c *config.Config) {
		// Nothing listens here: the mode only has to validate.
		c.Telegram.Mode = telegram.ModePolling
	})
	if err != nil {
		return err
	}
	logger := ops.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)
	client, err := telegram.New(cfg.Telegram.Token, telegram.Options{ChatIDs: cfg.Telegram.ChatIDs, Mode: telegram.ModePolling, Logger: logger})
	if err != nil {
		return err
	}
	username, err := client.Me(ctx)
	if err != nil {
		return err
	}
	logger.Info("preview: sending mock cards", "bot", username, "chat_ids", cfg.Telegram.ChatIDs, "scenarios", *scenario)
	if err := runPreviewWith(ctx, cfg, client, cfg.Telegram.ChatIDs, logger, *dbPath, strings.Split(*scenario, ","), *delay); err != nil {
		return err
	}
	fmt.Println("preview: done")
	return nil
}

// runPreviewWith sends the scenarios to chats through a throwaway store
// with its own engine and sender on client, so the live state is never
// touched. An
// empty dbPath uses a temporary file, removed afterwards, beside the real
// database (the only writable place in the container) or, when that
// directory is not writable, in the system temp dir.
func runPreviewWith(ctx context.Context, cfg *config.Config, client *telegram.Client, chats []int64, logger *slog.Logger, dbPath string, scenarios []string, delay time.Duration) error {
	path := dbPath
	if path == "" {
		f, err := os.CreateTemp(filepath.Dir(cfg.Storage.Path), "preview-*.db")
		if err != nil {
			f, err = os.CreateTemp("", "gitgram-preview-*.db")
		}
		if err != nil {
			return err
		}
		path = f.Name()
		f.Close()
		defer func() {
			for _, suffix := range []string{"", "-wal", "-shm"} {
				os.Remove(path + suffix)
			}
		}()
	}
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	defer st.Close()

	var sender *telegram.Sender
	eng := engine.New(cfg, st, nil, nil, func() { sender.Notify() }, logger)
	sender = telegram.NewSender(client, chats, outboxAdapter{st: st, primary: chats[0]}, eng, logger)
	runCtx, cancelRun := context.WithCancel(context.Background())
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		sender.Run(runCtx)
	}()
	runErr := preview.Run(ctx, eng, st, preview.Options{Group: cfg.GitLab.Group, Scenarios: scenarios, Delay: delay, Log: logger})
	cancelRun()
	<-senderDone
	return runErr
}
