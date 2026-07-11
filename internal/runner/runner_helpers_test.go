package runner

// Tests for blockIssue (internal/runner/runner_helpers.go:154), issue #63 AC8.
//
// failingIssueWriter is a new local fake distinct from stubIssueWriter in
// runner_support_test.go: stubIssueWriter always returns nil errors and is
// shared by ~60 other tests, so it cannot be reused to exercise blockIssue's
// failure paths without risking those other tests.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/labels"
)

// failingIssueWriter records calls and returns configurable errors from
// Comment and AddLabel, so tests can exercise blockIssue's two failure paths
// independently. RemoveLabel and CreatePR are no-ops; blockIssue does not
// call them.
type failingIssueWriter struct {
	commentErr  error
	addLabelErr error

	commentCalls   []string
	addLabelCalls  []string
	addLabelNumber int
}

func (f *failingIssueWriter) AddLabel(_ context.Context, number int, label string) error {
	f.addLabelCalls = append(f.addLabelCalls, label)
	f.addLabelNumber = number
	return f.addLabelErr
}

func (f *failingIssueWriter) RemoveLabel(_ context.Context, _ int, _ string) error {
	return nil
}

func (f *failingIssueWriter) Comment(_ context.Context, _ int, body string) error {
	f.commentCalls = append(f.commentCalls, body)
	return f.commentErr
}

func (f *failingIssueWriter) CreatePR(_ context.Context, _ PROptions) (string, error) {
	return "", nil
}

// Compile-time assertion: failingIssueWriter must implement IssueWriter.
var _ IssueWriter = &failingIssueWriter{}

func TestBlockIssue_CommentError(t *testing.T) {
	commentErr := errors.New("comment boom")
	fake := &failingIssueWriter{commentErr: commentErr}
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
	if len(fake.addLabelCalls) != 0 {
		t.Errorf("blockIssue: AddLabel was called (%v), want it not to be called when Comment fails", fake.addLabelCalls)
	}
}

func TestBlockIssue_AddLabelError(t *testing.T) {
	addLabelErr := errors.New("label boom")
	fake := &failingIssueWriter{addLabelErr: addLabelErr}
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
	if len(fake.commentCalls) != 1 {
		t.Fatalf("blockIssue: Comment calls = %d, want 1 (Comment must be called before AddLabel)", len(fake.commentCalls))
	}
	if len(fake.addLabelCalls) != 1 || fake.addLabelCalls[0] != labels.Blocked {
		t.Errorf("blockIssue: AddLabel called with %v, want [%q]", fake.addLabelCalls, labels.Blocked)
	}
}
