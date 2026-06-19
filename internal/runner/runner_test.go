package runner

// Core runner behaviour: pipeline advancement, step sequencing, state resume,
// template-argument substitution, and run logging. Review-step and ship-step
// behaviour live in runner_review_test.go and runner_ship_test.go. Checkpoint
// integration lives in runner_checkpoint_test.go.
//
// Tests are white-box (package runner) so the review unit tests can reference
// unexported functions; all runner tests share the single set of stubs and
// helpers defined in this file.

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
	"github.com/saaga0h/themis/internal/tracker"
)

// ---------------------------------------------------------------------------
// Shared stubs
// ---------------------------------------------------------------------------

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
type stubIssueWriter struct {
	labelsAdded   []string
	labelsRemoved []string
	comments      []string
	prBodySeen    string
	prBaseSeen    string
	prHeadSeen    string
	prURL         string
}

func (s *stubIssueWriter) AddLabel(_ context.Context, _ int, label string) error {
	s.labelsAdded = append(s.labelsAdded, label)
	return nil
}

func (s *stubIssueWriter) RemoveLabel(_ context.Context, _ int, label string) error {
	s.labelsRemoved = append(s.labelsRemoved, label)
	return nil
}

func (s *stubIssueWriter) Comment(_ context.Context, _ int, body string) error {
	s.comments = append(s.comments, body)
	return nil
}

func (s *stubIssueWriter) CreatePR(_ context.Context, opts PROptions) (string, error) {
	s.prBodySeen = opts.Body
	s.prBaseSeen = opts.Base
	s.prHeadSeen = opts.Head
	if s.prURL != "" {
		return s.prURL, nil
	}
	return "https://git.example.com/pr/99", nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

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
		MaxReviewCycles: 2,
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
func writeReviewResults(t *testing.T, workDir string, findings []ReviewFinding) {
	t.Helper()
	themisDir := filepath.Join(workDir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir .themis: %v", err)
	}
	data, err := json.Marshal(ReviewResults{Findings: findings})
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

// ---------------------------------------------------------------------------
// Core: pipeline advancement and step sequencing
// ---------------------------------------------------------------------------

// Pipeline runner loads profile, fetches issue, and runs the pipeline steps in order

func TestRunner_RunsStepsInOrder(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/1"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL == "" {
		t.Error("Run should produce a PR URL on success")
	}
}

// Pipeline runner loads the correct prompt template for each step and substitutes issue-specific arguments

func TestRunner_SubstitutesIssueNumberInTemplate(t *testing.T) {
	inv := &captureInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/2"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.prompts) == 0 {
		t.Fatal("no prompts sent to invoker")
	}
	for i, p := range inv.prompts {
		if strings.Contains(p, "{{ISSUE_NUMBER}}") {
			t.Errorf("prompt[%d]: placeholder {{ISSUE_NUMBER}} was not substituted", i)
		}
		if strings.Contains(p, "{{ACCEPTANCE_CRITERIA}}") {
			t.Errorf("prompt[%d]: placeholder {{ACCEPTANCE_CRITERIA}} was not substituted", i)
		}
	}
}

func TestRunner_IncludesIssueNumberInPrompt(t *testing.T) {
	inv := &captureInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/3"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	found := false
	for _, p := range inv.prompts {
		if strings.Contains(p, "42") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no prompt contained the issue number 42")
	}
}

// Pipeline state is saved to .themis/state.json after each step

