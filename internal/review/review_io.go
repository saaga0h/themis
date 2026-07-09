package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ReadReviewResults reads .themis/review-results.json from workDir.
// This and ReadACTargets are the only filesystem I/O in the review package; every
// other function is a pure analysis function operating on the loaded slices.
// ctx is accepted per convention but os.ReadFile has no context-aware variant.
func ReadReviewResults(_ context.Context, workDir string) ([]ReviewFinding, bool) {
	path := filepath.Join(workDir, ".themis", "review-results.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rr ReviewResults
	if err := json.Unmarshal(data, &rr); err != nil {
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
