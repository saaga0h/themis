package runner

// Review findings classification and formatting: the blocking-severity threshold,
// JSON-over-stdout sourcing, the severity table, and blocking-finding formatting.

import (
	"context"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
)

// TestBlockingThreshold_ValueIsMedium asserts the BlockingThreshold constant
// in the review package has the exact string value "medium".
func TestBlockingThreshold_ValueIsMedium(t *testing.T) {
	const want = "medium"
	if review.BlockingThreshold != want {
		t.Errorf("review.BlockingThreshold = %q, want %q", review.BlockingThreshold, want)
	}
}

// TestDeriveStepResult_ReviewStep_ReadsJSONNotStdout verifies that when
// .themis/review-results.json contains a critical finding and stdout is empty,
// the runner detects blocking findings from the JSON file.
func TestDeriveStepResult_ReviewStep_ReadsJSONNotStdout(t *testing.T) {
	workDir := t.TempDir()
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "critical", Description: "SQL injection vulnerability", File: "db.go", Line: 42},
	})

	result := &agent.InvokeResult{
		ExitCode:  0,
		Completed: true,
		Stdout:    "", // stdout is empty — blocking must come from JSON
	}

	cfg := Config{
		WorkDir: workDir,
	}

	sr := deriveStepResult(context.Background(), pipeline.StepReview, result, cfg)
	if !sr.BlockingFindings {
		t.Error("BlockingFindings must be true when review-results.json contains a critical finding, even with empty stdout")
	}
}

// TestDeriveStepResult_ReviewStep_StdoutAloneDoesNotTriggerBlocking verifies
// that stdout containing the old "BLOCKING_FINDINGS: YES" marker does not
// trigger blocking when .themis/review-results.json has no blocking findings.
func TestDeriveStepResult_ReviewStep_StdoutAloneDoesNotTriggerBlocking(t *testing.T) {
	workDir := t.TempDir()

	result := &agent.InvokeResult{
		ExitCode:  0,
		Completed: true,
		Stdout:    "BLOCKING_FINDINGS: YES\nSecurity issue found.",
	}

	cfg := Config{
		WorkDir: workDir,
	}

	// Writing an EMPTY findings JSON (no critical entries) alongside the same
	// stdout makes blocking FALSE — proving stdout is not read.
	writeReviewResults(t, workDir, []review.ReviewFinding{})

	srWithEmptyJSON := deriveStepResult(context.Background(), pipeline.StepReview, result, cfg)
	if srWithEmptyJSON.BlockingFindings {
		t.Error("BlockingFindings must be false when review-results.json has no findings, even when stdout contains old BLOCKING_FINDINGS marker")
	}
}

// TestDetermineBlockingStatus_SeverityTable verifies severity thresholds.
// critical, high, medium → blocking=true; low → blocking=false.
func TestDetermineBlockingStatus_SeverityTable(t *testing.T) {
	cases := []struct {
		severity string
		want     bool
	}{
		{"critical", true},
		{"high", true},
		{"medium", true},
		{"low", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.severity, func(t *testing.T) {
			findings := []review.ReviewFinding{
				{Severity: tc.severity, Description: "test finding"},
			}
			got := review.DetermineBlockingStatus(findings)
			if got != tc.want {
				t.Errorf("review.DetermineBlockingStatus([{severity:%q}]) = %v, want %v", tc.severity, got, tc.want)
			}
		})
	}
}

// TestExtractBlockingFindings_FormatsOrderedBySeverity verifies that given a
// ReviewResults with mixed-severity findings, the formatted output lists
// critical before high before medium, omits low, and is human-readable (not JSON).
func TestExtractBlockingFindings_FormatsOrderedBySeverity(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "low", Description: "minor style nit"},
		{Severity: "medium", Description: "medium risk issue"},
		{Severity: "critical", Description: "critical security hole"},
		{Severity: "high", Description: "high risk bug"},
	}

	formatted := review.FormatBlockingFindings(findings)

	// Must not contain low-severity finding.
	if strings.Contains(formatted, "minor style nit") {
		t.Error("formatted blocking findings must omit low-severity findings")
	}

	// Must contain all blocking severities.
	for _, desc := range []string{"critical security hole", "high risk bug", "medium risk issue"} {
		if !strings.Contains(formatted, desc) {
			t.Errorf("formatted findings missing description %q:\n%s", desc, formatted)
		}
	}

	// Must be human-readable, not raw JSON.
	if strings.HasPrefix(strings.TrimSpace(formatted), "{") || strings.HasPrefix(strings.TrimSpace(formatted), "[") {
		t.Errorf("formatted findings must not be raw JSON:\n%s", formatted)
	}

	// critical must appear before high, high before medium.
	critIdx := strings.Index(formatted, "critical security hole")
	highIdx := strings.Index(formatted, "high risk bug")
	medIdx := strings.Index(formatted, "medium risk issue")

	if critIdx < 0 || highIdx < 0 || medIdx < 0 {
		t.Fatalf("expected all three descriptions in output:\n%s", formatted)
	}
	if critIdx >= highIdx {
		t.Errorf("critical must appear before high in formatted output:\n%s", formatted)
	}
	if highIdx >= medIdx {
		t.Errorf("high must appear before medium in formatted output:\n%s", formatted)
	}
}
