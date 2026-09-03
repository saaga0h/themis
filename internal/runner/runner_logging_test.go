package runner

// Run logging: the messages the runner emits for step start/done, agent
// invocation and result, checkpoint outcome, and lifecycle events.

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

func TestRunner_LogsStepStartMessage(t *testing.T) {
	var buf bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/start"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// Every step that executes must appear by name in the log.
	for _, step := range []pipeline.Step{
		pipeline.StepFetch, pipeline.StepTestRed, pipeline.StepImplement,
		pipeline.StepReview, pipeline.StepShip,
	} {
		name := step.String()
		if !strings.Contains(output, name) {
			t.Errorf("log must mention step %q; got:\n%s", name, output)
		}
	}
}

func TestRunner_LogsStepDoneWithDuration(t *testing.T) {
	var buf bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/done"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "done") {
		t.Errorf("log must contain 'done' message for step completion; got:\n%s", output)
	}
	// Duration must appear in done messages — accept "Xms" or "X.Xs".
	if !strings.Contains(output, "ms") {
		t.Errorf("log done message must include millisecond duration (e.g. '5ms'); got:\n%s", output)
	}
}

func TestRunner_LogsFreshStartWhenNoStateFile(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir() // no .themis/state.json
	w := &stubIssueWriter{prURL: "https://example.com/pr/fresh"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "fresh start") {
		t.Errorf("expected 'fresh start' in log when no state file; got:\n%s", output)
	}
}

func TestRunner_LogsResumingFromStepWhenStateFileExists(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)

	w := &stubIssueWriter{prURL: "https://example.com/pr/resume"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "resuming") {
		t.Errorf("expected 'resuming' in log when state file exists; got:\n%s", output)
	}
	// The step name must appear so the user knows exactly where we resumed.
	if !strings.Contains(output, pipeline.StepImplement.String()) {
		t.Errorf("expected step name %q in resume log; got:\n%s", pipeline.StepImplement.String(), output)
	}
}

func TestRunner_LogsAgentInvocationModelAndMaxTurns(t *testing.T) {
	var buf bytes.Buffer
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := logConfig(t, &buf, w, inv)
	cfg.MaxTurns = 250

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// The default profile model is "sonnet" for the testred/implement/review steps.
	if !strings.Contains(output, "sonnet") {
		t.Errorf("expected model name 'sonnet' in agent invocation log; got:\n%s", output)
	}
	// Max turns must reflect the per-step budget, not the global ceiling: with a
	// 250 ceiling, TestRed is capped at its lower default of 80.
	if !strings.Contains(output, "maxTurns=80") {
		t.Errorf("expected per-step max turns 'maxTurns=80' in agent invocation log; got:\n%s", output)
	}
}

func TestRunner_LogsAgentResultCommitCount(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			// TestRed makes 2 commits and completes.
			{ExitCode: 0, Completed: true, CommitsMade: []string{"abc1234", "def5678"}},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/count"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// "2 commits" (or "2 commit") must appear in the result log.
	if !strings.Contains(output, "2") || !strings.Contains(strings.ToLower(output), "commit") {
		t.Errorf("expected '2 commits' in agent result log; got:\n%s", output)
	}
}

func TestRunner_LogsAgentResultCompletionStatus(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			// First invocation: not completed, no commits → runner retries TestRed.
			{ExitCode: 0, Completed: false, CommitsMade: nil},
			// Second invocation: completed → pipeline advances.
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/status"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	lc := strings.ToLower(output)
	// First invocation result must log "not completed" or equivalent.
	if !strings.Contains(lc, "not completed") && !strings.Contains(lc, "incomplete") {
		t.Errorf("expected 'not completed'/'incomplete' in agent result log when Completed=false; got:\n%s", output)
	}
	// Second invocation result must log "completed".
	if !strings.Contains(lc, "completed") {
		t.Errorf("expected 'completed' in agent result log when Completed=true; got:\n%s", output)
	}
}

func TestRunner_LogsCheckpointPassAfterStep(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	w := &stubIssueWriter{prURL: "https://example.com/pr/pass"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	lc := strings.ToLower(output)
	if !strings.Contains(lc, "checkpoint") {
		t.Errorf("expected 'checkpoint' in log after agent step; got:\n%s", output)
	}
	if !strings.Contains(lc, "pass") && !strings.Contains(lc, "ok") {
		t.Errorf("expected 'pass' or 'ok' in checkpoint log when it succeeds; got:\n%s", output)
	}
}

func TestRunner_LogsCheckpointFailureWithSpecificErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	const failMsg = "missing test(runner): prefix in last commit"
	failCheckpoint := func(_ context.Context, _ pipeline.Step, _ string) error {
		return fmt.Errorf("%s", failMsg)
	}

	w := &stubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: failCheckpoint,
		Logger:       &buf,
	}

	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run must return error when checkpoint fails")
	}

	output := buf.String()
	// The specific error text must appear so the operator knows why the checkpoint failed.
	if !strings.Contains(output, failMsg) {
		t.Errorf("checkpoint failure message %q must appear in log; got:\n%s", failMsg, output)
	}
}

