package main

// Run-loop stop conditions: the remaining-turns budget gate and per-issue
// pipeline-state file persistence across issues.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/tracker"
)

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
