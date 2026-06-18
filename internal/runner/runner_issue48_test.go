package runner

// AC1-AC10 tests for issue #48: replace stdout-based review detection with
// .themis/review-results.json parsing. Tests live in package runner (white-box)
// so they can reference unexported constants and functions.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// ---------------------------------------------------------------------------
// Helpers (white-box package — no access to runner_test stubs)
// ---------------------------------------------------------------------------

// issue48StubFetcher returns a fixed IssueData.
type issue48StubFetcher struct {
	issue *tracker.IssueData
}

func (s *issue48StubFetcher) Fetch(_ context.Context, _ int) (*tracker.IssueData, error) {
	return s.issue, nil
}

// issue48StubIssueWriter records issue tracker operations.
type issue48StubIssueWriter struct {
	labelsAdded []string
	comments    []string
	prURL       string
}

func (s *issue48StubIssueWriter) AddLabel(_ context.Context, _ int, label string) error {
	s.labelsAdded = append(s.labelsAdded, label)
	return nil
}

func (s *issue48StubIssueWriter) RemoveLabel(_ context.Context, _ int, label string) error {
	return nil
}

func (s *issue48StubIssueWriter) Comment(_ context.Context, _ int, body string) error {
	s.comments = append(s.comments, body)
	return nil
}

func (s *issue48StubIssueWriter) CreatePR(_ context.Context, opts PROptions) (string, error) {
	if s.prURL != "" {
		return s.prURL, nil
	}
	return "https://git.example.com/pr/99", nil
}

// issue48RecordingInvoker captures Invoke options and returns results in order.
type issue48RecordingInvoker struct {
	opts    []agent.InvokeOptions
	results []*agent.InvokeResult
	idx     int
}

func (r *issue48RecordingInvoker) Invoke(_ context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	r.opts = append(r.opts, opts)
	if r.idx < len(r.results) {
		res := r.results[r.idx]
		r.idx++
		return res, nil
	}
	r.idx++
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

// issue48StubInvoker returns results from a fixed list, then a default.
type issue48StubInvoker struct {
	results []*agent.InvokeResult
	calls   int
}

func (s *issue48StubInvoker) Invoke(_ context.Context, _ agent.InvokeOptions) (*agent.InvokeResult, error) {
	if s.calls < len(s.results) {
		r := s.results[s.calls]
		s.calls++
		return r, nil
	}
	s.calls++
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

func issue48SampleIssue() *tracker.IssueData {
	return &tracker.IssueData{
		Number: 42,
		Title:  "Test Issue",
		Body:   "## AC\n- [ ] First AC\n- [ ] Second AC",
		Labels: []string{"ready-for-agent"},
		URL:    "https://git.example.com/issues/42",
	}
}

func issue48NoopCheckpoint(_ context.Context, _ pipeline.Step, _ string) error {
	return nil
}

// issue48TemplateDir locates the real templates directory by walking up from cwd.
func issue48TemplateDir(t *testing.T) string {
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

// issue48MakeTemplateDir creates a temp dir populated with named template files.
func issue48MakeTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}
	return dir
}

// issue48SaveStateAt persists a PipelineState at step for issue #42.
func issue48SaveStateAt(t *testing.T, workDir string, step pipeline.Step) {
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

// ---------------------------------------------------------------------------
// AC1: blockingThreshold constant defined with value "medium"
// ---------------------------------------------------------------------------

// TestBlockingThreshold_ValueIsMedium asserts the package-level constant
// blockingThreshold has the exact string value "medium".
func TestBlockingThreshold_ValueIsMedium(t *testing.T) {
	const want = "medium"
	if blockingThreshold != want {
		t.Errorf("blockingThreshold = %q, want %q", blockingThreshold, want)
	}
}

// ---------------------------------------------------------------------------
// AC2: Runner reads .themis/review-results.json, not stdout
// ---------------------------------------------------------------------------

// TestDeriveStepResult_ReviewStep_ReadsJSONNotStdout verifies that when
// .themis/review-results.json contains a critical finding and stdout is empty,
// the runner detects blocking findings from the JSON file.
func TestDeriveStepResult_ReviewStep_ReadsJSONNotStdout(t *testing.T) {
	workDir := t.TempDir()
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "critical", Description: "SQL injection vulnerability", File: "db.go", Line: 42},
	})

	result := &agent.InvokeResult{
		ExitCode:  0,
		Completed: true,
		Stdout:    "", // stdout is empty — blocking must come from JSON
	}

	cfg := Config{
		WorkDir: workDir,
	}

	sr := deriveStepResult(pipeline.StepReview, result, cfg)
	if !sr.BlockingFindings {
		t.Error("BlockingFindings must be true when review-results.json contains a critical finding, even with empty stdout")
	}
}

