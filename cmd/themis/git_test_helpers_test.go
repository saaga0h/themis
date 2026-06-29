package main

import (
	"os/exec"
	"strings"
	"testing"
)

func assertBranchIsMain(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git symbolic-ref: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "main" {
		t.Errorf("expected branch 'main', got %q", got)
	}
}
