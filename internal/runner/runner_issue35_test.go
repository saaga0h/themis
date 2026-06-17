package runner_test

import (
	"context"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/runner"
)

// AC1 + AC2: Ship step returns an error naming the branch and "nothing to ship"
// when no commits exist on the issue branch relative to the base.
func TestRunner_ShipGuard_RejectsWhenNoBranchCommits(t *testing.T) {
	workDir := initIssue22RepoWithRemote(t)
	// Feature branch with zero commits on top of main.
	gitInDir(t, workDir, "checkout", "-b", "issue/35-no-commits")
	saveStateAt(t, workDir, pipeline.StepShip)

	w := &stubIssueWriter{prURL: "https://example.com/pr/35-ac2"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := runner.Run(context.Background(), cfg)

	if err == nil {
		t.Fatal("Run must return an error when no commits exist on the issue branch")
	}
	if !strings.Contains(err.Error(), "no commits on branch") {
		t.Errorf("error must contain 'no commits on branch'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "issue/35-no-commits") {
		t.Errorf("error must contain the branch name 'issue/35-no-commits'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "nothing to ship") {
		t.Errorf("error must contain 'nothing to ship'; got: %v", err)
	}
	if result != nil && result.PRURL != "" {
		t.Error("no PR must be created when ship guard rejects due to zero commits")
	}
	if w.prBodySeen != "" {
		t.Error("CreatePR must not be called when ship guard rejects due to zero commits")
	}
}

// AC3: Ship step returns an error when the current branch equals the base branch,
// indicating no issue branch was created.
func TestRunner_ShipGuard_RejectsWhenCurrentBranchIsBaseBranch(t *testing.T) {
	dir := initIssue22Repo(t)
	// Create and stay on a branch that will also serve as the base branch.
	gitInDir(t, dir, "checkout", "-b", "themis-2.0")
	saveStateAt(t, dir, pipeline.StepShip)

	issue := sampleIssue()
	issue.Ref = "themis-2.0" // base == current branch

	w := &stubIssueWriter{prURL: "https://example.com/pr/35-ac3"}
	cfg := runner.Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: issue},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := runner.Run(context.Background(), cfg)

	if err == nil {
		t.Fatal("Run must return an error when the current branch is the base branch")
	}
	if !strings.Contains(err.Error(), "current branch is the base branch") {
		t.Errorf("error must contain 'current branch is the base branch'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "no issue branch was created") {
		t.Errorf("error must contain 'no issue branch was created'; got: %v", err)
	}
	if result != nil && result.PRURL != "" {
		t.Error("no PR must be created when ship guard rejects due to base branch collision")
	}
	if w.prBodySeen != "" {
		t.Error("CreatePR must not be called when ship guard rejects due to base branch collision")
	}
}

// AC4: PR creation proceeds normally when commits exist on the branch and the
// branch differs from the base.
func TestRunner_ShipGuard_ProceedsWhenCommitsExistOnBranch(t *testing.T) {
	commits := []string{
		"test(runner): add failing tests for issue 35",
		"feat(runner): implement ship step guard",
	}
	workDir := initBranchWithCommits(t, commits)
	saveStateAt(t, workDir, pipeline.StepShip)

	const wantPRURL = "https://example.com/pr/35-ac4"
	w := &stubIssueWriter{prURL: wantPRURL}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed when commits exist on the issue branch: %v", err)
	}
	if result == nil || result.PRURL != wantPRURL {
		t.Errorf("PR URL: got %v, want %q", result, wantPRURL)
	}
	if w.prBodySeen == "" {
		t.Error("CreatePR must be called when commits exist on branch")
	}
}
