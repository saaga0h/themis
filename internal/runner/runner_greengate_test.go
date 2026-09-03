package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

// With a TestRunner reporting a green suite, Implement scores success.
func TestDeriveStepResult_GreenGate_PassesWhenSuiteGreen(t *testing.T) {
	cfg := Config{
		WorkDir:    "/x",
		TestRunner: func(context.Context, string) (bool, string) { return true, "ok" },
	}
	for _, step := range []pipeline.Step{pipeline.StepImplement} {
		sr, _ := deriveStepResult(context.Background(), step, &agent.InvokeResult{Completed: true}, cfg)
		if !sr.Success {
			t.Errorf("%v: expected success when suite is green", step)
		}
	}
}

// With a TestRunner reporting a red suite, Implement scores failure so the
// pipeline retries rather than advancing on a broken build.
func TestDeriveStepResult_GreenGate_FailsWhenSuiteRed(t *testing.T) {
	cfg := Config{
		WorkDir:    "/x",
		TestRunner: func(context.Context, string) (bool, string) { return false, "FAIL" },
	}
	for _, step := range []pipeline.Step{pipeline.StepImplement} {
		sr, _ := deriveStepResult(context.Background(), step, &agent.InvokeResult{Completed: true}, cfg)
		if sr.Success {
			t.Errorf("%v: expected failure when suite is red", step)
		}
	}
}

// Without a TestRunner the gate is bypassed and the legacy commit-only contract
// applies, so Implement advances on completion alone. This keeps callers that do
// not wire a runner (e.g. unit tests) working unchanged.
func TestDeriveStepResult_GreenGate_NilRunnerIsLegacySuccess(t *testing.T) {
	cfg := Config{WorkDir: "/x"} // TestRunner nil
	for _, step := range []pipeline.Step{pipeline.StepImplement} {
		sr, _ := deriveStepResult(context.Background(), step, &agent.InvokeResult{}, cfg)
		if !sr.Success {
			t.Errorf("%v: expected legacy success when TestRunner is nil", step)
		}
	}
}

// logGreenGate distinguishes the three outcomes so an operator can tell a stuck
// implementation (completed but red) from a truncated one (no completion signal).
func TestLogGreenGate_Messages(t *testing.T) {
	cases := []struct {
		name      string
		passed    bool
		completed bool
		want      string
	}{
		{"green", true, false, "green gate passed"},
		{"red but completed", false, true, "signalled completion"},
		{"red and truncated", false, false, "turn limit"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		logGreenGate(&buf, pipeline.StepImplement, c.passed, c.completed, 120, "VERIFY-FAIL-OUTPUT")
		got := buf.String()
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: log %q does not contain %q", c.name, got, c.want)
		}
		// On failure the verify output must be echoed so the operator (and the
		// retry) sees the actual cause, not a hardcoded "tests red".
		if !c.passed && !strings.Contains(got, "VERIFY-FAIL-OUTPUT") {
			t.Errorf("%s: failure log must echo the verify output; got %q", c.name, got)
		}
	}
}

// On a green-gate failure the verify output is fed into the next Implement
// attempt's prompt (via {{GREEN_GATE_FAILURE}}), so the retry targets the actual
// failure instead of re-deriving the same defect blind (the #78 gofmt episode).
func TestRunner_GreenGateFailureFedIntoRetryPrompt(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)

	tDir := makeTemplateDir(t, map[string]string{
		"implement.md":   "Implement {{ISSUE_NUMBER}}\nPRIOR_FAILURE:\n{{GREEN_GATE_FAILURE}}",
		"review.md":      "Review {{ISSUE_NUMBER}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})

	// Verify fails on the first Implement attempt with a distinctive marker, then
	// passes on the retry.
	var calls int
	verify := func(context.Context, string) (bool, string) {
		calls++
		if calls == 1 {
			return false, "gofmt -l flagged internal/runner/runner.go"
		}
		return true, ""
	}

	inv := &recordingInvoker{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/feedback"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      inv,
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		TestRunner:   verify,
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(inv.opts) < 2 {
		t.Fatalf("expected ≥2 Implement invocations (initial + retry); got %d", len(inv.opts))
	}
	if strings.Contains(inv.opts[0].Prompt, "gofmt -l flagged") {
		t.Errorf("first Implement prompt must NOT contain a prior failure; got:\n%s", inv.opts[0].Prompt)
	}
	if !strings.Contains(inv.opts[1].Prompt, "gofmt -l flagged internal/runner/runner.go") {
		t.Errorf("retry Implement prompt must carry the green-gate failure output; got:\n%s", inv.opts[1].Prompt)
	}
}
