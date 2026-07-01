package main

// Factory run-loop core: issue dispatch order, per-issue run invocation,
// continue-after-blocked behaviour, dry-run, and run-args wiring. Shared stubs
// (stubQuerier, stubTurns, makeReadyIssues) live here.

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
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
	cfg, err := newIssueConfig(ctx, 1, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, &cmdGitOps{}, parsed.maxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.MaxTurns != wantMaxTurns {
		t.Errorf("runner.Config.MaxTurns = %d, want %d — runRun must pass parsed.maxTurns to newIssueConfig", cfg.MaxTurns, wantMaxTurns)
	}
}
