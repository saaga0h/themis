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

// --- CleanWorkingTree (issue #56: crash recovery — clean the working tree on resume) ---

// TestCleanWorkingTree_DiscardsUncommittedTrackedChanges asserts that an
// uncommitted modification to a tracked file is reverted to the last commit's
// content after CleanWorkingTree runs — the core "discard uncommitted work"
// behaviour a crash-recovery resume relies on.
func TestCleanWorkingTree_DiscardsUncommittedTrackedChanges(t *testing.T) {
	dir := initTestRepo(t)
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("uncommitted change"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := CleanWorkingTree(context.Background(), dir); err != nil {
		t.Fatalf("CleanWorkingTree: %v", err)
	}

	got, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("expected README.md reverted to committed content %q, got %q", "hello", string(got))
	}
}

// TestCleanWorkingTree_RemovesUntrackedFilesAndDirectories asserts that both a
// bare untracked file and an untracked directory (with content inside it) are
// removed by CleanWorkingTree — a crashed agent step can leave either behind.
func TestCleanWorkingTree_RemovesUntrackedFilesAndDirectories(t *testing.T) {
	dir := initTestRepo(t)

	untrackedFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	untrackedDir := filepath.Join(dir, "untracked_dir")
	if err := os.MkdirAll(untrackedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(untrackedDir, "inside.txt"), []byte("inside"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := CleanWorkingTree(context.Background(), dir); err != nil {
		t.Fatalf("CleanWorkingTree: %v", err)
	}

	if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
		t.Errorf("expected untracked file removed, stat err = %v", err)
	}
	if _, err := os.Stat(untrackedDir); !os.IsNotExist(err) {
		t.Errorf("expected untracked directory removed, stat err = %v", err)
	}
}

// TestCleanWorkingTree_ErrorPropagatesFromGit asserts that CleanWorkingTree
// returns a non-nil, wrapped error when the underlying git command fails (here,
// because dir is not a git repository at all).
func TestCleanWorkingTree_ErrorPropagatesFromGit(t *testing.T) {
	dir := t.TempDir() // not a git repo
	if _, err := CleanWorkingTree(context.Background(), dir); err == nil {
		t.Error("expected CleanWorkingTree to return an error against a non-repo directory")
	}
}

// TestCleanWorkingTree_PreservesGitignoredThemisFiles asserts that .themis/*
// files (gitignored, holding the run's own state.json and
// review-results.json) survive a CleanWorkingTree call even though they are
// untracked — CleanWorkingTree must not use the "also remove ignored files"
// (-x) form of git clean, or a resume would destroy the very state it is
// trying to recover.
func TestCleanWorkingTree_PreservesGitignoredThemisFiles(t *testing.T) {
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".themis/*\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .gitignore: %v", err)
	}
	addCommit(t, dir, ".gitignore", ".themis/*\n", "add gitignore")

	themisDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("MkdirAll .themis: %v", err)
	}
	statePath := filepath.Join(themisDir, "state.json")
	reviewPath := filepath.Join(themisDir, "review-results.json")
	if err := os.WriteFile(statePath, []byte(`{"currentStep":0}`), 0o644); err != nil {
		t.Fatalf("WriteFile state.json: %v", err)
	}
	if err := os.WriteFile(reviewPath, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile review-results.json: %v", err)
	}

	// Dirty a tracked file too, so the clean has real work to do.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile README.md: %v", err)
	}

	if _, err := CleanWorkingTree(context.Background(), dir); err != nil {
		t.Fatalf("CleanWorkingTree: %v", err)
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("expected .themis/state.json to survive CleanWorkingTree, stat err = %v", err)
	}
	if _, err := os.Stat(reviewPath); err != nil {
		t.Errorf("expected .themis/review-results.json to survive CleanWorkingTree, stat err = %v", err)
	}
}

// TestCleanWorkingTree_PreservesCommittedHistory asserts that commits made by
// prior, already-completed pipeline steps are untouched by CleanWorkingTree —
// only the uncommitted/untracked layer on top is discarded.
func TestCleanWorkingTree_PreservesCommittedHistory(t *testing.T) {
	dir := initTestRepo(t)
	addCommit(t, dir, "feature.go", "package feature\n", "add feature")

	before, err := CommitsBefore(context.Background(), dir)
	if err != nil {
		t.Fatalf("CommitsBefore: %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("expected 2 commits before clean, got %d", len(before))
	}

	// Dirty the tracked file and add an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package feature\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile feature.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("scratch"), 0o644); err != nil {
		t.Fatalf("WriteFile scratch.txt: %v", err)
	}

	if _, err := CleanWorkingTree(context.Background(), dir); err != nil {
		t.Fatalf("CleanWorkingTree: %v", err)
	}

	after, err := CommitsBefore(context.Background(), dir)
	if err != nil {
		t.Fatalf("CommitsBefore after clean: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("expected commit count unchanged, before=%d after=%d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("commit SHA changed at index %d: before=%s after=%s", i, before[i], after[i])
		}
	}

	got, err := os.ReadFile(filepath.Join(dir, "feature.go"))
	if err != nil {
		t.Fatalf("ReadFile feature.go: %v", err)
	}
	if string(got) != "package feature\n" {
		t.Errorf("expected feature.go content reverted to committed version, got %q", string(got))
	}
}
