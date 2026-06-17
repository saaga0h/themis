package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// stubQuerier implements IssueQuerier for testing.
type stubQuerier struct {
	issues  []*tracker.IssueData
	listErr error
	openMap map[int]bool // issue number -> isOpen
}

func (s *stubQuerier) ListReadyIssues(_ context.Context) ([]*tracker.IssueData, error) {
	return s.issues, s.listErr
}

func (s *stubQuerier) IsOpen(_ context.Context, number int) (bool, error) {
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

// AC: all open issues labeled ready-for-agent processed in ascending order

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

// AC: each issue is processed by calling the same pipeline as themis issue (shared runner.Run)

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

// AC: if an issue blocks (cycle limit, test-fix limit), log failure and continue

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

// AC: if issue has "depends on #N" in body and #N is still open, skip with log message

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

// AC: --dry-run flag lists which issues would be processed without running them

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

// AC: the loop stops if fewer than 10% of max turns remain

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

// AC: state file from each completed issue is preserved until the next issue's run clears it

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
