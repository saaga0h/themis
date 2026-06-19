package runner

// Ship-step behaviour: branch guards (no commits, base-branch collision,
// detached HEAD branch-name resolution), PR creation, PR base selection, and
// PR body composition via the ship agent with fallback to buildPRBody.
//
// Shared stubs and helpers (stubInvoker, recordingInvoker, stubIssueWriter,
// baseConfig, sampleIssue, templateDir, makeTemplateDir, saveStateAt, gitInDir,
// initLocalRepo, initRepoWithRemote, initBranchWithCommits) are defined in
// runner_test.go.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

// pipelineAgentCallCount is the number of agent steps that run before Ship in the
// default happy-path pipeline (TestRed, Implement, Refactor, Review, Docs).
const pipelineAgentCallCount = 5

// failAfterInvoker succeeds for the first successLimit calls then returns err.
// Used to simulate ship agent failure after all pipeline steps have succeeded.
type failAfterInvoker struct {
	successLimit int
	calls        int
	err          error
}

func (f *failAfterInvoker) Invoke(_ context.Context, _ agent.InvokeOptions) (*agent.InvokeResult, error) {
	f.calls++
	if f.calls > f.successLimit {
		return nil, f.err
	}
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

// nonBlockingReviewInvoker wraps an inner invoker and writes an empty
// .themis/review-results.json before each call. Since the review verdict comes
// from that file (a missing file is treated as blocking), a full happy-path
// pipeline must produce it to pass the review step without entering a fix
// cycle — mirroring the real review agent, which writes the file.
type nonBlockingReviewInvoker struct {
	inner agent.Invoker
}

func (n *nonBlockingReviewInvoker) Invoke(ctx context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	if opts.WorkDir != "" {
		themisDir := filepath.Join(opts.WorkDir, ".themis")
		if err := os.MkdirAll(themisDir, 0o755); err == nil {
			_ = os.WriteFile(filepath.Join(themisDir, "review-results.json"), []byte(`{"findings":[]}`), 0o644)
		}
	}
	return n.inner.Invoke(ctx, opts)
}

// ---------------------------------------------------------------------------
// PR creation, base selection, and body content
// ---------------------------------------------------------------------------

// Pipeline runner creates PR targeting main on successful completion
func TestRunner_CreatesPROnSuccess(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/10"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL != "https://example.com/pr/10" {
		t.Errorf("PRURL: got %q, want %q", result.PRURL, "https://example.com/pr/10")
	}
}

// PR description includes Closes #N, AC verification reference, and review notes

func TestRunner_PRBodyIncludesClosesHash(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/11"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(w.prBodySeen, "Closes #42") {
		t.Errorf("PR body must contain 'Closes #42':\n%s", w.prBodySeen)
	}
}

func TestRunner_PRBodyIncludesACReference(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/12"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	body := strings.ToLower(w.prBodySeen)
	if !strings.Contains(body, "acceptance criteria") && !strings.Contains(body, "first ac") && !strings.Contains(body, "second ac") {
		t.Errorf("PR body must reference acceptance criteria:\n%s", w.prBodySeen)
	}
}

// The runner's Ship step uses issue.Ref as the PR base branch instead of hardcoded "main"

