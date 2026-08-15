package runner

// Shared stubs, fixtures, and repo helpers for the runner package's tests,
// plus the self-tests that exercise those helpers. Every runner_*_test.go file
// draws its harness from here.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
	"github.com/saaga0h/themis/internal/tracker"
)

// fakeGitOps is a configurable stub that satisfies the runner.GitOps interface
// (defined as part of issue #61). All methods are no-ops by default; individual
// fields may be overridden to return specific values or record calls.
//
// This stub is used by:
//   - TestGitOps_InterfaceHasRequiredMethods (compile-time assertion)
//   - TestConfig_GitFieldAcceptsGitOpsImpl   (Config.Git field acceptance)
//   - updated Config literals that previously set GitBranchFn/GitPushFn
type fakeGitOps struct {
	currentBranchFn func(ctx context.Context, dir string) (string, error)
	commitSHAsFn    func(ctx context.Context, dir string) ([]string, error)
	checkedOut      []string
	pushed          []string
	commitLog       string
	commitsAhead    int
	changedFiles    string
	diffLines       int
}

func (f *fakeGitOps) CheckoutNewBranch(ctx context.Context, dir, name string) error {
	f.checkedOut = append(f.checkedOut, name)
	return nil
}

func (f *fakeGitOps) Checkout(ctx context.Context, dir, name string) error {
	f.checkedOut = append(f.checkedOut, name)
	return nil
}

func (f *fakeGitOps) PushBranch(ctx context.Context, dir, branch string) error {
	f.pushed = append(f.pushed, branch)
	return nil
}

func (f *fakeGitOps) CurrentBranch(ctx context.Context, dir string) (string, error) {
	if f.currentBranchFn != nil {
		return f.currentBranchFn(ctx, dir)
	}
	return "HEAD", nil
}

func (f *fakeGitOps) BranchCommitLog(ctx context.Context, dir string) string {
	return f.commitLog
}

func (f *fakeGitOps) CommitsAheadOfBase(ctx context.Context, dir, base string) (int, error) {
	return f.commitsAhead, nil
}

func (f *fakeGitOps) ChangedFiles(_ context.Context, _ string) string {
	return f.changedFiles
}

func (f *fakeGitOps) DiffLineCount(_ context.Context, _ string) int {
	return f.diffLines
}

func (f *fakeGitOps) CommitSHAs(ctx context.Context, dir string) ([]string, error) {
	if f.commitSHAsFn != nil {
		return f.commitSHAsFn(ctx, dir)
	}
	return nil, nil
}

// Compile-time assertion: fakeGitOps must implement GitOps.
// This line will fail to compile until GitOps is defined in the runner package.
var _ GitOps = &fakeGitOps{}

// stubFetcher returns a fixed IssueData.
type stubFetcher struct {
	issue *tracker.IssueData
	err   error
}

func (s *stubFetcher) Fetch(_ context.Context, _ int) (*tracker.IssueData, error) {
	return s.issue, s.err
}

// stubInvoker returns results from a fixed list, then repeats a default result.
type stubInvoker struct {
	results []*agent.InvokeResult
	calls   int
}

func (s *stubInvoker) Invoke(_ context.Context, _ agent.InvokeOptions) (*agent.InvokeResult, error) {
	if s.calls < len(s.results) {
		r := s.results[s.calls]
		s.calls++
		return r, nil
	}
	s.calls++
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

// captureInvoker captures prompts sent to the agent.
type captureInvoker struct {
	prompts []string
	result  *agent.InvokeResult
}

func (c *captureInvoker) Invoke(_ context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	c.prompts = append(c.prompts, opts.Prompt)
	if c.result != nil {
		return c.result, nil
	}
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

// recordingInvoker captures each Invoke call's InvokeOptions and returns results
// from a fixed list; once exhausted it returns a default completed result.
type recordingInvoker struct {
	opts    []agent.InvokeOptions
	results []*agent.InvokeResult
	idx     int
}

func (r *recordingInvoker) Invoke(_ context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	r.opts = append(r.opts, opts)
	if r.idx < len(r.results) {
		res := r.results[r.idx]
		r.idx++
		return res, nil
	}
	r.idx++
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

// stubIssueWriter records issue tracker operations and the fields seen by CreatePR.
//
// addLabelErr and commentErr default to nil, preserving the always-succeeds
// behavior existing call sites rely on; set them to inject failures, e.g. for
// exercising blockIssue's failure paths.
type stubIssueWriter struct {
	labelsAdded   []string
	labelsRemoved []string
	comments      []string
	prBodySeen    string
	prBaseSeen    string
	prHeadSeen    string
	prDraftSeen   bool
	prURL         string
	addLabelErr   error
	commentErr    error
	createPRErr   error
}

func (s *stubIssueWriter) AddLabel(_ context.Context, _ int, label string) error {
	s.labelsAdded = append(s.labelsAdded, label)
	return s.addLabelErr
}

func (s *stubIssueWriter) RemoveLabel(_ context.Context, _ int, label string) error {
	s.labelsRemoved = append(s.labelsRemoved, label)
	return nil
}

func (s *stubIssueWriter) Comment(_ context.Context, _ int, body string) error {
	s.comments = append(s.comments, body)
	return s.commentErr
}

func (s *stubIssueWriter) CreatePR(_ context.Context, opts PROptions) (string, error) {
	s.prBodySeen = opts.Body
	s.prBaseSeen = opts.Base
	s.prHeadSeen = opts.Head
	s.prDraftSeen = opts.Draft
	if s.createPRErr != nil {
		return "", s.createPRErr
	}
	if s.prURL != "" {
		return s.prURL, nil
	}
	return "https://git.example.com/pr/99", nil
}

func sampleIssue() *tracker.IssueData {
	return &tracker.IssueData{
		Number: 42,
		Title:  "Test Issue",
		Body:   "## AC\n- [ ] First AC\n- [ ] Second AC",
		Labels: []string{"ready-for-agent"},
		URL:    "https://git.example.com/issues/42",
	}
}

func noopCheckpoint(_ context.Context, _ pipeline.Step, _ string) error {
	return nil
}

// templateDir locates the real templates directory by walking up from cwd.
func templateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "templates")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("templates/ not found")
		}
		dir = parent
	}
}

