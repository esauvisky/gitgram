// Command gitgram relays GitLab webhooks into one Telegram group with
// live-edited cards.
//
// Subcommands:
//
//	serve       --config path [--poll]
//	sync-hooks  --config path [--dry-run] [--group-hook]
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

const defaultConfigPath = "config.yaml"

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
  gitgram serve       --config %s [--poll]
  gitgram sync-hooks  --config %s [--dry-run] [--group-hook]
  gitgram healthcheck [--url http://127.0.0.1:8080/healthz]
  gitgram version
`, ops.Version, defaultConfigPath, defaultConfigPath)
}
