package review_test

// Tests for the internal/review package. This file FAILS TO COMPILE until
// the review package is implemented — that is the correct RED state.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/review"
)

// ---------------------------------------------------------------------------
// AC1 / AC7: struct types and JSON round-trip
// ---------------------------------------------------------------------------

func TestReview_PackageExports_ReviewFindingType(t *testing.T) {
	f := review.ReviewFinding{
		Severity:    "high",
		Description: "some problem",
		File:        "pkg/foo.go",
		Line:        42,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got review.ReviewFinding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.Severity != f.Severity {
		t.Errorf("Severity: got %q, want %q", got.Severity, f.Severity)
	}
	if got.Description != f.Description {
		t.Errorf("Description: got %q, want %q", got.Description, f.Description)
	}
	if got.File != f.File {
		t.Errorf("File: got %q, want %q", got.File, f.File)
	}
	if got.Line != f.Line {
		t.Errorf("Line: got %d, want %d", got.Line, f.Line)
	}
}

func TestReview_PackageExports_ReviewFinding_OmitsFileAndLineWhenZero(t *testing.T) {
	f := review.ReviewFinding{
		Severity:    "low",
		Description: "minor note",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, `"file"`) {
		t.Errorf("empty File should be omitted in JSON; got: %s", raw)
	}
	if strings.Contains(raw, `"line"`) {
		t.Errorf("zero Line should be omitted in JSON; got: %s", raw)
	}
}

func TestReview_PackageExports_ReviewResultsType(t *testing.T) {
	rr := review.ReviewResults{
		Findings: []review.ReviewFinding{
			{Severity: "critical", Description: "crash bug"},
			{Severity: "low", Description: "naming nit"},
		},
	}

	data, err := json.Marshal(rr)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got review.ReviewResults
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if len(got.Findings) != 2 {
		t.Fatalf("Findings len: got %d, want 2", len(got.Findings))
	}
	if got.Findings[0].Severity != "critical" {
		t.Errorf("Findings[0].Severity: got %q, want %q", got.Findings[0].Severity, "critical")
	}
}

// ---------------------------------------------------------------------------
// AC1: ReadReviewResults
// ---------------------------------------------------------------------------

func TestReview_ReadReviewResults_ReturnsFindings(t *testing.T) {
	workDir := t.TempDir()
	themisDir := filepath.Join(workDir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	want := []review.ReviewFinding{
		{Severity: "high", Description: "dangerous call", File: "main.go", Line: 10},
		{Severity: "low", Description: "unused import"},
	}
	rr := review.ReviewResults{Findings: want}
	data, _ := json.Marshal(rr)
	if err := os.WriteFile(filepath.Join(themisDir, "review-results.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	findings, ok := review.ReadReviewResults(context.Background(), workDir)
	if !ok {
		t.Fatal("ReadReviewResults: ok = false, want true")
	}
	if len(findings) != len(want) {
		t.Fatalf("len(findings): got %d, want %d", len(findings), len(want))
	}
	if findings[0].Severity != want[0].Severity {
		t.Errorf("findings[0].Severity: got %q, want %q", findings[0].Severity, want[0].Severity)
	}
	if findings[0].File != want[0].File {
		t.Errorf("findings[0].File: got %q, want %q", findings[0].File, want[0].File)
	}
	if findings[0].Line != want[0].Line {
		t.Errorf("findings[0].Line: got %d, want %d", findings[0].Line, want[0].Line)
	}
}

func TestReview_ReadReviewResults_ReturnsFalseWhenMissing(t *testing.T) {
	workDir := t.TempDir()

	findings, ok := review.ReadReviewResults(context.Background(), workDir)
	if ok {
		t.Error("ReadReviewResults: ok = true, want false when file is missing")
	}
	if findings != nil {
		t.Errorf("ReadReviewResults: findings = %v, want nil when file is missing", findings)
	}
}

func TestReview_ReadReviewResults_ReturnsFalseOnInvalidJSON(t *testing.T) {
	workDir := t.TempDir()
	themisDir := filepath.Join(workDir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themisDir, "review-results.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	findings, ok := review.ReadReviewResults(context.Background(), workDir)
	if ok {
		t.Error("ReadReviewResults: ok = true on invalid JSON, want false")
	}
	if findings != nil {
		t.Errorf("ReadReviewResults: findings = %v on invalid JSON, want nil", findings)
	}
}

// ---------------------------------------------------------------------------
// AC1: CountFindingsBySeverity
// ---------------------------------------------------------------------------

func TestReview_CountFindingsBySeverity_BlockingAndNonBlocking(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "critical", Description: "crash"},
		{Severity: "high", Description: "security hole"},
		{Severity: "medium", Description: "important issue"},
		{Severity: "low", Description: "style nit"},
		{Severity: "low", Description: "another nit"},
	}

	blocking, nonBlocking := review.CountFindingsBySeverity(findings)

	if blocking != 3 {
		t.Errorf("blocking: got %d, want 3 (critical + high + medium)", blocking)
	}
	if nonBlocking != 2 {
		t.Errorf("nonBlocking: got %d, want 2 (low + low)", nonBlocking)
	}
}

func TestReview_CountFindingsBySeverity_EmptySlice(t *testing.T) {
	blocking, nonBlocking := review.CountFindingsBySeverity(nil)
	if blocking != 0 {
		t.Errorf("blocking: got %d, want 0 for empty input", blocking)
	}
	if nonBlocking != 0 {
		t.Errorf("nonBlocking: got %d, want 0 for empty input", nonBlocking)
	}
}

func TestReview_CountFindingsBySeverity_AllNonBlocking(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "low", Description: "nit 1"},
		{Severity: "low", Description: "nit 2"},
	}
	blocking, nonBlocking := review.CountFindingsBySeverity(findings)
	if blocking != 0 {
		t.Errorf("blocking: got %d, want 0", blocking)
	}
	if nonBlocking != 2 {
		t.Errorf("nonBlocking: got %d, want 2", nonBlocking)
	}
}

// ---------------------------------------------------------------------------
// AC1: DetermineBlockingStatus — table-driven
// ---------------------------------------------------------------------------

func TestReview_DetermineBlockingStatus_SeverityTable(t *testing.T) {
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
			findings := []review.ReviewFinding{{Severity: tc.severity, Description: "test"}}
			got := review.DetermineBlockingStatus(findings)
			if got != tc.want {
				t.Errorf("DetermineBlockingStatus([{%s}]) = %v, want %v", tc.severity, got, tc.want)
			}
		})
	}
}

