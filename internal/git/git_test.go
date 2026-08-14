package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a real git repo in a temp directory, makes an initial commit,
// and returns the directory path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "--initial-branch=main", dir)
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")

	// Initial commit
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func addCommit(t *testing.T, dir, filename, content, msg string) {
	t.Helper()
	f := filepath.Join(dir, filename)
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// --- CommitsBefore ---

func TestCommitsBeforeReturnsCurrentSHAs(t *testing.T) {
	dir := initTestRepo(t)
	shas, err := CommitsBefore(context.Background(), dir)
	if err != nil {
		t.Fatalf("CommitsBefore failed: %v", err)
	}
	if len(shas) == 0 {
		t.Error("expected at least one SHA, got none")
	}
	for _, sha := range shas {
		if len(sha) != 40 {
			t.Errorf("SHA %q is not 40 chars", sha)
		}
	}
}

// --- CommitsAfter ---

func TestCommitsAfterFindsNewCommits(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	before, err := CommitsBefore(ctx, dir)
	if err != nil {
		t.Fatalf("CommitsBefore: %v", err)
	}

	addCommit(t, dir, "a.txt", "a", "add a")
	addCommit(t, dir, "b.txt", "b", "add b")

	newCommits, err := CommitsAfter(ctx, dir, before)
	if err != nil {
		t.Fatalf("CommitsAfter: %v", err)
	}
	if len(newCommits) != 2 {
		t.Errorf("expected 2 new commits, got %d: %v", len(newCommits), newCommits)
	}
}

func TestCommitsAfterNoNewCommits(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	before, err := CommitsBefore(ctx, dir)
	if err != nil {
		t.Fatalf("CommitsBefore: %v", err)
	}

	newCommits, err := CommitsAfter(ctx, dir, before)
	if err != nil {
		t.Fatalf("CommitsAfter: %v", err)
	}
	if len(newCommits) != 0 {
		t.Errorf("expected 0 new commits, got %d", len(newCommits))
	}
}

// --- WorkingTreeClean ---

func TestWorkingTreeCleanOnCleanRepo(t *testing.T) {
	dir := initTestRepo(t)
	clean, err := WorkingTreeClean(context.Background(), dir)
	if err != nil {
		t.Fatalf("WorkingTreeClean: %v", err)
	}
	if !clean {
		t.Error("expected clean working tree")
	}
}

func TestWorkingTreeCleanOnDirtyRepo(t *testing.T) {
	dir := initTestRepo(t)
	f := filepath.Join(dir, "dirty.txt")
	if err := os.WriteFile(f, []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	clean, err := WorkingTreeClean(context.Background(), dir)
	if err != nil {
		t.Fatalf("WorkingTreeClean: %v", err)
	}
	if clean {
		t.Error("expected dirty working tree")
	}
}

// --- CurrentBranch ---

func TestCurrentBranch(t *testing.T) {
	dir := initTestRepo(t)
	branch, err := CurrentBranch(context.Background(), dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Error("expected non-empty branch name")
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

// runGit runs an arbitrary git command in dir, failing the test on error.
func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// --- DiffLineCount ---

func TestDiffLineCountNoRemoteReturnsZero(t *testing.T) {
	dir := initTestRepo(t)
	if n := DiffLineCount(context.Background(), dir); n != 0 {
		t.Errorf("expected 0 with no remote, got %d", n)
	}
}

func TestDiffLineCountCountsAddedAndDeletedLines(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	// Establish a remote tracking branch at the initial commit.
	remote := t.TempDir()
	runGitIn(t, remote, "init", "--bare", "--initial-branch=main", remote)
	runGitIn(t, dir, "remote", "add", "origin", remote)
	runGitIn(t, dir, "push", "-u", "origin", "main")

	// Add a new file with three lines: 3 insertions vs the merge base.
	addCommit(t, dir, "feature.go", "line1\nline2\nline3\n", "add feature")

	if n := DiffLineCount(ctx, dir); n != 3 {
		t.Errorf("expected 3 changed lines, got %d", n)
	}
}

func TestInitTestRepo_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initTestRepo(t))
}

func TestInitTestRepo_PortableUnderDefaultBranchMain(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "main")
	assertBranchIsMain(t, initTestRepo(t))
}

func TestInitTestRepo_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initTestRepo(t))
}

// MergeBaseWith returns the merge-base with the NAMED base branch (trying
// origin/<base> then <base>), so a footprint/diff check compares against the
// issue's own base — not an arbitrary remote ref. Empty/unknown base yields "".
func TestMergeBaseWith(t *testing.T) {
	dir := initTestRepo(t) // on main, one commit
	runGitIn(t, dir, "checkout", "-b", "themis-2.0")
	addCommit(t, dir, "base.go", "package x\n", "base advance")
	baseTip, err := runGit(context.Background(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	baseTip = strings.TrimSpace(baseTip)
	runGitIn(t, dir, "checkout", "-b", "issue/1")
	addCommit(t, dir, "feat.go", "package x\n", "feature work")

	if got := MergeBaseWith(context.Background(), dir, "themis-2.0"); got != baseTip {
		t.Errorf("MergeBaseWith(themis-2.0) = %q, want the fork point %q", got, baseTip)
	}
	if got := MergeBaseWith(context.Background(), dir, ""); got != "" {
		t.Errorf("empty base must yield \"\", got %q", got)
	}
	if got := MergeBaseWith(context.Background(), dir, "no-such-branch"); got != "" {
		t.Errorf("unknown base must yield \"\", got %q", got)
	}
}
