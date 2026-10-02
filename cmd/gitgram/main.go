// Command gitgram relays GitLab webhooks into one Telegram group with
// live-edited cards.
//
// Subcommands:
//
//	serve       [--poll]
//	sync-hooks  [--dry-run] [--group-hook]
//	preview     [--scenario all] [--delay 4s] [--db path]
//
// Settings come from GITGRAM_* environment variables.
//
//	healthcheck [--url http://127.0.0.1:8080/healthz]
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/esauvisky/gitgram/internal/ops"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(ctx, os.Args[2:])
	case "sync-hooks":
		err = runSyncHooks(ctx, os.Args[2:])
	case "preview":
		err = runPreview(ctx, os.Args[2:])
	case "healthcheck":
		err = runHealthcheck(ctx, os.Args[2:])
	case "version":
		fmt.Println(ops.Version)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "gitgram: unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitgram:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gitgram %s

Usage:
  gitgram serve       [--poll]
  gitgram sync-hooks  [--dry-run] [--group-hook]
  gitgram preview     [--scenario all] [--delay 4s] [--db path]
  gitgram healthcheck [--url http://127.0.0.1:8080/healthz]
  gitgram version

Settings come from GITGRAM_* environment variables (see README).
`, ops.Version)
}
