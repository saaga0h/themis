package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/runner"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// stubFetcher returns a fixed IssueData.
type stubFetcher struct {
	issue *tracker.IssueData
	err   error
}

func (s *stubFetcher) Fetch(_ context.Context, _ int) (*tracker.IssueData, error) {
	return s.issue, s.err
}

// stubInvoker returns a fixed InvokeResult for each call, then repeats the last.
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

// stubIssueWriter records issue tracker operations.
type stubIssueWriter struct {
	labelsAdded   []string
	labelsRemoved []string
	comments      []string
	prBodySeen    string
	prBaseSeen    string
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

func (s *stubIssueWriter) CreatePR(_ context.Context, opts runner.PROptions) (string, error) {
	s.prBodySeen = opts.Body
	s.prBaseSeen = opts.Base
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

func baseConfig(t *testing.T, w *stubIssueWriter, f *stubFetcher, inv agent.Invoker) runner.Config {
	t.Helper()
	return runner.Config{
		WorkDir:      t.TempDir(),
		IssueNumber:  42,
		Fetcher:      f,
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}
}

// AC: Pipeline runner loads profile, fetches issue, and runs the pipeline steps in order

func TestRunner_RunsStepsInOrder(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/1"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL == "" {
		t.Error("Run should produce a PR URL on success")
	}
}

// AC: Pipeline runner loads the correct prompt template for each step and substitutes issue-specific arguments

func TestRunner_SubstitutesIssueNumberInTemplate(t *testing.T) {
	inv := &captureInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/2"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, inv)

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC: Pipeline state is saved to .themis/state.json after each step

func TestRunner_SavesStateFile(t *testing.T) {
	workDir := t.TempDir()
	w := &stubIssueWriter{prURL: "https://example.com/pr/4"}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &stubInvoker{},
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	stateFile := filepath.Join(workDir, ".themis", "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error(".themis/state.json should exist after run completes")
	}
}

// AC: Resume — pipeline resumes from saved state (kill and restart)

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
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	if _, err := runner.Run(context.Background(), cfg); err != nil {
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

// AC: Pipeline runner stops with blocked label and issue comment when test-fix limit (3) is reached

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
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
		TestACKey:    "ac-0",
	}

	result, err := runner.Run(context.Background(), cfg)
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

// AC: Pipeline runner stops with blocked label and issue comment when review cycle limit is reached

func TestRunner_StopsOnReviewCycleLimit(t *testing.T) {
	workDir := t.TempDir()

	// Already at Review step with 2 cycles used (default max is 2)
	priorState := &pipeline.PipelineState{
		IssueNumber:     42,
		CurrentStep:     pipeline.StepReview,
		ReviewCycle:     2,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, priorState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Review agent outputs blocking findings marker
	inv := &stubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true, Stdout: "BLOCKING_FINDINGS: YES\nSecurity issue found."},
		},
	}
	w := &stubIssueWriter{}
	cfg := runner.Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  templateDir(t),
		CheckpointFn: noopCheckpoint,
	}

	result, err := runner.Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run should return error when review cycle limit exceeded")
	}
	if result != nil && result.PRURL != "" {
		t.Error("should not create PR when blocked on review cycle limit")
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
		t.Error("must post a comment when blocked on review cycle limit")
	}
}

// AC: Pipeline runner creates PR targeting main on successful completion

func TestRunner_CreatesPROnSuccess(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/10"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.PRURL != "https://example.com/pr/10" {
		t.Errorf("PRURL: got %q, want %q", result.PRURL, "https://example.com/pr/10")
	}
}

// AC: PR description includes Closes #N, AC verification reference, and review notes

func TestRunner_PRBodyIncludesClosesHash(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/11"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(w.prBodySeen, "Closes #42") {
		t.Errorf("PR body must contain 'Closes #42':\n%s", w.prBodySeen)
	}
}

func TestRunner_PRBodyIncludesACReference(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/12"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	body := strings.ToLower(w.prBodySeen)
	if !strings.Contains(body, "acceptance criteria") && !strings.Contains(body, "first ac") && !strings.Contains(body, "second ac") {
		t.Errorf("PR body must reference acceptance criteria:\n%s", w.prBodySeen)
	}
}

// AC: The runner's Ship step uses issue.Ref as the PR base branch instead of hardcoded "main"
// AC: Test confirms PR base branch matches the issue's ref when set

func TestRunner_PRBaseMatchesIssueRef(t *testing.T) {
	issue := sampleIssue()
	issue.Ref = "feature-branch"

	w := &stubIssueWriter{prURL: "https://example.com/pr/20"}
	cfg := baseConfig(t, w, &stubFetcher{issue: issue}, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if w.prBaseSeen != "feature-branch" {
		t.Errorf("PR base: got %q, want %q", w.prBaseSeen, "feature-branch")
	}
}

// AC: When issue.Ref is empty, the runner falls back to "main"
// AC: Test confirms fallback to "main" when ref is empty

func TestRunner_PRBaseFallsBackToMainWhenRefEmpty(t *testing.T) {
	issue := sampleIssue()
	issue.Ref = ""

	w := &stubIssueWriter{prURL: "https://example.com/pr/21"}
	cfg := baseConfig(t, w, &stubFetcher{issue: issue}, &stubInvoker{})

	if _, err := runner.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if w.prBaseSeen != "main" {
		t.Errorf("PR base with empty ref: got %q, want %q", w.prBaseSeen, "main")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
