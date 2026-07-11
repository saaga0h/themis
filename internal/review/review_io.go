package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReadReviewResults reads .themis/review-results.json from workDir.
// The Read/Write functions here are the review package's only filesystem access;
// every other function is a pure analysis function operating on the loaded slices.
// ctx is accepted per convention but os.ReadFile has no context-aware variant.
func ReadReviewResults(_ context.Context, workDir string) ([]ReviewFinding, bool) {
	path := filepath.Join(workDir, ".themis", "review-results.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rr ReviewResults
	if err := json.Unmarshal(data, &rr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: review-results.json parse error: %v\n", err)
		return nil, false
	}
	return rr.Findings, true
}

// ReadACTargets reads .themis/ac-targets.json from workDir — the AC-to-test-target
// mapping test-architect persists during TestRed. Absent or malformed yields
// (nil, false), the same graceful-degradation contract as ReadReviewResults, so a
// run without the artifact simply produces no AC-coverage findings.
func ReadACTargets(_ context.Context, workDir string) ([]ACTarget, bool) {
	path := filepath.Join(workDir, ".themis", "ac-targets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var at ACTargets
	if err := json.Unmarshal(data, &at); err != nil {
		return nil, false
	}
	return at.ACs, true
}

// WriteReviewResults writes findings to .themis/review-results.json in workDir,
// replacing any existing file. The runner uses it to persist the deterministic
// AC-coverage findings it merges into the review agent's results, so the PR
// verdict and pr-composer see the full set.
func WriteReviewResults(workDir string, findings []ReviewFinding) error {
	dir := filepath.Join(workDir, ".themis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(ReviewResults{Findings: findings})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "review-results.json"), data, 0o644)
}
