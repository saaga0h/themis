package runner

// Core pipeline flow: step sequencing, state save/resume, and the test-fix
// retry ceiling.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

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
