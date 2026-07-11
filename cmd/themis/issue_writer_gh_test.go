package main

import (
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/runner"
)

// ---------------------------------------------------------------------------
// AC5 — ghIssueWriter.CreatePR
// ---------------------------------------------------------------------------

func TestGhIssueWriter_CreatePR_CLIFailure(t *testing.T) {
	dir := writeFakeGH(t, "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir)

	g := &ghIssueWriter{}
	url, err := g.CreatePR(t.Context(), runner.PROptions{Title: "x", Body: "y"})
	if url != "" {
		t.Errorf("CreatePR url = %q, want empty on CLI failure", url)
	}
	if err == nil {
		t.Fatal("CreatePR: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gh pr create") {
		t.Errorf("CreatePR error = %q, want it to contain %q", err.Error(), "gh pr create")
	}
}

func TestGhIssueWriter_CreatePR_Draft_PassesDraftFlag(t *testing.T) {
	dir := writeFakeGH(t, "#!/bin/sh\necho \"$*\"\n")
	t.Setenv("PATH", dir)

	g := &ghIssueWriter{}
	url, err := g.CreatePR(t.Context(), runner.PROptions{Title: "x", Body: "y", Draft: true})
	if err != nil {
		t.Fatalf("CreatePR: unexpected error: %v", err)
	}
	if !strings.Contains(url, "--draft") {
		t.Errorf("CreatePR returned %q, want it to contain %q", url, "--draft")
	}
}

// ---------------------------------------------------------------------------
// AC7 — runGH redaction
// ---------------------------------------------------------------------------

func TestRunGH_ErrorExcludesBodyArgumentValue(t *testing.T) {
	dir := writeFakeGH(t, "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir)

	const secret = "SECRET_MARKER_TEXT_DO_NOT_LEAK"
	err := runGH(t.Context(), "issue", "comment", "1", "--body", secret)
	if err == nil {
		t.Fatal("runGH: expected error, got nil")
	}
	// NOTE: this WILL fail RED against current code — current code embeds
	// args verbatim (fmt.Errorf("gh %v: ...", args, ...)) which includes the
	// literal --body value. That is correct and expected; Implement must
	// redact it.
	if strings.Contains(err.Error(), secret) {
		t.Errorf("runGH error leaks --body value: %q contains %q", err.Error(), secret)
	}
}