func TestRunner_PRBaseMatchesIssueRef(t *testing.T) {
	issue := sampleIssue()
	issue.Ref = "feature-branch"

	w := &stubIssueWriter{prURL: "https://example.com/pr/20"}
	cfg := baseConfig(t, w, &stubFetcher{issue: issue}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if w.prBaseSeen != "feature-branch" {
		t.Errorf("PR base: got %q, want %q", w.prBaseSeen, "feature-branch")
	}
}

// When issue.Ref is empty, the runner falls back to "main"

func TestRunner_PRBaseFallsBackToMainWhenRefEmpty(t *testing.T) {
	issue := sampleIssue()
	issue.Ref = ""

	w := &stubIssueWriter{prURL: "https://example.com/pr/21"}
	cfg := baseConfig(t, w, &stubFetcher{issue: issue}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if w.prBaseSeen != "main" {
		t.Errorf("PR base with empty ref: got %q, want %q", w.prBaseSeen, "main")
	}
}

// ---------------------------------------------------------------------------
// Branch-name resolution
// ---------------------------------------------------------------------------

// the runner resolves the ship branch via git.CurrentBranch (accepting context),
// so a detached HEAD yields "HEAD" rather than the old .git/HEAD reader's "main".
func TestRunner_ShipStep_BranchNameUsesGitCurrentBranch(t *testing.T) {
	// Use a remote-backed repo and add a commit ahead of origin/main so the ship
	// step's "no commits — nothing to ship" guard is satisfied; this test is only
	// about branch-name resolution in detached-HEAD state.
	dir := initRepoWithRemote(t)

	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package p"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "feature.go")
	gitInDir(t, dir, "commit", "-m", "feat: add feature ahead of base")

	// Detach HEAD: .git/HEAD becomes a bare SHA, not a branch ref.
	gitInDir(t, dir, "checkout", "--detach")

	state := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepShip,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(dir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      dir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		// Git.CurrentBranch must return "HEAD" to mirror git behaviour for detached HEAD
		// (the old .git/HEAD file reader returns "main" for this repo state, which is wrong).
		Git: &fakeGitOps{
			currentBranchFn: func(ctx context.Context, dir string) (string, error) {
				return "HEAD", nil
			},
			commitsAhead: 1, // satisfy the ship guard: at least 1 commit on branch
		},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// git.CurrentBranch returns "HEAD" for detached state; the old reader returns "main".
	if w.prHeadSeen != "HEAD" {
		t.Errorf("PR Head: got %q, want %q — runner must use git.CurrentBranch, not read .git/HEAD directly", w.prHeadSeen, "HEAD")
	}
}

// ---------------------------------------------------------------------------
// Ship agent invocation and PR body composition
// ---------------------------------------------------------------------------

// Ship step invokes Claude Code with ship.md template instead of calling buildPRBody() directly.
func TestRunner_ShipStepInvokesAgent(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Ship step must invoke the agent — one more call than the pipeline steps alone.
	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("ship step must invoke agent: want ≥%d calls (%d pipeline + 1 ship), got %d",
			want, pipelineAgentCallCount, len(inv.opts))
	}

	// The ship-step prompt (last invocation) must include the issue number.
	shipPrompt := inv.opts[len(inv.opts)-1].Prompt
	if !strings.Contains(shipPrompt, "42") {
		t.Errorf("ship step prompt must contain issue number 42; first 200 chars: %q",
			shipPrompt[:min(200, len(shipPrompt))])
	}
}

// ship.md template includes the pr-composition skill content or references it.
func TestShipMdTemplateReferencesPRCompositionSkill(t *testing.T) {
	tDir := templateDir(t)
	content, err := os.ReadFile(filepath.Join(tDir, "ship.md"))
	if err != nil {
		t.Fatalf("reading templates/ship.md: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "pr-composition") && !strings.Contains(body, "PR Composition") {
		t.Errorf("ship.md must include or reference the pr-composition skill;\ngot content:\n%s", body)
	}
}

// Agent receives review output, pipeline shape, commit log, and AC status as context.
func TestRunner_ShipPromptContainsContextualData(t *testing.T) {
	fakeLog := "abc1234 test(runner): add failing tests\n" +
		"def5678 feat(runner): implement ship agent invocation"

	workDir := t.TempDir()

	const reviewStdout = "Review complete: all ACs verified. No blocking findings."

	// Custom ship.md uses all required new context placeholders.
	tDir := makeTemplateDir(t, map[string]string{
		"test-red.md":     "Test red {{ISSUE_NUMBER}}",
		"implement.md":    "Implement {{ISSUE_NUMBER}}",
		"refactor.md":     "Refactor {{ISSUE_NUMBER}}",
		"review.md":       "Review {{ISSUE_NUMBER}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md": "Ship {{ISSUE_NUMBER}}\n" +
			"REVIEW:{{REVIEW_OUTPUT}}\n" +
			"SHAPE:{{PIPELINE_SHAPE}}\n" +
			"LOG:{{COMMIT_LOG}}\n" +
			"STATUS:{{AC_STATUS}}",
	})

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},                           // TestRed
			{ExitCode: 0, Completed: true},                           // Implement
			{ExitCode: 0, Completed: true},                           // Refactor
			{ExitCode: 0, Completed: true, Stdout: reviewStdout},     // Review (non-blocking)
			{ExitCode: 0, Completed: true},                           // Docs
			{ExitCode: 0, Completed: true, Stdout: "All ACs passed"}, // Ship
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &nonBlockingReviewInvoker{inner: inv},
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{
			commitLog:    fakeLog,
			commitsAhead: 2,
			currentBranchFn: func(_ context.Context, _ string) (string, error) {
				return "feature/work", nil
			},
		},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := pipelineAgentCallCount + 1
	if len(inv.opts) < want {
		t.Fatalf("ship step must invoke agent with context; want ≥%d calls, got %d", want, len(inv.opts))
	}

	shipPrompt := inv.opts[pipelineAgentCallCount].Prompt

	// Must contain review output.
	if !strings.Contains(shipPrompt, reviewStdout) {
		t.Errorf("ship prompt missing review output\nwant substring: %q\ngot prompt:\n%s",
			reviewStdout, shipPrompt)
	}

	// Must contain pipeline shape (commit type prefixes from branch commits).
	if !strings.Contains(shipPrompt, "test") {
		t.Errorf("ship prompt missing 'test' in pipeline shape\ngot prompt:\n%s", shipPrompt)
	}
	if !strings.Contains(shipPrompt, "feat") {
		t.Errorf("ship prompt missing 'feat' in pipeline shape\ngot prompt:\n%s", shipPrompt)
	}

	// Must contain commit log (a commit message from the branch).
	if !strings.Contains(shipPrompt, "add failing tests") {
		t.Errorf("ship prompt missing commit log entry\ngot prompt:\n%s", shipPrompt)
	}

	// {{AC_STATUS}} must be substituted, not left as a literal placeholder.
	if strings.Contains(shipPrompt, "{{AC_STATUS}}") {
		t.Error("{{AC_STATUS}} must be substituted in ship prompt, not left as literal placeholder")
	}
}

// PR body is the agent-composed output; PR is created via the API with that body.
func TestRunner_ShipUsesAgentOutputAsPRBody(t *testing.T) {
	const agentPRBody = "All ACs passed\n\n" +
		"## Implementation narrative\n\nWired ship step to invoke Claude Code.\n\n" +
		"## Pipeline shape\n\ntest → feat → docs — clean run.\n\n" +
		"## Review findings\n\nClean review, no findings."

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},                      // TestRed
			{ExitCode: 0, Completed: true},                      // Implement
			{ExitCode: 0, Completed: true},                      // Refactor
			{ExitCode: 0, Completed: true, Stdout: ""},          // Review (no blocking findings)
			{ExitCode: 0, Completed: true},                      // Docs
			{ExitCode: 0, Completed: true, Stdout: agentPRBody}, // Ship
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &nonBlockingReviewInvoker{inner: inv})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// PR must be created (CreatePR called).
	if w.prBodySeen == "" {
		t.Fatal("CreatePR must be called with the agent-composed body")
	}

	// PR body must be the agent's stdout, not buildPRBody() output.
	if w.prBodySeen != agentPRBody {
		t.Errorf("PR body must be agent stdout;\nwant: %q\ngot:  %q", agentPRBody, w.prBodySeen)
	}

	// Body must include the pr-composition structure sections.
	if !strings.Contains(w.prBodySeen, "Implementation narrative") {
		t.Errorf("PR body missing 'Implementation narrative' section;\ngot:\n%s", w.prBodySeen)
	}
	if !strings.Contains(w.prBodySeen, "Pipeline shape") {
		t.Errorf("PR body missing 'Pipeline shape' section;\ngot:\n%s", w.prBodySeen)
	}
	if !strings.Contains(w.prBodySeen, "Review findings") {
		t.Errorf("PR body missing 'Review findings' section;\ngot:\n%s", w.prBodySeen)
	}
}

