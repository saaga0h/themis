package runner

import (
	"context"
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