func TestRunner_SavesStateFile(t *testing.T) {
	workDir := t.TempDir()
	w := &stubIssueWriter{prURL: "https://example.com/pr/4"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	stateFile := filepath.Join(workDir, ".themis", "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error(".themis/state.json should exist after run completes")
	}
}

// Resume — pipeline resumes from saved state (kill and restart)

func TestRunner_ResumesFromSavedState(t *testing.T) {
	workDir := t.TempDir()

	// Pre-populate state: already at Implement step
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepImplement,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	inv := &captureInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/5"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// First prompt should NOT be the test-red template (we skipped it)
	if len(inv.prompts) == 0 {
		t.Fatal("no prompts invoked")
	}
	firstPrompt := strings.ToLower(inv.prompts[0])
	if strings.Contains(firstPrompt, "write failing tests") || strings.Contains(firstPrompt, "test-red") {
		t.Errorf("should resume at Implement, not TestRed — first prompt starts: %q", inv.prompts[0][:min(80, len(inv.prompts[0]))])
	}
}

// Pipeline runner stops with blocked label and issue comment when test-fix limit (3) is reached

func TestRunner_StopsOnTestFixLimit(t *testing.T) {
	workDir := t.TempDir()

	// TestFixAttempts for "ac-0" already at 3 — next advance will error
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepTestRed,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{"ac-0": 3},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Invoker returns a "tests failed" result so runner tries to advance TestRed again
	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: false, TestsPassed: false},
		},
	}
	w := &stubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		TestACKey:    "ac-0",
	}

	result, err := Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run should return error when test-fix limit exceeded")
	}
	if result != nil && result.PRURL != "" {
		t.Error("should not create PR when blocked")
	}

	hasBlocked := false
	for _, l := range w.labelsAdded {
		if l == "blocked" {
			hasBlocked = true
		}
	}
	if !hasBlocked {
		t.Errorf("blocked label not added; labelsAdded=%v", w.labelsAdded)
	}
	if len(w.comments) == 0 {
		t.Error("must post a comment explaining the block")
	}
}

// ---------------------------------------------------------------------------
// Core: template arguments (CHANGED_FILES, PIPELINE_SHAPE, COMMIT_LOG)
// ---------------------------------------------------------------------------

// changedFiles returns the list of files changed on the current branch vs the base branch,
// using git diff --name-only against the merge-base.
func TestRunner_ChangedFiles_IncludesFilesChangedOnBranch(t *testing.T) {
	workDir := initRepoWithRemote(t)

	// Create a feature branch and add a new file
	gitInDir(t, workDir, "checkout", "-b", "feat/changed-files")
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
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{changedFiles: "newfeature.go", commitsAhead: 1},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
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

// changedFiles returns empty string gracefully when git diff fails
// (e.g., no remote tracking branch exists).
func TestRunner_ChangedFiles_ReturnsEmptyStringWhenGitFails(t *testing.T) {
	dir := initLocalRepo(t) // no remote set up

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
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{commitsAhead: 1},
	}

	// Run must succeed even when changedFiles cannot diff against a remote
	_, err := Run(context.Background(), cfg)
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

// {{PIPELINE_SHAPE}} is a one-line summary computed from commit message prefixes
// for commits on the issue branch that are not on the base branch.
func TestRunner_PipelineShapeFromCommitPrefixes(t *testing.T) {
	fakeLog := "abc1234 test(runner): add failing tests\n" +
		"def5678 feat(runner): implement review output capture"

	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}\nShape:{{PIPELINE_SHAPE}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{commitLog: fakeLog, commitsAhead: 1},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected Docs step to invoke agent; got 0 calls")
	}
	docsPrompt := inv.opts[0].Prompt
	if strings.Contains(docsPrompt, "{{PIPELINE_SHAPE}}") {
		t.Error("{{PIPELINE_SHAPE}} placeholder must be substituted")
	}
	// The summary must reflect the commit type prefixes present on the branch
	if !strings.Contains(docsPrompt, "test") {
		t.Errorf("PIPELINE_SHAPE must include 'test' prefix from branch commits\ngot prompt: %s", docsPrompt)
	}
	if !strings.Contains(docsPrompt, "feat") {
		t.Errorf("PIPELINE_SHAPE must include 'feat' prefix from branch commits\ngot prompt: %s", docsPrompt)
	}
}

