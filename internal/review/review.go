package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// ctx is accepted per convention for I/O functions but is not passed to
// os.ReadFile, which has no context-aware variant.
func ReadReviewResults(_ context.Context, workDir string) ([]ReviewFinding, bool) {
	path := filepath.Join(workDir, ".themis", "review-results.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rr ReviewResults
	if err := json.Unmarshal(data, &rr); err != nil {
		return nil, false
	}
	return rr.Findings, true
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