func TestRunner_LogsShipPRURL(t *testing.T) {
	var buf bytes.Buffer
	const prURL = "https://example.com/pr/example"
	w := &stubIssueWriter{prURL: prURL}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL != prURL {
		t.Fatalf("PRURL: got %q, want %q", result.PRURL, prURL)
	}

	output := buf.String()
	if !strings.Contains(output, prURL) {
		t.Errorf("ship step must log PR URL %q; got:\n%s", prURL, output)
	}
}

func TestRunner_NilLoggerDefaultsToStderrWithoutPanic(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})
	cfg.Logger = nil // explicitly nil: must fall back to os.Stderr, not panic

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed when Logger is nil (defaults to os.Stderr): %v", err)
	}
}

// ---------------------------------------------------------------------------
// Crash recovery: resume logging surfaces attempt count and cleanup (#56)
// ---------------------------------------------------------------------------

// TestRun_Resume_LogsCurrentStepAndImplementAttemptCount asserts that resuming
// at StepImplement with a nonzero ImplementAttempts logs both the step name
// and the attempt count together on the resume line, so an operator reading
// the log knows not just where the run resumed but how many Implement
// attempts had already been spent.
func TestRun_Resume_LogsCurrentStepAndImplementAttemptCount(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	state := &pipeline.PipelineState{
		IssueNumber:       42,
		CurrentStep:       pipeline.StepImplement,
		ImplementAttempts: 2,
		TestFixAttempts:   map[string]int{},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	w := &stubIssueWriter{prURL: "https://example.com/pr/attempt-count"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	found := false
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), "resum") &&
			strings.Contains(line, pipeline.StepImplement.String()) &&
			strings.Contains(line, "2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a resume log line mentioning step %q and attempt count 2; got:\n%s", pipeline.StepImplement.String(), output)
	}
}

// TestRun_Resume_LogsDirtyFileCountAndCleanupActionWhenDirty asserts that a
// dirty resume logs both the number of files CleanWorkingTree removed and a
// cleanup-action keyword, and that a companion clean-tree resume logs neither
// — the dirty-count/cleanup line only appears when cleanup actually ran.
func TestRun_Resume_LogsDirtyFileCountAndCleanupActionWhenDirty(t *testing.T) {
	t.Run("dirty tree logs file count and cleanup action", func(t *testing.T) {
		var buf bytes.Buffer
		workDir := t.TempDir()
		saveStateAt(t, workDir, pipeline.StepImplement)

		git := &fakeGitOps{
			commitsAhead: 1,
			workingTreeCleanFn: func(context.Context, string) (bool, error) {
				return false, nil
			},
			cleanWorkingTreeFn: func(context.Context, string) (int, error) {
				return 3, nil
			},
		}

		cfg := Config{
			WorkDir:      workDir,
			IssueNumber:  42,
			Fetcher:      &stubFetcher{issue: sampleIssue()},
			Invoker:      &stubInvoker{},
			IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/dirty-log"},
			TemplateDir:  templateDir(t),
			CheckpointFn: noopCheckpoint,
			Git:          git,
			Logger:       &buf,
		}

		if _, err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run error: %v", err)
		}

		output := buf.String()
		lc := strings.ToLower(output)
		if !strings.Contains(output, "3") {
			t.Errorf("expected dirty-file count '3' in log; got:\n%s", output)
		}
		if !strings.Contains(lc, "dirty working tree") {
			t.Errorf("expected the resume cleanup log line ('dirty working tree') in log; got:\n%s", output)
		}
	})

	t.Run("clean tree logs no dirty-count or cleanup line", func(t *testing.T) {
		var buf bytes.Buffer
		workDir := t.TempDir()
		saveStateAt(t, workDir, pipeline.StepImplement)

		git := &fakeGitOps{commitsAhead: 1} // defaults to clean

		cfg := Config{
			WorkDir:      workDir,
			IssueNumber:  42,
			Fetcher:      &stubFetcher{issue: sampleIssue()},
			Invoker:      &stubInvoker{},
			IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/clean-log"},
			TemplateDir:  templateDir(t),
			CheckpointFn: noopCheckpoint,
			Git:          git,
			Logger:       &buf,
		}

		if _, err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run error: %v", err)
		}

		output := buf.String()
		if strings.Contains(strings.ToLower(output), "dirty working tree") {
			t.Errorf("expected no resume cleanup log line when tree is already clean; got:\n%s", output)
		}
	})
}
