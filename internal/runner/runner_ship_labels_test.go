package runner

// Ship-step label reconciliation and PR-creation idempotency: after a PR exists
// the runner drops the ready-for-agent selection label (so the loop stops
// re-processing a shipped issue) and marks needs-review; an already-existing PR
// is an idempotent success, while a genuine CreatePR failure stays blocking.

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/saaga0h/themis/internal/labels"
)

// After a successful ship the runner drops the ready-for-agent selection label
// (so the loop does not re-pick an already-shipped issue) and adds needs-review.
func TestRunner_ShipRemovesReadyForAgentAndMarksNeedsReview(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/30"}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !slices.Contains(w.labelsRemoved, labels.ReadyForAgent) {
		t.Errorf("ship must remove %q so the loop stops re-selecting a shipped issue; labelsRemoved=%v",
			labels.ReadyForAgent, w.labelsRemoved)
	}
	if !slices.Contains(w.labelsAdded, labels.NeedsReview) {
		t.Errorf("ship must add %q; labelsAdded=%v", labels.NeedsReview, w.labelsAdded)
	}
}

// A CreatePR that reports the PR already exists (a prior run created it) is an
// idempotent success, not a block: Run returns no error and the ready-for-agent
// label is still dropped so the loop stops re-processing the issue.
func TestRunner_ShipTreatsExistingPRAsIdempotent(t *testing.T) {
	w := &stubIssueWriter{createPRErr: ErrPRAlreadyExists}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run must treat an already-existing PR as success, got error: %v", err)
	}
	if !slices.Contains(w.labelsRemoved, labels.ReadyForAgent) {
		t.Errorf("an already-existing PR must still drop %q; labelsRemoved=%v",
			labels.ReadyForAgent, w.labelsRemoved)
	}
}

// A CreatePR failure that is NOT "already exists" stays a hard error, and the
// selection label is left in place so the issue is retried on the next run.
func TestRunner_ShipCreatePRFailureStaysBlocking(t *testing.T) {
	w := &stubIssueWriter{createPRErr: errors.New("500 internal server error")}
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})

	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("Run must return an error when CreatePR fails for a non-idempotent reason")
	}
	if slices.Contains(w.labelsRemoved, labels.ReadyForAgent) {
		t.Errorf("a genuine CreatePR failure must NOT drop %q (the issue should be retried); labelsRemoved=%v",
			labels.ReadyForAgent, w.labelsRemoved)
	}
}
