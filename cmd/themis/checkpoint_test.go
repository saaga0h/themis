package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/runner"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

type stubMainFetcher struct{}

func (s *stubMainFetcher) Fetch(_ context.Context, _ int) (*tracker.IssueData, error) {
	return &tracker.IssueData{Number: 42, Title: "test issue"}, nil
}

type stubMainIssueWriter struct{}

func (s *stubMainIssueWriter) AddLabel(_ context.Context, _ int, _ string) error    { return nil }
func (s *stubMainIssueWriter) RemoveLabel(_ context.Context, _ int, _ string) error { return nil }
func (s *stubMainIssueWriter) Comment(_ context.Context, _ int, _ string) error     { return nil }
func (s *stubMainIssueWriter) CreatePR(_ context.Context, _ runner.PROptions) (string, error) {
	return "https://example.com/pr/1", nil
}

func initGitRepoCheckpointTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "chore: initial commit")
	return dir
}

// cmd/themis/main.go sets CheckpointFn in the production runner.Config.
// newIssueConfig does not exist yet — this test will fail to compile until implemented.
func TestNewIssueConfig_SetsCheckpointFn(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{})
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.CheckpointFn == nil {
		t.Error("production runner.Config must have CheckpointFn set")
	}
}

// ---------------------------------------------------------------------------
// git init portability (AC1 target 5)
// ---------------------------------------------------------------------------

// TestInitGitRepoCheckpointTest_DefaultBranchIsMain verifies that the
// initGitRepoCheckpointTest helper creates repositories on "main", not on
// whatever the global init.defaultBranch config says.  This test will FAIL
// until initGitRepoCheckpointTest passes --initial-branch=main to git init
// (AC1 target 5).
func TestInitGitRepoCheckpointTest_DefaultBranchIsMain(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, out)
	}
	branch := strings.TrimSpace(string(out))
	if branch != "main" {
		t.Errorf("initGitRepoCheckpointTest must create branch 'main', got %q (add --initial-branch=main to git init)", branch)
	}
}

// TestInitGitRepoCheckpointTest_PortableUnderDefaultBranchMaster verifies that
// initGitRepoCheckpointTest still creates a "main" branch even when git's
// global init.defaultBranch is "master" (AC4 — regression guard for older git
// configs).
func TestInitGitRepoCheckpointTest_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")

	dir := initGitRepoCheckpointTest(t)
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, out)
	}
	branch := strings.TrimSpace(string(out))
	if branch != "main" {
		t.Errorf("initGitRepoCheckpointTest must produce branch 'main' regardless of init.defaultBranch=master, got %q (add --initial-branch=main to git init)", branch)
	}
}
