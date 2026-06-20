package runner

// Review-step runner integration: review-results.json lifecycle (seed, delete,
// preserve on resume), the review-cycle limit, and findings-summary logging.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
)

// fileCheckInvoker checks whether jsonPath exists before the first invocation
// and records the result, then delegates to inner.
type fileCheckInvoker struct {
	inner          *recordingInvoker
	jsonPath       string
	fileExistedPtr *bool
	called         bool
}

func (f *fileCheckInvoker) Invoke(ctx context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	if !f.called {
		f.called = true
		if _, err := os.Stat(f.jsonPath); err == nil {
			*f.fileExistedPtr = true
		}
	}
	return f.inner.Invoke(ctx, opts)
}

// TestRunner_ReviewStep_MissingJSONIsBlocking verifies that when the review
// step completes but .themis/review-results.json does not exist, the runner
// treats the result as blocking, logs a warning, and drives a fix cycle (or
// blocks at the cycle limit).
func TestRunner_ReviewStep_MissingJSONIsBlocking(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepReview)
	// No review-results.json written.

	var logBuf bytes.Buffer

	tDir := makeTemplateDir(t, map[string]string{
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
	})

	// Review returns no stdout, no JSON file — runner must treat as blocking.
	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			// review step: no blocking marker in stdout; no JSON file
			{ExitCode: 0, Completed: true, Stdout: ""},
			// fix step (if reached): complete successfully
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &logBuf,
	}

	_, runErr := Run(context.Background(), cfg)

	logOutput := logBuf.String()
	lc := strings.ToLower(logOutput)

	// Must log a warning about the missing file.
	if !strings.Contains(lc, "review-results.json") && !strings.Contains(lc, "warning") {
		t.Errorf("log must contain a warning about missing review-results.json when file is absent; got:\n%s", logOutput)
	}

	// The missing-JSON fail-safe must actually drive the pipeline: either it
	// enters a fix cycle (review → fix, so the invoker is called more than once),
	// or it blocks with a review-cycle-limit error. A warning alone is not enough.
	enteredFixCycle := inv.calls > 1
	hitCycleLimit := runErr != nil && strings.Contains(runErr.Error(), "review cycle")
	if !enteredFixCycle && !hitCycleLimit {
		t.Errorf("missing JSON must trigger a fix cycle (invoker called >1, got %d) or a review-cycle error (got %v); log:\n%s",
			inv.calls, runErr, logOutput)
	}
}

// TestRunner_FixTemplate_BlockingFindingsPlaceholderIsFormattedList is an
// integration test: with a review-results.json containing two blocking findings
// and a fix-findings.md template using {{BLOCKING_FINDINGS}}, the prompt
// delivered to the fix invoker must contain the human-readable finding
// descriptions, not raw JSON or stdout lines.
func TestRunner_FixTemplate_BlockingFindingsPlaceholderIsFormattedList(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepReview)

	// Write two blocking findings to review-results.json.
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "critical", Description: "SQL injection in login handler", File: "auth.go", Line: 55},
		{Severity: "high", Description: "Missing input validation", File: "api.go", Line: 120},
	})

	tDir := makeTemplateDir(t, map[string]string{
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix issue {{ISSUE_NUMBER}}\nFindings:\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
	})

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			// review step: completed, no stdout (blocking from JSON)
			{ExitCode: 0, Completed: true, Stdout: ""},
			// fix step: completed successfully
			{ExitCode: 0, Completed: true},
			// docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	// Find the fix step invocation (after review).
	if len(inv.opts) < 2 {
		t.Fatalf("expected at least 2 agent calls (review + fix); got %d", len(inv.opts))
	}
	fixPrompt := inv.opts[1].Prompt

	// Must contain the human-readable descriptions.
	if !strings.Contains(fixPrompt, "SQL injection in login handler") {
		t.Errorf("fix prompt must contain critical finding description; got:\n%s", fixPrompt)
	}
	if !strings.Contains(fixPrompt, "Missing input validation") {
		t.Errorf("fix prompt must contain high finding description; got:\n%s", fixPrompt)
	}

	// Must NOT be raw JSON (no leading { or array brackets for the findings block).
	findingsStart := strings.Index(fixPrompt, "Findings:\n")
	if findingsStart >= 0 {
		afterFindings := strings.TrimSpace(fixPrompt[findingsStart+len("Findings:\n"):])
		if strings.HasPrefix(afterFindings, "{") || strings.HasPrefix(afterFindings, "[") {
			t.Errorf("BLOCKING_FINDINGS in fix prompt must be human-readable, not raw JSON:\n%s", afterFindings)
		}
	}
}

// TestRunner_FreshStart_DeletesReviewResultsJSON verifies that when the runner
// starts fresh (no state file), a stale .themis/review-results.json is deleted
// before the pipeline proceeds.
func TestRunner_FreshStart_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// No state file — fresh start.

	// Write a stale review-results.json.
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "critical", Description: "stale finding from previous run"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	// Verify the file exists before the run.
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatal("precondition: review-results.json must exist before run")
	}

	w := &stubIssueWriter{prURL: "https://example.com/pr/fresh"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	// Run to completion (or error — we don't care about the pipeline result).
	_, _ = Run(context.Background(), cfg)

	// After a fresh start, .themis/review-results.json must no longer exist.
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("fresh start must delete .themis/review-results.json; file still exists after Run")
	}
}

