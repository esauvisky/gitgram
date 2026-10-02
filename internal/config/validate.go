package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// validate checks every field and compiles branch matchers. It returns one
// error per problem so the user can fix them all in one pass.
func (c *Config) validate() []error {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	oneOf := func(field, val string, allowed ...string) {
		if !slices.Contains(allowed, val) {
			bad("%s: %q must be one of %s", field, val, strings.Join(allowed, "|"))
		}
	}
	validURL := func(field, raw string) {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			bad("%s: %q must be an absolute http(s) URL", field, raw)
		}
	}
	validPath := func(field, p string) {
		if !strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
			bad("%s: %q must start with / and not end with /", field, p)
		}
	}
	validThreads := func(field string, threads map[string]int64) {
		for key, id := range threads {
			if key != ThreadDefault && !EventClass(key).Valid() {
				bad("%s.%s: unknown event class", field, key)
			}
			if id <= 0 {
				bad("%s.%s: topic id must be positive, got %d", field, key, id)
			}
		}
	}

	if c.Telegram.Token == "" {
		bad("telegram.token: required")
	}
	if len(c.Telegram.ChatIDs) == 0 {
		bad("telegram.chat_id: required")
	}
	seenChat := map[int64]bool{}
	for _, id := range c.Telegram.ChatIDs {
		switch {
		case id == 0:
			bad("telegram.chat_id: 0 is not a chat id")
		case seenChat[id]:
			bad("telegram.chat_id: %d listed twice", id)
		}
		seenChat[id] = true
	}
	oneOf("telegram.mode", c.Telegram.Mode, "webhook", "polling")
	if c.Telegram.Mode == "webhook" {
		if c.Telegram.WebhookSecret == "" {
			bad("telegram.webhook_secret: required in webhook mode")
		}
		if c.Server.PublicBaseURL == "" {
			bad("server.public_base_url: required in webhook mode")
		}
	}
	validThreads("telegram.threads", c.Telegram.Threads)

	if c.Server.PublicBaseURL != "" {
		validURL("server.public_base_url", c.Server.PublicBaseURL)
	}
	validPath("server.gitlab_webhook_path", c.Server.GitLabWebhookPath)
	validPath("server.telegram_webhook_path", c.Server.TelegramWebhookPath)
	if c.Server.GitLabWebhookPath == c.Server.TelegramWebhookPath {
		bad("server: gitlab_webhook_path and telegram_webhook_path must differ")
	}

	validURL("gitlab.base_url", c.GitLab.BaseURL)
	if c.GitLab.Group == "" {
		bad("gitlab.group: required")
	}
	if c.GitLab.WebhookSecret == "" {
		bad("gitlab.webhook_secret: required")
	}

	oneOf("logging.level", c.Logging.Level, "debug", "info", "warn", "error")
	oneOf("logging.format", c.Logging.Format, "text", "json")

	errs = append(errs, c.Defaults.validate("defaults")...)
	seen := map[string]bool{}
	for i := range c.Projects {
		p := &c.Projects[i]
		field := fmt.Sprintf("projects[%d]", i)
		if p.Path == "" {
			bad("%s.path: required", field)
		} else if seen[p.Path] {
			bad("%s.path: duplicate %q", field, p.Path)
		}
		seen[p.Path] = true
		errs = append(errs, p.Settings.validate(field)...)
		validThreads(field+".threads", p.Threads)
	}
	for name, id := range c.Users {
		if id <= 0 {
			bad("users.%s: telegram user id must be positive, got %d", name, id)
		}
	}
	return errs
}

func (s *Settings) validate(field string) []error {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(field+"."+format, args...))
	}
	for _, e := range s.Events {
		if !e.Valid() {
			bad("events: unknown event class %q", e)
		}
	}
	if s.Verbosity != "" && !slices.Contains([]string{"quiet", "normal", "verbose"}, s.Verbosity) {
		bad("verbosity: %q must be one of quiet|normal|verbose", s.Verbosity)
	}
	if s.Pipelines.ChildCards != "" && !slices.Contains([]string{"inline", "own", "both"}, s.Pipelines.ChildCards) {
		bad("pipelines.child_cards: %q must be one of inline|own|both", s.Pipelines.ChildCards)
	}
	if s.Pipelines.LogTail.Lines != nil && (*s.Pipelines.LogTail.Lines < 0 || *s.Pipelines.LogTail.Lines > 50) {
		bad("pipelines.log_tail.lines: must be 0..50, got %d", *s.Pipelines.LogTail.Lines)
	}
	if s.Push.MaxCommits != nil && *s.Push.MaxCommits < 1 {
		bad("push.max_commits: must be at least 1, got %d", *s.Push.MaxCommits)
	}
	for _, err := range s.Branches.compile() {
		bad("%w", err)
	}
	return errs
}
