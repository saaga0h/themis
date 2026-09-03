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

// ---------------------------------------------------------------------------
// Resume restores the issue branch (via Run), rather than merely warning. #94
// ---------------------------------------------------------------------------

// TestRun_Resume_RestoresIssueBranch verifies that resuming at a post-Branch step
// re-checks-out the issue branch — so an agent step (Implement here) never runs on
// the wrong/base branch — instead of the old behaviour of only logging a warning.
func TestRun_Resume_RestoresIssueBranch(t *testing.T) {
	workDir := t.TempDir()

	// Save state at an agent step after Branch, so Branch's checkout is skipped.
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// The workspace is left on some other branch (as the run loop's checkout-main
	// leaves it); the resume must switch to this issue's branch.
	git := &fakeGitOps{
		currentBranchFn: func(_ context.Context, _ string) (string, error) {
			return "issue/99-some-other-issue", nil
		},
		commitsAhead: 1,
	}

	var log bytes.Buffer
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/validate"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
		Logger:       &log,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	want := issueBranchName(42, sampleIssue().Title)
	found := false
	for _, b := range git.checkedOut {
		if b == want {
			found = true
		}
	}
	if !found {
		t.Errorf("resume must restore issue branch %q so no step runs on the wrong branch; checkedOut=%v\nlog:\n%s", want, git.checkedOut, log.String())
	}
}
