package runner

// White-box tests for validateResumedState, the helper extracted from Run().
// This file FAILS TO COMPILE until validateResumedState is extracted from
// Run() into a standalone function — that is the correct RED state.
//
// Expected signature (subject to implementer discretion on exact types):
//
//	func validateResumedState(ctx context.Context, state *pipeline.PipelineState, cfg Config, log io.Writer) *pipeline.PipelineState
//
// Returns nil to signal "discard the loaded state and start fresh".
// Returns the (possibly-adjusted) state pointer to signal "resume".

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
)

// Compile-time assertion: validateResumedState must exist with this signature.
// The blank assignment fails to compile if the function does not exist.
var _ func(context.Context, *pipeline.PipelineState, Config, io.Writer) *pipeline.PipelineState = validateResumedState

// TestValidateResumedState_NilStateReturnsFresh asserts that passing a nil
// state (nothing loaded from disk) causes validateResumedState to return nil,
// which Run() interprets as "start fresh".
func TestValidateResumedState_NilStateReturnsFresh(t *testing.T) {
	cfg := Config{IssueNumber: 42}
	var log bytes.Buffer

	result := validateResumedState(context.Background(), nil, cfg, &log)
	if result != nil {
		t.Errorf("validateResumedState(nil, ...) = %v, want nil (fresh start)", result)
	}
}

// TestValidateResumedState_IssueMismatch_ReturnsFresh asserts that when the
// loaded state belongs to a different issue number, validateResumedState
// returns nil so Run() starts fresh.
func TestValidateResumedState_IssueMismatch_ReturnsFresh(t *testing.T) {
	state := &pipeline.PipelineState{
		IssueNumber:     99,
		CurrentStep:     pipeline.StepImplement,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	cfg := Config{IssueNumber: 42}
	var log bytes.Buffer

	result := validateResumedState(context.Background(), state, cfg, &log)
	if result != nil {
		t.Errorf("validateResumedState with mismatched issue: got non-nil, want nil (fresh start)")
	}
}

// TestValidateResumedState_MatchingIssue_ReturnsState asserts that when the
// loaded state belongs to the configured issue, validateResumedState returns
// non-nil so Run() resumes at the saved step.
func TestValidateResumedState_MatchingIssue_ReturnsState(t *testing.T) {
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	cfg := Config{IssueNumber: 42}
	var log bytes.Buffer

	result := validateResumedState(context.Background(), state, cfg, &log)
	if result == nil {
		t.Error("validateResumedState with matching issue returned nil, want non-nil (resume)")
	}
}

// TestValidateResumedState_VersionMismatch_LogsWarning asserts that when the
// loaded state was created by a different code version, validateResumedState
// logs a warning containing "version" or "warning".
func TestValidateResumedState_VersionMismatch_LogsWarning(t *testing.T) {
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
		CodeVersion:     "v0.1",
	}
	cfg := Config{
		IssueNumber: 42,
		CodeVersion: "v0.2",
	}
	var log bytes.Buffer

	validateResumedState(context.Background(), state, cfg, &log)

	out := strings.ToLower(log.String())
	if !strings.Contains(out, "version") && !strings.Contains(out, "warning") {
		t.Errorf("expected version mismatch warning in log; got: %q", log.String())
	}
}

// TestValidateResumedState_BranchMismatch_LogsWarning asserts that when the
// current git branch does not contain the issue number,
// validateResumedState logs a warning that includes the branch name.
func TestValidateResumedState_BranchMismatch_LogsWarning(t *testing.T) {
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement, // > StepBranch
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}

	// currentBranchFn returns a branch name that does not contain "42".
	git := &fakeGitOps{
		currentBranchFn: func(_ context.Context, _ string) (string, error) {
			return "issue/99-some-other-issue", nil
		},
	}
	cfg := Config{
		IssueNumber: 42,
		Git:         git,
	}
	var log bytes.Buffer

	validateResumedState(context.Background(), state, cfg, &log)

	out := log.String()
	if !strings.Contains(out, "issue/99-some-other-issue") {
		t.Errorf("expected branch name in warning log; got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// AC6: integration test — resume-validation path is reachable via Run()
// ---------------------------------------------------------------------------

// TestRun_ResumeValidationDelegatesToHelper verifies the resume-validation
// path by exercising it through Run() with a GitOps stub that returns a
// branch not containing the issue number. Run() must log a warning containing
// the mismatched branch name and still complete successfully.
func TestRun_ResumeValidationDelegatesToHelper(t *testing.T) {
	workDir := t.TempDir()

	// Save state at a step after Branch so the branch-check path is triggered.
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	const wrongBranch = "issue/99-some-other-issue"
	git := &fakeGitOps{
		currentBranchFn: func(_ context.Context, _ string) (string, error) {
			return wrongBranch, nil
		},
		commitsAhead: 1,
	}

	var log bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/validate"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
		Logger:       &log,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := log.String()
	if !strings.Contains(out, wrongBranch) {
		t.Errorf("expected branch name %q in warning log; got:\n%s", wrongBranch, out)
	}
}
