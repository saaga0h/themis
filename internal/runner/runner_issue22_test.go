package runner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/runner"
)

// gitInDir runs a git command in dir with test author identity, failing the test on error.
func gitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
		"GIT_TERMINAL_PROMPT=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// initIssue22Repo creates a real git repo with a single initial commit.
func initIssue22Repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInDir(t, dir, "init")
	gitInDir(t, dir, "config", "user.email", "test@test")
	gitInDir(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "README.md")
	gitInDir(t, dir, "commit", "-m", "initial commit")
	return dir
}

// initIssue22RepoWithRemote creates a workspace cloned from a local bare repo with
// an initial commit pushed to origin/main, ready for feature branches.
func initIssue22RepoWithRemote(t *testing.T) string {
	t.Helper()

	bareDir := t.TempDir()
	gitInDir(t, bareDir, "init", "--bare")

	// Clone bare repo into a subdirectory (git clone creates the directory)
	parentDir := t.TempDir()
	gitInDir(t, parentDir, "clone", bareDir, "workspace")
	workDir := filepath.Join(parentDir, "workspace")
	gitInDir(t, workDir, "config", "user.email", "test@test")
	gitInDir(t, workDir, "config", "user.name", "test")

	// Initial commit, then push to establish origin/main tracking
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, workDir, "add", "README.md")
	gitInDir(t, workDir, "commit", "-m", "initial commit")
	gitInDir(t, workDir, "push", "-u", "origin", "HEAD:main")

	return workDir
}

// issueWriter22 captures the PR Head field, used to verify currentBranchName resolution.
type issueWriter22 struct {
	prURL      string
	prHeadSeen string
}

func (w *issueWriter22) AddLabel(_ context.Context, _ int, _ string) error    { return nil }
func (w *issueWriter22) RemoveLabel(_ context.Context, _ int, _ string) error { return nil }
func (w *issueWriter22) Comment(_ context.Context, _ int, _ string) error     { return nil }
func (w *issueWriter22) CreatePR(_ context.Context, opts runner.PROptions) (string, error) {
	w.prHeadSeen = opts.Head
	if w.prURL != "" {
		return w.prURL, nil
	}
	return "https://example.com/pr/22", nil
}

// AC2: currentBranchName in runner.go is replaced with a call to git.CurrentBranch (accepting context).
//
// In detached HEAD state git.CurrentBranch returns "HEAD", while the old currentBranchName
// reads .git/HEAD directly — the file contains a raw SHA, not a "ref: refs/heads/..." line,
// so the prefix strip fails and the function falls back to returning "main".
// The test therefore fails before the implementation.
func TestRunner_ShipStep_BranchNameUsesGitCurrentBranch(t *testing.T) {
	dir := initIssue22Repo(t)

	// Detach HEAD: .git/HEAD becomes a bare SHA, not a branch ref.
	gitInDir(t, dir, "checkout", "--detach")

	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepShip,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(dir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	w := &issueWriter22{prURL: "https://example.com/pr/22ac2"}
	cfg := runner.Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		GitPushFn:    nil,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// git.CurrentBranch returns "HEAD" for detached state; the old reader returns "main".
	if w.prHeadSeen != "HEAD" {
		t.Errorf("PR Head: got %q, want %q — runner must use git.CurrentBranch, not read .git/HEAD directly", w.prHeadSeen, "HEAD")
	}
}

// AC3: changedFiles returns the list of files changed on the current branch vs the base branch,
// using git diff --name-only against the merge-base.
//
// The test fails before the implementation because changedFiles currently always returns "".
func TestRunner_ChangedFiles_IncludesFilesChangedOnBranch(t *testing.T) {
	workDir := initIssue22RepoWithRemote(t)

	// Create a feature branch and add a new file
	gitInDir(t, workDir, "checkout", "-b", "feat/issue-22")
	if err := os.WriteFile(filepath.Join(workDir, "newfeature.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, workDir, "add", "newfeature.go")
	gitInDir(t, workDir, "commit", "-m", "add new feature")

	// Pre-populate to StepDocs: update-docs.md uses {{CHANGED_FILES}}
	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepDocs,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	inv := &captureInvoker{}
	w := &issueWriter22{prURL: "https://example.com/pr/22ac3"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		GitPushFn:    nil,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.prompts) == 0 {
		t.Fatal("no prompts captured; expected StepDocs to invoke the agent")
	}
	// The StepDocs prompt substitutes {{CHANGED_FILES}} — the file added on the branch must appear.
	docsPrompt := inv.prompts[0]
	if !strings.Contains(docsPrompt, "newfeature.go") {
		t.Errorf("StepDocs prompt must contain changed file 'newfeature.go' in {{CHANGED_FILES}};\ngot:\n%s", docsPrompt)
	}
}

// AC4: changedFiles returns empty string gracefully when git diff fails
// (e.g., no remote tracking branch exists).
//
// Run must succeed even when changedFiles cannot determine a merge-base,
// and the {{CHANGED_FILES}} placeholder must be substituted (not left literal).
func TestRunner_ChangedFiles_ReturnsEmptyStringWhenGitFails(t *testing.T) {
	dir := initIssue22Repo(t) // no remote set up

	// Feature branch with a commit but no remote tracking branch
	gitInDir(t, dir, "checkout", "-b", "feat/no-remote")
	if err := os.WriteFile(filepath.Join(dir, "localfile.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "localfile.go")
	gitInDir(t, dir, "commit", "-m", "add local file")

	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepDocs,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(dir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	inv := &captureInvoker{}
	w := &issueWriter22{prURL: "https://example.com/pr/22ac4"}
	cfg := runner.Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		GitPushFn:    nil,
	}

	// Run must succeed even when changedFiles cannot diff against a remote
	_, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed gracefully when changedFiles has no remote to diff against: %v", err)
	}

	// The {{CHANGED_FILES}} placeholder must be replaced (with empty string), not left literal
	for i, p := range inv.prompts {
		if strings.Contains(p, "{{CHANGED_FILES}}") {
			t.Errorf("prompt[%d] still contains literal {{CHANGED_FILES}} — must be substituted with empty string on git failure", i)
		}
	}
}