// TestDeriveStepResult_ReviewStep_StdoutAloneDoesNotTriggerBlocking verifies
// that stdout containing the old "BLOCKING_FINDINGS: YES" marker does not
// trigger blocking when .themis/review-results.json does not exist.
// Per AC5, missing file is treated as blocking — but the test documents that
// stdout alone is no longer the signal.
func TestDeriveStepResult_ReviewStep_StdoutAloneDoesNotTriggerBlocking(t *testing.T) {
	workDir := t.TempDir()
	// No .themis/review-results.json written — file absent.
	// Per AC5 the missing file path is blocking but must NOT be triggered by stdout.

	result := &agent.InvokeResult{
		ExitCode:  0,
		Completed: true,
		Stdout:    "BLOCKING_FINDINGS: YES\nSecurity issue found.",
	}

	cfg := Config{
		WorkDir: workDir,
	}

	// The new implementation reads JSON; missing file → blocking per AC5.
	// What we assert here is that if the JSON is absent, the reason must be
	// the missing-file path (AC5) — not the stdout content path.
	// We cannot distinguish these outcomes from the returned StepResult alone,
	// so we also assert that writing an EMPTY findings JSON (no critical entries)
	// alongside the same stdout makes blocking FALSE — proving stdout is not read.
	writeReviewResults(t, workDir, []ReviewFinding{})

	srWithEmptyJSON := deriveStepResult(pipeline.StepReview, result, cfg)
	if srWithEmptyJSON.BlockingFindings {
		t.Error("BlockingFindings must be false when review-results.json has no findings, even when stdout contains old BLOCKING_FINDINGS marker")
	}
}

// ---------------------------------------------------------------------------
// AC3 + AC4: Severity table — critical/high/medium block; low does not
// ---------------------------------------------------------------------------

