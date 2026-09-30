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

// Button is one inline keyboard button carrying callback data.
type Button struct {
	Text string `json:"text"`
	Data string `json:"data,omitempty"`
}

// Message is a rendered Telegram message: HTML body plus inline keyboard
// rows (nil when the message has no buttons).
type Message struct {
	// HTML is Bot API HTML for classic messages; Rich is rich message HTML
	// (headings, tables, details, footers, in-message buttons). Exactly one
	// is set.
	HTML     string
	Rich     string
	Keyboard [][]Button
}

// Hash returns the hex SHA-256 of the HTML followed by the canonical JSON
// encoding of the keyboard, for change detection before editing.
func (m Message) Hash() string {
	h := sha256.New()
	h.Write([]byte(m.HTML))
	h.Write([]byte(m.Rich))
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
	// MaxLen is the message length limit; 0 means DefaultMaxLen. Messages
	// are truncated to MaxLen-Margin.
	MaxLen int
	// ShowDescription includes MR and issue descriptions as an expandable
	// blockquote (config mr.show_description).
	ShowDescription bool
	// Location is the zone for clock times such as "started 14:05"; nil is
	// UTC.
	Location *time.Location
	// Caps decides which in-message operation buttons are drawn; nil draws
	// none.
	Caps actions.Capabilities
}

func (o Options) can(k actions.Kind, a actions.Action) bool {
	return o.Caps != nil && o.Caps.Can(k, a)
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

// lead is every card's first block: who (bold handle) did what in which
// project, in small text, then a separator. verb is the past-tense phrase
// between the actor and the linked project name, e.g. "pushed to".
func (o Options) lead(d *htmlfmt.Doc, actor event.User, verb string, p event.Project) {
	name := p.Name
	if name == "" {
		name = p.Path
	}
	project := htmlfmt.Esc(name)
	if p.WebURL != "" {
		project = htmlfmt.A(name, p.WebURL)
	}
	line := htmlfmt.Esc(verb) + " " + project
	if !actor.IsZero() {
		line = "<b>" + o.handle(actor) + "</b> " + line
	}
	d.Block(htmlfmt.Footer(line))
	d.Block(htmlfmt.Divider())
}

// updated is the closing footer's timestamp fragment. Renders are pure, so
// the time is the card's last event time, passed in by the renderer.
func (o Options) updated(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return "Updated " + o.clock(t)
}

func (o Options) limit() int {
	max := o.MaxLen
	if max == 0 {
		max = DefaultMaxLen
	}
	return max - Margin
}

// user renders a user as an italic @handle, a Telegram mention when
// mapped; the display name in italics when the payload carries no handle.
func (o Options) user(u event.User) string {
	return "<i>" + o.handle(u) + "</i>"
}

// handle is the @handle (or display name) as a mention when mapped.
func (o Options) handle(u event.User) string {
	name := displayName(u)
	if u.Username != "" {
		name = "@" + u.Username
	}
	return htmlfmt.Mention(name, htmlfmt.MentionID(o.Mentions, u.Username))
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

// humanize turns a snake_case status into words.
func humanize(s string) string { return strings.ReplaceAll(s, "_", " ") }

// clip shortens text to max runes, ending in an ellipsis when it cut.
func clip(text string, max int) string {
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	return strings.TrimRight(string(r[:max-1]), " ") + "…"
}

// plural returns "n word" or "n words".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
