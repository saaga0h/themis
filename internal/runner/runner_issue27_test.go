package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/runner"
)

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

// pipelineAgentCallCount is the number of agent steps that run before Ship in the
// default happy-path pipeline (TestRed, Implement, Refactor, Review, Docs).
const pipelineAgentCallCount = 5

// AC1: Ship step invokes Claude Code with ship.md template instead of calling buildPRBody() directly.
func TestRunner_ShipStepInvokesAgent(t *testing.T) {
	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/27-ac1"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC2: ship.md template includes the pr-composition skill content or references it.
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

// AC3: Agent receives review output, pipeline shape, commit log, and AC status as context.
func TestRunner_ShipPromptContainsContextualData(t *testing.T) {
	commits := []string{
		"test(runner): add failing tests for issue 27",
		"feat(runner): implement ship agent invocation",
	}
	workDir := initBranchWithCommits(t, commits)

	const reviewStdout = "Review complete: all ACs verified. No blocking findings."

	// Custom ship.md uses all required new context placeholders.
	// AC_STATUS is a new placeholder the implementation must add to masterArgs.
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
	w := &stubIssueWriter{prURL: "https://example.com/pr/27-ac3"}
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
	if !strings.Contains(shipPrompt, "add failing tests for issue 27") {
		t.Errorf("ship prompt missing commit log entry\ngot prompt:\n%s", shipPrompt)
	}

	// {{AC_STATUS}} must be substituted, not left as a literal placeholder.
	if strings.Contains(shipPrompt, "{{AC_STATUS}}") {
		t.Error("{{AC_STATUS}} must be substituted in ship prompt, not left as literal placeholder")
	}
}

// AC4 + AC6: PR body is the agent-composed output; PR is created via the API with that body.
func TestRunner_ShipUsesAgentOutputAsPRBody(t *testing.T) {
	const agentPRBody = "All ACs passed\n\n" +
		"## Implementation narrative\n\nWired ship step to invoke Claude Code.\n\n" +
		"## Pipeline shape\n\ntest → feat → docs — clean run.\n\n" +
		"## Review findings\n\nClean review, no findings."

	inv := &recordingInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true},              // TestRed
			{ExitCode: 0, Completed: true},              // Implement
			{ExitCode: 0, Completed: true},              // Refactor
			{ExitCode: 0, Completed: true, Stdout: ""},  // Review (no blocking findings)
			{ExitCode: 0, Completed: true},              // Docs
			{ExitCode: 0, Completed: true, Stdout: agentPRBody}, // Ship
		},
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/27-ac4"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC5: If agent invocation fails, Ship step falls back to buildPRBody() and logs a warning.
func TestRunner_ShipFallsBackToBuildPRBodyWhenAgentFails(t *testing.T) {
	inv := &failAfterInvoker{
		successLimit: pipelineAgentCallCount, // pipeline steps succeed, ship step fails
		err:          fmt.Errorf("claude code: agent invocation failed"),
	}
	w := &stubIssueWriter{prURL: "https://example.com/pr/27-ac5"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	// Run must succeed even when the ship agent call fails (fallback path).
	if _, err := runner.Run(context.Background(), cfg); err != nil {
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
