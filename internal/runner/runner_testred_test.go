package runner

import (
	"context"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

// A new commit (the normal fresh-run path) makes TestRed succeed regardless of
// the branch scan.
func TestDeriveStepResult_TestRed_NewCommitSucceeds(t *testing.T) {
	cfg := Config{WorkDir: "/x", Git: &fakeGitOps{}}
	r := &agent.InvokeResult{ExitCode: 0, CommitsMade: []string{"abc123"}}
	sr := deriveStepResult(context.Background(), pipeline.StepTestRed, r, cfg)
	if !sr.Success {
		t.Error("TestRed must succeed when a new commit was made")
	}
}

// When TestRed produces no new commit and no completion marker but the branch
// already carries committed test files (a resumed or rebased run), the step is
// already satisfied and must be scored success — otherwise the pipeline burns a
// retry attempt for work that is already done.
func TestDeriveStepResult_TestRed_TestsAlreadyOnBranchSucceeds(t *testing.T) {
	cfg := Config{
		WorkDir: "/x",
		Git:     &fakeGitOps{changedFiles: "internal/review/review.go\ninternal/review/review_test.go"},
	}
	r := &agent.InvokeResult{ExitCode: 0, Completed: false} // no commit, no completion
	sr := deriveStepResult(context.Background(), pipeline.StepTestRed, r, cfg)
	if !sr.Success {
		t.Error("TestRed must succeed when failing tests already exist on the branch")
	}
}

// Without a new commit, a completion marker, or any test file on the branch,
// TestRed is a genuine failure and must report the AC key so the retry counter
// advances.
func TestDeriveStepResult_TestRed_NoCommitNoTestsFails(t *testing.T) {
	cfg := Config{
		WorkDir: "/x",
		Git:     &fakeGitOps{changedFiles: "internal/review/review.go"}, // no _test.go
	}
	r := &agent.InvokeResult{ExitCode: 0, Completed: false}
	sr := deriveStepResult(context.Background(), pipeline.StepTestRed, r, cfg)
	if sr.Success {
		t.Error("TestRed must fail when there is no commit, no completion, and no test file on the branch")
	}
	if sr.TestACKey != "tests" {
		t.Errorf("expected TestACKey %q, got %q", "tests", sr.TestACKey)
	}
}

// branchHasTestFiles recognises a Go test file anywhere in the changed-files
// list and ignores non-test files.
func TestBranchHasTestFiles(t *testing.T) {
	cases := []struct {
		name    string
		changed string
		want    bool
	}{
		{"empty", "", false},
		{"only source", "a.go\ninternal/review/review.go", false},
		{"a test file present", "internal/review/review.go\ninternal/review/review_test.go", true},
		{"test file with surrounding whitespace", "  pkg/foo_test.go  ", true},
		{"substring not suffix", "pkg/test_helper.go", false},
	}
	for _, c := range cases {
		if got := branchHasTestFiles(c.changed); got != c.want {
			t.Errorf("%s: branchHasTestFiles(%q) = %v, want %v", c.name, c.changed, got, c.want)
		}
	}
}

// filterTestFiles keeps only the *_test.go entries, preserving them one per line,
// so the Implement step is handed exactly the tests TestRed committed.
func TestFilterTestFiles(t *testing.T) {
	cases := []struct {
		name    string
		changed string
		want    string
	}{
		{"empty", "", ""},
		{"only source", "a.go\ninternal/review/review.go", ""},
		{"mixed keeps only tests", "internal/review/review.go\ninternal/review/review_test.go", "internal/review/review_test.go"},
		{"multiple tests", "pkg/a_test.go\npkg/a.go\npkg/b_test.go", "pkg/a_test.go\npkg/b_test.go"},
		{"substring not suffix", "pkg/test_helper.go", ""},
	}
	for _, c := range cases {
		if got := filterTestFiles(c.changed); got != c.want {
			t.Errorf("%s: filterTestFiles(%q) = %q, want %q", c.name, c.changed, got, c.want)
		}
	}
}
