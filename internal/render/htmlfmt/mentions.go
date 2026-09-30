package htmlfmt

import (
	"regexp"
	"strings"
)

// mentionRE matches @username preceded by start of text or a character that
// cannot belong to a path, e-mail or word, so "a/b@c" and "x@y.z" are left
// alone. Group 1 is the prefix, group 2 the username.
var mentionRE = regexp.MustCompile(`(^|[^\w/.-])@([A-Za-z0-9_.-]+)`)

// RewriteMentions replaces @username occurrences in an already-escaped body
// with a Telegram user mention when the username is mapped in m and with
// inline code otherwise. Trailing dots are treated as punctuation, not part
// of the username.
func RewriteMentions(escapedBody string, m map[string]int64) string {
	idx := mentionRE.FindAllStringSubmatchIndex(escapedBody, -1)
	if len(idx) == 0 {
		return escapedBody
	}
	var sb strings.Builder
	sb.Grow(len(escapedBody) + 32*len(idx))
	last := 0
	for _, loc := range idx {
		prefix := escapedBody[loc[2]:loc[3]]
		name := escapedBody[loc[4]:loc[5]]
		trimmed := strings.TrimRight(name, ".")
		tail := name[len(trimmed):]
		sb.WriteString(escapedBody[last:loc[0]])
		sb.WriteString(prefix)
		if trimmed == "" {
			sb.WriteString("@" + name)
		} else if id := MentionID(m, trimmed); id != 0 {
			sb.WriteString(Mention("@"+trimmed, id))
			sb.WriteString(tail)
		} else {
			sb.WriteString("<code>@" + trimmed + "</code>")
			sb.WriteString(tail)
		}
		last = loc[1]
	}
	sb.WriteString(escapedBody[last:])
	return sb.String()
}
