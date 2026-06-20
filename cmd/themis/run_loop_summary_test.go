package main

// Run-loop end-of-run summary: processed/blocked/skipped counts, placement
// relative to per-issue lines, and dry-run wording.

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

func TestRunLoop_Summary_AllProcessedNoBlocked(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn:   func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:   &stubTurns{fractions: []float64{1.0}},
		Logger:  &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	if !strings.Contains(logStr, "3 processed") {
		t.Errorf("summary must contain '3 processed'; got: %s", logStr)
	}
	// Either "0 blocked" is present, or blocked is omitted when zero — both are valid.
	// The test asserts the summary exists by requiring "3 processed".
}

func TestRunLoop_Summary_ProcessedAndBlocked(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			if issue.Number == 2 {
				return fmt.Errorf("cycle limit exceeded")
			}
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	if !strings.Contains(logStr, "2 processed, 1 blocked") {
		t.Errorf("summary must contain '2 processed, 1 blocked'; got: %s", logStr)
	}
}

func TestRunLoop_Summary_SkippedDependency(t *testing.T) {
	var log bytes.Buffer

	issue2 := &tracker.IssueData{
		Number: 2,
		Title:  "Issue 2",
		Body:   "depends on #5\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:  []*tracker.IssueData{makeReadyIssues(1)[0], issue2},
			openMap: map[int]bool{5: true},
		},
		RunFn:  func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	if !strings.Contains(logStr, "skipped (dependency)") {
		t.Errorf("summary must contain 'skipped (dependency)'; got: %s", logStr)
	}
	if !strings.Contains(logStr, "1 skipped (dependency)") {
		t.Errorf("summary must contain '1 skipped (dependency)'; got: %s", logStr)
	}
}

func TestRunLoop_Summary_SkippedTurns(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn:   func(_ context.Context, _ *tracker.IssueData) error { return nil },
		// 50% for issue 1, then 5% triggers early exit before issue 2 is reached.
		Turns:  &stubTurns{fractions: []float64{0.5, 0.05}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	if !strings.Contains(logStr, "skipped (turns)") {
		t.Errorf("summary must contain 'skipped (turns)'; got: %s", logStr)
	}
	if !strings.Contains(logStr, "2 skipped (turns)") {
		t.Errorf("summary must contain '2 skipped (turns)'; got: %s", logStr)
	}
}

func TestRunLoop_Summary_EmptyIssueList(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: nil},
		RunFn:   func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:   &stubTurns{fractions: []float64{1.0}},
		Logger:  &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	if !strings.Contains(logStr, "no ready-for-agent issues found") {
		t.Errorf("summary must contain 'no ready-for-agent issues found'; got: %s", logStr)
	}
}

func TestRunLoop_Summary_AppearsAfterPerIssueLines(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			if issue.Number == 1 {
				return fmt.Errorf("cycle limit exceeded")
			}
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(log.String(), "\n"), "\n")

	// Find the last per-issue "blocked:" line index.
	blockedIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "issue #") && strings.Contains(l, "blocked") {
			blockedIdx = i
		}
	}
	if blockedIdx == -1 {
		t.Fatalf("expected a 'blocked' per-issue line in log; got: %s", log.String())
	}

	// Find the summary line — it must contain "processed".
	summaryIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "processed") {
			summaryIdx = i
		}
	}
	if summaryIdx == -1 {
		t.Fatalf("summary line containing 'processed' not found; got: %s", log.String())
	}

	if summaryIdx <= blockedIdx {
		t.Errorf("summary line (index %d) must appear after last per-issue 'blocked' line (index %d); log:\n%s",
			summaryIdx, blockedIdx, log.String())
	}
}

func TestRunLoop_Summary_AppearsExactlyOnce(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			if issue.Number == 2 {
				return fmt.Errorf("cycle limit exceeded")
			}
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()

	// Mixed case (2 processed, 1 blocked) would have produced two summary lines
	// before the fix. There must now be exactly one "run summary:" line.
	count := strings.Count(logStr, "run summary:")
	if count != 1 {
		t.Errorf("'run summary:' must appear exactly once (got %d); log:\n%s", count, logStr)
	}
}

func TestRunLoop_Summary_DryRunCountsWouldProcess(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn:   func(_ context.Context, _ *tracker.IssueData) error { return nil },
		DryRun:  true,
		Turns:   &stubTurns{fractions: []float64{1.0}},
		Logger:  &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop --dry-run error: %v", err)
	}

	logStr := log.String()

	// Summary must reflect 3 issues as "would process" in dry-run mode.
	if !strings.Contains(logStr, "3 would process") {
		t.Errorf("dry-run summary must contain '3 would process'; got: %s", logStr)
	}

	// No issues were actually blocked in dry-run, so "blocked" must not appear in the summary.
	// (Per-issue dry-run lines say "would process", not "blocked".)
	lines := strings.Split(logStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "blocked") {
			t.Errorf("dry-run summary must not contain 'blocked'; offending line: %q; full log: %s", line, logStr)
		}
	}
}