// CommitCountFn must be wired from GitOps.CommitSHAs so the agent layer can
// snapshot commits before/after an invocation and populate InvokeResult.CommitsMade.
// Before this fix the runner never set CommitCountFn, leaving CommitsMade empty
// and causing the pipeline to loop on TestRed.
func TestRunner_SetsCommitCountFnWhenGitIsConfigured(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}

	var sentinelCalled bool
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{
			commitsAhead: 1,
			commitSHAsFn: func(_ context.Context, _ string) ([]string, error) {
				sentinelCalled = true
				return nil, nil
			},
		},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected Docs step to invoke agent; got 0 calls")
	}
	if inv.opts[0].CommitCountFn == nil {
		t.Fatal("CommitCountFn must be non-nil when cfg.Git is set")
	}
	// The wired function must be GitOps.CommitSHAs, not some unrelated closure.
	if _, err := inv.opts[0].CommitCountFn(context.Background(), workDir); err != nil {
		t.Fatalf("wired CommitCountFn returned error: %v", err)
	}
	if !sentinelCalled {
		t.Error("CommitCountFn must delegate to GitOps.CommitSHAs")
	}
}

// {{COMMIT_LOG}} contains git log --oneline output for commits on the issue branch
// relative to the base branch.
func TestRunner_CommitLogContainsBranchCommits(t *testing.T) {
	commits := []string{
		"test(runner): add failing tests",
		"feat(runner): implement feature",
	}
	fakeLog := "abc1234 " + commits[0] + "\ndef5678 " + commits[1]

	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}\nLog:{{COMMIT_LOG}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{commitLog: fakeLog, commitsAhead: 1},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected Docs step to invoke agent; got 0 calls")
	}
	docsPrompt := inv.opts[0].Prompt
	if strings.Contains(docsPrompt, "{{COMMIT_LOG}}") {
		t.Error("{{COMMIT_LOG}} placeholder must be substituted")
	}
	// Every commit on the branch must appear in the log output
	for _, msg := range commits {
		if !strings.Contains(docsPrompt, msg) {
			t.Errorf("COMMIT_LOG missing branch commit %q\ngot prompt:\n%s", msg, docsPrompt)
		}
	}
}

// Both context args (PIPELINE_SHAPE, COMMIT_LOG) must be present in masterArgs
// so filterArgs can pass them to templates that reference them.
func TestRunner_AllNewTemplateArgsAvailableInMasterArgs(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	// Template references both context args — verifies each is in masterArgs
	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "{{ISSUE_NUMBER}} {{PIPELINE_SHAPE}} {{COMMIT_LOG}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	// Must succeed: all new args must be in masterArgs so filterArgs can supply them
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: new args missing from masterArgs (filterArgs cannot protect against absent args): %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected Docs step to invoke agent; got 0 calls")
	}
	// No unresolved placeholders may remain in the rendered prompt
	docsPrompt := inv.opts[0].Prompt
	if strings.Contains(docsPrompt, "{{") {
		t.Errorf("unresolved placeholder remains in Docs prompt; all new args must be in masterArgs:\n%s", docsPrompt)
	}
}

// ---------------------------------------------------------------------------
// Core: run logging and observability
// ---------------------------------------------------------------------------

// every step transition prints start and done messages to stderr with duration.

func TestRunner_LogsStepStartMessage(t *testing.T) {
	var buf bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/start"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// Every step that executes must appear by name in the log.
	for _, step := range []pipeline.Step{
		pipeline.StepFetch, pipeline.StepTestRed, pipeline.StepImplement,
		pipeline.StepReview, pipeline.StepShip,
	} {
		name := step.String()
		if !strings.Contains(output, name) {
			t.Errorf("log must mention step %q; got:\n%s", name, output)
		}
	}
}

func TestRunner_LogsStepDoneWithDuration(t *testing.T) {
	var buf bytes.Buffer
	w := &stubIssueWriter{prURL: "https://example.com/pr/done"}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "done") {
		t.Errorf("log must contain 'done' message for step completion; got:\n%s", output)
	}
	// Duration must appear in done messages — accept "Xms" or "X.Xs".
	if !strings.Contains(output, "ms") {
		t.Errorf("log done message must include millisecond duration (e.g. '5ms'); got:\n%s", output)
	}
}

// state resume is logged: "resuming from step X" vs. "fresh start".

func TestRunner_LogsFreshStartWhenNoStateFile(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir() // no .themis/state.json
	w := &stubIssueWriter{prURL: "https://example.com/pr/fresh"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "fresh start") {
		t.Errorf("expected 'fresh start' in log when no state file; got:\n%s", output)
	}
}

