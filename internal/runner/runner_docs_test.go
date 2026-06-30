package runner

// Docs step: surface-triggered skipping. The step runs only when the change
// touches a declared documented surface; otherwise it is skipped without
// spawning an agent.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

// stepFailingInvoker returns an error only when invoked for the named pipeline
// step; every other step gets a default completed result. It models a single
// flaky step (e.g. the Docs agent crashing on a large diff) without failing the
// rest of the run.
type stepFailingInvoker struct {
	failOn string
	err    error
}

func (s *stepFailingInvoker) Invoke(_ context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	if opts.PipelineStep == s.failOn {
		return nil, s.err
	}
	return &agent.InvokeResult{ExitCode: 0, Completed: true}, nil
}

func TestDocsSurfaceTouched(t *testing.T) {
	cases := []struct {
		name     string
		surfaces []string
		changed  string
		want     bool
	}{
		{"no surfaces declared -> always runs", nil, "internal/x.go", true},
		{"directory prefix match", []string{"cmd/themis/"}, "cmd/themis/run.go", true},
		{"exact file match", []string{"README.md"}, "README.md\nx.go", true},
		{"glob match", []string{"docs/*.md"}, "docs/guide.md", true},
		{"no match", []string{"cmd/themis/", "README.md"}, "internal/a.go\ninternal/b.go", false},
	}
	for _, c := range cases {
		if got := docsSurfaceTouched(c.surfaces, c.changed); got != c.want {
			t.Errorf("%s: docsSurfaceTouched(%v, %q) = %v, want %v", c.name, c.surfaces, c.changed, got, c.want)
		}
	}
}

// When surfaces are declared and the diff touches none of them, the Docs step is
// skipped (no agent spawned) and the pipeline proceeds to Ship.
func TestRunner_DocsStep_SkippedWhenNoSurfaceTouched(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	var buf bytes.Buffer
	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})
	w := &stubIssueWriter{prURL: "https://example.com/pr/skip"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &recordingInvoker{},
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
		DocSurfaces:  []string{"cmd/themis/", "README.md"},
		Git:          &fakeGitOps{changedFiles: "internal/runner/runner.go\ninternal/runner/runner_helpers.go", commitsAhead: 1},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Docs: skipped") {
		t.Errorf("Docs step must be skipped when no documented surface is touched; log:\n%s", buf.String())
	}
}

// A Docs-step agent crash must NOT abort the run. Docs is optional (it is already
// skipped when no documented surface is touched), runs last on the weakest model,
// and a flake there should not discard a fully implemented, reviewed,
// ready-to-ship PR. The failure is logged and emitted (so the cause reaches the
// diagnostic sink), and the pipeline proceeds to Ship. Critical steps (TestRed,
// Implement, Review) stay fatal — only Docs degrades to best-effort.
func TestRunner_DocsStep_AgentFailureIsNonFatal(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepDocs)

	var buf bytes.Buffer
	tDir := makeTemplateDir(t, map[string]string{
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})
	em := &captureEmitter{}
	w := &stubIssueWriter{prURL: "https://example.com/pr/docs-flaked"}
	cfg := Config{
		WorkDir:     workDir,
		IssueNumber: 42,
		Fetcher:     &stubFetcher{issue: sampleIssue()},
		Invoker: &stepFailingInvoker{
			failOn: pipeline.StepDocs.String(),
			err:    errors.New("claude exited with error: exit status 1\nstderr (last lines):\nturn limit reached"),
		},
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
		Emitter:      em,
		DocSurfaces:  []string{"cmd/themis/", "README.md"},
		Git:          &fakeGitOps{changedFiles: "cmd/themis/run.go", commitsAhead: 1},
	}

	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("a Docs agent crash must be non-fatal; Run returned: %v", err)
	}
	if result == nil || result.PRURL == "" {
		t.Fatal("expected a PR to be created despite the Docs failure")
	}

	var docsErr *StepRecord
	for i := range em.records {
		if em.records[i].Stage == pipeline.StepDocs.String() && em.records[i].Outcome == "error" {
			docsErr = &em.records[i]
		}
	}
	if docsErr == nil {
		t.Fatal("a non-fatal Docs failure must still emit an error record for diagnosis")
	}
	if !strings.Contains(docsErr.Detail, "turn limit reached") {
		t.Errorf("the Docs error record must carry the failure cause; got %q", docsErr.Detail)
	}
	if !strings.Contains(buf.String(), "Docs") {
		t.Errorf("the Docs failure should be logged; log:\n%s", buf.String())
	}
}