func TestReview_DetermineBlockingStatus_EmptyIsNotBlocking(t *testing.T) {
	if review.DetermineBlockingStatus(nil) {
		t.Error("DetermineBlockingStatus(nil) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// AC1: FormatBlockingFindings
// ---------------------------------------------------------------------------

func TestReview_FormatBlockingFindings_OrderedAndOmitsLow(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "low", Description: "style nit"},
		{Severity: "medium", Description: "medium issue"},
		{Severity: "high", Description: "high issue"},
		{Severity: "critical", Description: "crash bug"},
	}

	out := review.FormatBlockingFindings(findings)

	if strings.Contains(out, "style nit") {
		t.Error("FormatBlockingFindings must omit low-severity findings")
	}
	if !strings.Contains(out, "crash bug") {
		t.Error("FormatBlockingFindings must include critical findings")
	}
	if !strings.Contains(out, "high issue") {
		t.Error("FormatBlockingFindings must include high findings")
	}
	if !strings.Contains(out, "medium issue") {
		t.Error("FormatBlockingFindings must include medium findings")
	}

	critIdx := strings.Index(out, "crash bug")
	highIdx := strings.Index(out, "high issue")
	medIdx := strings.Index(out, "medium issue")

	if critIdx < 0 || highIdx < 0 || medIdx < 0 {
		t.Fatal("expected all three blocking findings in output")
	}
	if critIdx > highIdx {
		t.Errorf("critical must appear before high; critIdx=%d highIdx=%d", critIdx, highIdx)
	}
	if highIdx > medIdx {
		t.Errorf("high must appear before medium; highIdx=%d medIdx=%d", highIdx, medIdx)
	}
}

func TestReview_FormatBlockingFindings_IncludesFileAndLine(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "high", Description: "nil deref", File: "server.go", Line: 77},
	}

	out := review.FormatBlockingFindings(findings)

	if !strings.Contains(out, "server.go") {
		t.Errorf("FormatBlockingFindings must include File; got: %s", out)
	}
	if !strings.Contains(out, "77") {
		t.Errorf("FormatBlockingFindings must include Line; got: %s", out)
	}
}

func TestReview_FormatBlockingFindings_FileWithoutLine(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "high", Description: "bad call", File: "server.go"},
	}

	out := review.FormatBlockingFindings(findings)

	if !strings.Contains(out, "server.go") {
		t.Errorf("FormatBlockingFindings must include File when set; got: %s", out)
	}
	if strings.Contains(out, ":0") {
		t.Errorf("FormatBlockingFindings must omit ':0' when Line=0; got: %s", out)
	}
}

func TestReview_FormatBlockingFindings_IsHumanReadableNotJSON(t *testing.T) {
	findings := []review.ReviewFinding{
		{Severity: "critical", Description: "bad call"},
	}

	out := review.FormatBlockingFindings(findings)

	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Error("FormatBlockingFindings must produce human-readable text, not JSON")
	}
	if !strings.Contains(out, "bad call") {
		t.Errorf("FormatBlockingFindings must include the description; got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// AC2: BlockingThreshold exported constant
// ---------------------------------------------------------------------------

func TestReview_BlockingThreshold_ValueIsMedium(t *testing.T) {
	if review.BlockingThreshold != "medium" {
		t.Errorf("BlockingThreshold = %q, want %q", review.BlockingThreshold, "medium")
	}
}
