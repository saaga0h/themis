package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
)

// Resuming at a post-Branch step must re-check-out the issue branch: the Branch
// step is skipped on resume and the run loop leaves the workspace on the base
// branch, so without this the resumed Ship (or an agent step) runs on main. #94.
func TestRunner_ResumeRestoresIssueBranch(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepShip) // resume past Branch
	issue := sampleIssue()                     // #42 "Test Issue"
	wantBranch := issueBranchName(issue.Number, issue.Title)

	// The workspace sits on main until the resume checks the issue branch out;
	// afterwards CurrentBranch reports the issue branch (so Ship's base-branch
	// guard passes and it can push).
	git := &fakeGitOps{commitsAhead: 1}
	git.currentBranchFn = func(context.Context, string) (string, error) {
		for _, b := range git.checkedOut {
			if b == wantBranch {
				return wantBranch, nil
			}
		}
		return "main", nil
	}

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: issue},
		Invoker:      &recordingInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/94"},
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Git:          git,
	}

	result, err := Run(context.Background(), cfg)

	found := false
	for _, b := range git.checkedOut {
		if b == wantBranch {
			found = true
		}
	}
	if !found {
		t.Fatalf("resume must check out issue branch %q; checkedOut=%v (err=%v)", wantBranch, git.checkedOut, err)
	}
	if err != nil {
		t.Fatalf("resumed run should complete once on the issue branch, got: %v", err)
	}
	if result == nil || result.PRURL == "" {
		t.Error("expected a PR from the resumed Ship")
	}
}

// ---------------------------------------------------------------------------
// Crash recovery: clean the working tree on resume (issue #56)
// ---------------------------------------------------------------------------

// TestRun_Resume_DirtyWorkingTree_CleansBeforeAgentInvoke asserts that
// resuming at a post-Branch step with a dirty working tree (left behind by a
// crashed agent step) triggers exactly one CleanWorkingTree call before the
// pipeline proceeds — the resumed run must not hand the agent a workspace
// still carrying the previous, interrupted attempt's stray changes.
func TestRun_Resume_DirtyWorkingTree_CleansBeforeAgentInvoke(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)

	git := &fakeGitOps{
		commitsAhead: 1,
		workingTreeCleanFn: func(context.Context, string) (bool, error) {
			return false, nil // dirty
		},
	}

	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/dirty-resume"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if git.cleanWorkingTreeCalls != 1 {
		t.Errorf("expected CleanWorkingTree called exactly once on dirty resume, got %d calls", git.cleanWorkingTreeCalls)
	}
}

// TestRun_FreshStart_DoesNotCleanWorkingTree asserts that a fresh start (no
// saved state) never calls CleanWorkingTree, even when the git fake reports a
// dirty tree — cleanup is a resume-specific recovery step, not something a
// normal first run should ever trigger.
func TestRun_FreshStart_DoesNotCleanWorkingTree(t *testing.T) {
	workDir := t.TempDir() // no state file: fresh start

	git := &fakeGitOps{
		commitsAhead: 1,
		workingTreeCleanFn: func(context.Context, string) (bool, error) {
			return false, nil // dirty
		},
	}

	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/fresh-noclean"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if git.cleanWorkingTreeCalls != 0 {
		t.Errorf("expected CleanWorkingTree not called on fresh start, got %d calls", git.cleanWorkingTreeCalls)
	}
}

// TestRun_Resume_CleanWorkingTree_NotCalledWhenTreeAlreadyClean asserts that
// resuming with an already-clean working tree never calls CleanWorkingTree —
// only WorkingTreeClean should be consulted; the destructive cleanup call
// itself must be conditional on that check reporting dirty.
func TestRun_Resume_CleanWorkingTree_NotCalledWhenTreeAlreadyClean(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)

	git := &fakeGitOps{commitsAhead: 1} // workingTreeCleanFn nil -> defaults to clean

	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/clean-resume"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if git.cleanWorkingTreeCalls != 0 {
		t.Errorf("expected CleanWorkingTree not called when tree already clean, got %d calls", git.cleanWorkingTreeCalls)
	}
}

// ---------------------------------------------------------------------------
// Resuming past Branch when the issue branch itself is absent (issue #56 AC6)
// ---------------------------------------------------------------------------

// TestRun_Resume_BranchAbsent_ResetsToFreshStart asserts that when the resume
// checkout fails because the issue branch no longer exists (e.g. deleted
// between crash and restart), Run resets to a fresh start — discarding the
// stale attempt counters — rather than surfacing the generic "cannot check
// out its branch" error.
func TestRun_Resume_BranchAbsent_ResetsToFreshStart(t *testing.T) {
	workDir := t.TempDir()

	state := &pipeline.PipelineState{
		IssueNumber:       42,
		CurrentStep:       pipeline.StepImplement,
		ImplementAttempts: 2,
		TestFixAttempts:   map[string]int{"AC-1": 3},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	git := &fakeGitOps{
		commitsAhead: 1,
		checkoutErr:  fmt.Errorf("error: pathspec 'issue/42-test-issue' did not match any file(s) known to git"),
	}

	var log bytes.Buffer
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/branch-absent"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
		Logger:       &log,
	}

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed by resetting to a fresh start when the issue branch is absent, got error: %v", err)
	}

	output := log.String()
	if strings.Contains(output, "cannot check out its branch") {
		t.Errorf("branch-absent resume must not surface the generic checkout-failure message; got:\n%s", output)
	}
	if !strings.Contains(output, pipeline.StepFetch.String()) {
		t.Errorf("expected evidence the pipeline restarted at %s; got:\n%s", pipeline.StepFetch.String(), output)
	}
}

// TestRun_Resume_CheckoutFailsForOtherReason_StillErrors asserts that a
// checkout failure NOT matching the branch-absent signature (e.g. a network
// failure) still surfaces the "cannot check out its branch" error — the
// fresh-start reset is narrowly scoped to the branch-absent case, not a
// blanket swallow of every checkout failure.
func TestRun_Resume_CheckoutFailsForOtherReason_StillErrors(t *testing.T) {
	workDir := t.TempDir()

	state := &pipeline.PipelineState{
		IssueNumber:       42,
		CurrentStep:       pipeline.StepImplement,
		ImplementAttempts: 2,
		TestFixAttempts:   map[string]int{"AC-1": 3},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	git := &fakeGitOps{
		commitsAhead: 1,
		checkoutErr:  fmt.Errorf("fatal: unable to access 'https://git.example.com/repo.git/': Could not resolve host: git.example.com"),
	}

	var log bytes.Buffer
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/checkout-fail"},
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git:          git,
		Logger:       &log,
	}

	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run must return an error when checkout fails for a reason other than branch absence")
	}
	if !strings.Contains(err.Error(), "cannot check out its branch") {
		t.Errorf("expected error to contain %q; got: %v", "cannot check out its branch", err)
	}

	output := log.String()
	if strings.Contains(strings.ToLower(output), "fresh start") {
		t.Errorf("expected no fresh-start evidence when checkout fails for a non-branch-absence reason; got:\n%s", output)
	}
}
