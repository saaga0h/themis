package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
)

// At Review, a behavioral AC with no test target (from test-architect's persisted
// mapping) is folded into the review findings as a blocking finding — the
// deterministic AC-coverage check, no LLM grep. Verified end-to-end through Run.
func TestRunner_ReviewStep_UncoveredACBecomesBlockingFinding(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepReview)

	var buf bytes.Buffer
	tDir := makeTemplateDir(t, map[string]string{
		"review.md":      "Review {{ISSUE_NUMBER}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})
	w := &stubIssueWriter{prURL: "https://example.com/pr/ac"}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &recordingInvoker{},
		IssueWriter:  w,
		TemplateDir:  tDir,
		CheckpointFn: noopCheckpoint,
		Logger:       &buf,
		// The review agent wrote empty results; the mapping has one uncovered
		// behavioral AC and one structural AC (covered by a check block, ignored).
		ReviewResultsLoader: func(context.Context, string) ([]review.ReviewFinding, bool) {
			return nil, true
		},
		ACTargetsLoader: func(context.Context, string) ([]review.ACTarget, bool) {
			return []review.ACTarget{
				{Criterion: "returns 200 on valid input", Kind: "behavioral"},
				{Criterion: "no GiteaQuerier remains in cmd", Kind: review.KindStructural},
			}, true
		},
		DocSurfaces: []string{"README.md"}, // not touched -> Docs skipped
		Git:         &fakeGitOps{changedFiles: "internal/x.go", commitsAhead: 1},
	}

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "AC coverage: 1 acceptance criteria have no test target") {
		t.Errorf("the deterministic AC-coverage finding should be logged; log:\n%s", got)
	}
	if !strings.Contains(got, "1 blocking") {
		t.Errorf("an uncovered behavioral AC must count as a blocking finding (structural ignored); log:\n%s", got)
	}
}