// TestDetermineBlockingStatus_SeverityTable verifies severity thresholds.
// critical, high, medium → blocking=true; low → blocking=false.
func TestDetermineBlockingStatus_SeverityTable(t *testing.T) {
	cases := []struct {
		severity string
		want     bool
	}{
		{"critical", true},
		{"high", true},
		{"medium", true},
		{"low", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.severity, func(t *testing.T) {
			findings := []ReviewFinding{
				{Severity: tc.severity, Description: "test finding"},
			}
			got := determineBlockingStatus(findings)
			if got != tc.want {
				t.Errorf("determineBlockingStatus([{severity:%q}]) = %v, want %v", tc.severity, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC5: Missing .themis/review-results.json treats review as blocking + warning
// ---------------------------------------------------------------------------

// TestRunner_ReviewStep_MissingJSONIsBlocking verifies that when the review
// step completes but .themis/review-results.json does not exist, the runner
// treats the result as blocking and logs a warning containing "review-results.json"
// or "warning".
func TestRunner_ReviewStep_MissingJSONIsBlocking(t *testing.T) {
	workDir := t.TempDir()
	issue48SaveStateAt(t, workDir, pipeline.StepReview)
	// No review-results.json written.

	var logBuf bytes.Buffer

	tDir := issue48MakeTemplateDir(t, map[string]string{
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
	})

	// Review returns no stdout, no JSON file — runner must treat as blocking.
	inv := &issue48StubInvoker{
		results: []*agent.InvokeResult{
			// review step: no blocking marker in stdout; no JSON file
			{ExitCode: 0, Completed: true, Stdout: ""},
			// fix step (if reached): complete successfully
			{ExitCode: 0, Completed: true},
		},
	}
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac5"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
		Logger:       &logBuf,
	}

	// Run — the review cycle limit must eventually be hit (or pipeline advances to fix).
	// We only need to observe that at the review step, blocking=true was detected.
	// The runner will either enter a fix cycle or hit the cycle limit.
	_, runErr := Run(context.Background(), cfg)

	logOutput := logBuf.String()
	lc := strings.ToLower(logOutput)

	// Must log a warning about the missing file.
	if !strings.Contains(lc, "review-results.json") && !strings.Contains(lc, "warning") {
		t.Errorf("log must contain a warning about missing review-results.json when file is absent; got:\n%s", logOutput)
	}

	// The missing-JSON fail-safe must actually drive the pipeline: either it
	// enters a fix cycle (review → fix, so the invoker is called more than once),
	// or it blocks with a review-cycle-limit error. A warning alone is not enough.
	enteredFixCycle := inv.calls > 1
	hitCycleLimit := runErr != nil && strings.Contains(runErr.Error(), "review cycle")
	if !enteredFixCycle && !hitCycleLimit {
		t.Errorf("missing JSON must trigger a fix cycle (invoker called >1, got %d) or a review-cycle error (got %v); log:\n%s",
			inv.calls, runErr, logOutput)
	}
}

// ---------------------------------------------------------------------------
// AC6: {{BLOCKING_FINDINGS}} receives formatted list ordered by severity
// ---------------------------------------------------------------------------

// TestExtractBlockingFindings_FormatsOrderedBySeverity verifies that given a
// ReviewResults with mixed-severity findings, the formatted output lists
// critical before high before medium, omits low, and is human-readable (not JSON).
func TestExtractBlockingFindings_FormatsOrderedBySeverity(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "low", Description: "minor style nit"},
		{Severity: "medium", Description: "medium risk issue"},
		{Severity: "critical", Description: "critical security hole"},
		{Severity: "high", Description: "high risk bug"},
	}

	formatted := formatBlockingFindings(findings)

	// Must not contain low-severity finding.
	if strings.Contains(formatted, "minor style nit") {
		t.Error("formatted blocking findings must omit low-severity findings")
	}

	// Must contain all blocking severities.
	for _, desc := range []string{"critical security hole", "high risk bug", "medium risk issue"} {
		if !strings.Contains(formatted, desc) {
			t.Errorf("formatted findings missing description %q:\n%s", desc, formatted)
		}
	}

	// Must be human-readable, not raw JSON.
	if strings.HasPrefix(strings.TrimSpace(formatted), "{") || strings.HasPrefix(strings.TrimSpace(formatted), "[") {
		t.Errorf("formatted findings must not be raw JSON:\n%s", formatted)
	}

	// critical must appear before high, high before medium.
	critIdx := strings.Index(formatted, "critical security hole")
	highIdx := strings.Index(formatted, "high risk bug")
	medIdx := strings.Index(formatted, "medium risk issue")

	if critIdx < 0 || highIdx < 0 || medIdx < 0 {
		t.Fatalf("expected all three descriptions in output:\n%s", formatted)
	}
	if critIdx >= highIdx {
		t.Errorf("critical must appear before high in formatted output:\n%s", formatted)
	}
	if highIdx >= medIdx {
		t.Errorf("high must appear before medium in formatted output:\n%s", formatted)
	}
}

// TestRunner_FixTemplate_BlockingFindingsPlaceholderIsFormattedList is an
// integration test: with a review-results.json containing two blocking findings
// and a fix-findings.md template using {{BLOCKING_FINDINGS}}, the prompt
// delivered to the fix invoker must contain the human-readable finding
// descriptions, not raw JSON or stdout lines.
func TestRunner_FixTemplate_BlockingFindingsPlaceholderIsFormattedList(t *testing.T) {
	workDir := t.TempDir()
	issue48SaveStateAt(t, workDir, pipeline.StepReview)

	// Write two blocking findings to review-results.json.
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "critical", Description: "SQL injection in login handler", File: "auth.go", Line: 55},
		{Severity: "high", Description: "Missing input validation", File: "api.go", Line: 120},
	})

	tDir := issue48MakeTemplateDir(t, map[string]string{
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix issue {{ISSUE_NUMBER}}\nFindings:\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
	})

	inv := &issue48RecordingInvoker{
		results: []*agent.InvokeResult{
			// review step: completed, no stdout (blocking from JSON)
			{ExitCode: 0, Completed: true, Stdout: ""},
			// fix step: completed successfully
			{ExitCode: 0, Completed: true},
			// docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac6"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	// Find the fix step invocation (after review).
	if len(inv.opts) < 2 {
		t.Fatalf("expected at least 2 agent calls (review + fix); got %d", len(inv.opts))
	}
	fixPrompt := inv.opts[1].Prompt

	// Must contain the human-readable descriptions.
	if !strings.Contains(fixPrompt, "SQL injection in login handler") {
		t.Errorf("fix prompt must contain critical finding description; got:\n%s", fixPrompt)
	}
	if !strings.Contains(fixPrompt, "Missing input validation") {
		t.Errorf("fix prompt must contain high finding description; got:\n%s", fixPrompt)
	}

	// Must NOT be raw JSON (no leading { or array brackets for the findings block).
	findingsStart := strings.Index(fixPrompt, "Findings:\n")
	if findingsStart >= 0 {
		afterFindings := strings.TrimSpace(fixPrompt[findingsStart+len("Findings:\n"):])
		if strings.HasPrefix(afterFindings, "{") || strings.HasPrefix(afterFindings, "[") {
			t.Errorf("BLOCKING_FINDINGS in fix prompt must be human-readable, not raw JSON:\n%s", afterFindings)
		}
	}
}

// ---------------------------------------------------------------------------
// AC7: Fresh start deletes .themis/review-results.json if present
// ---------------------------------------------------------------------------

// TestRunner_FreshStart_DeletesReviewResultsJSON verifies that when the runner
// starts fresh (no state file), a stale .themis/review-results.json is deleted
// before the pipeline proceeds.
func TestRunner_FreshStart_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// No state file — fresh start.

	// Write a stale review-results.json.
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "critical", Description: "stale finding from previous run"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	// Verify the file exists before the run.
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatal("precondition: review-results.json must exist before run")
	}

	// Use a blocking invoker that stops after the first step (TestRed fails)
	// so the test doesn't need to drive the full pipeline.
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac7-fresh"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      &issue48StubInvoker{},
		IssueWriter:  w,
		TemplateDir:  issue48TemplateDir(t),
		CheckpointFn: issue48NoopCheckpoint,
	}

	// Run to completion (or error — we don't care about the pipeline result).
	_, _ = Run(context.Background(), cfg)

	// After a fresh start, .themis/review-results.json must no longer exist.
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("fresh start must delete .themis/review-results.json; file still exists after Run")
	}
}

