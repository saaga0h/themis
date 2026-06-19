package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// stubQuerier implements IssueQuerier for testing.
type stubQuerier struct {
	issues    []*tracker.IssueData
	listErr   error
	openMap   map[int]bool // issue number -> isOpen
	isOpenErr error        // when set, returned for all IsOpen calls
}

func (s *stubQuerier) ListReadyIssues(_ context.Context) ([]*tracker.IssueData, error) {
	return s.issues, s.listErr
}

func (s *stubQuerier) IsOpen(_ context.Context, number int) (bool, error) {
	if s.isOpenErr != nil {
		return false, s.isOpenErr
	}
	if open, ok := s.openMap[number]; ok {
		return open, nil
	}
	return false, nil
}

// stubTurns returns configurable remaining fractions, repeating the last entry.
type stubTurns struct {
	fractions []float64
	calls     int
}

func (s *stubTurns) RemainingFraction() float64 {
	if s.calls < len(s.fractions) {
		f := s.fractions[s.calls]
		s.calls++
		return f
	}
	if len(s.fractions) > 0 {
		return s.fractions[len(s.fractions)-1]
	}
	return 1.0
}

func makeReadyIssues(numbers ...int) []*tracker.IssueData {
	var issues []*tracker.IssueData
	for _, n := range numbers {
		issues = append(issues, &tracker.IssueData{
			Number: n,
			Title:  fmt.Sprintf("Issue %d", n),
			Body:   "## AC\n- [ ] Do something",
			Labels: []string{"ready-for-agent"},
		})
	}
	return issues
}

// all open issues labeled ready-for-agent processed in ascending order

func TestRunLoop_ProcessesIssuesInAscendingOrder(t *testing.T) {
	var processed []int

	cfg := loopConfig{
		Querier: &stubQuerier{
			issues: makeReadyIssues(3, 1, 2), // deliberately unordered
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

	if len(processed) != 3 {
		t.Fatalf("processed %d issues, want 3; got %v", len(processed), processed)
	}
	for i, want := range []int{1, 2, 3} {
		if processed[i] != want {
			t.Errorf("issue at position %d: got %d, want %d", i, processed[i], want)
		}
	}
}

// each issue is processed by calling the same pipeline as themis issue (shared runner.Run)

func TestRunLoop_CallsRunFnForEachIssue(t *testing.T) {
	var called []int

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			called = append(called, issue.Number)
			return nil
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if len(called) != 3 {
		t.Errorf("RunFn called %d times, want 3; issues: %v", len(called), called)
	}
}

// if an issue blocks (cycle limit, test-fix limit), log failure and continue

func TestRunLoop_ContinuesAfterBlockedIssue(t *testing.T) {
	var processed []int

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			if issue.Number == 2 {
				return fmt.Errorf("cycle limit exceeded for issue #2")
			}
			return nil
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop must not return error after a blocked issue; got: %v", err)
	}

	if len(processed) != 3 {
		t.Fatalf("all 3 issues must be attempted (including blocked); processed: %v", processed)
	}
	found3 := false
	for _, n := range processed {
		if n == 3 {
			found3 = true
		}
	}
	if !found3 {
		t.Errorf("issue 3 must be processed after issue 2 blocked; processed: %v", processed)
	}
}

func TestRunLoop_LogsBlockedIssueFailure(t *testing.T) {
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			if issue.Number == 1 {
				return fmt.Errorf("test-fix limit exceeded")
			}
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if !bytes.Contains(log.Bytes(), []byte("1")) {
		t.Errorf("log must mention blocked issue #1; got: %s", log.String())
	}
}

// if issue has "depends on #N" in body and #N is still open, skip with log message

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

// --dry-run flag lists which issues would be processed without running them

func TestRunLoop_DryRunDoesNotCallRunFn(t *testing.T) {
	var log bytes.Buffer
	runCalled := false

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, _ *tracker.IssueData) error {
			runCalled = true
			return nil
		},
		DryRun: true,
		Turns:  &stubTurns{fractions: []float64{1.0}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop --dry-run error: %v", err)
	}

	if runCalled {
		t.Error("RunFn must not be called in --dry-run mode")
	}

	// The loop must still list the issues it would process
	for _, want := range []string{"1", "2", "3"} {
		if !bytes.Contains(log.Bytes(), []byte(want)) {
			t.Errorf("--dry-run must log issue %s; got: %s", want, log.String())
		}
	}
}

// the loop stops if fewer than 10% of max turns remain

func TestRunLoop_StopsImmediatelyWhenInsufficientTurnsRemain(t *testing.T) {
	var processed []int
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			return nil
		},
		Turns:  &stubTurns{fractions: []float64{0.05}}, // 5% < 10% threshold
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if len(processed) != 0 {
		t.Errorf("no issues should be processed when turns < 10%%; processed: %v", processed)
	}

	if !bytes.Contains(log.Bytes(), []byte("insufficient turns remaining")) {
		t.Errorf("log must contain 'insufficient turns remaining'; got: %s", log.String())
	}
}

