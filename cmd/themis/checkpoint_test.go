package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/saaga0h/themis/internal/runner"
	"github.com/saaga0h/themis/internal/tracker"
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
	run("git", "init", "--initial-branch=main")
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

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.CheckpointFn == nil {
		t.Error("production runner.Config must have CheckpointFn set")
	}
}

// TestNewIssueConfig_PropagatesMaxTurnsToRunnerConfig verifies that the maxTurns
// argument passed to newIssueConfig flows into runner.Config.MaxTurns (AC1 wiring).
func TestNewIssueConfig_PropagatesMaxTurnsToRunnerConfig(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, 300)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.MaxTurns != 300 {
		t.Errorf("runner.Config.MaxTurns = %d, want 300 — newIssueConfig must propagate maxTurns arg to cfg.MaxTurns", cfg.MaxTurns)
	}
}

func TestInitGitRepoCheckpointTest_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initGitRepoCheckpointTest(t))
}

func TestInitGitRepoCheckpointTest_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initGitRepoCheckpointTest(t))
}