// TestRunner_IssueMismatch_DeletesReviewResultsJSON verifies that when the
// state file is for a different issue number, the runner starts fresh and
// deletes an existing .themis/review-results.json.
func TestRunner_IssueMismatch_DeletesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()

	// State file is for issue #99, but we run with issue #42.
	mismatchState := &pipeline.PipelineState{
		IssueNumber:     99,
		CurrentStep:     pipeline.StepFix,
		MaxReviewCycles: 2,
		TestFixAttempts: map[string]int{},
	}
	if err := pipeline.SaveState(workDir, mismatchState); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Write review-results.json that belongs to the old run.
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "high", Description: "finding from issue 99"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac7-mismatch"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42, // different from state
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      &issue48StubInvoker{},
		IssueWriter:  w,
		TemplateDir:  issue48TemplateDir(t),
		CheckpointFn: issue48NoopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	// Issue mismatch triggers fresh start, which must delete review-results.json.
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("issue mismatch must trigger fresh start and delete .themis/review-results.json; file still exists")
	}
}

// ---------------------------------------------------------------------------
// AC8: Resume with matching issue number preserves .themis/review-results.json
// ---------------------------------------------------------------------------

// TestRunner_Resume_PreservesReviewResultsJSON verifies that when resuming
// a valid state for the same issue, review-results.json is not deleted.
// The issue48FileCheckInvoker checks the file exists at the moment the fix step runs.
func TestRunner_Resume_PreservesReviewResultsJSON(t *testing.T) {
	workDir := t.TempDir()
	// Resume at StepFix — same issue #42 as cfg.IssueNumber.
	issue48SaveStateAt(t, workDir, pipeline.StepFix)

	// Write review-results.json that should be preserved.
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "high", Description: "finding preserved from review"},
	})
	jsonPath := filepath.Join(workDir, ".themis", "review-results.json")

	fileExistedAtFixInvocation := false

	inner := &issue48RecordingInvoker{
		results: []*agent.InvokeResult{
			// fix step
			{ExitCode: 0, Completed: true},
			// docs step
			{ExitCode: 0, Completed: true},
		},
	}

	tDir := issue48MakeTemplateDir(t, map[string]string{
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}\n{{BLOCKING_FINDINGS}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md":         "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	customInv := &issue48FileCheckInvoker{
		inner:          inner,
		jsonPath:       jsonPath,
		fileExistedPtr: &fileExistedAtFixInvocation,
	}

	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac8"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      customInv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
	}

	_, _ = Run(context.Background(), cfg)

	if !fileExistedAtFixInvocation {
		t.Error("review-results.json must be preserved when resuming with matching issue number; it was missing at fix step invocation")
	}
}

