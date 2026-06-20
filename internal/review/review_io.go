package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ReadReviewResults reads .themis/review-results.json from workDir.
// This is the sole filesystem I/O function in the review package; all other
// functions are pure analysis functions operating on ReviewFinding slices.
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
