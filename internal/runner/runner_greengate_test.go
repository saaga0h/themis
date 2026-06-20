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
		sr := deriveStepResult(context.Background(), step, &agent.InvokeResult{Completed: true}, cfg)
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
		sr := deriveStepResult(context.Background(), step, &agent.InvokeResult{Completed: true}, cfg)
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
		sr := deriveStepResult(context.Background(), step, &agent.InvokeResult{}, cfg)
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
		{"red but completed", false, true, "may be stuck"},
		{"red and truncated", false, false, "turn limit"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		logGreenGate(&buf, pipeline.StepImplement, c.passed, c.completed, 120)
		got := buf.String()
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: log %q does not contain %q", c.name, got, c.want)
		}
	}
}
