package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/issuespec"
)

// composeVerify builds the combined verify list the way newIssueConfig is
// expected to: the project's declared verify commands plus the issue's
// extracted `check` blocks, appended (never committed — scaffolding for this
// run only). Mirrors the composition newIssueConfig will perform once #98 is
// implemented: append(desc.Verify, issuespec.ParseCheckBlocks(issue.Body)...).
func composeVerify(declared []string, issueBody string) []string {
	return append(append([]string{}, declared...), issuespec.ParseCheckBlocks(issueBody)...)
}

// AC4: green-gate failure feedback names which issue-declared check failed, so
// the bounded retry targets it.
func TestGreenGate_FailureNamesFailingIssueDeclaredCheck(t *testing.T) {
	dir := t.TempDir()
	body := "## Acceptance Criteria\n- [ ] Something\n\n```check\nfalse\n```\n"
	verify := composeVerify([]string{"echo build-ok"}, body)

	ok, out := verifyRunner(verify, "")(context.Background(), dir)
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
	runGate := verifyRunner(verify, "")

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

// #100: the factory's orchestration secrets must never reach a verify subprocess —
// those commands run arbitrary shell and their output is published (tracker,
// telemetry). A command that echoes all three factory secrets sees empty values,
// and none of the secret values appears in the returned output.
func TestVerifyRunner_StripsFactorySecrets(t *testing.T) {
	t.Setenv("GITEA_TOKEN", "gitea-secret-xyz")
	t.Setenv("GITHUB_TOKEN", "github-secret-xyz")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-secret-xyz")
	t.Setenv("THEMIS_TEST_SENTINEL", "sentinel-ok")

	cmd := `echo "gitea=[$GITEA_TOKEN] github=[$GITHUB_TOKEN] oauth=[$CLAUDE_CODE_OAUTH_TOKEN] sentinel=[$THEMIS_TEST_SENTINEL]"`
	passed, out := verifyRunner([]string{cmd}, "")(context.Background(), t.TempDir())

	if !passed {
		t.Fatalf("gate should pass, got failure:\n%s", out)
	}
	for _, secret := range []string{"gitea-secret-xyz", "github-secret-xyz", "oauth-secret-xyz"} {
		if strings.Contains(out, secret) {
			t.Errorf("factory secret %q leaked into published verify output:\n%s", secret, out)
		}
	}
	if !strings.Contains(out, "gitea=[] github=[] oauth=[]") {
		t.Errorf("factory secrets should read empty in the subprocess, got:\n%s", out)
	}
	// A non-secret parent variable must survive — this is a denylist, not an allowlist.
	if !strings.Contains(out, "sentinel=[sentinel-ok]") {
		t.Errorf("non-secret parent env must be preserved, got:\n%s", out)
	}
}

// PATH must survive the strip so builds/tests still resolve their tools.
func TestVerifyRunner_PreservesPath(t *testing.T) {
	passed, out := verifyRunner([]string{"command -v bash"}, "")(context.Background(), t.TempDir())
	if !passed {
		t.Fatalf("PATH must survive so bash resolves; gate failed:\n%s", out)
	}
}

// #100 AC4: the strip applies uniformly to every verify command — including the
// issue-declared check blocks appended to the workflow.yaml verify list.
func TestVerifyRunner_StripsSecretsForCheckBlocks(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-secret-xyz")

	body := "## Acceptance Criteria\n- [ ] x\n\n```check\n" +
		`echo "check-oauth=[$CLAUDE_CODE_OAUTH_TOKEN]"` + "\n```\n"
	verify := composeVerify([]string{"true"}, body)

	passed, out := verifyRunner(verify, "")(context.Background(), t.TempDir())
	if !passed {
		t.Fatalf("gate should pass, got failure:\n%s", out)
	}
	if strings.Contains(out, "oauth-secret-xyz") {
		t.Errorf("check-block command leaked the OAuth token:\n%s", out)
	}
	if !strings.Contains(out, "check-oauth=[]") {
		t.Errorf("check-block command should see an empty OAuth token, got:\n%s", out)
	}
}
