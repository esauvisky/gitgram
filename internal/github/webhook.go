// Package github turns GitHub webhook deliveries into the bot's events and
// talks to the GitHub REST API. Pushes become push events, Actions
// workflow runs and jobs become pipeline and job events (each job its own
// stage), and pull requests become merge request events, so the engine
// and the cards treat both hosts alike.
//
// GitHub repositories are stored with negative project ids (minus the
// repository id), so they never collide with GitLab projects in the
// store; event.Project.IsGitHub tells them apart.
package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// ErrIgnored is returned by Parse for deliveries the bot does not process.
var ErrIgnored = errors.New("github: event ignored")

// NewHandler returns the HTTP handler for the GitHub webhook endpoint. It
// rejects bodies over maxBody (413) and deliveries whose
// X-Hub-Signature-256 does not match secret (401), answers ping and
// ignored events with 200, and passes parsed events to handle keyed by
// X-GitHub-Delivery. Only an error from handle yields 500.
func NewHandler(secret string, maxBody int64, handle func(ctx context.Context, key string, ev event.Event) error, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		if !validSignature(r.Header.Get("X-Hub-Signature-256"), body, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		kind := r.Header.Get("X-GitHub-Event")
		key := "gh:" + r.Header.Get("X-GitHub-Delivery")
		ev, err := Parse(kind, body, time.Now())
		if err != nil {
			if errors.Is(err, ErrIgnored) {
				logger.Debug("github webhook: ignored", "event", kind, "reason", err)
			} else {
				logger.Warn("github webhook: unparsable payload", "event", kind, "err", err)
			}
			plain(w, "ignored")
			return
		}
		if err := handle(context.WithoutCancel(r.Context()), key, ev); err != nil {
			logger.Error("github webhook: handle failed", "kind", ev.EventKind(), "project", ev.Proj().Path, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		plain(w, "ok")
	})
}

func validSignature(header string, body []byte, secret string) bool {
	sig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	want, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(want, mac.Sum(nil))
}

func plain(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, msg+"\n")
}

// Parse decodes one delivery of the given X-GitHub-Event kind.
func Parse(kind string, body []byte, received time.Time) (event.Event, error) {
	meta := event.Meta{Received: received}
	switch kind {
	case "push":
		return parsePush(body, meta)
	case "workflow_run":
		return parseRun(body, meta)
	case "workflow_job":
		return parseJob(body, meta)
	case "pull_request":
		return parsePullRequest(body, meta)
	}
	return nil, fmt.Errorf("%w: %q", ErrIgnored, kind)
}

type rawUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func (u *rawUser) event() event.User {
	if u == nil || u.Login == "" {
		return event.User{}
	}
	return event.User{ID: u.ID, Username: u.Login, Name: u.Login}
}

type rawRepo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
}

func (r rawRepo) event() event.Project {
	return event.Project{ID: -r.ID, Path: r.FullName, Name: r.Name, WebURL: r.HTMLURL, DefaultBranch: r.DefaultBranch}
}

func commitTitle(message string) string {
	title, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	return strings.TrimSpace(title)
}

type rawPush struct {
	Ref     string  `json:"ref"`
	Before  string  `json:"before"`
	After   string  `json:"after"`
	Forced  bool    `json:"forced"`
	Repo    rawRepo `json:"repository"`
	Sender  rawUser `json:"sender"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		URL     string `json:"url"`
		Author  struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"author"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"commits"`
}

func parsePush(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawPush
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("github: decode push: %w", err)
	}
	if !strings.HasPrefix(raw.Ref, "refs/heads/") {
		return nil, fmt.Errorf("%w: push to %q", ErrIgnored, raw.Ref)
	}
	p := &event.Push{Meta: meta, Project: raw.Repo.event(), User: raw.Sender.event(), Ref: raw.Ref, Before: raw.Before, After: raw.After, Forced: raw.Forced}
	for _, c := range raw.Commits {
		p.Commits = append(p.Commits, event.Commit{SHA: c.ID, Title: commitTitle(c.Message), Message: c.Message, URL: c.URL,
			Author: event.User{Name: c.Author.Name, Username: c.Author.Username}, Timestamp: c.Timestamp})
	}
	p.TotalCommitsCount = len(p.Commits)
	return p, nil
}

// status maps a GitHub status and conclusion onto the bot's pipeline and
// job statuses.
func status(st, conclusion string) string {
	switch st {
	case "completed":
		switch conclusion {
		case "success", "neutral":
			return event.StatusSuccess
		case "cancelled":
			return event.StatusCanceled
		case "skipped", "stale":
			return event.StatusSkipped
		case "action_required":
			return event.StatusManual
		}
		return event.StatusFailed
	case "in_progress":
		return event.StatusRunning
	case "waiting":
		return event.StatusManual
	}
	return event.StatusPending
}

// source maps the event that started a run onto the bot's pipeline
// sources.
func source(ev string) string {
	switch ev {
	case "pull_request", "pull_request_target":
		return "merge_request_event"
	case "workflow_dispatch":
		return "web"
	}
	return ev
}

