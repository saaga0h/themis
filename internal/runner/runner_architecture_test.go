package runner

// Architectural guards: package boundaries, interface shape, and the file-size
// limits that keep the runner package legible.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitOps_InterfaceHasRequiredMethods asserts that the GitOps interface
// exists in the runner package with exactly the method signatures used by
// runner.go. The compile-time assertion is the var _ GitOps = &fakeGitOps{}
// line at the top of the shared stubs section; this test body provides a
// named anchor for the mapping and verifies the stub can be created.
func TestGitOps_InterfaceHasRequiredMethods(t *testing.T) {
	// fakeGitOps (defined in shared stubs) implements every GitOps method.
	// If GitOps does not exist in the runner package this file fails to compile.
	g := &fakeGitOps{commitsAhead: 2}

	ctx := context.Background()
	dir := t.TempDir()

	// CheckoutNewBranch
	if err := g.CheckoutNewBranch(ctx, dir, "feat/x"); err != nil {
		t.Errorf("CheckoutNewBranch: %v", err)
	}
	// Checkout
	if err := g.Checkout(ctx, dir, "main"); err != nil {
		t.Errorf("Checkout: %v", err)
	}
	// PushBranch
	if err := g.PushBranch(ctx, dir, "feat/x"); err != nil {
		t.Errorf("PushBranch: %v", err)
	}
	// CurrentBranch
	branch, err := g.CurrentBranch(ctx, dir)
	if err != nil {
		t.Errorf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Error("CurrentBranch must return a non-empty string")
	}
	// BranchCommitLog
	_ = g.BranchCommitLog(ctx, dir)
	// CommitsAheadOfBase
	n, err := g.CommitsAheadOfBase(ctx, dir, "main")
	if err != nil {
		t.Errorf("CommitsAheadOfBase: %v", err)
	}
	if n != 2 {
		t.Errorf("CommitsAheadOfBase = %d, want 2", n)
	}
}

// TestConfig_GitFieldAcceptsGitOpsImpl asserts that Config has a Git field of
// type GitOps and accepts a fakeGitOps value. Fails to compile until Config.Git
// is defined in the runner package.
func TestConfig_GitFieldAcceptsGitOpsImpl(t *testing.T) {
	g := &fakeGitOps{}
	cfg := Config{
		Git: g, // fails to compile: no Git field on Config yet
	}
	if cfg.Git == nil {
		t.Error("Config.Git must be non-nil after assignment")
	}
}

// TestConfig_HasProfileLoaderField asserts that Config has a ProfileLoader
// field of type func(dir string) (*profile.Profile, error). This mirrors the
// signature of profile.Load so the runner can call ProfileLoader(cfg.WorkDir)
// instead of importing internal/profile directly.
//
// Fails to compile until Config.ProfileLoader is defined in the runner package.
func TestConfig_HasProfileLoaderField(t *testing.T) {
	called := false
	cfg := Config{
		ProfileLoader: func(dir string) (ProfileData, error) {
			called = true
			return ProfileData{ImplementModel: "sonnet", ReviewModel: "sonnet"}, nil
		},
	}
	if cfg.ProfileLoader == nil {
		t.Error("Config.ProfileLoader must be non-nil after assignment")
	}
	// Call it to confirm the function signature is correct.
	p, err := cfg.ProfileLoader(t.TempDir())
	if err != nil {
		t.Fatalf("ProfileLoader returned unexpected error: %v", err)
	}
	if p.ImplementModel == "" {
		t.Error("ProfileLoader must return a ProfileData with ImplementModel set")
	}
	if !called {
		t.Error("ProfileLoader was not called")
	}
}

// TestRunner_BlockingThresholdNotDefinedInRunnerPackage asserts that the
// blockingThreshold constant has been moved out of runner.go into internal/review/.
// FAILS NOW: runner.go still contains "const blockingThreshold".
func TestRunner_BlockingThresholdNotDefinedInRunnerPackage(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	if strings.Contains(string(src), "const blockingThreshold") {
		t.Error("runner.go must not define const blockingThreshold — it belongs in internal/review/")
	}
}

// TestRunner_NoReviewTypeDeclarations_InRunnerGo asserts that ReviewFinding and
// ReviewResults have been extracted from runner.go.
// FAILS NOW: both types are declared in runner.go.
func TestRunner_NoReviewTypeDeclarations_InRunnerGo(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	content := string(src)
	if strings.Contains(content, "type ReviewFinding") {
		t.Error("runner.go must not declare type ReviewFinding — it belongs in internal/review/")
	}
	if strings.Contains(content, "type ReviewResults") {
		t.Error("runner.go must not declare type ReviewResults — it belongs in internal/review/")
	}
}

// TestRunner_NoReviewFuncDeclarations_InRunnerGo asserts that the four
// review-results functions have been extracted from runner.go.
// FAILS NOW: all four are still defined in runner.go.
func TestRunner_NoReviewFuncDeclarations_InRunnerGo(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	content := string(src)
	for _, fn := range []string{
		"func readReviewResults",
		"func countFindingsBySeverity",
		"func determineBlockingStatus",
		"func formatBlockingFindings",
	} {
		if strings.Contains(content, fn) {
			t.Errorf("runner.go must not define %s — it belongs in internal/review/", fn)
		}
	}
}

// TestRunner_ImportsList_ContainsInternalReview asserts that runner.go imports
// the internal/review package after extraction.
// FAILS NOW: the import is not present.
func TestRunner_ImportsList_ContainsInternalReview(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	if !strings.Contains(string(src), `"github.com/saaga0h/themis/internal/review"`) {
		t.Error(`runner.go must import "github.com/saaga0h/themis/internal/review" after extraction`)
	}
}

// TestRunner_RunnerGoIsUnder500Lines asserts that runner.go has fewer than 500
// lines after the review-results extraction.
// FAILS NOW: runner.go is over 500 lines.
func TestRunnerGo_LineCountUnder500(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("ReadFile runner.go: %v", err)
	}
	lines := strings.Count(string(src), "\n")
	if lines >= 500 {
		t.Errorf("runner.go has %d lines; must be < 500 after extraction", lines)
	}
}

// maxTestFileLines caps every runner_*_test.go file. The package's tests are
// organised by concern (one file per concern, shared harness in
// runner_support_test.go); this guard stops any one file from silently growing
// back into a catch-all. When a file trips it, split it by concern rather than
// raising the limit.
const maxTestFileLines = 600

func TestRunnerTestFiles_StayUnderLineLimit(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("Glob test files: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", f, err)
		}
		if lines := strings.Count(string(src), "\n"); lines >= maxTestFileLines {
			t.Errorf("%s has %d lines; must be < %d — split it by concern", f, lines, maxTestFileLines)
		}
	}
}
