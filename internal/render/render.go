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

// Message is a rendered Telegram message: Bot API HTML body plus inline
// keyboard rows (nil when the message has no buttons).
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
	// MaxLen is the message length limit; 0 means DefaultMaxLen. Messages
	// are truncated to MaxLen-Margin.
	MaxLen int
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

// Every card is built from the same kinds of line, in classic Bot API
// HTML where italic stands in for small text and a blank line for a rule:
//   - the headline: who (bold handle, never a link) did what in which
//     project (branch); fixed once posted, no emoji;
//   - small lines, italic: the detail under it;
//   - the commits fold, an expandable quote.

// headline writes the card's title, which never changes once the card is
// posted: lead (who did what, formatted HTML), the preposition, the linked
// project name, and ref in parentheses when set (formatted HTML):
// `@ada pushed to demo (feat/x)`. No emoji, ever.
func headline(b *htmlfmt.Builder, lead, prep string, p event.Project, ref string) {
	name := p.Name
	if name == "" {
		name = p.Path
	}
	project := htmlfmt.Esc(name)
	if p.WebURL != "" {
		project = htmlfmt.A(name, p.WebURL)
	}
	line := lead + " " + prep + " " + project
	if ref != "" {
		line += " (" + ref + ")"
	}
	b.Line(line)
}

// tagRef is a tag as a code chip linked to its page.
func tagRef(p event.Project, tag string) string {
	chip := htmlfmt.Code(tag)
	if p.WebURL == "" {
		return chip
	}
	return `<a href="` + htmlfmt.Esc(p.WebURL+"/-/tags/"+tag) + `">` + chip + "</a>"
}

// branchRef is a branch as a code chip linked to its tree.
func branchRef(p event.Project, branch string) string {
	chip := htmlfmt.Code(branch)
	if p.WebURL == "" {
		return chip
	}
	return `<a href="` + htmlfmt.Esc(p.WebURL+"/-/tree/"+branch) + `">` + chip + "</a>"
}

// who is the actor's handle in bold, "Someone" when the payload names
// nobody.
func (o Options) who(u event.User) string {
	if u.IsZero() {
		return "Someone"
	}
	return "<b>" + o.handle(u) + "</b>"
}

// small writes an italic detail line; empty inner writes nothing. Italic
// runs inside inner (handles) are flattened into the line's own italics.
func small(b *htmlfmt.Builder, inner string) {
	if inner != "" {
		b.Line("<i>" + unitalic.Replace(inner) + "</i>")
	}
}

var unitalic = strings.NewReplacer("<i>", "", "</i>", "")

func (o Options) limit() int {
	max := o.MaxLen
	if max == 0 {
		max = DefaultMaxLen
	}
	return max - Margin
}

// handle is the escaped @handle, or the display name without one. A word
// joiner after the @ keeps Telegram from detecting the handle as a
// mention and linking it.
func (o Options) handle(u event.User) string {
	if u.Username != "" {
		return htmlfmt.Esc("@\u2060" + u.Username)
	}
	return htmlfmt.Esc(displayName(u))
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

// plural returns "n word" or "n words".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
