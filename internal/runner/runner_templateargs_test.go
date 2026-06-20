package runner

// Template-argument construction: the context packets (issue number, changed
// files, pipeline shape, commit log) substituted into each step's prompt.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
)

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
		Git:          &fakeGitOps{changedFiles: "newfeature.go", commitsAhead: 1},
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
		Git:          &fakeGitOps{commitsAhead: 1},
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
		Git:          &fakeGitOps{commitLog: fakeLog, commitsAhead: 1},
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
		Git:          &fakeGitOps{commitLog: fakeLog, commitsAhead: 1},
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
