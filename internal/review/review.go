// Package review owns the structured output of the Review step: the finding
// types the review agent writes to .themis/review-results.json, the
// blocking-severity threshold, and pure functions that classify findings. Its
// filesystem reader is injected into the runner rather than called directly, to
// keep that coupling explicit and testable.
package review

import (
	"fmt"
	"strings"
)

const BlockingThreshold = "medium"

// ReviewFinding is a single finding produced by the review step.
type ReviewFinding struct {
	Severity    string `json:"severity"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// ReviewResults holds the structured output written by the review step agent.
type ReviewResults struct {
	Findings []ReviewFinding `json:"findings"`
}

// CountFindingsBySeverity partitions findings into blocking (critical, high, or
// the medium BlockingThreshold) and non-blocking counts.
func CountFindingsBySeverity(findings []ReviewFinding) (blocking, nonBlocking int) {
	for _, f := range findings {
		switch f.Severity {
		case "critical", "high", BlockingThreshold:
			blocking++
		default:
			nonBlocking++
		}
	}
	return
}

// DetermineBlockingStatus reports whether findings contain any blocking finding.
func DetermineBlockingStatus(findings []ReviewFinding) bool {
	blocking, _ := CountFindingsBySeverity(findings)
	return blocking > 0
}

// FormatBlockingFindings produces a human-readable list ordered critical → high → medium, omitting low.
func FormatBlockingFindings(findings []ReviewFinding) string {
	order := []string{"critical", "high", BlockingThreshold}
	var sb strings.Builder
	for _, sev := range order {
		for _, f := range findings {
			if f.Severity != sev {
				continue
			}
			fmt.Fprintf(&sb, "- %s: %s", strings.ToUpper(sev), f.Description)
			if f.File != "" {
				fmt.Fprintf(&sb, " (%s", f.File)
				if f.Line > 0 {
					fmt.Fprintf(&sb, ":%d", f.Line)
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
