package runner

// Invoker wiring: the InvokeOptions fields (issue number, pipeline step,
// profile-driven model) the runner passes through to the agent.

import (
	"context"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
)

func TestRunnerAgentStepPassesIssueNumberToInvoker(t *testing.T) {
	workDir := t.TempDir()
	// Start at TestRed so we get exactly one agent-step invocation before the test ends.
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/otel"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected at least one agent invocation")
	}
	// The first agent-step invocation must carry the configured issue number.
	if inv.opts[0].IssueNumber != cfg.IssueNumber {
		t.Errorf("agent-step InvokeOptions.IssueNumber = %d, want %d", inv.opts[0].IssueNumber, cfg.IssueNumber)
	}
}

func TestRunnerAgentStepPassesPipelineStepToInvoker(t *testing.T) {
	workDir := t.TempDir()
	// Start at TestRed so the first invocation corresponds to that step.
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/otel"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected at least one agent invocation")
	}
	// The first agent-step invocation must carry the step name "TestRed".
	if inv.opts[0].PipelineStep != pipeline.StepTestRed.String() {
		t.Errorf("agent-step InvokeOptions.PipelineStep = %q, want %q",
			inv.opts[0].PipelineStep, pipeline.StepTestRed.String())
	}
}

func TestRunnerShipStepPassesIssueNumberToInvoker(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/ship-otel"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// The ship step is the last invocation (index pipelineAgentCallCount).
	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", want, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.IssueNumber != cfg.IssueNumber {
		t.Errorf("ship-step InvokeOptions.IssueNumber = %d, want %d", shipOpts.IssueNumber, cfg.IssueNumber)
	}
}

func TestRunnerShipStepPassesPipelineStepToInvoker(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/ship-otel"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", want, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.PipelineStep != pipeline.StepShip.String() {
		t.Errorf("ship-step InvokeOptions.PipelineStep = %q, want %q",
			shipOpts.PipelineStep, pipeline.StepShip.String())
	}
}

// TestRunner_UsesProfileLoaderFromConfig starts the pipeline at StepTestRed and
// injects a ProfileLoader stub that returns a known Review model name
// ("stub-model"). It then verifies that the agent is invoked with that model
// at the Review step, which can only happen if the runner calls
// cfg.ProfileLoader instead of profile.Load directly.
//
// Fails at runtime (not compile-time) until runner.go is updated.
func TestRunner_UsesProfileLoaderFromConfig(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	// ReviewResults needed to mark review non-blocking after the Review step.
	writeReviewResults(t, workDir, []review.ReviewFinding{})

	const stubModel = "stub-model-for-review"

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/profile-loader"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		ProfileLoader: func(dir string) (ProfileData, error) {
			return ProfileData{
				ReviewModel:    stubModel,
				ImplementModel: "sonnet",
			}, nil
		},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Find the Review step invocation and confirm the model used matches stubModel.
	found := false
	for _, opts := range inv.opts {
		if opts.PipelineStep == pipeline.StepReview.String() {
			if opts.Model != stubModel {
				t.Errorf("Review step: model = %q, want %q — runner must use ProfileLoader, not profile.Load directly",
					opts.Model, stubModel)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Review step was never invoked; got %d invocations with steps: %v",
			len(inv.opts), func() []string {
				steps := make([]string, len(inv.opts))
				for i, o := range inv.opts {
					steps[i] = o.PipelineStep
				}
				return steps
			}())
	}
}
