// Package config loads, expands, validates and resolves the Gitgram YAML
// configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML schema.
type Config struct {
	Telegram Telegram         `yaml:"telegram"`
	Server   Server           `yaml:"server"`
	GitLab   GitLab           `yaml:"gitlab"`
	Storage  Storage          `yaml:"storage"`
	Logging  Logging          `yaml:"logging"`
	Defaults Settings         `yaml:"defaults"`
	Projects []Project        `yaml:"projects"`
	Users    map[string]int64 `yaml:"users"`
}

// Telegram configures the bot transport and the destination chat.
type Telegram struct {
	Token         string           `yaml:"token"`
	ChatIDs       ChatIDs          `yaml:"chat_id"`
	Mode          string           `yaml:"mode"` // webhook | polling
	WebhookSecret string           `yaml:"webhook_secret"`
	Threads       map[string]int64 `yaml:"threads"` // event class or "default" → forum topic id
}

// ChatIDs is telegram.chat_id: one id, a comma-separated list of ids, or a
// YAML list. Every card goes to each chat; the first is the primary, the
// one the bot's bookkeeping follows.
type ChatIDs []int64

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *ChatIDs) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.SequenceNode:
		var ids []int64
		if err := n.Decode(&ids); err != nil {
			return err
		}
		*c = ids
		return nil
	case yaml.ScalarNode:
		var ids []int64
		for _, part := range strings.Split(n.Value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return fmt.Errorf("line %d: telegram.chat_id: %q is not a chat id", n.Line, part)
			}
			ids = append(ids, id)
		}
		*c = ids
		return nil
	}
	return fmt.Errorf("line %d: telegram.chat_id: want an id or a comma-separated list", n.Line)
}

// Server configures the HTTP listener and the public webhook paths.
type Server struct {
	Listen              string `yaml:"listen"`
	PublicBaseURL       string `yaml:"public_base_url"`
	GitLabWebhookPath   string `yaml:"gitlab_webhook_path"`
	TelegramWebhookPath string `yaml:"telegram_webhook_path"`
}

// GitLab configures the instance, the accepted group and the tokens.
type GitLab struct {
	BaseURL       string `yaml:"base_url"`
	Group         string `yaml:"group"`
	ReadToken     string `yaml:"read_token"`
	HooksToken    string `yaml:"hooks_token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// Storage configures the SQLite database.
type Storage struct {
	Path string `yaml:"path"`
}

// Logging configures slog.
type Logging struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // text | json
}

// Settings holds the per-project tunables. The same shape is used for
// `defaults:` and for each `projects[]` entry; unset fields inherit from the
// layer below (project → defaults → built-in).
type Settings struct {
	Events    []EventClass     `yaml:"events"`
	Verbosity string           `yaml:"verbosity"` // quiet | normal | verbose
	Branches  Branches         `yaml:"branches"`
	Pipelines Pipelines        `yaml:"pipelines"`
	MR        MR               `yaml:"mr"`
	Push      Push             `yaml:"push"`
	Threads   map[string]int64 `yaml:"threads"`
}

// Branches is the allow/deny filter for refs. Patterns are globs
// (github.com/gobwas/glob) or, with the "re:" prefix, Go regular expressions.
type Branches struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`

	allow, deny []matcher
}

// Pipelines tunes pipeline cards.
type Pipelines struct {
	ChildCards   string  `yaml:"child_cards"` // inline | own | both
	QuietSuccess *bool   `yaml:"quiet_success"`
	LogTail      LogTail `yaml:"log_tail"`
}

// LogTail tunes the failed-job log on pipeline cards: the last Lines of the
// failed job's log, shown under its stage. Lines 0 disables it. Needs
// gitlab.read_token.
type LogTail struct {
	Lines *int `yaml:"lines"`
}

// MR tunes merge request cards.
type MR struct {
	CollapseNotes   *bool `yaml:"collapse_notes"`
	ShowDescription *bool `yaml:"show_description"`
	ShowSystemNotes *bool `yaml:"show_system_notes"`
}

// Push tunes push summaries.
type Push struct {
	MaxCommits *int `yaml:"max_commits"`
}

// Project is a per-project override, matched by path_with_namespace.
type Project struct {
	Path     string `yaml:"path"`
	Settings `yaml:",inline"`
}

// Load reads the YAML file at path, expands ${VAR} / ${VAR:-default}
// references from the environment, applies built-in defaults, runs the
// overrides (command-line flags such as --poll) and validates. Every
// validation problem is reported in the returned error.
func Load(path string, overrides ...func(*Config)) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	expanded, err := expandEnv(string(raw))
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(expanded))
	dec.KnownFields(true)
	// An empty file decodes as io.EOF; let validation report what is missing.
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	cfg.applyDefaults()
	for _, o := range overrides {
		o(&cfg)
	}
	if errs := cfg.validate(); len(errs) > 0 {
		return nil, fmt.Errorf("config %s:\n%w", path, errors.Join(errs...))
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Telegram.Mode == "" {
		c.Telegram.Mode = "webhook"
	}
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Server.GitLabWebhookPath == "" {
		c.Server.GitLabWebhookPath = "/webhook/gitlab"
	}
	if c.Server.TelegramWebhookPath == "" {
		c.Server.TelegramWebhookPath = "/webhook/telegram"
	}
	if c.GitLab.BaseURL == "" {
		c.GitLab.BaseURL = "https://gitlab.com"
	}
	c.GitLab.BaseURL = strings.TrimRight(c.GitLab.BaseURL, "/")
	c.Server.PublicBaseURL = strings.TrimRight(c.Server.PublicBaseURL, "/")
	c.GitLab.Group = strings.Trim(c.GitLab.Group, "/")
	if c.Storage.Path == "" {
		c.Storage.Path = "/data/gitgram.db"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	d := &c.Defaults
	if d.Events == nil {
		d.Events = AllEventClasses
	}
	if d.Verbosity == "" {
		d.Verbosity = "normal"
	}
	if d.Branches.Allow == nil {
		d.Branches.Allow = []string{"*"}
	}
	if d.Branches.Deny == nil {
		d.Branches.Deny = []string{}
	}
	if d.Pipelines.ChildCards == "" {
		d.Pipelines.ChildCards = "inline"
	}
	if d.Pipelines.QuietSuccess == nil {
		d.Pipelines.QuietSuccess = ptr(true)
	}
	if d.Pipelines.LogTail.Lines == nil {
		d.Pipelines.LogTail.Lines = ptr(10)
	}
	if d.MR.CollapseNotes == nil {
		d.MR.CollapseNotes = ptr(false)
	}
	if d.MR.ShowDescription == nil {
		d.MR.ShowDescription = ptr(true)
	}
	if d.MR.ShowSystemNotes == nil {
		d.MR.ShowSystemNotes = ptr(false)
	}
	if d.Push.MaxCommits == nil {
		d.Push.MaxCommits = ptr(10)
	}
	users := make(map[string]int64, len(c.Users))
	for name, id := range c.Users {
		users[strings.ToLower(name)] = id
	}
	c.Users = users
}

func ptr[T any](v T) *T { return &v }
