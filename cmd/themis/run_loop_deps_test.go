package main

// Run-loop dependency gating: skipping issues whose dependency is still open,
// IsOpen/ListReadyIssues error handling, and skip-before-budget ordering.

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

func TestRunLoop_SkipsIssueThatDependsOnOpenIssue(t *testing.T) {
	issue2 := &tracker.IssueData{
		Number: 2,
		Title:  "Issue 2",
		Body:   "depends on #5\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	var processed []int
	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:  []*tracker.IssueData{makeReadyIssues(1)[0], issue2},
			openMap: map[int]bool{5: true}, // #5 is open
		},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			return nil
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	for _, n := range processed {
		if n == 2 {
			t.Error("issue 2 must be skipped because it depends on open issue #5")
		}
	}

	found1 := false
	for _, n := range processed {
		if n == 1 {
			found1 = true
		}
	}
	if !found1 {
		t.Errorf("issue 1 (no dependency) must be processed; processed: %v", processed)
	}
}

func TestRunLoop_DoesNotSkipIssueWhoseDependencyIsClosed(t *testing.T) {
	issue1 := &tracker.IssueData{
		Number: 1,
		Title:  "Issue 1",
		Body:   "depends on #99\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	var processed []int
	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:  []*tracker.IssueData{issue1},
			openMap: map[int]bool{99: false}, // #99 is closed
		},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			return nil
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if len(processed) != 1 || processed[0] != 1 {
		t.Errorf("issue 1 must be processed when its dependency #99 is closed; processed: %v", processed)
	}
}

func TestRunLoop_LogsSkippedDependency(t *testing.T) {
	var log bytes.Buffer

	issue1 := &tracker.IssueData{
		Number: 1,
		Title:  "Issue 1",
		Body:   "depends on #99\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:  []*tracker.IssueData{issue1},
			openMap: map[int]bool{99: true},
		},
		RunFn:  func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if !bytes.Contains(log.Bytes(), []byte("1")) {
		t.Errorf("log must mention skipped issue #1; got: %s", log.String())
	}
	if !bytes.Contains(log.Bytes(), []byte("99")) {
		t.Errorf("log must mention open dependency #99; got: %s", log.String())
	}
}

// AC1 & AC2: When IsOpen returns an error for a dependency check, runLoop logs a
// message containing the issue number and the dependency number, skips the issue,
// and continues to the next issue — it does not return an error.
func TestRunLoop_IsOpenError_SkipsIssueAndLogsDetails(t *testing.T) {
	var log bytes.Buffer

	issue1 := &tracker.IssueData{
		Number: 1,
		Title:  "Issue 1",
		Body:   "depends on #5\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}
	issue2 := &tracker.IssueData{
		Number: 2,
		Title:  "Issue 2",
		Body:   "## AC\n- [ ] Something else",
		Labels: []string{"ready-for-agent"},
	}

	var processed []int
	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:    []*tracker.IssueData{issue1, issue2},
			isOpenErr: fmt.Errorf("network timeout"),
		},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	// AC1: must not return an error when IsOpen fails
	err := runLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("runLoop must not return error when IsOpen fails; got: %v", err)
	}

	// AC1: issue 1 must not be processed (it should be skipped)
	for _, n := range processed {
		if n == 1 {
			t.Error("issue 1 must be skipped when IsOpen returns an error — it must not be processed")
		}
	}

	// AC2: issue 2 must still be processed after issue 1 is skipped
	found2 := false
	for _, n := range processed {
		if n == 2 {
			found2 = true
		}
	}
	if !found2 {
		t.Errorf("issue 2 must be processed after issue 1 is dep-skipped; processed: %v", processed)
	}

	// AC1: log message must mention the skipped issue number and the dependency number
	logStr := log.String()
	if !bytes.Contains(log.Bytes(), []byte("#1")) {
		t.Errorf("log must mention skipped issue #1; got: %s", logStr)
	}
	if !bytes.Contains(log.Bytes(), []byte("#5")) {
		t.Errorf("log must mention dependency #5; got: %s", logStr)
	}
}

// AC3: The dependency-skip check (depends on #N) runs before the turn-budget check
// (RemainingFraction() < 0.10) so a dep-skippable issue does not trigger early exit.
func TestRunLoop_DependencySkipRunsBeforeTurnBudgetCheck(t *testing.T) {
	var log bytes.Buffer

	issue1 := &tracker.IssueData{
		Number: 1,
		Title:  "Issue 1",
		Body:   "depends on #5\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	turns := &stubTurns{fractions: []float64{0.05}} // below 10% — would trigger early exit if checked

	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:  []*tracker.IssueData{issue1},
			openMap: map[int]bool{5: true}, // #5 is open → issue 1 is dep-eligible for skip
		},
		RunFn:  func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:  turns,
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()
	// If the budget check fires before the dep check, the loop exits with "insufficient turns remaining".
	// If the dep check fires first, issue 1 is dep-skipped and the budget is never consulted.
	if bytes.Contains(log.Bytes(), []byte("insufficient turns remaining")) {
		t.Errorf("turn-budget check fired before dependency check — dep-eligible issue consumed the budget; log: %s", logStr)
	}

	// The dep-skip message must appear: the issue was handled by the dep check, not the budget check.
	if !bytes.Contains(log.Bytes(), []byte("1")) {
		t.Errorf("log must mention dep-skipped issue #1; got: %s", logStr)
	}

	// RemainingFraction must not have been called for the dep-skipped issue.
	if turns.calls != 0 {
		t.Errorf("RemainingFraction must not be called for a dep-skipped issue; got %d call(s)", turns.calls)
	}
}

// AC3 (error branch): When IsOpen returns an error for a dependency, the dep-skip
// fires before the turn-budget check, so RemainingFraction is never called.
func TestRunLoop_IsOpenError_DepSkipRunsBeforeTurnBudgetCheck(t *testing.T) {
	var log bytes.Buffer

	issue1 := &tracker.IssueData{
		Number: 1,
		Title:  "Issue 1",
		Body:   "depends on #5\n## AC\n- [ ] Something",
		Labels: []string{"ready-for-agent"},
	}

	turns := &stubTurns{fractions: []float64{0.05}} // below 10% — would trigger early exit if checked

	cfg := loopConfig{
		Querier: &stubQuerier{
			issues:    []*tracker.IssueData{issue1},
			isOpenErr: fmt.Errorf("network timeout"),
		},
		RunFn:  func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:  turns,
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop must not return error when IsOpen fails; got: %v", err)
	}

	// The dep-skip (error branch) must fire before the turn-budget check.
	if turns.calls != 0 {
		t.Errorf("RemainingFraction must not be called for an IsOpen-error dep-skip; got %d call(s)", turns.calls)
	}

	// The early-exit log must not appear — the budget was not consulted.
	if bytes.Contains(log.Bytes(), []byte("insufficient turns remaining")) {
		t.Errorf("turn-budget check fired before IsOpen-error dep-skip; log: %s", log.String())
	}
}

// AC4: When ListReadyIssues returns an error, runLoop returns a non-nil error
// containing the string "listing ready issues".
func TestRunLoop_ListReadyIssuesError_ReturnsWrappedError(t *testing.T) {
	cfg := loopConfig{
		Querier: &stubQuerier{
			listErr: fmt.Errorf("API unavailable"),
		},
		RunFn: func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	err := runLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("runLoop must return a non-nil error when ListReadyIssues fails")
	}
	if !strings.Contains(err.Error(), "listing ready issues") {
		t.Errorf("error must contain 'listing ready issues'; got: %v", err)
	}
}
