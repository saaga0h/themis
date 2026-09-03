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

// ---------------------------------------------------------------------------
// Issue #112 — redactBodyArg must also redact the combined "--body=<value>" form
// ---------------------------------------------------------------------------

func TestRunGH_ErrorExcludesCombinedBodyArgumentValue(t *testing.T) {
	dir := writeFakeGH(t, "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir)

	const secret = "SECRET_MARKER_TEXT_DO_NOT_LEAK"
	err := runGH(t.Context(), "issue", "comment", "1", "--body="+secret)
	if err == nil {
		t.Fatal("runGH: expected error, got nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("runGH error leaks --body=<value> value: %q contains %q", err.Error(), secret)
	}
}

func TestRedactBodyArg(t *testing.T) {
	const secret = "SECRET_MARKER_TEXT_DO_NOT_LEAK"

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "combined form redacts value and strips secret",
			args: []string{"issue", "comment", "1", "--body=" + secret},
			want: []string{"issue", "comment", "1", "--body=[REDACTED]"},
		},
		{
			name: "separate token form still redacted (regression)",
			args: []string{"issue", "comment", "1", "--body", secret},
			want: []string{"issue", "comment", "1", "--body", "[REDACTED]"},
		},
		{
			name: "--body-file is not treated as --body",
			args: []string{"issue", "comment", "1", "--body-file", secret},
			want: []string{"issue", "comment", "1", "--body-file", secret},
		},
		{
			name: "positional argument containing 'body' substring is unchanged",
			args: []string{"issue", "comment", "1", "somebody"},
			want: []string{"issue", "comment", "1", "somebody"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactBodyArg(c.args)
			if len(got) != len(c.want) {
				t.Fatalf("redactBodyArg(%v) = %v, want %v", c.args, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("redactBodyArg(%v)[%d] = %q, want %q", c.args, i, got[i], c.want[i])
				}
			}
		})
	}
}
