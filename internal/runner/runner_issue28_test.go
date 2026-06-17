package runner_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/runner"
)

// logConfig returns a Config wired to buf for log capture.
// cfg.Logger does not exist yet — this causes a compile failure until the implementation adds it.
func logConfig(t *testing.T, buf *bytes.Buffer, w *stubIssueWriter, inv agent.Invoker) runner.Config {
	t.Helper()
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.Logger = buf
	return cfg
}

// minimalTemplates returns a template dir with trivial single-placeholder content,
// keeping tests focused on logging rather than template expansion.
func minimalTemplates(t *testing.T) string {
	t.Helper()
	return makeTemplateDir(t, map[string]string{
		"test-red.md":     "Test {{ISSUE_NUMBER}}",
		"implement.md":    "Implement {{ISSUE_NUMBER}}",
		"refactor.md":     "Refactor {{ISSUE_NUMBER}}",
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md":         "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})
}

// fullRunResults returns results for a recordingInvoker that drives the default
// happy-path pipeline (TestRed → Implement → Refactor → Review → Docs → Ship).
// Callers can override specific indices to inject custom step output.
func fullRunResults(overrides map[int]*agent.InvokeResult) []*agent.InvokeResult {
	defaults := make([]*agent.InvokeResult, 6)
	for i := range defaults {
		defaults[i] = &agent.InvokeResult{ExitCode: 0, Completed: true}
	}
	for i, r := range overrides {
		defaults[i] = r
	}
	return defaults
}

// AC1: every step transition prints start and done messages to stderr with duration.

func TestRunner_LogsStepStartMessage(t *testing.T) {
	var buf bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac1-start"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac1-done"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC2: state resume is logged: "resuming from step X" vs. "fresh start".

func TestRunner_LogsFreshStartWhenNoStateFile(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir() // no .themis/state.json
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac2-fresh"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac2-resume"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC3: agent invocations log the model and max turns.

func TestRunner_LogsAgentInvocationModelAndMaxTurns(t *testing.T) {
	var buf bytes.Buffer
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac3"}
	cfg := logConfig(t, &buf, w, inv)

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// The default profile model is "sonnet" for all non-review agent steps.
	if !strings.Contains(output, "sonnet") {
		t.Errorf("expected model name 'sonnet' in agent invocation log; got:\n%s", output)
	}
	// Max turns is always 100 for all agent invocations.
	if !strings.Contains(output, "100") {
		t.Errorf("expected max turns '100' in agent invocation log; got:\n%s", output)
	}
}

// AC4: agent results log commit count and completion status.

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
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac4-count"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac4-status"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC5: checkpoint results are logged (pass or the specific failure).

func TestRunner_LogsCheckpointPassAfterStep(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac5-pass"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: failCheckpoint,
		Logger:       &buf,
	}

	_, err := runner.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run must return error when checkpoint fails")
	}

	output := buf.String()
	// The specific error text must appear so the operator knows why the checkpoint failed.
	if !strings.Contains(output, failMsg) {
		t.Errorf("checkpoint failure message %q must appear in log; got:\n%s", failMsg, output)
	}
}

// AC6: review findings are summarized: "N blocking, M non-blocking".

func TestRunner_LogsReviewFindingsSummaryWithCounts(t *testing.T) {
	var buf bytes.Buffer

	// 1 BLOCKING + 2 NON-BLOCKING lines in the review output.
	const reviewOutput = "BLOCKING: missing test for Ship step (runner.go:158)\n" +
		"NON-BLOCKING: variable name could be more descriptive (runner.go:243)\n" +
		"NON-BLOCKING: consider extracting helper function (runner.go:300)"

	// Happy-path fresh run; Review is index 3 in the agent call sequence.
	inv := &recordingInvoker{
		results: fullRunResults(map[int]*agent.InvokeResult{
			3: {ExitCode: 0, Completed: true, Stdout: reviewOutput}, // Review: 1 blocking
		}),
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac6-counts"}
	cfg := runner.Config{
		WorkDir:      t.TempDir(),
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  minimalTemplates(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1 blocking") {
		t.Errorf("expected '1 blocking' in review findings summary; got:\n%s", output)
	}
	if !strings.Contains(output, "2 non-blocking") {
		t.Errorf("expected '2 non-blocking' in review findings summary; got:\n%s", output)
	}
}

func TestRunner_LogsReviewFindingsSummaryWhenNoFindings(t *testing.T) {
	var buf bytes.Buffer

	// Clean review: no BLOCKING or NON-BLOCKING prefix lines.
	inv := &recordingInvoker{
		results: fullRunResults(map[int]*agent.InvokeResult{
			3: {ExitCode: 0, Completed: true, Stdout: "No blocking findings. All ACs verified."},
		}),
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac6-clean"}
	cfg := runner.Config{
		WorkDir:      t.TempDir(),
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  minimalTemplates(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0 blocking") {
		t.Errorf("expected '0 blocking' in review summary when no findings; got:\n%s", output)
	}
}

// AC7: ship step logs the PR URL.

func TestRunner_LogsShipPRURL(t *testing.T) {
	var buf bytes.Buffer
	const prURL = "https://example.com/pr/28-ac7"
	w := &stubIssueWriter{prURL: prURL}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	result, err := runner.Run(context.Background(), cfg)
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

// AC8: all output goes to stderr (stdout is reserved for the final PR URL).
// When Logger is nil, Run must default to os.Stderr and must not panic.

func TestRunner_NilLoggerDefaultsToStderrWithoutPanic(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/28-ac8"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})
	cfg.Logger = nil // explicitly nil: must fall back to os.Stderr, not panic

	_, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed when Logger is nil (defaults to os.Stderr): %v", err)
	}
}
