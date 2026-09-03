package checkpoint_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/saaga0h/themis/internal/checkpoint"
	"github.com/saaga0h/themis/internal/pipeline"
)

// initGitRepo and makeCommit helpers are defined in checkpoint_test.go (same package).

// checkpoint function maps pipeline steps to expected commit prefixes.
// It calls VerifyCleanWorkingTree only for committing steps (those with a
// prefix); non-committing steps (Review, Ship, ...) skip the clean-tree guard.
// Refactor and Docs steps allow no new commit (may be no-ops).

func TestNewStepCheckpoint_TestRed_CorrectPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "test(scope): add failing tests")
	if err := fn(ctx, pipeline.StepTestRed, dir); err != nil {
		t.Errorf("expected no error for test( commit at TestRed step, got: %v", err)
	}
}

func TestNewStepCheckpoint_TestRed_WrongPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "feat(scope): wrong prefix for TestRed")
	if err := fn(ctx, pipeline.StepTestRed, dir); err == nil {
		t.Error("expected error when TestRed commit does not start with test(")
	}
}

func TestNewStepCheckpoint_Implement_CorrectPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "feat(runner): implement the feature")
	if err := fn(ctx, pipeline.StepImplement, dir); err != nil {
		t.Errorf("expected no error for feat( commit at Implement step, got: %v", err)
	}
}

func TestNewStepCheckpoint_Implement_WrongPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "test(scope): wrong prefix for Implement")
	if err := fn(ctx, pipeline.StepImplement, dir); err == nil {
		t.Error("expected error when Implement commit does not start with feat(")
	}
}

func TestNewStepCheckpoint_Refactor_CorrectPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "refactor(checkpoint): clean up logic")
	if err := fn(ctx, pipeline.StepRefactor, dir); err != nil {
		t.Errorf("expected no error for refactor( commit at Refactor step, got: %v", err)
	}
}

// Refactor step allows no new commit.
func TestNewStepCheckpoint_Refactor_NoNewCommit_OK(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	// No new commit — Refactor is a no-op; this must not be an error.
	if err := fn(ctx, pipeline.StepRefactor, dir); err != nil {
		t.Errorf("Refactor step should allow no new commit (no-op), got: %v", err)
	}
}

func TestNewStepCheckpoint_Fix_CorrectPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "fix(checkpoint): resolve blocking issue")
	if err := fn(ctx, pipeline.StepFix, dir); err != nil {
		t.Errorf("expected no error for fix( commit at Fix step, got: %v", err)
	}
}

func TestNewStepCheckpoint_Fix_WrongPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "feat(scope): wrong prefix for Fix step")
	if err := fn(ctx, pipeline.StepFix, dir); err == nil {
		t.Error("expected error when Fix commit does not start with fix(")
	}
}

func TestNewStepCheckpoint_Docs_CorrectPrefix(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "docs(readme): update the documentation")
	if err := fn(ctx, pipeline.StepDocs, dir); err != nil {
		t.Errorf("expected no error for docs( commit at Docs step, got: %v", err)
	}
}

// Docs step allows no new commit.
func TestNewStepCheckpoint_Docs_NoNewCommit_OK(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	// No new commit — Docs is a no-op; this must not be an error.
	if err := fn(ctx, pipeline.StepDocs, dir); err != nil {
		t.Errorf("Docs step should allow no new commit (no-op), got: %v", err)
	}
}

// checkpoint calls VerifyCleanWorkingTree — dirty tree must fail even with correct prefix.
func TestNewStepCheckpoint_DirtyWorkingTree_Fails(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	makeCommit(t, dir, "test(scope): add tests to pipeline")
	dirty := filepath.Join(dir, "uncommitted.txt")
	if err := os.WriteFile(dirty, []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fn(ctx, pipeline.StepTestRed, dir); err == nil {
		t.Error("checkpoint should fail when working tree has uncommitted changes")
	}
}

// VerifyCleanWorkingTree is called even for no-op steps (Refactor with dirty tree must fail).
func TestNewStepCheckpoint_Refactor_NoNewCommit_DirtyTree_Fails(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	dirty := filepath.Join(dir, "uncommitted.txt")
	if err := os.WriteFile(dirty, []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fn(ctx, pipeline.StepRefactor, dir); err == nil {
		t.Error("checkpoint should fail when working tree is dirty, even for a no-op Refactor step")
	}
}

// Review commits nothing, so a dirty tree there — e.g. a binary left by a
// `go build`/`go test` the Review agent ran to verify the code — must NOT block
// the run. Regression for issue #53's "checkpoint failed after step Review".
func TestNewStepCheckpoint_Review_DirtyTree_Passes(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	// A newly created, uncommitted build artifact — the exact case from #53.
	artifact := filepath.Join(dir, "heimdall")
	if err := os.WriteFile(artifact, []byte("\x7fELF binary-ish"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fn(ctx, pipeline.StepReview, dir); err != nil {
		t.Errorf("Review commits nothing; a dirty tree must not fail its checkpoint, got: %v", err)
	}
}

// Ship likewise produces no commit of its own — a dirty tree must not block it.
func TestNewStepCheckpoint_Ship_DirtyTree_Passes(t *testing.T) {
	dir := initGitRepo(t)
	ctx := context.Background()
	fn, err := checkpoint.NewStepCheckpoint(ctx, dir)
	if err != nil {
		t.Fatalf("NewStepCheckpoint: %v", err)
	}
	dirty := filepath.Join(dir, "artifact.bin")
	if err := os.WriteFile(dirty, []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fn(ctx, pipeline.StepShip, dir); err != nil {
		t.Errorf("Ship commits nothing; a dirty tree must not fail its checkpoint, got: %v", err)
	}
}