type rawRun struct {
	Run struct {
		ID              int64      `json:"id"`
		Name            string     `json:"name"`
		RunNumber       int64      `json:"run_number"`
		Event           string     `json:"event"`
		Status          string     `json:"status"`
		Conclusion      string     `json:"conclusion"`
		HeadBranch      string     `json:"head_branch"`
		HeadSHA         string     `json:"head_sha"`
		HTMLURL         string     `json:"html_url"`
		CreatedAt       time.Time  `json:"created_at"`
		UpdatedAt       time.Time  `json:"updated_at"`
		RunStartedAt    *time.Time `json:"run_started_at"`
		Actor           *rawUser   `json:"actor"`
		TriggeringActor *rawUser   `json:"triggering_actor"`
		PullRequests    []struct {
			Number int64 `json:"number"`
		} `json:"pull_requests"`
		HeadCommit struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"head_commit"`
	} `json:"workflow_run"`
	Repo rawRepo `json:"repository"`
}

func parseRun(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawRun
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("github: decode workflow_run: %w", err)
	}
	r := raw.Run
	project := raw.Repo.event()
	actor := r.TriggeringActor
	if actor == nil {
		actor = r.Actor
	}
	p := &event.Pipeline{Meta: meta, Project: project, User: actor.event(), ID: r.ID, IID: r.RunNumber, URL: r.HTMLURL,
		Ref: r.HeadBranch, SHA: r.HeadSHA, Source: source(r.Event), Status: status(r.Status, r.Conclusion), CreatedAt: r.CreatedAt,
		Commit: event.Commit{SHA: r.HeadCommit.ID, Title: commitTitle(r.HeadCommit.Message), Message: r.HeadCommit.Message,
			URL: project.WebURL + "/commit/" + r.HeadCommit.ID, Author: event.User{Name: r.HeadCommit.Author.Name}}}
	if r.Status == "completed" {
		end := r.UpdatedAt
		p.FinishedAt = &end
		if r.RunStartedAt != nil {
			d := int(end.Sub(*r.RunStartedAt).Seconds())
			p.Duration = &d
		}
	}
	if len(r.PullRequests) > 0 {
		n := r.PullRequests[0].Number
		p.MR = &event.MRRef{IID: n, URL: fmt.Sprintf("%s/pull/%d", project.WebURL, n)}
	}
	return p, nil
}

type rawJob struct {
	Job struct {
		ID          int64      `json:"id"`
		RunID       int64      `json:"run_id"`
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		HeadBranch  string     `json:"head_branch"`
		HeadSHA     string     `json:"head_sha"`
		HTMLURL     string     `json:"html_url"`
		CreatedAt   time.Time  `json:"created_at"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
	} `json:"workflow_job"`
	Repo   rawRepo `json:"repository"`
	Sender rawUser `json:"sender"`
}

func parseJob(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawJob
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("github: decode workflow_job: %w", err)
	}
	j := raw.Job
	ev := &event.Job{Meta: meta, Project: raw.Repo.event(), User: raw.Sender.event(), ID: j.ID, Name: j.Name, Stage: j.Name,
		Status: status(j.Status, j.Conclusion), CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, URL: j.HTMLURL,
		Ref: j.HeadBranch, SHA: j.HeadSHA, PipelineID: j.RunID}
	if j.Status == "completed" && j.CompletedAt != nil {
		ev.FinishedAt = j.CompletedAt
		if j.StartedAt != nil {
			d := j.CompletedAt.Sub(*j.StartedAt).Seconds()
			ev.Duration = &d
		}
	}
	return ev, nil
}

type rawPullRequest struct {
	Action string `json:"action"`
	PR     struct {
		ID             int64      `json:"id"`
		Number         int64      `json:"number"`
		Title          string     `json:"title"`
		Body           string     `json:"body"`
		HTMLURL        string     `json:"html_url"`
		State          string     `json:"state"`
		Draft          bool       `json:"draft"`
		Merged         bool       `json:"merged"`
		MergedAt       *time.Time `json:"merged_at"`
		MergeableState string     `json:"mergeable_state"`
		Additions      int        `json:"additions"`
		Deletions      int        `json:"deletions"`
		ChangedFiles   int        `json:"changed_files"`
		User           rawUser    `json:"user"`
		Head           struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Repo   rawRepo `json:"repository"`
	Sender rawUser `json:"sender"`
}

func parsePullRequest(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawPullRequest
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("github: decode pull_request: %w", err)
	}
	pr := raw.PR
	m := &event.MergeRequest{Meta: meta, Project: raw.Repo.event(), User: raw.Sender.event(), ID: pr.ID, IID: pr.Number,
		Title: pr.Title, Description: pr.Body, URL: pr.HTMLURL, Draft: pr.Draft, SourceBranch: pr.Head.Ref, TargetBranch: pr.Base.Ref,
		Author: pr.User.event(), MergedAt: pr.MergedAt, Additions: pr.Additions, Deletions: pr.Deletions, ChangedFiles: pr.ChangedFiles}
	switch {
	case pr.Merged:
		m.State = event.MRStateMerged
	case pr.State == "closed":
		m.State = event.MRStateClosed
	default:
		m.State = event.MRStateOpened
	}
	switch raw.Action {
	case "opened":
		m.Action = event.MRActionOpen
	case "reopened":
		m.Action = event.MRActionReopen
	case "closed":
		m.Action = event.MRActionClose
		if pr.Merged {
			m.Action = event.MRActionMerge
		}
	default:
		m.Action = event.MRActionUpdate
	}
	switch pr.MergeableState {
	case "clean", "has_hooks", "unstable":
		m.DetailedMergeStatus = event.MergeStatusMergeable
	case "dirty":
		m.DetailedMergeStatus = event.MergeStatusConflict
	default:
		m.DetailedMergeStatus = pr.MergeableState
	}
	return m, nil
}
