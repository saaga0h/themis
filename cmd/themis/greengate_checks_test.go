package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

// composeVerify builds the combined verify list the way newIssueConfig is
// expected to: the project's declared verify commands plus the issue's
// extracted `check` blocks, appended (never committed — scaffolding for this
// run only). Mirrors the composition newIssueConfig will perform once #98 is
// implemented: append(desc.Verify, tracker.ParseCheckBlocks(issue.Body)...).
func composeVerify(declared []string, issueBody string) []string {
	return append(append([]string{}, declared...), tracker.ParseCheckBlocks(issueBody)...)
}

// AC4: green-gate failure feedback names which issue-declared check failed, so
// the bounded retry targets it.
func TestGreenGate_FailureNamesFailingIssueDeclaredCheck(t *testing.T) {
	dir := t.TempDir()
	body := "## Acceptance Criteria\n- [ ] Something\n\n```check\nfalse\n```\n"
	verify := composeVerify([]string{"echo build-ok"}, body)

	ok, out := verifyRunner(verify)(context.Background(), dir)
	if ok {
		t.Fatal("expected green gate to fail when an issue-declared check fails")
	}
	if !strings.Contains(out, "false") {
		t.Errorf("expected failure output to name the failing issue-declared check %q, got %q", "false", out)
	}
}

// AC5, the regression proof: re-running the #68 scenario (new types present,
// old not deleted) goes RED at the gate on the declared negative check, and
// the retry is driven to delete the old code.
func TestGreenGate_Issue68Scenario_RedOnDeclaredNegativeCheck_ThenGreenAfterDeletion(t *testing.T) {
	dir := t.TempDir()

	cmdDir := filepath.Join(dir, "cmd", "themis")
	trackerDir := filepath.Join(dir, "internal", "tracker")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir cmd/themis: %v", err)
	}
	if err := os.MkdirAll(trackerDir, 0o755); err != nil {
		t.Fatalf("mkdir internal/tracker: %v", err)
	}

	// New type present (the "move" landed)...
	newFile := filepath.Join(trackerDir, "new.go")
	if err := os.WriteFile(newFile, []byte("package tracker\n\ntype GiteaQuerier struct{}\n"), 0o644); err != nil {
		t.Fatalf("write internal/tracker/new.go: %v", err)
	}
	// ...but the old type was left behind (this is exactly #68).
	oldFile := filepath.Join(cmdDir, "old.go")
	if err := os.WriteFile(oldFile, []byte("package main\n\ntype GiteaQuerier struct{}\n"), 0o644); err != nil {
		t.Fatalf("write cmd/themis/old.go: %v", err)
	}

	body := "## Acceptance Criteria\n" +
		"- [ ] tracker.GiteaQuerier exists in internal/tracker\n" +
		"- [ ] No GiteaQuerier struct remains in cmd/themis\n\n" +
		"```check\n! grep -rq 'type GiteaQuerier' cmd/themis/\n```\n"

	verify := composeVerify([]string{"true"}, body)
	runGate := verifyRunner(verify)

	// Run #1: old.go still present — the declared negative check must fail the gate.
	passed, out := runGate(context.Background(), dir)
	if passed {
		t.Fatal("expected green gate to fail while cmd/themis/old.go (GiteaQuerier) still exists")
	}
	if !strings.Contains(out, "grep") {
		t.Errorf("expected failure output to name the failing issue-declared check, got %q", out)
	}

	// Simulate the retry-driven fix: delete the old code.
	if err := os.Remove(oldFile); err != nil {
		t.Fatalf("remove cmd/themis/old.go: %v", err)
	}

	// Run #2: old.go gone — the same declared check must now pass.
	passed, out = runGate(context.Background(), dir)
	if !passed {
		t.Fatalf("expected green gate to pass once cmd/themis/old.go is deleted, got failure: %s", out)
	}
}