func TestRunLoop_StopsAfterTurnsBudgetDropsBelow10Percent(t *testing.T) {
	var processed []int
	var log bytes.Buffer

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2, 3)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			processed = append(processed, issue.Number)
			return nil
		},
		// Issue 1 gets 50% remaining, then drops to 5% before issue 2
		Turns:  &stubTurns{fractions: []float64{0.5, 0.05}},
		Logger: &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if len(processed) != 1 || processed[0] != 1 {
		t.Errorf("only issue 1 should be processed; processed: %v", processed)
	}

	if !bytes.Contains(log.Bytes(), []byte("insufficient turns remaining")) {
		t.Errorf("log must contain 'insufficient turns remaining'; got: %s", log.String())
	}
}

// state file from each completed issue is preserved until the next issue's run clears it

func TestRunLoop_StateFileExistsAfterIssueCompletes(t *testing.T) {
	workDir := t.TempDir()

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			state := &pipeline.PipelineState{
				IssueNumber:     issue.Number,
				CurrentStep:     pipeline.StepShip,
				TestFixAttempts: map[string]int{},
			}
			return pipeline.SaveState(workDir, state)
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	stateFile := filepath.Join(workDir, ".themis", "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state file must exist after issue completes (loop must not delete it)")
	}
}

func TestRunLoop_StateFileFromPreviousIssueExistsWhenNextStarts(t *testing.T) {
	workDir := t.TempDir()
	var stateWhenIssue2Starts *pipeline.PipelineState

	cfg := loopConfig{
		Querier: &stubQuerier{issues: makeReadyIssues(1, 2)},
		RunFn: func(_ context.Context, issue *tracker.IssueData) error {
			if issue.Number == 1 {
				state := &pipeline.PipelineState{
					IssueNumber:     1,
					CurrentStep:     pipeline.StepShip,
					TestFixAttempts: map[string]int{},
				}
				return pipeline.SaveState(workDir, state)
			}
			// When issue 2 starts, state from issue 1 must still be on disk
			state, err := pipeline.LoadState(workDir)
			if err != nil {
				return fmt.Errorf("loading state: %w", err)
			}
			stateWhenIssue2Starts = state
			return nil
		},
		Turns: &stubTurns{fractions: []float64{1.0}},
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	if stateWhenIssue2Starts == nil {
		t.Fatal("state must exist when issue 2 starts (preserved from issue 1, not deleted by loop)")
	}
	if stateWhenIssue2Starts.IssueNumber != 1 {
		t.Errorf("state from issue 1 must be preserved; got issue number %d", stateWhenIssue2Starts.IssueNumber)
	}
}

// ---------------------------------------------------------------------------
// MaxTurns wiring: runArgs.maxTurns flows into runner.Config.MaxTurns (AC3)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Issue #43: resilience — runLoop skips on dependency check error
// ---------------------------------------------------------------------------

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
	if !strings.Contains(logStr, "2 processed") {
		t.Errorf("summary must contain '2 processed'; got: %s", logStr)
	}
	if !strings.Contains(logStr, "1 blocked") {
		t.Errorf("summary must contain '1 blocked'; got: %s", logStr)
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
		if strings.Contains(l, "blocked") {
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
		RunFn:   func(_ context.Context, _ *tracker.IssueData) error { return nil },
		Turns:   &stubTurns{fractions: []float64{1.0}},
		Logger:  &log,
	}

	if err := runLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runLoop error: %v", err)
	}

	logStr := log.String()

	// Count occurrences of a summary-specific marker. The summary line is the
	// only line that should contain "3 processed" — per-issue lines do not.
	count := strings.Count(logStr, "3 processed")
	if count != 1 {
		t.Errorf("'3 processed' must appear exactly once in log (got %d); log:\n%s", count, logStr)
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

	// Summary must reflect 3 issues as processed (or "would process").
	hasCount := strings.Contains(logStr, "3 processed") || strings.Contains(logStr, "3 would process")
	if !hasCount {
		t.Errorf("dry-run summary must contain '3 processed' or '3 would process'; got: %s", logStr)
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

// TestRunRun_MaxTurnsIsPassedFromRunArgsToRunnerConfig verifies that the maxTurns
// field on runArgs (populated by --max-turns) is forwarded into the runner.Config.MaxTurns
// built inside runRun's RunFn closure (AC3 wiring layer).
//
// Strategy: call newIssueConfig directly with a maxTurns value that would have come from
// parseRunArgs, and assert that runner.Config.MaxTurns equals that value. This mirrors
// what runRun must do: `issueCfg, err := newIssueConfig(ctx, ..., parsed.maxTurns)`.
func TestRunRun_MaxTurnsIsPassedFromRunArgsToRunnerConfig(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	const wantMaxTurns = 300

	// Simulate what runRun does: parse args then pass maxTurns to newIssueConfig.
	parsed, err := parseRunArgs([]string{"--provider", "gitea", "--max-turns", "300"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if parsed.maxTurns != wantMaxTurns {
		t.Fatalf("parseRunArgs maxTurns = %d, want %d", parsed.maxTurns, wantMaxTurns)
	}

	// newIssueConfig must accept maxTurns and write it into runner.Config.MaxTurns.
	cfg, err := newIssueConfig(ctx, 1, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, parsed.maxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.MaxTurns != wantMaxTurns {
		t.Errorf("runner.Config.MaxTurns = %d, want %d — runRun must pass parsed.maxTurns to newIssueConfig", cfg.MaxTurns, wantMaxTurns)
	}
}
