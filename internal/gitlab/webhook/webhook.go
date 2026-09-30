// Package webhook turns GitLab webhook deliveries (headers + JSON body) into
// normalized event values, verifies the shared secret token and derives a
// stable delivery key for deduplication.
//
// Routing is done on the body's object_kind (with event_type as a hint),
// falling back to the X-Gitlab-Event header only when object_kind is absent.
// Kinds the bot does not handle (wiki pages, emoji, access tokens, feature
// flags, members, subgroups, projects, vulnerabilities, milestones, anything
// unknown) yield ErrIgnored so the HTTP layer can answer 200 without doing
// any work.
package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// ErrIgnored is returned by Parse for deliveries the bot deliberately does
// not process. Callers should answer 200 and drop them.
var ErrIgnored = errors.New("webhook: event ignored")

// object_kind values GitLab sends for the hooks this bot handles.
const (
	kindPush         = "push"
	kindTagPush      = "tag_push"
	kindIssue        = "issue"
	kindWorkItem     = "work_item"
	kindNote         = "note"
	kindMergeRequest = "merge_request"
	kindPipeline     = "pipeline"
	kindBuild        = "build"
	kindDeployment   = "deployment"
	kindRelease      = "release"
)

// headerKinds maps X-Gitlab-Event header values to object_kind for payloads
// that lack object_kind.
var headerKinds = map[string]string{
	"Push Hook":               kindPush,
	"Tag Push Hook":           kindTagPush,
	"Issue Hook":              kindIssue,
	"Confidential Issue Hook": kindIssue,
	"Note Hook":               kindNote,
	"Confidential Note Hook":  kindNote,
	"Merge Request Hook":      kindMergeRequest,
	"Pipeline Hook":           kindPipeline,
	"Job Hook":                kindBuild,
	"Deployment Hook":         kindDeployment,
	"Release Hook":            kindRelease,
}

// Parse decodes one webhook body into its normalized event. eventHeader is
// the X-Gitlab-Event header value (used only when the body has no
// object_kind); received is stamped into the event's Meta. It returns an
// error wrapping ErrIgnored for kinds the bot does not handle and a plain
// error for malformed payloads.
func Parse(eventHeader string, body []byte, received time.Time) (event.Event, error) {
	var env struct {
		ObjectKind string `json:"object_kind"`
		EventType  string `json:"event_type"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("webhook: decode envelope: %w", err)
	}
	kind := env.ObjectKind
	if kind == "" {
		kind = headerKinds[eventHeader]
	}
	meta := event.Meta{Received: received}
	switch kind {
	case kindPush:
		return parsePush(body, meta)
	case kindTagPush:
		return parseTagPush(body, meta)
	case kindIssue, kindWorkItem:
		return parseIssue(body, env.EventType, meta)
	case kindNote:
		return parseNote(body, meta)
	case kindMergeRequest:
		return parseMergeRequest(body, meta)
	case kindPipeline:
		return parsePipeline(body, meta)
	case kindBuild:
		return parseJob(body, meta)
	case kindDeployment:
		return parseDeployment(body, meta)
	case kindRelease:
		return parseRelease(body, meta)
	case "":
		return nil, fmt.Errorf("%w: no object_kind (header %q)", ErrIgnored, eventHeader)
	}
	return nil, fmt.Errorf("%w: object_kind %q", ErrIgnored, kind)
}
