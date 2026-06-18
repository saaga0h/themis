package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/runner"
)

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

// makeTemplateDir creates a temporary directory populated with the given template files.
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

// initBranchWithCommits extends a remote-backed repo with a new branch containing
// one commit per entry in messages (conventional-commit subjects recommended).
func initBranchWithCommits(t *testing.T, messages []string) string {
	t.Helper()
	workDir := initIssue22RepoWithRemote(t)
	gitInDir(t, workDir, "checkout", "-b", "issue/26-test")
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

// AC1 + AC2: Runner accumulates review-step stdout in a runtime map (not persisted
// to state.json) and exposes it as {{REVIEW_OUTPUT}} in subsequent step templates.
//
// The stdout-driven variant of this test was removed for issue #48: with the
// review verdict now coming from .themis/review-results.json, advancing past a
// non-blocking review requires the JSON fixture. The replacement lives in
// runner_issue48_test.go (TestRunner_ReviewOutputAccumulatedAndAvailableAsTemplateArg_WithJSONFixture).

// AC5: When the pipeline resumes at a step after Review (review never ran this session),
// {{REVIEW_OUTPUT}} must be substituted as empty string rather than a literal placeholder.
func TestRunner_ReviewOutputIsEmptyWhenResumedPastReview(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	// Sentinel pattern: REVIEW::{{REVIEW_OUTPUT}}::END → REVIEW::::END after empty substitution
	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs for issue {{ISSUE_NUMBER}}\nREVIEW::{{REVIEW_OUTPUT}}::END",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/26-ac5"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: REVIEW_OUTPUT must be available as empty string even without review step: %v", err)
	}

	if len(inv.opts) == 0 {
		t.Fatal("expected Docs step to invoke agent; got 0 calls")
	}
	docsPrompt := inv.opts[0].Prompt
	if strings.Contains(docsPrompt, "{{REVIEW_OUTPUT}}") {
		t.Error("{{REVIEW_OUTPUT}} must be substituted (as empty string), not left as a literal placeholder")
	}
	// Empty substitution collapses the sentinel to "REVIEW::::END"
	if !strings.Contains(docsPrompt, "REVIEW::::END") {
		t.Errorf("REVIEW_OUTPUT must substitute to empty string when no review ran\ngot prompt: %s", docsPrompt)
	}
}

// AC3: {{PIPELINE_SHAPE}} is a one-line summary computed from commit message prefixes
// for commits on the issue branch that are not on the base branch.
func TestRunner_PipelineShapeFromCommitPrefixes(t *testing.T) {
	commits := []string{
		"test(runner): add failing tests for issue 26",
		"feat(runner): implement review output capture",
	}
	workDir := initBranchWithCommits(t, commits)
	saveStateAt(t, workDir, pipeline.StepDocs)

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}\nShape:{{PIPELINE_SHAPE}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/26-ac3"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC4: {{COMMIT_LOG}} contains git log --oneline output for commits on the issue branch
// relative to the base branch.
func TestRunner_CommitLogContainsBranchCommits(t *testing.T) {
	commits := []string{
		"test(runner): add failing tests for issue 26",
		"feat(runner): implement feature for issue 26",
	}
	workDir := initBranchWithCommits(t, commits)
	saveStateAt(t, workDir, pipeline.StepDocs)

	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}\nLog:{{COMMIT_LOG}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/26-ac4"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC6: All three new args (REVIEW_OUTPUT, PIPELINE_SHAPE, COMMIT_LOG) must be present
// in masterArgs so filterArgs can pass them to templates that reference them.
// A template referencing all three must render without substitution error.
func TestRunner_AllNewTemplateArgsAvailableInMasterArgs(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	// Template references all three new args — verifies each is in masterArgs
	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "{{ISSUE_NUMBER}} {{REVIEW_OUTPUT}} {{PIPELINE_SHAPE}} {{COMMIT_LOG}}",
	})

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/26-ac6"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
	}

	// Must succeed: all new args must be in masterArgs so filterArgs can supply them
	if _, err := runner.Run(context.Background(), cfg); err != nil {
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
