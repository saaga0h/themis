package main

// Test helper only — no test functions here. Provides a fake `gh` executable
// on PATH so ghIssueWriter/runGH tests (issue_writer_gh_test.go) can exercise
// the CLI-invocation paths without a real gh binary or network access.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFakeGH creates a temp directory containing an executable `gh` shell
// script with the given contents and returns the directory. Callers should
// install it on PATH with t.Setenv("PATH", dir) so exec.CommandContext(ctx,
// "gh", ...) resolves to the fake.
func writeFakeGH(t *testing.T, script string) (pathDir string) {
	t.Helper()
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake gh script: %v", err)
	}
	return dir
}
