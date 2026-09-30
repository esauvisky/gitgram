// Package htmlfmt holds the Telegram-HTML primitives render is built from:
// escaping, inline tags, mentions, duration formatting and a line Builder
// that truncates without ever cutting inside a tag.
//
// Convention: functions taking a text argument escape it; functions taking
// an inner argument expect already-formatted HTML.
package htmlfmt

import (
	"math"
	"strconv"
	"strings"
)

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// Esc escapes text for Telegram HTML (&, <, >, ").
func Esc(text string) string { return escaper.Replace(text) }

// A renders a link. text is escaped; url is escaped for the attribute.
func A(text, url string) string { return `<a href="` + Esc(url) + `">` + Esc(text) + `</a>` }

// B renders bold text.
func B(text string) string { return "<b>" + Esc(text) + "</b>" }

// I renders italic text.
func I(text string) string { return "<i>" + Esc(text) + "</i>" }

// Pre renders text as a code block; lang, when set, is the syntax
// highlighting hint clients honour (<code class="language-lang">).
func Pre(text, lang string) string {
	if lang == "" {
		return "<pre>" + Esc(text) + "</pre>"
	}
	return `<pre><code class="language-` + Esc(lang) + `">` + Esc(text) + "</code></pre>"
}

// S wraps escaped text in strikethrough.
func S(text string) string { return "<s>" + Esc(text) + "</s>" }

// Code renders inline code.
func Code(text string) string { return "<code>" + Esc(text) + "</code>" }

// Blockquote wraps already-formatted HTML in a blockquote, collapsible when
// expandable is true.
func Blockquote(inner string, expandable bool) string {
	if expandable {
		return "<blockquote expandable>" + inner + "</blockquote>"
	}
	return "<blockquote>" + inner + "</blockquote>"
}

// Mention renders name as a Telegram user mention when tgID is non-zero and
// as escaped text otherwise.
func Mention(name string, tgID int64) string {
	if tgID == 0 {
		return Esc(name)
	}
	return `<a href="tg://user?id=` + strconv.FormatInt(tgID, 10) + `">` + Esc(name) + "</a>"
}

// MentionID looks up a GitLab username in a username → Telegram id map,
// exact match first, then case-insensitively. It returns 0 when unmapped.
func MentionID(m map[string]int64, username string) int64 {
	if id, ok := m[username]; ok {
		return id
	}
	var (
		bestKey string
		bestID  int64
	)
	for k, id := range m {
		if strings.EqualFold(k, username) && (bestKey == "" || k < bestKey) {
			bestKey, bestID = k, id
		}
	}
	return bestID
}

// ShortSHA returns the first 8 characters of a commit SHA.
func ShortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// Dur formats a duration in seconds as 12s, 4m12s or 1h04m. It is the only
// place render turns a float into text.
func Dur(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	s := int64(math.Round(seconds))
	h, m, sec := s/3600, s%3600/60, s%60
	switch {
	case h > 0:
		return strconv.FormatInt(h, 10) + "h" + pad2(m) + "m"
	case m > 0:
		return strconv.FormatInt(m, 10) + "m" + pad2(sec) + "s"
	}
	return strconv.FormatInt(sec, 10) + "s"
}

// Clock formats a duration in seconds as m:ss or h:mm:ss, the way the
// cards show stage times.
func Clock(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	s := int64(math.Round(seconds))
	h, m, sec := s/3600, s%3600/60, s%60
	if h > 0 {
		return strconv.FormatInt(h, 10) + ":" + pad2(m) + ":" + pad2(sec)
	}
	return strconv.FormatInt(m, 10) + ":" + pad2(sec)
}

// Size formats bytes as B, KB, MB or GB with one decimal above KB.
func Size(b int64) string {
	switch {
	case b >= 1<<30:
		return strconv.FormatFloat(float64(b)/(1<<30), 'f', 1, 64) + " GB"
	case b >= 1<<20:
		return strconv.FormatFloat(float64(b)/(1<<20), 'f', 1, 64) + " MB"
	case b >= 1<<10:
		return strconv.FormatInt(b>>10, 10) + " KB"
	}
	return strconv.FormatInt(b, 10) + " B"
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
