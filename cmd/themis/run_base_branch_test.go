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
		// A factory issue branch must never be used as the base — neither the issue's
		// own leftover (which makes Ship see current==base) nor a sibling's.
		{"origin is a stale issue branch → main, not self", "", "issue/2-add-harness", "main"},
		{"origin is another issue branch → main, no stacking", "", "issue/5-delete-flow", "main"},
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
