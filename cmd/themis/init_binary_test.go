package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeSentinelWorkflowFile writes a pre-existing, non-skeleton
// .themis/workflow.yaml into dir so non-destructive-init tests can assert it
// was left alone (or deliberately overwritten with --force).
func writeSentinelWorkflowFile(t *testing.T, dir string) (path string, content []byte) {
	t.Helper()
	themisDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir .themis: %v", err)
	}
	content = []byte("sentinel: true\n")
	path = filepath.Join(themisDir, "workflow.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write sentinel workflow.yaml: %v", err)
	}
	return path, content
}

// AC1: themis init in an empty directory creates .themis/workflow.yaml.
func TestInit_CreatesWorkflowFile(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("themis init must exit 0 in an empty directory, got: %v\noutput: %s", err, out)
	}
	path := filepath.Join(dir, ".themis", "workflow.yaml")
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("expected %s to exist after themis init, stat error: %v", path, statErr)
	}
}

// AC5: themis init prints next-steps guidance naming .themis/workflow.yaml.
func TestInit_PrintsNextStepsGuidanceNamingWorkflowFile(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("themis init must exit 0 in an empty directory, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), ".themis/workflow.yaml") {
		t.Errorf("expected themis init output to name .themis/workflow.yaml as next-steps guidance, got: %s", out)
	}
}

// AC4: themis init is non-destructive — an existing workflow.yaml is left
// unchanged and the run reports it was skipped.
func TestInit_ReportsSkippedWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelWorkflowFile(t, dir)

	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(strings.ToLower(string(out)), "skip") {
		t.Errorf("expected themis init output to report the existing file was skipped, got: %s", out)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after themis init: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("existing workflow.yaml must be left unchanged, got: %q, want: %q", got, sentinel)
	}
}

// AC4: themis init --force overwrites an existing workflow.yaml with the skeleton.
func TestInit_ForceFlagOverwritesViaCLI(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelWorkflowFile(t, dir)

	cmd := exec.Command(binary, "init", "--force")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("themis init --force must exit 0, got: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after themis init --force: %v", err)
	}
	if string(got) == string(sentinel) {
		t.Fatal("themis init --force must overwrite the existing sentinel content")
	}

	// Compare against a fresh scaffold produced by themis init in an empty
	// directory, to confirm the forced overwrite equals the skeleton content
	// (not merely "different from the sentinel").
	freshDir := t.TempDir()
	freshCmd := exec.Command(binary, "init")
	freshCmd.Dir = freshDir
	if out, err := freshCmd.CombinedOutput(); err != nil {
		t.Fatalf("themis init (fresh) must exit 0, got: %v\noutput: %s", err, out)
	}
	want, err := os.ReadFile(filepath.Join(freshDir, ".themis", "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading fresh scaffold: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("forced overwrite content must equal fresh skeleton content;\ngot:\n%s\nwant:\n%s", got, want)
	}
}
