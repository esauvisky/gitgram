package config

import "strings"

// EffectiveProject is the fully merged view of the settings that apply to
// one GitLab project: `defaults:` overlaid with the matching `projects[]`
// entry.
type EffectiveProject struct {
	Path      string
	Events    []EventClass
	Verbosity string
	Branches  Branches
	Pipelines PipelineSettings
	MR        MRSettings
	Push      PushSettings
	Threads   map[string]int64 // project-level thread overrides only

	cfg *Config
}

// PipelineSettings is the resolved `pipelines:` block.
type PipelineSettings struct {
	ChildCards   string
	QuietSuccess bool
}

// MRSettings is the resolved `mr:` block.
type MRSettings struct {
	CollapseNotes   bool
	ShowDescription bool
	ShowSystemNotes bool
}

// PushSettings is the resolved `push:` block.
type PushSettings struct {
	MaxCommits int
}

// Resolve returns the effective settings for a project identified by its
// path_with_namespace. Projects without an explicit entry get `defaults:`.
func (c *Config) Resolve(projectPath string) EffectiveProject {
	d := c.Defaults
	p := EffectiveProject{
		Path:      projectPath,
		Events:    d.Events,
		Verbosity: d.Verbosity,
		Branches:  d.Branches,
		Pipelines: PipelineSettings{ChildCards: d.Pipelines.ChildCards, QuietSuccess: *d.Pipelines.QuietSuccess},
		MR: MRSettings{
			CollapseNotes:   *d.MR.CollapseNotes,
			ShowDescription: *d.MR.ShowDescription,
			ShowSystemNotes: *d.MR.ShowSystemNotes,
		},
		Push: PushSettings{MaxCommits: *d.Push.MaxCommits},
		cfg:  c,
	}
	for i := range c.Projects {
		if c.Projects[i].Path == projectPath {
			p.apply(&c.Projects[i].Settings)
			break
		}
	}
	return p
}

func (p *EffectiveProject) apply(s *Settings) {
	if s.Events != nil {
		p.Events = s.Events
	}
	if s.Verbosity != "" {
		p.Verbosity = s.Verbosity
	}
	if s.Branches.Allow != nil {
		p.Branches.Allow, p.Branches.allow = s.Branches.Allow, s.Branches.allow
	}
	if s.Branches.Deny != nil {
		p.Branches.Deny, p.Branches.deny = s.Branches.Deny, s.Branches.deny
	}
	if s.Pipelines.ChildCards != "" {
		p.Pipelines.ChildCards = s.Pipelines.ChildCards
	}
	if s.Pipelines.QuietSuccess != nil {
		p.Pipelines.QuietSuccess = *s.Pipelines.QuietSuccess
	}
	if s.MR.CollapseNotes != nil {
		p.MR.CollapseNotes = *s.MR.CollapseNotes
	}
	if s.MR.ShowDescription != nil {
		p.MR.ShowDescription = *s.MR.ShowDescription
	}
	if s.MR.ShowSystemNotes != nil {
		p.MR.ShowSystemNotes = *s.MR.ShowSystemNotes
	}
	if s.Push.MaxCommits != nil {
		p.Push.MaxCommits = *s.Push.MaxCommits
	}
	p.Threads = s.Threads
}

// EventEnabled reports whether events of the given class are relayed for
// this project.
func (p EffectiveProject) EventEnabled(class EventClass) bool {
	for _, e := range p.Events {
		if e == class {
			return true
		}
	}
	return false
}

// BranchAllowed reports whether a ref (branch or tag name, with or without
// the refs/heads/ or refs/tags/ prefix) passes the allow/deny filter.
func (p EffectiveProject) BranchAllowed(ref string) bool {
	return p.Branches.Allowed(ref)
}

// ThreadFor returns the forum topic id for an event class of this project:
// the project's own `threads` (class, then "default"), then the global
// `telegram.threads`. 0 means the General topic.
func (p EffectiveProject) ThreadFor(class EventClass) int64 {
	if id := lookupThread(p.Threads, class); id != 0 {
		return id
	}
	return p.cfg.ThreadFor(class)
}

// ThreadFor returns the global forum topic id for an event class from
// `telegram.threads` (class, then "default"). 0 means the General topic.
func (c *Config) ThreadFor(class EventClass) int64 {
	return lookupThread(c.Telegram.Threads, class)
}

func lookupThread(threads map[string]int64, class EventClass) int64 {
	if id, ok := threads[string(class)]; ok {
		return id
	}
	return threads[ThreadDefault]
}

// TelegramIDFor maps a GitLab username (case-insensitive) to the Telegram
// user id from `users:`.
func (c *Config) TelegramIDFor(gitlabUsername string) (int64, bool) {
	id, ok := c.Users[strings.ToLower(gitlabUsername)]
	return id, ok
}

// Accepts reports whether a project path belongs to the configured group
// (subgroups included) or has an explicit `projects[]` entry.
func (c *Config) Accepts(projectPath string) bool {
	if strings.HasPrefix(projectPath, c.GitLab.Group+"/") {
		return true
	}
	for i := range c.Projects {
		if c.Projects[i].Path == projectPath {
			return true
		}
	}
	return false
}