func TestRunner_LogsResumingFromStepWhenStateFileExists(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)

	w := &stubIssueWriter{prURL: "https://example.com/pr/resume"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "resuming") {
		t.Errorf("expected 'resuming' in log when state file exists; got:\n%s", output)
	}
	// The step name must appear so the user knows exactly where we resumed.
	if !strings.Contains(output, pipeline.StepImplement.String()) {
		t.Errorf("expected step name %q in resume log; got:\n%s", pipeline.StepImplement.String(), output)
	}
}

// agent invocations log the model and max turns.

func TestRunner_LogsAgentInvocationModelAndMaxTurns(t *testing.T) {
	var buf bytes.Buffer
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := logConfig(t, &buf, w, inv)
	cfg.MaxTurns = 250

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// The default profile model is "sonnet" for all non-review agent steps.
	if !strings.Contains(output, "sonnet") {
		t.Errorf("expected model name 'sonnet' in agent invocation log; got:\n%s", output)
	}
	// Max turns must reflect the configured value, not a hardcoded constant.
	if !strings.Contains(output, "250") {
		t.Errorf("expected max turns '250' in agent invocation log; got:\n%s", output)
	}
}

// agent results log commit count and completion status.

func TestRunner_LogsAgentResultCommitCount(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			// TestRed makes 2 commits and completes.
			{ExitCode: 0, Completed: true, CommitsMade: []string{"abc1234", "def5678"}},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/count"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	// "2 commits" (or "2 commit") must appear in the result log.
	if !strings.Contains(output, "2") || !strings.Contains(strings.ToLower(output), "commit") {
		t.Errorf("expected '2 commits' in agent result log; got:\n%s", output)
	}
}

func TestRunner_LogsAgentResultCompletionStatus(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			// First invocation: not completed, no commits → runner retries TestRed.
			{ExitCode: 0, Completed: false, CommitsMade: nil},
			// Second invocation: completed → pipeline advances.
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/status"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	lc := strings.ToLower(output)
	// First invocation result must log "not completed" or equivalent.
	if !strings.Contains(lc, "not completed") && !strings.Contains(lc, "incomplete") {
		t.Errorf("expected 'not completed'/'incomplete' in agent result log when Completed=false; got:\n%s", output)
	}
	// Second invocation result must log "completed".
	if !strings.Contains(lc, "completed") {
		t.Errorf("expected 'completed' in agent result log when Completed=true; got:\n%s", output)
	}
}

// checkpoint results are logged (pass or the specific failure).

func TestRunner_LogsCheckpointPassAfterStep(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	w := &stubIssueWriter{prURL: "https://example.com/pr/pass"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	lc := strings.ToLower(output)
	if !strings.Contains(lc, "checkpoint") {
		t.Errorf("expected 'checkpoint' in log after agent step; got:\n%s", output)
	}
	if !strings.Contains(lc, "pass") && !strings.Contains(lc, "ok") {
		t.Errorf("expected 'pass' or 'ok' in checkpoint log when it succeeds; got:\n%s", output)
	}
}

func TestRunner_LogsCheckpointFailureWithSpecificErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	const failMsg = "missing test(runner): prefix in last commit"
	failCheckpoint := func(_ context.Context, _ pipeline.Step, _ string) error {
		return fmt.Errorf("%s", failMsg)
	}

	w := &stubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{results: []*agent.InvokeResult{{ExitCode: 0, Completed: true}}},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: failCheckpoint,
		Logger:       &buf,
	}

	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run must return error when checkpoint fails")
	}

	output := buf.String()
	// The specific error text must appear so the operator knows why the checkpoint failed.
	if !strings.Contains(output, failMsg) {
		t.Errorf("checkpoint failure message %q must appear in log; got:\n%s", failMsg, output)
	}
}

// ship step logs the PR URL.

func TestRunner_LogsShipPRURL(t *testing.T) {
	var buf bytes.Buffer
	const prURL = "https://example.com/pr/example"
	w := &stubIssueWriter{prURL: prURL}
	cfg := logConfig(t, &buf, w, &stubInvoker{})

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL != prURL {
		t.Fatalf("PRURL: got %q, want %q", result.PRURL, prURL)
	}

	output := buf.String()
	if !strings.Contains(output, prURL) {
		t.Errorf("ship step must log PR URL %q; got:\n%s", prURL, output)
	}
}

