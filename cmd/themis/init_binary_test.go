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

// Bare `themis init` (no content flags) refuses — it must not write a
// plausible-but-broken skeleton. (Interactive TTY init is a later slice.)
func TestInit_BareRefusesWithoutContentFlags(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bare `themis init` must exit non-zero, got success\noutput: %s", out)
	}
	if !strings.Contains(string(out), "--language") {
		t.Errorf("error must name --language, got: %s", out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".themis", "workflow.yaml")); statErr == nil {
		t.Error("bare init must not write a workflow.yaml")
	}
}

// `themis init --language go` creates a working config.
func TestInit_LanguageCreatesWorkflow(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--language", "go")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("themis init --language go must exit 0, got %v\noutput: %s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".themis", "workflow.yaml")); statErr != nil {
		t.Errorf("expected .themis/workflow.yaml to exist: %v", statErr)
	}
	if !strings.Contains(string(out), "created .themis/workflow.yaml") {
		t.Errorf("expected created message, got: %s", out)
	}
}

// A content flag without --language is a headless invocation missing its floor.
func TestInit_ImageWithoutLanguageErrors(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--image", "foo:latest")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("--image without --language must exit non-zero\noutput: %s", out)
	}
	if !strings.Contains(string(out), "--language") {
		t.Errorf("error must name --language, got: %s", out)
	}
}

// An unknown language (not a preset, not "other") is rejected with the choices.
func TestInit_UnknownLanguageErrors(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--language", "cobol")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("unknown language must exit non-zero\noutput: %s", out)
	}
	if !strings.Contains(string(out), "other") || !strings.Contains(string(out), "go") {
		t.Errorf("error must list valid languages plus other, got: %s", out)
	}
}

// `--language other` succeeds but prints the intentionally-incomplete note.
func TestInit_OtherPrintsIncompleteNote(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--language", "other")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("themis init --language other must exit 0, got %v\noutput: %s", err, out)
	}
	s := string(out)
	if !strings.Contains(strings.ToLower(s), "incomplete") {
		t.Errorf("other init must note the config is incomplete, got: %s", s)
	}
	if !strings.Contains(s, "docs/configuration-reference.md") {
		t.Errorf("other init must point at the reference docs, got: %s", s)
	}
}

// `--provider bogus` is rejected.
func TestInit_ProviderBogusErrors(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--language", "go", "--provider", "bogus")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("--provider bogus must exit non-zero\noutput: %s", out)
	}
	if !strings.Contains(string(out), "github") || !strings.Contains(string(out), "gitea") {
		t.Errorf("error must name the valid providers, got: %s", out)
	}
}

// Non-destructive: an existing workflow.yaml is skipped without --force.
func TestInit_ReportsSkippedWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelWorkflowFile(t, dir)

	cmd := exec.Command(binary, "init", "--language", "go")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(strings.ToLower(string(out)), "skip") {
		t.Errorf("expected a skipped report for the existing file, got: %s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after init: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("existing workflow.yaml must be unchanged, got: %q", got)
	}
}

// `--force` overwrites with the composed config (equal to a fresh run with the
// same inputs; a fixed --image keeps the comparison independent of dir name).
func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelWorkflowFile(t, dir)

	args := []string{"init", "--language", "go", "--image", "themis-fixed:latest", "--force"}
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init --force must exit 0, got %v\noutput: %s", err, out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after --force: %v", err)
	}
	if string(got) == string(sentinel) {
		t.Fatal("--force must overwrite the sentinel")
	}

	freshDir := t.TempDir()
	freshCmd := exec.Command(binary, "init", "--language", "go", "--image", "themis-fixed:latest")
	freshCmd.Dir = freshDir
	if out, err := freshCmd.CombinedOutput(); err != nil {
		t.Fatalf("fresh init must exit 0, got %v\noutput: %s", err, out)
	}
	want, err := os.ReadFile(filepath.Join(freshDir, ".themis", "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading fresh scaffold: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("forced overwrite must equal fresh composed config;\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// init writes .env.example and ignores .env.
func TestInit_WritesEnvExampleAndGitignore(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "init", "--language", "go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init must exit 0, got %v\noutput: %s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".env.example")); statErr != nil {
		t.Errorf(".env.example must exist: %v", statErr)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".env") {
		t.Errorf(".gitignore must ignore .env, got: %s", gi)
	}
}
