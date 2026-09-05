package issuespec

import (
	"fmt"
	"regexp"
	"strings"
)

// Footprint is a parsed ` ```footprint ` declaration from an issue body — the
// change-surface contract for the issue (design #111). The author declares intent
// (a package allowlist, or `wide`); CheckCommand translates it to a green-gate
// check so the factory owns the git-diff grep rather than every author re-deriving
// it.
//
// Block grammar (inside a top-level ` ```footprint ` fence):
//   - a first meaningful line of `wide` (optionally `wide: <reason>` or
//     `wide <reason>`) waives the gate for a genuine sweep — Wide, with the reason
//     captured for the PR;
//   - otherwise each non-empty, non-`#` line is a package-path prefix the diff may
//     touch (e.g. `internal/tracker`).
type Footprint struct {
	Packages []string // declared package-path prefixes; empty when Wide or undeclared
	Wide     bool     // the `wide` escape — footprint gate waived for this issue
	Reason   string   // justification captured after `wide`
	declared bool
}

// Declared reports whether the issue carried a ` ```footprint ` block at all.
func (f Footprint) Declared() bool { return f.declared }

// ParseFootprint reads the top-level ` ```footprint ` block from body (the first
// one wins). Fenced examples nested inside other fences are ignored, same as
// ParseCheckBlocks. An absent block yields a zero Footprint with Declared() false.
func ParseFootprint(body string) Footprint {
	for _, f := range topLevelFences(body) {
		if f.info != "footprint" {
			continue
		}
		return parseFootprintContent(body[f.contentStart:f.contentEnd])
	}
	return Footprint{}
}

func parseFootprintContent(content string) Footprint {
	fp := Footprint{declared: true}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fp.Wide {
			continue // everything after `wide` is prose
		}
		if kw, rest, isWide := cutWide(line); isWide {
			fp.Wide = true
			fp.Reason = strings.TrimSpace(strings.TrimLeft(rest, ": "))
			_ = kw
			fp.Packages = nil
			continue
		}
		fp.Packages = append(fp.Packages, strings.TrimSuffix(line, "/"))
	}
	return fp
}

// cutWide reports whether line begins the `wide` escape, returning the remainder
// (the reason) when it does. Matches `wide`, `wide: x`, `wide x` case-insensitively.
func cutWide(line string) (kw, rest string, ok bool) {
	low := strings.ToLower(line)
	if low == "wide" {
		return "wide", "", true
	}
	if strings.HasPrefix(low, "wide:") || strings.HasPrefix(low, "wide ") {
		return line[:4], line[4:], true
	}
	return "", "", false
}

// CheckCommand renders the footprint as a green-gate check command (bash; exit 0 =
// the diff stays within the declared packages). Returns "" when the footprint is
// undeclared or Wide — no gate in those cases.
//
// The command honours the same BASE/COMMIT contract as cmd/backtest, so a
// generated footprint check is backtestable by construction: at runtime COMMIT is
// unset and it diffs the working branch against BASE; under backtest COMMIT is a
// historical commit and it diffs that commit. An empty BASE (no resolvable base
// branch) passes rather than false-blocking. exempt paths (e.g. go.mod, go.sum)
// are always allowed.
func (f Footprint) CheckCommand(exempt []string) string {
	if !f.declared || f.Wide || len(f.Packages) == 0 {
		return ""
	}
	var alts []string
	for _, p := range f.Packages {
		alts = append(alts, "^"+regexp.QuoteMeta(p)+"/") // package prefix
	}
	for _, e := range exempt {
		alts = append(alts, "^"+regexp.QuoteMeta(e)+"$") // exact file
	}
	// Always exempt factory/project meta: .themis/ (the factory's own state,
	// config, and scratch artifacts) and .gitignore. These are never feature
	// files, so an issue must not be footprint-blocked over them.
	alts = append(alts,
		"^"+regexp.QuoteMeta(".themis")+"/",    // ^\.themis/
		"^"+regexp.QuoteMeta(".gitignore")+"$", // ^\.gitignore$
	)
	allowed := strings.Join(alts, "|")
	return fmt.Sprintf(
		`if [ -z "$BASE" ]; then exit 0; fi; `+
			`out=$(git diff --name-only "$BASE" ${COMMIT} | grep -vE '%s' || true); `+
			`if [ -n "$out" ]; then echo "footprint: files outside declared packages [%s]:"; echo "$out"; exit 1; fi`,
		allowed, strings.Join(f.Packages, " "),
	)
}
