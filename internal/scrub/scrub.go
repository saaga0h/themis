// Package scrub redacts secrets and sensitive infrastructure values from strings
// before they leave the process — e.g. an error surfaced to a tracked issue
// comment or the telemetry sink. A git/transport error can embed the remote URL
// (credentials + internal host); this keeps such material out of anything the
// factory publishes.
package scrub

import (
	"regexp"
	"sort"
	"strings"
)

const redacted = "***"

// urlCredsRE matches the "://userinfo@" of a URL, so an embedded credential can
// be redacted while the scheme (before ://) and host (after @) stay readable.
var urlCredsRE = regexp.MustCompile(`://[^/@\s]+@`)

// New returns a scrubber that redacts, from any string: (1) each non-empty value
// in secrets, by exact substring match — pass tokens and sensitive hosts here;
// and (2) URL-embedded credentials, by pattern (a backstop for credentials this
// process doesn't hold verbatim). Empty values are ignored. The returned func is
// nil-safe to call and does nothing surprising on strings with no sensitive data.
func New(secrets ...string) func(string) string {
	// Non-empty, de-duplicated, longest-first so a value that contains a shorter
	// one as a substring is redacted before the shorter (avoids partial leaks).
	seen := map[string]bool{}
	var vals []string
	for _, s := range secrets {
		if s != "" && !seen[s] {
			seen[s] = true
			vals = append(vals, s)
		}
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })

	return func(in string) string {
		out := urlCredsRE.ReplaceAllString(in, "://"+redacted+"@")
		for _, v := range vals {
			out = strings.ReplaceAll(out, v, redacted)
		}
		return out
	}
}
