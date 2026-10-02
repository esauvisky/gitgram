package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// timeLayouts are tried in order. GitLab emits ISO 8601 since 19.0; the
// space-separated forms are what older hooks (and some current docs samples)
// carry.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05 -0700",
}

// ts is a JSON timestamp that tolerates null and empty strings (left zero).
type ts struct{ time.Time }

// UnmarshalJSON implements json.Unmarshaler.
func (t *ts) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("timestamp: %w", err)
	}
	if s == "" {
		return nil
	}
	var firstErr error
	for _, layout := range timeLayouts {
		parsed, err := time.Parse(layout, s)
		if err == nil {
			t.Time = parsed
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return fmt.Errorf("timestamp %q: %w", s, firstErr)
}

// ptr returns nil for a zero timestamp, otherwise a pointer to the time.
func (t ts) ptr() *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.Time
	return &v
}

// rawProject is the project{} object shared by every payload.
type rawProject struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	WebURL            string `json:"web_url"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
}

func (p rawProject) event() event.Project {
	return event.Project{
		ID:            p.ID,
		Path:          p.PathWithNamespace,
		Name:          p.Name,
		WebURL:        strings.TrimSuffix(p.WebURL, "/"),
		DefaultBranch: p.DefaultBranch,
	}
}

// rawUser is the user{} object shared by most payloads.
type rawUser struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func (u rawUser) event() event.User {
	return event.User{ID: u.ID, Username: u.Username, Name: u.Name, AvatarURL: u.AvatarURL}
}

// rawCommit is the commit object in push, pipeline, MR (last_commit) and
// release payloads.
type rawCommit struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Timestamp ts     `json:"timestamp"`
	Author    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

func (c rawCommit) event() event.Commit {
	return event.Commit{
		SHA:       c.ID,
		Title:     firstLine(c.Title, c.Message),
		Message:   c.Message,
		URL:       c.URL,
		Author:    event.User{Name: c.Author.Name, Username: c.Author.Email},
		Timestamp: c.Timestamp.Time,
	}
}

func commits(raw []rawCommit) []event.Commit {
	if len(raw) == 0 {
		return nil
	}
	out := make([]event.Commit, len(raw))
	for i, c := range raw {
		out[i] = c.event()
	}
	return out
}

// firstLine returns title when set, otherwise the first line of message.
func firstLine(title, message string) string {
	if title != "" {
		return title
	}
	line, _, _ := strings.Cut(message, "\n")
	return strings.TrimSpace(line)
}

// rawLabel is one entry of labels[].
type rawLabel struct {
	Title string `json:"title"`
}

// rawSourcePipeline is source_pipeline{} in pipeline and job payloads.
type rawSourcePipeline struct {
	Project struct {
		ID                int64  `json:"id"`
		WebURL            string `json:"web_url"`
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	PipelineID int64 `json:"pipeline_id"`
	JobID      int64 `json:"job_id"`
}

func (s *rawSourcePipeline) event() *event.PipelineRef {
	if s == nil || s.PipelineID == 0 {
		return nil
	}
	return &event.PipelineRef{
		ProjectID:     s.Project.ID,
		ProjectPath:   s.Project.PathWithNamespace,
		ProjectWebURL: strings.TrimSuffix(s.Project.WebURL, "/"),
		PipelineID:    s.PipelineID,
		JobID:         s.JobID,
	}
}

// decode unmarshals body into v with a kind-tagged error.
func decode(body []byte, kind string, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("webhook: decode %s: %w", kind, err)
	}
	return nil
}
