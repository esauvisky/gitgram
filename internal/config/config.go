// Package config reads and validates the Gitgram settings from GITGRAM_*
// environment variables (a .env file under Docker Compose).
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds every setting the bot reads.
type Config struct {
	Telegram Telegram
	Server   Server
	GitLab   GitLab
	Storage  Storage
	Logging  Logging
	// LogLines is how many lines of a failed job's log a card shows; 0
	// disables failure logs.
	LogLines int
}

// Telegram configures the bot transport and the destination chats.
type Telegram struct {
	Token string
	// ChatIDs receive every card; the first is the primary, the one the
	// bot's bookkeeping follows.
	ChatIDs []int64
	// DebugChatID, when set, is where `gitgram preview` sends its mock
	// cards instead of ChatIDs; /preview is accepted there too.
	DebugChatID int64
	// Mode is webhook or polling.
	Mode          string
	WebhookSecret string
}

// Server configures the HTTP listener and the webhook paths.
type Server struct {
	Listen              string
	PublicBaseURL       string
	GitLabWebhookPath   string
	TelegramWebhookPath string
}

// GitLab configures the GitLab instance, the group the bot serves and its
// tokens.
type GitLab struct {
	BaseURL string
	// Group is the top-level group; every project under it (subgroups
	// included) is accepted.
	Group string
	// ReadToken (read_api) enables diff stats, failure logs, artifacts,
	// force-push detection and the reconciler.
	ReadToken string
	// HooksToken (api) enables sync-hooks and the card buttons.
	HooksToken    string
	WebhookSecret string
}

// Storage is where the SQLite database lives.
type Storage struct {
	Path string
}

// Logging configures slog.
type Logging struct {
	Level  string
	Format string
}

// Load reads the settings from the environment, applies the defaults and
// validates them. Overrides run before validation (the --poll flag sets
// the mode). Every problem is reported at once.
func Load(overrides ...func(*Config)) (*Config, error) {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	env := func(name, def string) string {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
		return def
	}
	number := func(name string, def int) int {
		raw := env(name, "")
		if raw == "" {
			return def
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			bad("%s: %q is not a number", name, raw)
		}
		return n
	}

	c := &Config{
		Telegram: Telegram{
			Token:         env("GITGRAM_TELEGRAM_TOKEN", ""),
			Mode:          env("GITGRAM_TELEGRAM_MODE", "webhook"),
			WebhookSecret: env("GITGRAM_TG_WEBHOOK_SECRET", ""),
		},
		Server: Server{
			Listen:              env("GITGRAM_LISTEN", ":8080"),
			PublicBaseURL:       strings.TrimRight(env("GITGRAM_PUBLIC_URL", ""), "/"),
			GitLabWebhookPath:   "/webhook/gitlab",
			TelegramWebhookPath: "/webhook/telegram",
		},
		GitLab: GitLab{
			BaseURL:       strings.TrimRight(env("GITGRAM_GITLAB_URL", "https://gitlab.com"), "/"),
			Group:         strings.Trim(env("GITGRAM_GITLAB_GROUP", ""), "/"),
			ReadToken:     env("GITGRAM_GITLAB_TOKEN", ""),
			HooksToken:    env("GITGRAM_GITLAB_HOOKS_TOKEN", ""),
			WebhookSecret: env("GITGRAM_WEBHOOK_SECRET", ""),
		},
		Storage:  Storage{Path: env("GITGRAM_DB", "/data/gitgram.db")},
		Logging:  Logging{Level: env("GITGRAM_LOG_LEVEL", "info"), Format: env("GITGRAM_LOG_FORMAT", "text")},
		LogLines: number("GITGRAM_LOG_LINES", 10),
	}
	seen := map[int64]bool{}
	for _, part := range strings.Split(env("GITGRAM_CHAT_ID", ""), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		switch {
		case err != nil || id == 0:
			bad("GITGRAM_CHAT_ID: %q is not a chat id", part)
		case seen[id]:
			bad("GITGRAM_CHAT_ID: %d listed twice", id)
		default:
			seen[id] = true
			c.Telegram.ChatIDs = append(c.Telegram.ChatIDs, id)
		}
	}
	if raw := env("GITGRAM_DEBUG_CHAT_ID", ""); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id == 0 {
			bad("GITGRAM_DEBUG_CHAT_ID: %q is not a chat id", raw)
		}
		c.Telegram.DebugChatID = id
	}
	for _, o := range overrides {
		o(c)
	}

	required := []struct{ name, value string }{
		{"GITGRAM_TELEGRAM_TOKEN", c.Telegram.Token},
		{"GITGRAM_GITLAB_GROUP", c.GitLab.Group},
		{"GITGRAM_WEBHOOK_SECRET", c.GitLab.WebhookSecret},
	}
	for _, r := range required {
		if r.value == "" {
			bad("%s: required", r.name)
		}
	}
	if len(c.Telegram.ChatIDs) == 0 {
		bad("GITGRAM_CHAT_ID: required")
	}
	switch c.Telegram.Mode {
	case "webhook":
		if c.Telegram.WebhookSecret == "" {
			bad("GITGRAM_TG_WEBHOOK_SECRET: required in webhook mode")
		}
		if c.Server.PublicBaseURL == "" {
			bad("GITGRAM_PUBLIC_URL: required in webhook mode")
		}
	case "polling":
	default:
		bad("GITGRAM_TELEGRAM_MODE: %q must be webhook or polling", c.Telegram.Mode)
	}
	for _, u := range []struct{ name, value string }{{"GITGRAM_PUBLIC_URL", c.Server.PublicBaseURL}, {"GITGRAM_GITLAB_URL", c.GitLab.BaseURL}} {
		if p, err := url.Parse(u.value); u.value != "" && (err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "") {
			bad("%s: %q must be an absolute http(s) URL", u.name, u.value)
		}
	}
	if c.LogLines < 0 || c.LogLines > 50 {
		bad("GITGRAM_LOG_LINES: must be 0..50, got %d", c.LogLines)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("config: %w", errors.Join(errs...))
	}
	return c, nil
}

// CommandChats are the chats the bot answers slash commands in: every
// card chat and the debug chat.
func (c *Config) CommandChats() []int64 {
	if c.Telegram.DebugChatID == 0 {
		return c.Telegram.ChatIDs
	}
	return append(append([]int64{}, c.Telegram.ChatIDs...), c.Telegram.DebugChatID)
}

// PreviewChats are where `gitgram preview` sends its mock cards: the debug
// chat when set, otherwise every card chat.
func (c *Config) PreviewChats() []int64 {
	if c.Telegram.DebugChatID != 0 {
		return []int64{c.Telegram.DebugChatID}
	}
	return c.Telegram.ChatIDs
}

// Accepts reports whether a project path belongs to the configured group
// (subgroups included).
func (c *Config) Accepts(projectPath string) bool {
	return strings.HasPrefix(projectPath, c.GitLab.Group+"/")
}