// issue48FileCheckInvoker is an invoker that checks whether a file exists
// before the first invocation and records the result.
type issue48FileCheckInvoker struct {
	inner          *issue48RecordingInvoker
	jsonPath       string
	fileExistedPtr *bool
	called         bool
}

func (f *issue48FileCheckInvoker) Invoke(ctx context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	if !f.called {
		f.called = true
		if _, err := os.Stat(f.jsonPath); err == nil {
			*f.fileExistedPtr = true
		}
	}
	return f.inner.Invoke(ctx, opts)
}

// ---------------------------------------------------------------------------
// AC10: Update existing tests — Instance 1: TestRunner_StopsOnReviewCycleLimit
// ---------------------------------------------------------------------------
// The original test in runner_test.go used Stdout: "BLOCKING_FINDINGS: YES\n..."
// to trigger blocking. The updated version writes a JSON fixture instead.
// This test replaces that pattern and must pass with the new implementation.

func TestRunner_StopsOnReviewCycleLimit_WithJSONFixture(t *testing.T) {
	workDir := t.TempDir()

	// Already at Review step with 2 cycles used (default max is 2).
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

	// Write a critical finding so the review step returns blocking=true.
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "critical", Description: "Security issue found.", File: "main.go", Line: 10},
	})

	// Review agent returns no blocking stdout — blocking comes entirely from JSON.
	inv := &issue48StubInvoker{
		results: []*agent.InvokeResult{
			{ExitCode: 0, Completed: true, Stdout: ""},
		},
	}
	w := &issue48StubIssueWriter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  issue48TemplateDir(t),
		CheckpointFn: issue48NoopCheckpoint,
	}

	result, err := Run(context.Background(), cfg)
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

// ---------------------------------------------------------------------------
// AC10: Update existing tests — Instance 2: LogsReviewFindingsSummaryWithCounts
// ---------------------------------------------------------------------------
// The original test used BLOCKING:/NON-BLOCKING: stdout prefixes. The updated
// version writes a JSON fixture with 1 blocking and 2 non-blocking findings.