// all output goes to stderr (stdout is reserved for the final PR URL).
// When Logger is nil, Run must default to os.Stderr and must not panic.

func TestRunner_NilLoggerDefaultsToStderrWithoutPanic(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})
	cfg.Logger = nil // explicitly nil: must fall back to os.Stderr, not panic

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed when Logger is nil (defaults to os.Stderr): %v", err)
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

// ---------------------------------------------------------------------------
// OTEL: runner passes IssueNumber and PipelineStep to agent invoker
// ---------------------------------------------------------------------------

func TestRunnerAgentStepPassesIssueNumberToInvoker(t *testing.T) {
	workDir := t.TempDir()
	// Start at TestRed so we get exactly one agent-step invocation before the test ends.
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/otel"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected at least one agent invocation")
	}
	// The first agent-step invocation must carry the configured issue number.
	if inv.opts[0].IssueNumber != cfg.IssueNumber {
		t.Errorf("agent-step InvokeOptions.IssueNumber = %d, want %d", inv.opts[0].IssueNumber, cfg.IssueNumber)
	}
}

func TestRunnerAgentStepPassesPipelineStepToInvoker(t *testing.T) {
	workDir := t.TempDir()
	// Start at TestRed so the first invocation corresponds to that step.
	saveStateAt(t, workDir, pipeline.StepTestRed)

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/otel"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected at least one agent invocation")
	}
	// The first agent-step invocation must carry the step name "TestRed".
	if inv.opts[0].PipelineStep != pipeline.StepTestRed.String() {
		t.Errorf("agent-step InvokeOptions.PipelineStep = %q, want %q",
			inv.opts[0].PipelineStep, pipeline.StepTestRed.String())
	}
}

func TestRunnerShipStepPassesIssueNumberToInvoker(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/ship-otel"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// The ship step is the last invocation (index pipelineAgentCallCount).
	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", want, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.IssueNumber != cfg.IssueNumber {
		t.Errorf("ship-step InvokeOptions.IssueNumber = %d, want %d", shipOpts.IssueNumber, cfg.IssueNumber)
	}
}

func TestRunnerShipStepPassesPipelineStepToInvoker(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/ship-otel"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", want, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.PipelineStep != pipeline.StepShip.String() {
		t.Errorf("ship-step InvokeOptions.PipelineStep = %q, want %q",
			shipOpts.PipelineStep, pipeline.StepShip.String())
	}
}

// ---------------------------------------------------------------------------
// MaxTurns: Config.MaxTurns propagates to every agent invocation (issue #52)
// ---------------------------------------------------------------------------

// TestRunner_PassesConfigMaxTurnsToEveryAgentStep verifies that when
// Config.MaxTurns is set, every agent pipeline invocation (all 5 agent steps
// plus the ship step) receives that value in InvokeOptions.MaxTurns.
func TestRunner_PassesConfigMaxTurnsToEveryAgentStep(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/maxturns"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.MaxTurns = 300

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// pipelineAgentCallCount (5) agent steps + 1 ship step = 6 total invocations.
	totalExpected := pipelineAgentCallCount + 1
	if len(inv.opts) < totalExpected {
		t.Fatalf("expected at least %d invocations, got %d", totalExpected, len(inv.opts))
	}
	for i, opts := range inv.opts {
		if opts.MaxTurns != 300 {
			t.Errorf("invocation %d: InvokeOptions.MaxTurns = %d, want 300", i, opts.MaxTurns)
		}
	}
}

// TestRunner_PassesConfigMaxTurnsToShipStep verifies specifically that the ship
// step invocation uses Config.MaxTurns, not a hardcoded literal.
func TestRunner_PassesConfigMaxTurnsToShipStep(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/maxturns-ship"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)
	cfg.MaxTurns = 300

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	totalExpected := pipelineAgentCallCount + 1
	if len(inv.opts) < totalExpected {
		t.Fatalf("expected at least %d invocations (pipeline + ship), got %d", totalExpected, len(inv.opts))
	}
	shipOpts := inv.opts[pipelineAgentCallCount]
	if shipOpts.MaxTurns != 300 {
		t.Errorf("ship-step InvokeOptions.MaxTurns = %d, want 300", shipOpts.MaxTurns)
	}
}

