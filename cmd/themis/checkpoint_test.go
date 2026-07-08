package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/runner"
	"github.com/saaga0h/themis/internal/tracker"
	"github.com/saaga0h/themis/internal/workflow"
)

// stubMainFetcher is the shared Fetcher stub for cmd/themis tests. Body is
// optional (zero value "" is safe for tests that don't care about issue
// content) — set it to exercise issue-declared check blocks and destructive-AC
// validation (issue #98).
type stubMainFetcher struct {
	Body string
}

func (s *stubMainFetcher) Fetch(_ context.Context, _ int) (*tracker.IssueData, error) {
	return &tracker.IssueData{Number: 42, Title: "test issue", Body: s.Body}, nil
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

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.CheckpointFn == nil {
		t.Error("production runner.Config must have CheckpointFn set")
	}
}

// newIssueConfig wires the production TestRunner so the Implement/Fix green gate
// is active in real runs (it is nil-bypassed only in tests).
func TestNewIssueConfig_SetsTestRunner(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.TestRunner == nil {
		t.Error("production runner.Config must have TestRunner set for the green gate")
	}
}

// TestNewIssueConfig_PropagatesMaxTurnsToRunnerConfig verifies that the maxTurns
// argument passed to newIssueConfig flows into runner.Config.MaxTurns (AC1 wiring).
func TestNewIssueConfig_PropagatesMaxTurnsToRunnerConfig(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	cfg, err := newIssueConfig(ctx, 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, &cmdGitOps{}, 300)
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

// ---------------------------------------------------------------------------
// Issue #98: extract issue-declared `check` blocks into the green gate.
// newIssueConfig is expected to fetch the issue body, extract its `check`
// fences, validate destructive ACs, and feed append(desc.Verify, issueChecks...)
// into verifyRunner. None of that is wired yet, so these tests fail against
// today's newIssueConfig (which never calls fetcher.Fetch).
// ---------------------------------------------------------------------------

// AC1: issue-declared check blocks are appended to the green gate for this run.
func TestNewIssueConfig_AppendsIssueDeclaredChecksToVerify(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()
	body := "## Acceptance Criteria\n- [ ] Something behavioural\n\n" +
		"```check\necho ISSUE_CHECK_MARKER\n```\n"
	fetcher := &stubMainFetcher{Body: body}

	cfg, err := newIssueConfig(ctx, 42, dir, dir, fetcher, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.TestRunner == nil {
		t.Fatal("cfg.TestRunner must be set")
	}
	_, out := cfg.TestRunner(ctx, dir)
	if !strings.Contains(out, "ISSUE_CHECK_MARKER") {
		t.Errorf("expected green gate output to contain the issue-declared check's marker, got %q", out)
	}
}

// AC2: the universal .themis/workflow.yaml verify contract is unchanged —
// issue checks are additive and per-run only, never committed to the descriptor.
func TestNewIssueConfig_DoesNotMutateWorkflowDescriptorOrFile(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()

	themisDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir .themis: %v", err)
	}
	workflowPath := filepath.Join(themisDir, "workflow.yaml")
	before := []byte("stack: go\nverify:\n  - go build ./...\n")
	if err := os.WriteFile(workflowPath, before, 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	body := "## Acceptance Criteria\n- [ ] Something behavioural\n\n" +
		"```check\necho ISSUE_CHECK_MARKER\n```\n"
	fetcher := &stubMainFetcher{Body: body}

	if _, err := newIssueConfig(ctx, 42, dir, dir, fetcher, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns); err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}

	after, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("reading workflow.yaml after newIssueConfig: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("newIssueConfig must not mutate .themis/workflow.yaml on disk; before=%q after=%q", before, after)
	}

	desc, err := workflow.Load(dir)
	if err != nil {
		t.Fatalf("workflow.Load: %v", err)
	}
	if len(desc.Verify) != 1 || desc.Verify[0] != "go build ./..." {
		t.Errorf("workflow.Load(dir).Verify must contain only the originally declared entries (no issue-check leakage), got %v", desc.Verify)
	}
}

// AC3: a deterministic meta-check fails the step when a destructive AC carries
// no check directive, naming the offending AC.
func TestNewIssueConfig_FailsWhenDestructiveACHasNoCheckBlock(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()
	const offendingAC = "No GiteaQuerier struct remains in cmd/themis"
	body := "## Acceptance Criteria\n- [ ] " + offendingAC + "\n"
	fetcher := &stubMainFetcher{Body: body}

	_, err := newIssueConfig(ctx, 42, dir, dir, fetcher, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns)
	if err == nil {
		t.Fatal("expected newIssueConfig to fail when a destructive AC has no check block")
	}
	if !strings.Contains(err.Error(), offendingAC) {
		t.Errorf("newIssueConfig error must name the offending AC %q, got %q", offendingAC, err.Error())
	}
}

// AC3: a destructive AC paired with its check block does not fail newIssueConfig,
// and the rest of the config is still wired.
func TestNewIssueConfig_PassesWhenDestructiveACHasCheckBlock(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	ctx := context.Background()
	body := "## Acceptance Criteria\n- [ ] No GiteaQuerier struct remains in cmd/themis\n\n" +
		"```check\n! grep -rq 'type GiteaQuerier' cmd/themis/\n```\n"
	fetcher := &stubMainFetcher{Body: body}

	cfg, err := newIssueConfig(ctx, 42, dir, dir, fetcher, &stubMainIssueWriter{}, &cmdGitOps{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if cfg.CheckpointFn == nil {
		t.Error("cfg.CheckpointFn must still be set")
	}
	if cfg.TestRunner == nil {
		t.Error("cfg.TestRunner must still be set")
	}
}
