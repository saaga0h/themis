package main

import (
	"context"
	"strings"
	"testing"
)

// With no verify commands declared the gate is a no-op pass — the warning is
// emitted at config time, not here.
func TestVerifyRunner_EmptyIsNoOpPass(t *testing.T) {
	ok, out := verifyRunner(nil, "")(context.Background(), t.TempDir())
	if !ok {
		t.Error("empty verify must pass (no-op)")
	}
	if !strings.Contains(out, "no verify") {
		t.Errorf("expected no-op note, got %q", out)
	}
}

// Declared commands run in order; all exit 0 → green.
func TestVerifyRunner_RunsCommandsAndPasses(t *testing.T) {
	ok, out := verifyRunner([]string{"echo first", "echo second"}, "")(context.Background(), t.TempDir())
	if !ok {
		t.Fatalf("expected pass, got fail: %s", out)
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("expected both command outputs, got %q", out)
	}
}

// The first non-zero exit fails the gate and stops — later commands do not run.
func TestVerifyRunner_FailsOnFirstNonZeroAndStops(t *testing.T) {
	ok, out := verifyRunner([]string{"echo before", "false", "echo after"}, "")(context.Background(), t.TempDir())
	if ok {
		t.Fatal("expected fail when a command exits non-zero")
	}
	if !strings.Contains(out, "before") {
		t.Errorf("expected output up to failure, got %q", out)
	}
	if strings.Contains(out, "after") {
		t.Errorf("must stop at first failure; 'after' should not run: %q", out)
	}
}