// ---------------------------------------------------------------------------
// AC2–AC5 / AC7–AC8: GitOps interface, Config.Git, Config.ProfileLoader (issue #61)
// ---------------------------------------------------------------------------

// TestGitOps_InterfaceHasRequiredMethods asserts that the GitOps interface
// exists in the runner package with exactly the method signatures used by
// runner.go. The compile-time assertion is the var _ GitOps = &fakeGitOps{}
// line at the top of the shared stubs section; this test body provides a
// named anchor for the mapping and verifies the stub can be created.
func TestGitOps_InterfaceHasRequiredMethods(t *testing.T) {
	// fakeGitOps (defined in shared stubs) implements every GitOps method.
	// If GitOps does not exist in the runner package this file fails to compile.
	g := &fakeGitOps{commitsAhead: 2}

	ctx := context.Background()
	dir := t.TempDir()

	// CheckoutNewBranch
	if err := g.CheckoutNewBranch(ctx, dir, "feat/x"); err != nil {
		t.Errorf("CheckoutNewBranch: %v", err)
	}
	// Checkout
	if err := g.Checkout(ctx, dir, "main"); err != nil {
		t.Errorf("Checkout: %v", err)
	}
	// PushBranch
	if err := g.PushBranch(ctx, dir, "feat/x"); err != nil {
		t.Errorf("PushBranch: %v", err)
	}
	// CurrentBranch
	branch, err := g.CurrentBranch(ctx, dir)
	if err != nil {
		t.Errorf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Error("CurrentBranch must return a non-empty string")
	}
	// BranchCommitLog
	_ = g.BranchCommitLog(ctx, dir)
	// CommitsAheadOfBase
	n, err := g.CommitsAheadOfBase(ctx, dir, "main")
	if err != nil {
		t.Errorf("CommitsAheadOfBase: %v", err)
	}
	if n != 2 {
		t.Errorf("CommitsAheadOfBase = %d, want 2", n)
	}
}

// TestConfig_GitFieldAcceptsGitOpsImpl asserts that Config has a Git field of
// type GitOps and accepts a fakeGitOps value. Fails to compile until Config.Git
// is defined in the runner package.
func TestConfig_GitFieldAcceptsGitOpsImpl(t *testing.T) {
	g := &fakeGitOps{}
	cfg := Config{
		Git: g, // fails to compile: no Git field on Config yet
	}
	if cfg.Git == nil {
		t.Error("Config.Git must be non-nil after assignment")
	}
}

// TestConfig_HasProfileLoaderField asserts that Config has a ProfileLoader
// field of type func(dir string) (*profile.Profile, error). This mirrors the
// signature of profile.Load so the runner can call ProfileLoader(cfg.WorkDir)
// instead of importing internal/profile directly.
//
// Fails to compile until Config.ProfileLoader is defined in the runner package.
func TestConfig_HasProfileLoaderField(t *testing.T) {
	called := false
	cfg := Config{
		ProfileLoader: func(dir string) (ProfileData, error) {
			called = true
			return ProfileData{ImplementModel: "sonnet", ReviewModel: "sonnet"}, nil
		},
	}
	if cfg.ProfileLoader == nil {
		t.Error("Config.ProfileLoader must be non-nil after assignment")
	}
	// Call it to confirm the function signature is correct.
	p, err := cfg.ProfileLoader(t.TempDir())
	if err != nil {
		t.Fatalf("ProfileLoader returned unexpected error: %v", err)
	}
	if p.ImplementModel == "" {
		t.Error("ProfileLoader must return a ProfileData with ImplementModel set")
	}
	if !called {
		t.Error("ProfileLoader was not called")
	}
}

// ---------------------------------------------------------------------------
// AC2–AC4: structural properties of runner.go (source-reading tests)
// ---------------------------------------------------------------------------

