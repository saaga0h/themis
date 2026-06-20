package runner

// Branch-step behaviour: seeding .themis/review-results.json and creating the
// issue branch. Tests here call runBranchStep directly to exercise the error
// paths that cannot be reached via Run() without triggering earlier failures
// in pipeline.SaveState.
//
// Shared stubs and helpers (sampleIssue) are defined in runner_test.go.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

// TestRunBranchStep_MkdirAllFailure verifies that runBranchStep returns an
// error mentioning ".themis directory" when os.MkdirAll cannot create the
// .themis directory (because a file exists at that path).
func TestRunBranchStep_MkdirAllFailure(t *testing.T) {
	workDir := t.TempDir()

	// Place a regular file at the .themis path so MkdirAll cannot create a
	// directory there.
	if err := os.WriteFile(filepath.Join(workDir, ".themis"), []byte("block"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := Config{
		WorkDir:     workDir,
		IssueNumber: 42,
	}
	issue := &tracker.IssueData{Number: 42, Title: "test issue"}

	err := runBranchStep(context.Background(), cfg, issue, io.Discard)
	if err == nil {
		t.Fatal("runBranchStep must return error when .themis cannot be created as a directory")
	}
	if !strings.Contains(err.Error(), "creating .themis directory") {
		t.Errorf("error must mention 'creating .themis directory'; got: %v", err)
	}
}

// TestRunBranchStep_WriteFileFailure verifies that runBranchStep returns an
// error mentioning "seeding review-results.json" when os.WriteFile cannot
// write to .themis/ (directory exists but is not writable).
func TestRunBranchStep_WriteFileFailure(t *testing.T) {
	workDir := t.TempDir()
	themisDir := filepath.Join(workDir, ".themis")

	// Create .themis as a read-only directory so MkdirAll succeeds but
	// WriteFile fails.
	if err := os.MkdirAll(themisDir, 0o555); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(themisDir, 0o755) })

	cfg := Config{
		WorkDir:     workDir,
		IssueNumber: 42,
	}
	issue := &tracker.IssueData{Number: 42, Title: "test issue"}

	err := runBranchStep(context.Background(), cfg, issue, io.Discard)
	if err == nil {
		t.Fatal("runBranchStep must return error when review-results.json cannot be written")
	}
	if !strings.Contains(err.Error(), "seeding review-results.json") {
		t.Errorf("error must mention 'seeding review-results.json'; got: %v", err)
	}
}
