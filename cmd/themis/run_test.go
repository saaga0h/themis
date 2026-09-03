package main

import (
	"testing"
	"time"

	"github.com/saaga0h/themis/internal/runner"
	"github.com/saaga0h/themis/internal/tracker"
)

// ---------------------------------------------------------------------------
// AC4 + AC5 (issue #68): queriers live in internal/tracker, the interface stays
// at the consumer.
// ---------------------------------------------------------------------------

// TestQuerierConstructors_SatisfyConsumerInterface guards issue #68 AC4 and AC5:
// the GiteaQuerier/GitHubQuerier implementations were moved into internal/tracker,
// but the IssueQuerier interface stays defined here at the consumer. runRun relies
// on tracker.NewGiteaQuerier(...) / tracker.NewGitHubQuerier() returning values
// that satisfy this local interface. These assignments fail to compile if a
// constructor's return type drifts, or if IssueQuerier is moved into
// internal/tracker (which would force the consumer to import it from there).
func TestQuerierConstructors_SatisfyConsumerInterface(t *testing.T) {
	var gitea IssueQuerier = tracker.NewGiteaQuerier("owner", "repo", "https://example.com", "token", time.Second)
	if gitea == nil {
		t.Fatal("tracker.NewGiteaQuerier returned nil")
	}
	var github IssueQuerier = tracker.NewGitHubQuerier()
	if github == nil {
		t.Fatal("tracker.NewGitHubQuerier returned nil")
	}
}

// ---------------------------------------------------------------------------
// AC7: cmd/themis provides concrete GitOps and ProfileLoader implementations (issue #61)
// ---------------------------------------------------------------------------

// TestCmdGitOps_ImplementsRunnerGitOps asserts that cmd/themis declares a
// concrete type (cmdGitOps or similar) that satisfies runner.GitOps.
// Fails to compile until cmd/themis declares such a type.
func TestCmdGitOps_ImplementsRunnerGitOps(t *testing.T) {
	// This compile-time assertion will fail until cmd/themis defines a type
	// named cmdGitOps (or equivalent) that implements runner.GitOps.
	var _ runner.GitOps = &cmdGitOps{}
}

// TestCmdProfileLoader_SignatureMatchesRunnerConfig asserts that the concrete
// profileLoader function defined in cmd/themis has the same signature as
// Config.ProfileLoader: func(string) (runner.ProfileData, error).
//
// The assignment compiles only when the function exists with the correct signature.
func TestCmdProfileLoader_SignatureMatchesRunnerConfig(t *testing.T) {
	// profileLoader must be a package-level function in cmd/themis with signature
	// func(dir string) (runner.ProfileData, error).
	// The variable type enforces the signature at compile time.
	var fn func(string) (runner.ProfileData, error) = profileLoader
	if fn == nil {
		t.Error("profileLoader must be a non-nil function in cmd/themis")
	}
}
