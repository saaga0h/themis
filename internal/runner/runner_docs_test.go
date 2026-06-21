package runner

// Docs step: surface-triggered skipping. The step runs only when the change
// touches a declared documented surface; otherwise it is skipped without
// spawning an agent.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
)

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