// TestRunner_BlockingThresholdNotDefinedInRunnerPackage asserts that the
// blockingThreshold constant has been moved out of runner.go into internal/review/.
// FAILS NOW: runner.go still contains "const blockingThreshold".
func TestRunner_BlockingThresholdNotDefinedInRunnerPackage(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	if strings.Contains(string(src), "const blockingThreshold") {
		t.Error("runner.go must not define const blockingThreshold — it belongs in internal/review/")
	}
}

// TestRunner_NoReviewTypeDeclarations_InRunnerGo asserts that ReviewFinding and
// ReviewResults have been extracted from runner.go.
// FAILS NOW: both types are declared in runner.go.
func TestRunner_NoReviewTypeDeclarations_InRunnerGo(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	content := string(src)
	if strings.Contains(content, "type ReviewFinding") {
		t.Error("runner.go must not declare type ReviewFinding — it belongs in internal/review/")
	}
	if strings.Contains(content, "type ReviewResults") {
		t.Error("runner.go must not declare type ReviewResults — it belongs in internal/review/")
	}
}

// TestRunner_NoReviewFuncDeclarations_InRunnerGo asserts that the four
// review-results functions have been extracted from runner.go.
// FAILS NOW: all four are still defined in runner.go.
func TestRunner_NoReviewFuncDeclarations_InRunnerGo(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	content := string(src)
	for _, fn := range []string{
		"func readReviewResults",
		"func countFindingsBySeverity",
		"func determineBlockingStatus",
		"func formatBlockingFindings",
	} {
		if strings.Contains(content, fn) {
			t.Errorf("runner.go must not define %s — it belongs in internal/review/", fn)
		}
	}
}

// TestRunner_ImportsList_ContainsInternalReview asserts that runner.go imports
// the internal/review package after extraction.
// FAILS NOW: the import is not present.
func TestRunner_ImportsList_ContainsInternalReview(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	if !strings.Contains(string(src), `"github.com/saaga0h/themis/internal/review"`) {
		t.Error(`runner.go must import "github.com/saaga0h/themis/internal/review" after extraction`)
	}
}

// TestRunner_RunnerGoIsUnder500Lines asserts that runner.go has fewer than 500
// lines after the review-results extraction.
// FAILS NOW: runner.go is over 500 lines.
func TestRunnerGo_LineCountUnder500(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	lines := strings.Count(string(src), "\n")
	if lines >= 500 {
		t.Errorf("runner.go has %d lines; must be < 500 after extraction", lines)
	}
}

// TestRunner_UsesProfileLoaderFromConfig starts the pipeline at StepTestRed and
// injects a ProfileLoader stub that returns a known Review model name
// ("stub-model"). It then verifies that the agent is invoked with that model
// at the Review step, which can only happen if the runner calls
// cfg.ProfileLoader instead of profile.Load directly.
//
// Fails at runtime (not compile-time) until runner.go is updated.
func TestRunner_UsesProfileLoaderFromConfig(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepTestRed)

	// ReviewResults needed to mark review non-blocking after the Review step.
	writeReviewResults(t, workDir, []ReviewFinding{})

	const stubModel = "stub-model-for-review"

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/profile-loader"}
	cfg := Config{
		WorkDir:     workDir,
		IssueNumber: 42,
		Fetcher:     &stubFetcher{issue: sampleIssue()},
		Invoker:     inv,
		IssueWriter: w,
		TemplateDir: templateDir(t),
		CheckpointFn: noopCheckpoint,
		ProfileLoader: func(dir string) (ProfileData, error) {
			return ProfileData{
				ReviewModel:    stubModel,
				ImplementModel: "sonnet",
			}, nil
		},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Find the Review step invocation and confirm the model used matches stubModel.
	found := false
	for _, opts := range inv.opts {
		if opts.PipelineStep == pipeline.StepReview.String() {
			if opts.Model != stubModel {
				t.Errorf("Review step: model = %q, want %q — runner must use ProfileLoader, not profile.Load directly",
					opts.Model, stubModel)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Review step was never invoked; got %d invocations with steps: %v",
			len(inv.opts), func() []string {
				steps := make([]string, len(inv.opts))
				for i, o := range inv.opts {
					steps[i] = o.PipelineStep
				}
				return steps
			}())
	}
}
