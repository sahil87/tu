package harness

import (
	"regexp"
	"strconv"
	"strings"
)

// Redact replaces every absolute path rooted in `home` (when non-empty) or in
// the generic /home/<user> and /Users/<user> roots with ~/redacted-<n>, where
// n starts at 1 and is assigned in first-seen order per distinct path within
// one call. It operates on raw bytes via regexp — never by decoding and
// re-encoding JSON — so byte fidelity of everything around a redacted path is
// preserved.
//
// A match runs from the path root up to the next `"` or whitespace, which is
// where a JSON string value containing a path ends. The custom home is a
// fixed prefix, so it binds only at a path-component boundary: the exact home
// or the home as a leading component, never a longer sibling name sharing the
// prefix (/srv/home/u must not match /srv/home/user/…). Strings that merely
// contain a path fragment without a leading slash — Claude's encoded project
// directory form like `-home-sahil-code-…` — are not absolute paths and are
// deliberately left untouched, as are hostnames, model names, and numbers.
func Redact(raw []byte, home string) ([]byte, int) {
	roots := make([]string, 0, 3)
	if home != "" {
		roots = append(roots, regexp.QuoteMeta(home))
	}
	roots = append(roots, `/home/[^/"\s]+`, `/Users/[^/"\s]+`)
	re := regexp.MustCompile(`(?:` + strings.Join(roots, "|") + `)[^"\s]*`)

	assigned := map[string]string{}
	replacements := 0
	out := make([]byte, 0, len(raw))
	last := 0
	for _, loc := range re.FindAllIndex(raw, -1) {
		token := raw[loc[0]:loc[1]]
		// A candidate beginning with the custom home is under it only when
		// the match is the exact home or continues with a separator.
		if home != "" && strings.HasPrefix(string(token), home) &&
			len(token) > len(home) && token[len(home)] != '/' {
			continue
		}
		replacements++
		key := string(token)
		if _, ok := assigned[key]; !ok {
			assigned[key] = "~/redacted-" + strconv.Itoa(len(assigned)+1)
		}
		out = append(out, raw[last:loc[0]]...)
		out = append(out, assigned[key]...)
		last = loc[1]
	}
	return append(out, raw[last:]...), replacements
}
