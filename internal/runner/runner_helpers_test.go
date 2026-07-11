package runner

// Tests for blockIssue (internal/runner/runner_helpers.go:154), issue #63 AC8.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/labels"
)

func TestBlockIssue_CommentError(t *testing.T) {
	commentErr := errors.New("comment boom")
	fake := &stubIssueWriter{commentErr: commentErr}
	cfg := Config{
		IssueNumber: 42,
		IssueWriter: fake,
	}
	reason := errors.New("some failure")

	err := blockIssue(context.Background(), cfg, reason)
	if err == nil {
		t.Fatal("blockIssue: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "posting block comment") {
		t.Errorf("blockIssue error = %q, want it to contain %q", err.Error(), "posting block comment")
	}
	if !errors.Is(err, commentErr) {
		t.Errorf("blockIssue error = %v, want errors.Is to reach the original comment error %v", err, commentErr)
	}
	if len(fake.labelsAdded) != 0 {
		t.Errorf("blockIssue: AddLabel was called (%v), want it not to be called when Comment fails", fake.labelsAdded)
	}
}

func TestBlockIssue_AddLabelError(t *testing.T) {
	addLabelErr := errors.New("label boom")
	fake := &stubIssueWriter{addLabelErr: addLabelErr}
	cfg := Config{
		IssueNumber: 42,
		IssueWriter: fake,
	}
	reason := errors.New("some failure")

	err := blockIssue(context.Background(), cfg, reason)
	if err == nil {
		t.Fatal("blockIssue: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "adding blocked label") {
		t.Errorf("blockIssue error = %q, want it to contain %q", err.Error(), "adding blocked label")
	}
	if !errors.Is(err, addLabelErr) {
		t.Errorf("blockIssue error = %v, want errors.Is to reach the original add-label error %v", err, addLabelErr)
	}
	if len(fake.comments) != 1 {
		t.Fatalf("blockIssue: Comment calls = %d, want 1 (Comment must be called before AddLabel)", len(fake.comments))
	}
	if len(fake.labelsAdded) != 1 || fake.labelsAdded[0] != labels.Blocked {
		t.Errorf("blockIssue: AddLabel called with %v, want [%q]", fake.labelsAdded, labels.Blocked)
	}
}
