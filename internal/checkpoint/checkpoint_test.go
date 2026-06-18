package checkpoint_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/checkpoint"
)

// initGitRepo creates a temp git repo with an initial commit and a remote whose
// tracking branch (origin/main) marks the base, then returns the repo path.
//
// The remote is required because the checkpoint walks BranchCommitLog, which
// enumerates commits relative to the nearest remote-tracking branch (merge-base
// against refs/remotes/). A repo with no remote yields an empty log — exactly
// the situation in production, where the working tree is always cloned from the
// tracker. The initial commit becomes the base; commits added afterwards by
// makeCommit are "ahead" and therefore visible to the checkpoint.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bareDir := t.TempDir()

	run := func(workDir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run(bareDir, "git", "init", "--bare", "--initial-branch=main")

	run(dir, "git", "init", "--initial-branch=main")
	run(dir, "git", "config", "user.email", "test@test.com")
	run(dir, "git", "config", "user.name", "Test")

	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run(dir, "git", "add", "README.md")
	run(dir, "git", "commit", "-m", "chore: initial commit")

	// Establish origin/main as the base so merge-base resolution succeeds.
	run(dir, "git", "remote", "add", "origin", bareDir)
	run(dir, "git", "push", "origin", "HEAD:refs/heads/main")
	run(dir, "git", "fetch", "origin")

	return dir
}

// makeCommit creates a file and commits it with the given message.
func makeCommit(t *testing.T, dir, message string) {
	t.Helper()
	f := filepath.Join(dir, "file-"+message[:10]+".txt")
	if err := os.WriteFile(f, []byte(message), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "add", ".")
	run("git", "commit", "-m", message)
}

// Checkpoint verification after TestRed confirms last commit message starts with test(
// Checkpoint verification after Implement confirms last commit message starts with feat(

func TestVerifyCommitPrefix_Matches(t *testing.T) {
	dir := initGitRepo(t)
	makeCommit(t, dir, "test(scope): add failing tests")
	ctx := context.Background()
	if err := checkpoint.VerifyCommitPrefix(ctx, dir, "test("); err != nil {
		t.Errorf("VerifyCommitPrefix should pass for matching prefix, got: %v", err)
	}
}

func TestVerifyCommitPrefix_Mismatch(t *testing.T) {
	dir := initGitRepo(t)
	makeCommit(t, dir, "feat(scope): implement something")
	ctx := context.Background()
	err := checkpoint.VerifyCommitPrefix(ctx, dir, "test(")
	if err == nil {
		t.Error("VerifyCommitPrefix should fail when last commit does not start with test(")
	}
}

func TestVerifyCommitPrefix_ImplementPrefix(t *testing.T) {
	dir := initGitRepo(t)
	makeCommit(t, dir, "feat(pipeline): implement issue runner")
	ctx := context.Background()
	if err := checkpoint.VerifyCommitPrefix(ctx, dir, "feat("); err != nil {
		t.Errorf("VerifyCommitPrefix should pass for feat( prefix, got: %v", err)
	}
}

// Checkpoint verification detects dirty working tree and reports error

func TestVerifyCleanWorkingTree_Clean(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	if err := checkpoint.VerifyCleanWorkingTree(ctx, dir); err != nil {
		t.Errorf("VerifyCleanWorkingTree on clean repo should pass, got: %v", err)
	}
}

func TestVerifyCleanWorkingTree_Dirty(t *testing.T) {
	dir := initGitRepo(t)
	dirty := filepath.Join(dir, "uncommitted.txt")
	if err := os.WriteFile(dirty, []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	err := checkpoint.VerifyCleanWorkingTree(ctx, dir)
	if err == nil {
		t.Error("VerifyCleanWorkingTree on dirty repo should return error")
	}
}

func assertBranchIsMain(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git symbolic-ref: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "main" {
		t.Errorf("expected branch 'main', got %q", got)
	}
}

func TestInitGitRepo_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initGitRepo(t))
}

func TestInitGitRepo_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initGitRepo(t))
}