// If agent invocation fails, Ship step falls back to buildPRBody() and logs a warning.
func TestRunner_ShipFallsBackToBuildPRBodyWhenAgentFails(t *testing.T) {
	inv := &failAfterInvoker{
		successLimit: pipelineAgentCallCount, // pipeline steps succeed, ship step fails
		err:          fmt.Errorf("claude code: agent invocation failed"),
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &nonBlockingReviewInvoker{inner: inv})

	// Run must succeed even when the ship agent call fails (fallback path).
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() must succeed when ship agent fails (should fall back to buildPRBody): %v", err)
	}

	// Ship step must have attempted the agent invocation before falling back.
	if inv.calls < pipelineAgentCallCount+1 {
		t.Fatalf("ship step must attempt agent invocation; want ≥%d calls, got %d",
			pipelineAgentCallCount+1, inv.calls)
	}

	// PR must still be created via fallback.
	if w.prBodySeen == "" {
		t.Error("PR must be created via fallback buildPRBody() when ship agent fails")
	}

	// Fallback body must follow buildPRBody() format: starts with "Closes #N".
	if !strings.Contains(w.prBodySeen, "Closes #42") {
		t.Errorf("fallback PR body must contain 'Closes #42'; got:\n%s", w.prBodySeen)
	}
}

