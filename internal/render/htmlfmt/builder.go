package htmlfmt

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Builder accumulates a Telegram HTML message line by line and knows the
// visible length of what it holds, counted the way Telegram does (UTF-16
// code units after entity parsing). Lines are the unit of truncation: every
// line must be self-contained HTML with balanced tags.
type Builder struct {
	lines []line
}

type line struct {
	html    string
	visible int
	// inner is set for blockquotes, which Truncate shortens before dropping
	// whole lines.
	inner      string
	quote      bool
	expandable bool
}

// Line appends one line of formatted HTML. An empty string is a blank line.
func (b *Builder) Line(html string) {
	b.lines = append(b.lines, line{html: html, visible: VisibleLen(html)})
}

// Quote appends a blockquote holding inner (already-formatted HTML, may span
// several lines), collapsed when expandable. Quotes are the first thing
// Truncate shortens.
func (b *Builder) Quote(inner string, expandable bool) {
	html := Blockquote(inner, expandable)
	b.lines = append(b.lines, line{html: html, visible: VisibleLen(html), inner: inner, quote: true, expandable: expandable})
}

// Len is the visible length of the message including line breaks.
func (b *Builder) Len() int {
	n := 0
	for i, l := range b.lines {
		if i > 0 {
			n++
		}
		n += l.visible
	}
	return n
}

// String joins the lines with newlines, untruncated.
func (b *Builder) String() string {
	parts := make([]string, len(b.lines))
	for i, l := range b.lines {
		parts[i] = l.html
	}
	return strings.Join(parts, "\n")
}

// Truncate returns the message shortened to at most limit visible units.
// Blockquotes are shortened first (at an inner line break when
// one falls in the second half of what fits, never inside a tag or entity),
// then whole lines are dropped from the end. When anything was cut a final
// "… read more" line pointing at moreURL is appended (just "…" when moreURL
// is empty).
func (b *Builder) Truncate(limit int, moreURL string) string {
	if b.Len() <= limit {
		return b.String()
	}
	more := "…"
	if moreURL != "" {
		more = "… " + A("read more", moreURL)
	}
	budget := limit - VisibleLen(more) - 1
	lines := make([]line, len(b.lines))
	copy(lines, b.lines)
	t := Builder{lines: lines}

	for i := len(t.lines) - 1; i >= 0 && t.Len() > budget; i-- {
		l := t.lines[i]
		if !l.quote {
			continue
		}
		allowed := l.visible - (t.Len() - budget) - 1
		if allowed <= 0 {
			t.lines = append(t.lines[:i], t.lines[i+1:]...)
			continue
		}
		prefix := prefixByVisible(l.inner, allowed)
		if nl := strings.LastIndex(prefix, "\n"); nl > len(prefix)/2 {
			prefix = prefix[:nl]
		}
		inner := strings.TrimRight(prefix, " \n")
		inner += closeOpen(inner) + "…"
		html := Blockquote(inner, l.expandable)
		t.lines[i] = line{html: html, visible: VisibleLen(html), inner: inner, quote: true, expandable: l.expandable}
	}
	for len(t.lines) > 0 && t.Len() > budget {
		t.lines = t.lines[:len(t.lines)-1]
	}
	for len(t.lines) > 0 && t.lines[len(t.lines)-1].html == "" {
		t.lines = t.lines[:len(t.lines)-1]
	}
	t.Line(more)
	return t.String()
}

// VisibleLen counts the UTF-16 code units of html as Telegram will display
// it: tags are skipped and each entity counts as one unit.
func VisibleLen(html string) int {
	n := 0
	for i := 0; i < len(html); {
		switch html[i] {
		case '<':
			end := strings.IndexByte(html[i:], '>')
			if end < 0 {
				return n + utf16Len(html[i:])
			}
			i += end + 1
		case '&':
			end := entityEnd(html, i)
			if end < 0 {
				n++
				i++
				continue
			}
			n++
			i = end
		default:
			r, size := utf8.DecodeRuneInString(html[i:])
			n += utf16.RuneLen(r)
			i += size
		}
	}
	return n
}

// prefixByVisible returns the longest prefix of html with at most max
// visible units whose end falls outside any tag or entity.
func prefixByVisible(html string, max int) string {
	n := 0
	for i := 0; i < len(html); {
		switch html[i] {
		case '<':
			end := strings.IndexByte(html[i:], '>')
			if end < 0 {
				return html[:i]
			}
			i += end + 1
		case '&':
			end := entityEnd(html, i)
			if end < 0 {
				end = i + 1
			}
			if n+1 > max {
				return html[:i]
			}
			n++
			i = end
		default:
			r, size := utf8.DecodeRuneInString(html[i:])
			if n+utf16.RuneLen(r) > max {
				return html[:i]
			}
			n += utf16.RuneLen(r)
			i += size
		}
	}
	return html
}

// closeOpen returns the closing tags needed to balance html, innermost
// first.
func closeOpen(html string) string {
	var open []string
	for i := 0; i < len(html); {
		if html[i] != '<' {
			i++
			continue
		}
		end := strings.IndexByte(html[i:], '>')
		if end < 0 {
			break
		}
		tag := html[i+1 : i+end]
		i += end + 1
		if strings.HasPrefix(tag, "/") {
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			continue
		}
		name := tag
		if sp := strings.IndexAny(name, " \t\n"); sp >= 0 {
			name = name[:sp]
		}
		open = append(open, name)
	}
	var sb strings.Builder
	for i := len(open) - 1; i >= 0; i-- {
		sb.WriteString("</" + open[i] + ">")
	}
	return sb.String()
}

// entityEnd returns the index just past the entity starting at i, or -1
// when the ampersand does not start a well-formed entity.
func entityEnd(html string, i int) int {
	end := strings.IndexByte(html[i:], ';')
	if end < 1 || end > 10 {
		return -1
	}
	for _, c := range html[i+1 : i+end] {
		if !(c == '#' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return -1
		}
	}
	return i + end + 1
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
