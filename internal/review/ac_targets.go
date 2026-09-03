package review

import "fmt"

// ACTarget maps one acceptance criterion to the test targets that cover it. It is
// produced by test-architect during TestRed and persisted to
// .themis/ac-targets.json, so AC coverage can be verified deterministically by the
// runner at Review time instead of re-derived by the review agent with a grep.
//
// A behavioral AC must have at least one target. A structural AC — negative,
// placement, or delegation — is covered by an issue-declared `check` block (#98),
// not a test, so it carries no target and is not checked here.
type ACTarget struct {
	Criterion string   `json:"criterion"`
	Kind      string   `json:"kind"` // "behavioral" | "structural"
	Targets   []string `json:"targets"`
}

// acTargets is the on-disk shape of .themis/ac-targets.json.
type acTargets struct {
	ACs []ACTarget `json:"acs"`
}

// KindStructural marks an AC that is verified by a check block rather than a test.
const KindStructural = "structural"

// UncoveredACFindings returns a high-severity finding for every behavioral AC that
// has no test target — the deterministic AC-coverage check. Structural ACs are
// skipped (their check blocks are validated separately). It is a pure function:
// the same mapping always yields the same findings, no model in the loop.
func UncoveredACFindings(targets []ACTarget) []ReviewFinding {
	var findings []ReviewFinding
	for _, t := range targets {
		if t.Kind == KindStructural {
			continue
		}
		if len(t.Targets) == 0 {
			findings = append(findings, ReviewFinding{
				Severity:    "high",
				Description: fmt.Sprintf("acceptance criterion has no test: %q", t.Criterion),
			})
		}
	}
	return findings
}