func TestRunner_LogsReviewFindingsSummaryWithCounts_WithJSONFixture(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	tDir := issue48MakeTemplateDir(t, map[string]string{
		"test-red.md":     "Test {{ISSUE_NUMBER}}",
		"implement.md":    "Implement {{ISSUE_NUMBER}}",
		"refactor.md":     "Refactor {{ISSUE_NUMBER}}",
		"review.md":       "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"fix-findings.md": "Fix {{ISSUE_NUMBER}}",
		"update-docs.md":  "Docs {{ISSUE_NUMBER}}",
		"ship.md":         "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	// fullRunResults48 provides results for the default happy-path sequence,
	// with the review step returning empty stdout (blocking from JSON).
	results := make([]*agent.InvokeResult, 6)
	for i := range results {
		results[i] = &agent.InvokeResult{ExitCode: 0, Completed: true}
	}
	// Review is index 3; write JSON with 1 blocking + 2 non-blocking findings.
	// The runner must log "1 blocking" and "2 non-blocking".
	results[3] = &agent.InvokeResult{ExitCode: 0, Completed: true, Stdout: ""}

	// Pre-write review results so they exist when the review step runs.
	// We need a hook to write the JSON at the right time. Since writeReviewResults
	// writes before the run, and the runner fresh-starts, it will delete the file
	// (AC7). Instead we start at the Review step via saved state.
	issue48SaveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []ReviewFinding{
		{Severity: "high", Description: "missing test for Ship step"},
		{Severity: "low", Description: "variable name could be more descriptive"},
		{Severity: "low", Description: "consider extracting helper function"},
	})

	inv := &issue48StubInvoker{
		results: []*agent.InvokeResult{
			// Review: blocking (high finding in JSON), cycles remaining → goes to Fix
			{ExitCode: 0, Completed: true, Stdout: ""},
			// Fix step
			{ExitCode: 0, Completed: true},
			// Docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac10-counts"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1 blocking") {
		t.Errorf("expected '1 blocking' in review findings summary; got:\n%s", output)
	}
	if !strings.Contains(output, "2 non-blocking") {
		t.Errorf("expected '2 non-blocking' in review findings summary; got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// AC10: Update existing tests — Instance 3: LogsReviewFindingsSummaryWhenNoFindings
// ---------------------------------------------------------------------------
// The original test used clean stdout. The updated version writes an empty
// findings array to .themis/review-results.json.

func TestRunner_LogsReviewFindingsSummaryWhenNoFindings_WithJSONFixture(t *testing.T) {
	var buf bytes.Buffer
	workDir := t.TempDir()

	tDir := issue48MakeTemplateDir(t, map[string]string{
		"review.md":      "Review {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	// Start at Review step; write empty findings (no blocking).
	issue48SaveStateAt(t, workDir, pipeline.StepReview)
	writeReviewResults(t, workDir, []ReviewFinding{}) // empty — 0 blocking

	inv := &issue48StubInvoker{
		results: []*agent.InvokeResult{
			// Review: non-blocking (empty JSON)
			{ExitCode: 0, Completed: true, Stdout: ""},
			// Docs step
			{ExitCode: 0, Completed: true},
		},
	}
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac10-nofindings"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
		Logger:       &buf,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0 blocking") {
		t.Errorf("expected '0 blocking' in review summary when no findings; got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// AC10: Update existing tests — Instance 4: ReviewOutputAccumulatedAndAvailableAsTemplateArg
// ---------------------------------------------------------------------------
// The original test relied on non-blocking stdout alone to advance past review.
// The updated version adds a .themis/review-results.json with empty findings so
// the JSON-based path sees no blocking findings, while preserving the
// {{REVIEW_OUTPUT}} assertion (stdout is still captured as REVIEW_OUTPUT).

func TestRunner_ReviewOutputAccumulatedAndAvailableAsTemplateArg_WithJSONFixture(t *testing.T) {
	workDir := t.TempDir()
	issue48SaveStateAt(t, workDir, pipeline.StepReview)

	// Write empty findings so review is non-blocking.
	writeReviewResults(t, workDir, []ReviewFinding{})

	const reviewStdout = "Review complete: all checks passed. No blocking issues found."
	tDir := issue48MakeTemplateDir(t, map[string]string{
		"review.md":      "Review issue {{ISSUE_NUMBER}}\n{{ACCEPTANCE_CRITERIA}}",
		"update-docs.md": "Docs for issue {{ISSUE_NUMBER}}\nFullReview:\n{{REVIEW_OUTPUT}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}\n{{AC_STATUS}}",
	})

	inv := &issue48RecordingInvoker{
		results: []*agent.InvokeResult{
			// Review step: non-blocking (JSON has empty findings); stdout captured as REVIEW_OUTPUT.
			{ExitCode: 0, Completed: true, Stdout: reviewStdout},
		},
	}
	w := &issue48StubIssueWriter{prURL: "https://example.com/pr/48-ac10-reviewoutput"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &issue48StubFetcher{issue: issue48SampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: issue48NoopCheckpoint,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Must have called both Review (idx 0) and Docs (idx 1).
	if len(inv.opts) < 2 {
		t.Fatalf("expected ≥2 agent calls (review + docs); got %d", len(inv.opts))
	}
	docsPrompt := inv.opts[1].Prompt
	if !strings.Contains(docsPrompt, reviewStdout) {
		t.Errorf("Docs prompt must contain full review stdout as {{REVIEW_OUTPUT}}\nwant substring: %q\ngot prompt:\n%s",
			reviewStdout, docsPrompt)
	}

	// Accumulation is runtime-only — stdout must NOT be written to state.json.
	stateBytes, err := os.ReadFile(filepath.Join(workDir, ".themis", "state.json"))
	if err != nil {
		t.Fatalf("reading state.json: %v", err)
	}
	if strings.Contains(string(stateBytes), reviewStdout) {
		t.Error("review stdout must NOT be persisted to state.json (runtime map only)")
	}
}
