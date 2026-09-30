package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// envRef matches ${VAR} and ${VAR:-default}.
var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandEnv replaces every ${VAR} / ${VAR:-default} reference in s. A
// reference to an unset variable without a default is an error; all such
// references are reported together.
func expandEnv(s string) (string, error) {
	missing := map[string]bool{}
	out := envRef.ReplaceAllStringFunc(s, func(ref string) string {
		m := envRef.FindStringSubmatch(ref)
		name, def, hasDef := m[1], m[2], strings.Contains(ref, ":-")
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		if hasDef {
			return def
		}
		missing[name] = true
		return ""
	})
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for n := range missing {
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("unset environment variables: %s", strings.Join(names, ", "))
	}
	return out, nil
}
