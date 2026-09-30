// Package ops holds process-level concerns: logging setup and the build
// version.
package ops

import (
	"log/slog"
	"os"
)

// Version is the build version, injected at link time with
// -ldflags "-X github.com/esauvisky/gitgram/internal/ops.Version=<v>".
var Version = "dev"

// SetupLogging builds a slog.Logger from the `logging:` config values
// (level: debug|info|warn|error, format: text|json), installs it as the
// default logger and returns it.
func SetupLogging(level, format string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
