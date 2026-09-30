package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

// matcher tests a branch or tag name against one pattern.
type matcher interface {
	Match(string) bool
}

type regexpMatcher struct{ *regexp.Regexp }

func (m regexpMatcher) Match(s string) bool { return m.MatchString(s) }

// compilePattern builds a matcher from a glob, or from a regular expression
// when the pattern starts with "re:".
func compilePattern(p string) (matcher, error) {
	if re, ok := strings.CutPrefix(p, "re:"); ok {
		r, err := regexp.Compile(re)
		if err != nil {
			return nil, err
		}
		return regexpMatcher{r}, nil
	}
	g, err := glob.Compile(p)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// compile builds the matcher lists. Every bad pattern is reported.
func (b *Branches) compile() []error {
	var errs []error
	build := func(field string, patterns []string) []matcher {
		ms := make([]matcher, 0, len(patterns))
		for _, p := range patterns {
			m, err := compilePattern(p)
			if err != nil {
				errs = append(errs, fmt.Errorf("branches.%s: pattern %q: %w", field, p, err))
				continue
			}
			ms = append(ms, m)
		}
		return ms
	}
	b.allow = build("allow", b.Allow)
	b.deny = build("deny", b.Deny)
	return errs
}

// refName strips the refs/heads/ or refs/tags/ prefix so patterns are
// written against plain branch and tag names.
func refName(ref string) string {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return strings.TrimPrefix(ref, "refs/tags/")
}

// Allowed reports whether ref matches an allow pattern and no deny pattern.
func (b *Branches) Allowed(ref string) bool {
	name := refName(ref)
	for _, m := range b.deny {
		if m.Match(name) {
			return false
		}
	}
	for _, m := range b.allow {
		if m.Match(name) {
			return true
		}
	}
	return false
}