// makeTemplateDir creates a temp dir populated with the given template files.
func makeTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}
	return dir
}

// saveStateAt persists a minimal PipelineState at the given step to workDir.
func saveStateAt(t *testing.T, workDir string, step pipeline.Step) {
	t.Helper()
	s := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     step,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
}

func baseConfig(t *testing.T, w *stubIssueWriter, f *stubFetcher, inv agent.Invoker) Config {
	t.Helper()
	return Config{
		WorkDir:      t.TempDir(),
		IssueNumber:  42,
		Fetcher:      f,
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}
}

// logConfig returns a Config wired to buf for log capture.
func logConfig(t *testing.T, buf *bytes.Buffer, w *stubIssueWriter, inv agent.Invoker) Config {
	t.Helper()
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.Logger = buf
	return cfg
}

// writeReviewResults marshals findings to .themis/review-results.json in workDir.
func writeReviewResults(t *testing.T, workDir string, findings []review.ReviewFinding) {
	t.Helper()
	themisDir := filepath.Join(workDir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir .themis: %v", err)
	}
	data, err := json.Marshal(review.ReviewResults{Findings: findings})
	if err != nil {
		t.Fatalf("marshal review results: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themisDir, "review-results.json"), data, 0o644); err != nil {
		t.Fatalf("write review-results.json: %v", err)
	}
}

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

// initLocalRepo creates a real git repo with a single initial commit and no remote.
func initLocalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInDir(t, dir, "init", "--initial-branch=main")
	gitInDir(t, dir, "config", "user.email", "test@test")
	gitInDir(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "README.md")
	gitInDir(t, dir, "commit", "-m", "initial commit")
	return dir
}

// initRepoWithRemote creates a workspace cloned from a local bare repo with an
// initial commit pushed to origin/main, ready for feature branches.
func initRepoWithRemote(t *testing.T) string {
	t.Helper()

	bareDir := t.TempDir()
	gitInDir(t, bareDir, "init", "--bare", "--initial-branch=main")

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

// initBranchWithCommits extends a remote-backed repo with a new branch containing
// one commit per entry in messages (conventional-commit subjects recommended).
func initBranchWithCommits(t *testing.T, messages []string) string {
	t.Helper()
	workDir := initRepoWithRemote(t)
	gitInDir(t, workDir, "checkout", "-b", "feature/work")
	for i, msg := range messages {
		fname := fmt.Sprintf("file_%d.go", i)
		if err := os.WriteFile(filepath.Join(workDir, fname), []byte("package p"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		gitInDir(t, workDir, "add", fname)
		gitInDir(t, workDir, "commit", "-m", msg)
	}
	return workDir
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

func TestInitLocalRepo_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initLocalRepo(t))
}

func TestInitLocalRepo_PortableUnderDefaultBranchMain(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "main")
	assertBranchIsMain(t, initLocalRepo(t))
}

func TestInitLocalRepo_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initLocalRepo(t))
}

func TestInitRepoWithRemote_DefaultBranchIsMain(t *testing.T) {
	assertBranchIsMain(t, initRepoWithRemote(t))
}

func TestInitRepoWithRemote_PortableUnderDefaultBranchMain(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "main")
	assertBranchIsMain(t, initRepoWithRemote(t))
}

func TestInitRepoWithRemote_PortableUnderDefaultBranchMaster(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "master")
	assertBranchIsMain(t, initRepoWithRemote(t))
}
