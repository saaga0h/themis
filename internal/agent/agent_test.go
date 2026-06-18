package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- Interface compliance ---

func TestClaudeCodeInvokerImplementsInvoker(t *testing.T) {
	var _ Invoker = &ClaudeCodeInvoker{}
}

// --- Fake invoker for testing ---

type fakeInvoker struct {
	result *InvokeResult
	err    error
}

func (f *fakeInvoker) Invoke(_ context.Context, _ InvokeOptions) (*InvokeResult, error) {
	return f.result, f.err
}

var _ Invoker = &fakeInvoker{}

func TestFakeInvokerReturnsResult(t *testing.T) {
	fi := &fakeInvoker{
		result: &InvokeResult{
			ExitCode:    0,
			Stdout:      "done",
			CommitsMade: []string{"abc123"},
			TestsPassed: true,
			Completed:   true,
		},
	}

	result, err := fi.Invoke(context.Background(), InvokeOptions{
		Prompt:  "do something",
		Model:   "sonnet",
		MaxTurns: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Completed {
		t.Error("expected Completed=true")
	}
	if len(result.CommitsMade) != 1 {
		t.Errorf("expected 1 commit, got %d", len(result.CommitsMade))
	}
}

func TestFakeInvokerReturnsError(t *testing.T) {
	fi := &fakeInvoker{err: errors.New("agent crashed")}
	_, err := fi.Invoke(context.Background(), InvokeOptions{})
	if err == nil {
		t.Error("expected error from fake invoker")
	}
}

// --- InvokeOptions fields ---

func TestInvokeOptionsFields(t *testing.T) {
	opts := InvokeOptions{
		Prompt:       "hello",
		Model:        "opus",
		MaxTurns:     42,
		WorkDir:      "/tmp/work",
		AllowedTools: []string{"Read", "Write", "Edit"},
	}
	if opts.Prompt != "hello" {
		t.Error("Prompt not stored")
	}
	if opts.Model != "opus" {
		t.Error("Model not stored")
	}
	if opts.MaxTurns != 42 {
		t.Error("MaxTurns not stored")
	}
	if opts.WorkDir != "/tmp/work" {
		t.Error("WorkDir not stored")
	}
	if len(opts.AllowedTools) != 3 {
		t.Error("AllowedTools not stored")
	}
}

// --- InvokeResult fields ---

func TestInvokeResultFields(t *testing.T) {
	r := InvokeResult{
		ExitCode:    0,
		Stdout:      "output",
		CommitsMade: []string{"sha1", "sha2"},
		TestsPassed: true,
		Completed:   true,
	}
	if r.ExitCode != 0 {
		t.Error("ExitCode not stored")
	}
	if r.Stdout != "output" {
		t.Error("Stdout not stored")
	}
	if len(r.CommitsMade) != 2 {
		t.Error("CommitsMade not stored")
	}
	if !r.TestsPassed {
		t.Error("TestsPassed not stored")
	}
	if !r.Completed {
		t.Error("Completed not stored")
	}
}

// --- Completed detection ---

func TestCompletionMarkerDetection(t *testing.T) {
	cases := []struct {
		output    string
		completed bool
	}{
		{"Task complete. All ACs pass.", true},
		{"COMPLETED: implementation done", true},
		{"some output without marker", false},
		{"", false},
	}
	for _, tc := range cases {
		got := containsCompletionMarker(tc.output)
		if got != tc.completed {
			t.Errorf("output %q: got completed=%v, want %v", tc.output, got, tc.completed)
		}
	}
}

// --- ClaudeCodeInvoker cmd building ---

func TestBuildCommandArgs(t *testing.T) {
	invoker := &ClaudeCodeInvoker{}
	opts := InvokeOptions{
		Model:    "sonnet",
		MaxTurns: 50,
	}
	args := invoker.buildArgs(opts)

	mustContain := func(flag string) {
		t.Helper()
		for _, a := range args {
			if strings.Contains(a, flag) {
				return
			}
		}
		// Also check adjacent pairs
		for i := 0; i < len(args)-1; i++ {
			if args[i] == flag {
				return
			}
		}
		t.Errorf("args %v missing flag %q", args, flag)
	}

	mustContain("--print")
	mustContain("--dangerously-skip-permissions")
	mustContain("--max-turns")
	mustContain("--model")
}

func TestBuildCommandIncludesModel(t *testing.T) {
	invoker := &ClaudeCodeInvoker{}
	args := invoker.buildArgs(InvokeOptions{Model: "opus", MaxTurns: 10})
	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "opus" {
			found = true
		}
	}
	if !found {
		t.Errorf("--model opus not found in args: %v", args)
	}
}

func TestBuildCommandIncludesMaxTurns(t *testing.T) {
	invoker := &ClaudeCodeInvoker{}
	args := invoker.buildArgs(InvokeOptions{Model: "sonnet", MaxTurns: 42})
	found := false
	for i, a := range args {
		if a == "--max-turns" && i+1 < len(args) && args[i+1] == "42" {
			found = true
		}
	}
	if !found {
		t.Errorf("--max-turns 42 not found in args: %v", args)
	}
}

// --- Context cancellation ---

func TestInvokeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	invoker := &ClaudeCodeInvoker{}
	_, err := invoker.Invoke(ctx, InvokeOptions{
		Prompt:   "test",
		Model:    "sonnet",
		MaxTurns: 1,
		WorkDir:  t.TempDir(),
	})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

// --- OTEL: InvokeOptions new fields ---

func TestInvokeOptionsHasIssueNumber(t *testing.T) {
	opts := InvokeOptions{IssueNumber: 42}
	if opts.IssueNumber != 42 {
		t.Errorf("IssueNumber: got %d, want 42", opts.IssueNumber)
	}
}

func TestInvokeOptionsHasPipelineStep(t *testing.T) {
	opts := InvokeOptions{PipelineStep: "TestRed"}
	if opts.PipelineStep != "TestRed" {
		t.Errorf("PipelineStep: got %q, want %q", opts.PipelineStep, "TestRed")
	}
}

// --- OTEL: buildOTELResourceAttributes ---

func TestBuildOTELResourceAttributes_NoParentEnv(t *testing.T) {
	result := buildOTELResourceAttributes(99, "Implement", nil)
	want := "issue.number=99,pipeline.step=Implement"
	if result != want {
		t.Errorf("buildOTELResourceAttributes(99, \"Implement\", nil) = %q, want %q", result, want)
	}
}

func TestBuildOTELResourceAttributes_ContainsIssueNumber(t *testing.T) {
	result := buildOTELResourceAttributes(42, "TestRed", nil)
	if !strings.Contains(result, "issue.number=42") {
		t.Errorf("buildOTELResourceAttributes(42, \"TestRed\", nil) = %q; want it to contain \"issue.number=42\"", result)
	}
}

func TestBuildOTELResourceAttributes_ContainsPipelineStep(t *testing.T) {
	result := buildOTELResourceAttributes(42, "TestRed", nil)
	if !strings.Contains(result, "pipeline.step=TestRed") {
		t.Errorf("buildOTELResourceAttributes(42, \"TestRed\", nil) = %q; want it to contain \"pipeline.step=TestRed\"", result)
	}
}

func TestBuildOTELResourceAttributes_PreservesExistingAttrs(t *testing.T) {
	parent := []string{"OTEL_RESOURCE_ATTRIBUTES=existing.key=oldval", "HOME=/root"}
	result := buildOTELResourceAttributes(42, "Review", parent)
	if !strings.Contains(result, "existing.key=oldval") {
		t.Errorf("result %q must contain existing parent attr \"existing.key=oldval\"", result)
	}
	if !strings.Contains(result, "issue.number=42") {
		t.Errorf("result %q must contain \"issue.number=42\"", result)
	}
	if !strings.Contains(result, "pipeline.step=Review") {
		t.Errorf("result %q must contain \"pipeline.step=Review\"", result)
	}
}

// --- OTEL: buildCmdEnv ---

func TestBuildCmdEnv_SetsOTELResourceAttributes(t *testing.T) {
	result := buildCmdEnv([]string{"PATH=/usr/bin"}, 7, "Fix")

	var otelVal string
	for _, entry := range result {
		if strings.HasPrefix(entry, "OTEL_RESOURCE_ATTRIBUTES=") {
			otelVal = strings.TrimPrefix(entry, "OTEL_RESOURCE_ATTRIBUTES=")
			break
		}
	}
	if otelVal == "" {
		t.Fatalf("OTEL_RESOURCE_ATTRIBUTES not found in buildCmdEnv result: %v", result)
	}
	if !strings.Contains(otelVal, "issue.number=7") {
		t.Errorf("OTEL_RESOURCE_ATTRIBUTES value %q must contain \"issue.number=7\"", otelVal)
	}
	if !strings.Contains(otelVal, "pipeline.step=Fix") {
		t.Errorf("OTEL_RESOURCE_ATTRIBUTES value %q must contain \"pipeline.step=Fix\"", otelVal)
	}
}

func TestBuildCmdEnv_DoesNotSetExporter(t *testing.T) {
	result := buildCmdEnv([]string{"PATH=/usr/bin"}, 7, "Fix")
	for _, entry := range result {
		if strings.HasPrefix(entry, "OTEL_EXPORTER_") {
			t.Errorf("buildCmdEnv must not set OTEL_EXPORTER_* vars; found %q", entry)
		}
		if strings.HasPrefix(entry, "OTEL_TRACES_EXPORTER") {
			t.Errorf("buildCmdEnv must not set OTEL_TRACES_EXPORTER; found %q", entry)
		}
	}
}
