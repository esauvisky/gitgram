// Package render turns card state and one-shot events into Telegram HTML
// messages with inline keyboards. Rendering is pure and deterministic:
// identical input yields identical bytes, so the sender can hash a message
// and skip edits that would not change anything. Nothing here reads the
// clock or does I/O.
package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// DefaultMaxLen is Telegram's message text limit in UTF-16 units.
const DefaultMaxLen = 4096

// Margin is kept free under Options.MaxLen so entity-parsing differences
// between our count and Telegram's never push a message over the limit.
const Margin = 200

// Button is one inline keyboard button. Exactly one of URL or Data is set.
type Button struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
	Data string `json:"data,omitempty"`
}

// Message is a rendered Telegram message: HTML body plus inline keyboard
// rows (nil when the message has no buttons).
type Message struct {
	HTML     string
	Keyboard [][]Button
}

// Hash returns the hex SHA-256 of the HTML followed by the canonical JSON
// encoding of the keyboard, for change detection before editing.
func (m Message) Hash() string {
	h := sha256.New()
	h.Write([]byte(m.HTML))
	kb, err := json.Marshal(m.Keyboard)
	if err != nil {
		panic(err)
	}
	h.Write(kb)
	return hex.EncodeToString(h.Sum(nil))
}

// Options controls rendering for one project.
type Options struct {
	// Verbosity is quiet, normal or verbose; anything else renders as normal.
	Verbosity string
	// Mentions maps GitLab usernames to Telegram user ids.
	Mentions map[string]int64
	// Caps decides which action buttons are drawn; nil draws none.
	Caps actions.Capabilities
	// MaxLen is the message length limit; 0 means DefaultMaxLen. Messages
	// are truncated to MaxLen-Margin.
	MaxLen int
	// ShowDescription includes MR and issue descriptions as an expandable
	// blockquote (config mr.show_description).
	ShowDescription bool
	// Location is the zone for clock times such as "started 14:05"; nil is
	// UTC.
	Location *time.Location
}

func (o Options) limit() int {
	max := o.MaxLen
	if max == 0 {
		max = DefaultMaxLen
	}
	return max - Margin
}

func (o Options) can(k actions.Kind, a actions.Action) bool {
	return o.Caps != nil && o.Caps.Can(k, a)
}

// user renders a user's display name, as a Telegram mention when mapped.
func (o Options) user(u event.User) string {
	return htmlfmt.Mention(displayName(u), htmlfmt.MentionID(o.Mentions, u.Username))
}

// users renders a user list joined with ", ".
func (o Options) users(us []event.User) string {
	parts := make([]string, len(us))
	for i, u := range us {
		parts[i] = o.user(u)
	}
	return strings.Join(parts, ", ")
}

// clock formats a wall-clock time as HH:MM in Options.Location.
func (o Options) clock(t time.Time) string {
	loc := o.Location
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("15:04")
}

func displayName(u event.User) string {
	switch {
	case u.Name != "":
		return u.Name
	case u.Username != "":
		return "@" + u.Username
	}
	return "someone"
}

// actionButton encodes a callback button; it returns false when the
// callback cannot be encoded.
func actionButton(text string, cb actions.Callback) (Button, bool) {
	data, err := cb.Encode()
	if err != nil {
		return Button{}, false
	}
	return Button{Text: text, Data: data}, true
}

// humanize turns a snake_case status into words.
func humanize(s string) string { return strings.ReplaceAll(s, "_", " ") }

// plural returns "n word" or "n words".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