// TestRunner_IssueMismatch_DeletesReviewResultsJSON verifies that when the
// state file is for a different issue number, the runner starts fresh and
// deletes an existing .themis/review-results.json.
func TestRunner_IssueMismatch_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()

	// State file is for issue #99, but we run with issue #42.
	mismatchState := &pipeline.PipelineState{
		IssueNumber:     99,
		CurrentStep:     pipeline.StepFix,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, mismatchState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Write review-results.json that belongs to the old run.
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "high", Description: "finding from issue 99"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	w := &stubIssueWriter{prURL: "https://example.com/pr/mismatch"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42, // different from state
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	// Issue mismatch triggers fresh start, which must delete review-results.json.
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("issue mismatch must trigger fresh start and delete .themis/review-results.json; file still exists")
	}
}

// TestRunner_Resume_PreservesReviewResultsJSON verifies that when resuming
// a valid state for the same issue, review-results.json is not deleted.
// The fileCheckInvoker checks the file exists at the moment the fix step runs.
func TestRunner_Resume_PreservesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// Resume at StepFix — same issue #42 as cfg.IssueNumber.
	saveStateAt(t, workDir, pipeline.StepFix)

	// Write review-results.json that should be preserved.
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "high", Description: "finding preserved from review"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	fileExistedAtFixInvocation := false

	inner := &recordingInvoker{
		results: []*agent.InvokeResult{
			// fix step
			{ExitCode: 0, Completed: true},
			// docs step
			{ExitCode: 0, Completed: true},
		},
	}

	tDir := makeTemplateDir(t, map[string]string{
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md":         "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	customInv := &fileCheckInvoker{
		inner:          inner,
		jsonPath:       jsonPath,
		fileExistedPtr: &fileExistedAtFixInvocation,
	}

	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      customInv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	if !fileExistedAtFixInvocation {
		t.Error("review-results.json must be preserved when resuming with matching issue number; it was missing at fix step invocation")
	}
}

// Pipeline runner stops with blocked label and issue comment when review cycle limit is reached.
func TestRunner_StopsOnReviewCycleLimit(t *testing.T) {
	workDir := t.TempDir()

	// Already at Review step with 2 cycles used (default max is 2)
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepReview,
		ReviewCycle:     2,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Review agent outputs blocking findings marker
	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true, Stdout: "BLOCKING_FINDINGS: YES\nSecurity issue found."},
		},
	}
	w := &stubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run should return error when review cycle limit exceeded")
	}
	if result != nil && result.PRURL != "" {
		t.Error("should not create PR when blocked on review cycle limit")
	}

	hasBlocked := false
	for _, l := range w.labelsAdded {
		if l == "blocked" {
			hasBlocked = true
		}
	}
	if !hasBlocked {
		t.Errorf("blocked label not added; labelsAdded=%v", w.labelsAdded)
	}
	if len(w.comments) == 0 {
		t.Error("must post a comment when blocked on review cycle limit")
	}
}

// TestRunner_StopsOnReviewCycleLimit_WithJSONFixture drives the cycle limit
// using a blocking review-results.json fixture instead of a stdout marker.
func TestRunner_StopsOnReviewCycleLimit_WithJSONFixture(t *testing.T) {
	workDir := t.TempDir()

	// Already at Review step with 2 cycles used (default max is 2).
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepReview,
		ReviewCycle:     2,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Write a critical finding so the review step returns blocking=true.
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "critical", Description: "Security issue found.", File: "main.go", Line: 10},
	})

	// Review agent returns no blocking stdout — blocking comes entirely from JSON.
	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true, Stdout: ""},
		},
	}
	w := &stubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run should return error when review cycle limit exceeded")
	}
	if result != nil && result.PRURL != "" {
		t.Error("should not create PR when blocked on review cycle limit")
	}

	hasBlocked := false
	for _, l := range w.labelsAdded {
		if l == "blocked" {
			hasBlocked = true
		}
	}
	if !hasBlocked {
		t.Errorf("blocked label not added; labelsAdded=%v", w.labelsAdded)
	}
	if len(w.comments) == 0 {
		t.Error("must post a comment when blocked on review cycle limit")
	}
}

func TestRunner_LogsReviewFindingsSummaryWithCounts_WithJSONFixture(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	tDir := makeTemplateDir(t, map[string]string{
		"test-red.md":     "Test {{ISSUE_NUMBER}}",
		"implement.md":    "Implement {{ISSUE_NUMBER}}",
		"refactor.md":     "Refactor {{ISSUE_NUMBER}}",
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md":         "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	// Start at the Review step via saved state, then write a JSON fixture with
	// 1 blocking + 2 non-blocking findings so the runner logs the counts.
	saveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "high", Description: "missing test for Ship step"},
		{Severity: "low", Description: "variable name could be more descriptive"},
		{Severity: "low", Description: "consider extracting helper function"},
	})

	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			// Review: blocking (high finding in JSON), cycles remaining → goes to Fix
			{ExitCode: 0, Completed: true, Stdout: ""},
			// Fix step
			{ExitCode: 0, Completed: true},
			// Docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/counts"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
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

func TestRunner_LogsReviewFindingsSummaryWhenNoFindings_WithJSONFixture(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	tDir := makeTemplateDir(t, map[string]string{
		"review.md":      "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	// Start at Review step; write empty findings (no blocking).
	saveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []review.ReviewFinding{}) // empty — 0 blocking

	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			// Review: non-blocking (empty JSON)
			{ExitCode: 0, Completed: true, Stdout: ""},
			// Docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/nofindings"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0 blocking") {
		t.Errorf("expected '0 blocking' in review summary when no findings; got:\n%s", output)
	}
}
