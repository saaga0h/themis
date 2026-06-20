package runner

// Per-step turn budgets and model selection (turnsForStep, modelForStep, and
// the --max-turns global ceiling).

import (
	"context"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
)

// TestRunner_MaxTurnsActsAsGlobalCeiling verifies that Config.MaxTurns is a hard
// ceiling: when it is lower than every per-step default, every agent invocation
// (all 5 agent steps plus the ship step) is clamped to that ceiling.
func TestRunner_MaxTurnsActsAsGlobalCeiling(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/maxturns"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.MaxTurns = 20 // below every per-step default — the ceiling binds everywhere

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// pipelineAgentCallCount (5) agent steps + 1 ship step = 6 total invocations.
	totalExpected := pipelineAgentCallCount + 1
	if len(inv.opts) < totalExpected {
		t.Fatalf("expected at least %d invocations, got %d", totalExpected, len(inv.opts))
	}
	for i, opts := range inv.opts {
		if opts.MaxTurns != 20 {
			t.Errorf("invocation %d: InvokeOptions.MaxTurns = %d, want 20 (ceiling)", i, opts.MaxTurns)
		}
	}
}

// TestRunner_PassesPerStepTurnsToShipStep verifies the ship step receives its
// per-step turn budget (60) rather than the larger global ceiling.
func TestRunner_PassesPerStepTurnsToShipStep(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/maxturns-ship"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.MaxTurns = 300 // above the ship default — the per-step cap (60) wins

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	totalExpected := pipelineAgentCallCount + 1
	if len(inv.opts) < totalExpected {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", totalExpected, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.MaxTurns != 60 {
		t.Errorf("ship-step InvokeOptions.MaxTurns = %d, want 60 (per-step cap)", shipOpts.MaxTurns)
	}
}

// TestTurnsForStep verifies the per-step turn budget: a step's default applies
// when it is below the global ceiling; otherwise the ceiling applies.
func TestTurnsForStep(t *testing.T) {
	tests := []struct {
		step     pipeline.Step
		maxTurns int
		want     int
	}{
		// Ceiling well above every default → each step gets its default.
		{pipeline.StepTestRed, 1000, 80},
		{pipeline.StepImplement, 1000, 120},
		{pipeline.StepRefactor, 1000, 30},
		{pipeline.StepReview, 1000, 80},
		{pipeline.StepFix, 1000, 60},
		{pipeline.StepDocs, 1000, 40},
		{pipeline.StepShip, 1000, 60},
		// Ceiling below every default → ceiling binds everywhere.
		{pipeline.StepImplement, 25, 25},
		{pipeline.StepTestRed, 25, 25},
		// A step with no per-step default falls through to the ceiling.
		{pipeline.StepBranch, 250, 250},
	}
	for _, tc := range tests {
		if got := turnsForStep(tc.step, tc.maxTurns); got != tc.want {
			t.Errorf("turnsForStep(%v, %d) = %d, want %d", tc.step, tc.maxTurns, got, tc.want)
		}
	}
}

// TestModelForStep_AllSteps verifies that modelForStep returns ReviewModel for
// StepReview, the fixed "haiku" tier for the mechanical Refactor and Docs steps,
// and ImplementModel for every other pipeline step.
func TestModelForStep_AllSteps(t *testing.T) {
	prof := ProfileData{
		ImplementModel: "sonnet",
		ReviewModel:    "opus",
	}
	cases := []struct {
		step pipeline.Step
		want string
	}{
		{pipeline.StepFetch, "sonnet"},
		{pipeline.StepScan, "sonnet"},
		{pipeline.StepBranch, "sonnet"},
		{pipeline.StepTestRed, "sonnet"},
		{pipeline.StepImplement, "sonnet"},
		{pipeline.StepRefactor, "haiku"},
		{pipeline.StepReview, "opus"},
		{pipeline.StepFix, "sonnet"},
		{pipeline.StepDocs, "haiku"},
		{pipeline.StepShip, "sonnet"},
	}
	for _, tc := range cases {
		got := modelForStep(tc.step, prof)
		if got != tc.want {
			t.Errorf("modelForStep(%v): got %q, want %q", tc.step, got, tc.want)
		}
	}
}
