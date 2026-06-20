package runner

// Review-step runner integration: review-results.json lifecycle (seed, delete,
// preserve on resume) and findings-summary logging. The review step never loops
// or blocks the pipeline — its findings are logged for the PR verdict only.

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

// TestRunner_FreshStart_DeletesReviewResultsJSON verifies that when the runner
// starts fresh (no state file), a stale .themis/review-results.json is deleted
// before the pipeline proceeds.
func TestRunner_FreshStart_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// No state file — fresh start.

	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "critical", Description: "stale finding from previous run"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

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

	_, _ = Run(context.Background(), cfg)

	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("fresh start must delete .themis/review-results.json; file still exists after Run")
	}
}

// TestRunner_IssueMismatch_DeletesReviewResultsJSON verifies that when the
// state file is for a different issue number, the runner starts fresh and
// deletes an existing .themis/review-results.json.
func TestRunner_IssueMismatch_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()

	mismatchState := &pipeline.PipelineState{
		IssueNumber:     99,
		CurrentStep:     pipeline.StepReview,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, mismatchState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

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

	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("issue mismatch must trigger fresh start and delete .themis/review-results.json; file still exists")
	}
}

// TestRunner_Resume_PreservesReviewResultsJSON verifies that when resuming a
// valid state for the same issue, review-results.json is not deleted. The
// fileCheckInvoker checks the file exists at the moment the review step runs.
func TestRunner_Resume_PreservesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// Resume at StepReview — same issue #42 as cfg.IssueNumber.
	saveStateAt(t, workDir, pipeline.StepReview)

	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "high", Description: "finding preserved from review"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	fileExisted := false
	inner := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true}, // review
			{ExitCode: 0, Completed: true}, // docs
		},
	}
	tDir := makeTemplateDir(t, map[string]string{
		"review.md":      "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})
	customInv := &fileCheckInvoker{inner: inner, jsonPath: jsonPath, fileExistedPtr: &fileExisted}

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

	if !fileExisted {
		t.Error("review-results.json must be preserved when resuming with a matching issue number; it was missing at the review step invocation")
	}
}

// The review step logs a blocking/non-blocking findings summary (for the PR
// verdict) — it does not gate the pipeline, which proceeds to Docs regardless.
func TestRunner_LogsReviewFindingsSummaryWithCounts_WithJSONFixture(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	tDir := makeTemplateDir(t, map[string]string{
		"review.md":      "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	saveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []review.ReviewFinding{
		{Severity: "high", Description: "missing test for Ship step"},
		{Severity: "low", Description: "variable name could be more descriptive"},
		{Severity: "low", Description: "consider extracting helper function"},
	})

	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true}, // review
			{ExitCode: 0, Completed: true}, // docs
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

	saveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []review.ReviewFinding{}) // empty — 0 blocking

	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true}, // review
			{ExitCode: 0, Completed: true}, // docs
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
