package runner

// Ship PR-base resolution: the PR must target the resolved base branch the work
// was cut from (Config.BaseBranch), not default an empty issue Ref to "main" —
// otherwise a factory running off an integration branch opens PRs against a
// stale "main".

import (
	"context"
	"testing"
)

// When Config.BaseBranch is set, Ship uses it as the PR base even though the
// issue's Ref is empty (which would otherwise default to "main").
func TestRunner_ShipUsesConfigBaseBranchOverEmptyRef(t *testing.T) {
	w := &stubIssueWriter{prURL: "https://example.com/pr/40"}
	// sampleIssue has an empty Ref; without BaseBranch the base would fall to "main".
	cfg := baseConfig(t, w, &stubFetcher{issue: sampleIssue()}, &stubInvoker{})
	cfg.BaseBranch = "themis-2.0"

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if w.prBaseSeen != "themis-2.0" {
		t.Errorf("PR base: got %q, want %q — Ship must use cfg.BaseBranch, not default an empty Ref to main",
			w.prBaseSeen, "themis-2.0")
	}
}
