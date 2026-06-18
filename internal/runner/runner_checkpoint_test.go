package runner

// Checkpoint integration: the runner wiring around CheckpointFn — a failing
// checkpoint stops the pipeline, and the real step checkpoint rejects a step
// that produced no commit.
//
// Shared stubs and helpers (stubFetcher, stubInvoker, stubIssueWriter,
// sampleIssue, templateDir) are defined in runner_test.go.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/checkpoint"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
)

func initGitRepoForRunner(t *testing.T) string {
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

// Runner stops with an error when checkpoint fails.
func TestRunner_FailingCheckpointStopsPipeline(t *testing.T) {
	workDir := t.TempDir()

	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepTestRed,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	failCheckpoint := func(_ context.Context, step pipeline.Step, _ string) error {
		return fmt.Errorf("simulated checkpoint failure for step %v", step)
	}

	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  &stubIssueWriter{},
		TemplateDir:  templateDir(t),
		CheckpointFn: failCheckpoint,
	}

	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run should return an error when checkpoint fails")
	}
	if !strings.Contains(err.Error(), "checkpoint failed") {
		t.Errorf("error should contain 'checkpoint failed', got: %v", err)
	}
}

// Using the real step checkpoint — missing commit on TestRed must stop the pipeline.
// checkpoint.NewStepCheckpoint does not exist yet — this test will fail to compile until implemented.
func TestRunner_StepCheckpoint_NoCommit_StopsPipeline(t *testing.T) {
	dir := initGitRepoForRunner(t)
	ctx := context.Background()

	checkpointFn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}

	// Start at TestRed; the stub agent returns success but makes no real commit.
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepTestRed,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(dir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	cfg := Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  &stubIssueWriter{},
		TemplateDir:  templateDir(t),
		CheckpointFn: checkpointFn,
	}

	_, err = Run(ctx, cfg)
	if err == nil {
		t.Fatal("Run should fail: TestRed step made no commit, checkpoint must reject it")
	}
	if !strings.Contains(err.Error(), "checkpoint failed") {
		t.Errorf("error should mention checkpoint failure, got: %v", err)
	}
}

func TestInitGitRepoForRunner_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initGitRepoForRunner(t))
}

func TestInitGitRepoForRunner_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initGitRepoForRunner(t))
}