// ---------------------------------------------------------------------------
// Ship guards: zero commits and base-branch collision
// ---------------------------------------------------------------------------

// Ship step returns an error naming the branch and "nothing to ship"
// when no commits exist on the issue branch relative to the base.
func TestRunner_ShipGuard_RejectsWhenNoBranchCommits(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepShip)

	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{
			currentBranchFn: func(_ context.Context, _ string) (string, error) {
				return "feature/no-commits", nil
			},
			commitsAhead: 0,
		},
	}

	result, err := Run(context.Background(), cfg)

	if err == nil {
		t.Fatal("Run must return an error when no commits exist on the issue branch")
	}
	if !strings.Contains(err.Error(), "no commits on branch") {
		t.Errorf("error must contain 'no commits on branch'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "feature/no-commits") {
		t.Errorf("error must contain the branch name 'feature/no-commits'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "nothing to ship") {
		t.Errorf("error must contain 'nothing to ship'; got: %v", err)
	}
	if result != nil && result.PRURL != "" {
		t.Error("no PR must be created when ship guard rejects due to zero commits")
	}
	if w.prBodySeen != "" {
		t.Error("CreatePR must not be called when ship guard rejects due to zero commits")
	}
}

// Ship step returns an error when the current branch equals the base branch,
// indicating no issue branch was created.
func TestRunner_ShipGuard_RejectsWhenCurrentBranchIsBaseBranch(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepShip)

	issue := sampleIssue()
	issue.Ref = "themis-2.0" // base == current branch

	w := &stubIssueWriter{prURL: "https://example.com/pr/example"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: issue},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{
			currentBranchFn: func(_ context.Context, _ string) (string, error) {
				return "themis-2.0", nil
			},
		},
	}

	result, err := Run(context.Background(), cfg)

	if err == nil {
		t.Fatal("Run must return an error when the current branch is the base branch")
	}
	if !strings.Contains(err.Error(), "current branch is the base branch") {
		t.Errorf("error must contain 'current branch is the base branch'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "no issue branch was created") {
		t.Errorf("error must contain 'no issue branch was created'; got: %v", err)
	}
	if result != nil && result.PRURL != "" {
		t.Error("no PR must be created when ship guard rejects due to base branch collision")
	}
	if w.prBodySeen != "" {
		t.Error("CreatePR must not be called when ship guard rejects due to base branch collision")
	}
}

// PR creation proceeds normally when commits exist on the branch and the
// branch differs from the base.
func TestRunner_ShipGuard_ProceedsWhenCommitsExistOnBranch(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepShip)

	const wantPRURL = "https://example.com/pr/example"
	w := &stubIssueWriter{prURL: wantPRURL}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		Git: &fakeGitOps{
			currentBranchFn: func(_ context.Context, _ string) (string, error) {
				return "feature/work", nil
			},
			commitsAhead: 2,
		},
	}

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run must succeed when commits exist on the issue branch: %v", err)
	}
	if result == nil || result.PRURL != wantPRURL {
		t.Errorf("PR URL: got %v, want %q", result, wantPRURL)
	}
	if w.prBodySeen == "" {
		t.Error("CreatePR must be called when commits exist on branch")
	}
}
