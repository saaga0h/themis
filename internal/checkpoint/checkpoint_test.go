package checkpoint_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/checkpoint"
)

// initGitRepo creates a temp git repo with an initial commit and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
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

// AC: Checkpoint verification after TestRed confirms last commit message starts with test(
// AC: Checkpoint verification after Implement confirms last commit message starts with feat(

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

// AC: Checkpoint verification detects dirty working tree and reports error

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
