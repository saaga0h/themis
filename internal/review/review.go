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
