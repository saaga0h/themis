package main

import (
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

// baseBranchForIssue is the provider-agnostic base the loop resets to before each
// issue. Ref wins when present; otherwise the originating branch; "main" last.
func TestBaseBranchForIssue(t *testing.T) {
	cases := []struct {
		name   string
		ref    string
		origin string
		want   string
	}{
		{"ref used when set (Gitea)", "themis-2.0", "some-branch", "themis-2.0"},
		{"ref normalized (refs/heads stripped)", "refs/heads/develop", "main", "develop"},
		{"ref trimmed of whitespace", "  release  ", "main", "release"},
		{"empty ref falls back to origin branch (GitHub)", "", "themis-2.0", "themis-2.0"},
		{"empty ref + empty origin falls back to main", "", "", "main"},
		{"whitespace-only ref falls back to origin", "   ", "master", "master"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := baseBranchForIssue(&tracker.IssueData{Ref: c.ref}, c.origin)
			if got != c.want {
				t.Errorf("baseBranchForIssue(ref=%q, origin=%q) = %q, want %q", c.ref, c.origin, got, c.want)
			}
		})
	}
}
