package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Step constants ---

func TestAllStepsDefined(t *testing.T) {
	steps := []struct {
		name string
		step Step
	}{
		{"Fetch", StepFetch},
		{"Scan", StepScan},
		{"Branch", StepBranch},
		{"TestRed", StepTestRed},
		{"Implement", StepImplement},
		{"Refactor", StepRefactor},
		{"Review", StepReview},
		{"Fix", StepFix},
		{"Docs", StepDocs},
		{"Ship", StepShip},
	}
	if len(steps) != 10 {
		t.Fatalf("expected 10 steps, got %d", len(steps))
	}
	seen := map[Step]string{}
	for _, s := range steps {
		if prev, ok := seen[s.step]; ok {
			t.Errorf("step %v (%s) has the same value as %s", s.step, s.name, prev)
		}
		seen[s.step] = s.name
	}
}

// --- PipelineState struct ---

func TestPipelineStateFields(t *testing.T) {
	ps := PipelineState{
		IssueNumber:       42,
		CurrentStep:       StepFetch,
		TestFixAttempts:   map[string]int{"AC-1": 1},
		ImplementAttempts: 1,
		Commits:           []string{"abc123"},
		StartedAt:         time.Now(),
		StepHistory:       []StepResult{{Success: true}},
	}
	if ps.IssueNumber != 42 {
		t.Error("IssueNumber not stored")
	}
	if ps.CurrentStep != StepFetch {
		t.Error("CurrentStep not stored")
	}
	if ps.ImplementAttempts != 1 {
		t.Error("ImplementAttempts not stored")
	}
	if ps.TestFixAttempts["AC-1"] != 1 {
		t.Error("TestFixAttempts not stored")
	}
	if len(ps.Commits) != 1 {
		t.Error("Commits not stored")
	}
	if len(ps.StepHistory) != 1 {
		t.Error("StepHistory not stored")
	}
}

// --- Advance() happy path: linear, no Fix/Refactor, review never loops ---

func TestAdvanceHappyPath(t *testing.T) {
	happyPath := []struct {
		from Step
		to   Step
	}{
		{StepFetch, StepScan},
		{StepScan, StepBranch},
		{StepBranch, StepTestRed},
		{StepTestRed, StepImplement},
		{StepImplement, StepReview},
		{StepReview, StepDocs},
		{StepDocs, StepShip},
	}

	for _, tc := range happyPath {
		ps := &PipelineState{
			CurrentStep:     tc.from,
			TestFixAttempts: map[string]int{},
		}
		next, err := ps.Advance(StepResult{Success: true})
		if err != nil {
			t.Errorf("%v → Advance(success): unexpected error: %v", tc.from, err)
			continue
		}
		if next != tc.to {
			t.Errorf("%v → Advance(success): got %v, want %v", tc.from, next, tc.to)
		}
	}
}

// A successful TestRed result advances to Implement even when the retry counter
// is already at the maximum — success short-circuits the attempt check, so a
// resumed or rebased branch whose failing tests already exist does not stall on
// the ceiling.
func TestAdvanceTestRedSuccessAdvancesAtMaxAttempts(t *testing.T) {
	ps := &PipelineState{
		CurrentStep:     StepTestRed,
		TestFixAttempts: map[string]int{"tests": 3},
	}
	next, err := ps.Advance(StepResult{Success: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != StepImplement {
		t.Errorf("got %v, want %v", next, StepImplement)
	}
}

// A failed TestRed result at the maximum attempt count still returns the ceiling
// error.
func TestAdvanceTestRedFailureAtMaxAttemptsErrors(t *testing.T) {
	ps := &PipelineState{
		CurrentStep:     StepTestRed,
		TestFixAttempts: map[string]int{"tests": 3},
	}
	if _, err := ps.Advance(StepResult{Success: false, TestACKey: "tests"}); err == nil {
		t.Fatal("expected error when TestRed fails at the maximum attempt count")
	}
}

// --- Implement green gate ---

// A red Implement result re-runs Implement and increments the attempt counter,
// rather than advancing to Review with a broken build.
func TestAdvanceImplementFailureRetries(t *testing.T) {
	ps := &PipelineState{CurrentStep: StepImplement, TestFixAttempts: map[string]int{}}
	next, err := ps.Advance(StepResult{Success: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != StepImplement {
		t.Errorf("got %v, want StepImplement (retry)", next)
	}
	if ps.ImplementAttempts != 1 {
		t.Errorf("expected ImplementAttempts=1, got %d", ps.ImplementAttempts)
	}
}

// A red Implement result at the attempt ceiling returns an error so the issue is
// blocked for a human rather than looping forever.
func TestAdvanceImplementFailureAtMaxErrors(t *testing.T) {
	ps := &PipelineState{CurrentStep: StepImplement, ImplementAttempts: maxGreenGateAttempts, TestFixAttempts: map[string]int{}}
	if _, err := ps.Advance(StepResult{Success: false}); err == nil {
		t.Fatal("expected error when Implement fails at the attempt ceiling")
	}
}

// A green Implement result advances straight to Review (no Refactor step).
func TestAdvanceImplementSuccessAdvancesToReview(t *testing.T) {
	ps := &PipelineState{CurrentStep: StepImplement, TestFixAttempts: map[string]int{}}
	next, err := ps.Advance(StepResult{Success: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != StepReview {
		t.Errorf("got %v, want StepReview", next)
	}
}

// --- Review never loops ---

// Review always proceeds to Docs and never routes to a fix step — its findings
// inform the PR verdict, not control flow. This holds regardless of the result.
func TestAdvanceReviewAlwaysProceedsToDocs(t *testing.T) {
	for _, success := range []bool{true, false} {
		ps := &PipelineState{CurrentStep: StepReview, TestFixAttempts: map[string]int{}}
		next, err := ps.Advance(StepResult{Success: success})
		if err != nil {
			t.Fatalf("success=%v: unexpected error: %v", success, err)
		}
		if next != StepDocs {
			t.Errorf("success=%v: got %v, want StepDocs", success, next)
		}
	}
}

// --- Test-fix attempt limits ---

func TestAdvanceTestFixAttemptRetry(t *testing.T) {
	ps := &PipelineState{
		CurrentStep:     StepTestRed,
		TestFixAttempts: map[string]int{"AC-1": 1},
	}
	next, err := ps.Advance(StepResult{Success: false, TestACKey: "AC-1"})
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}
	if next != StepTestRed {
		t.Errorf("expected StepTestRed for retry, got %v", next)
	}
	if ps.TestFixAttempts["AC-1"] != 2 {
		t.Errorf("expected attempt count 2, got %d", ps.TestFixAttempts["AC-1"])
	}
}

func TestAdvanceTestFixAttemptExceeded(t *testing.T) {
	ps := &PipelineState{
		CurrentStep:     StepTestRed,
		TestFixAttempts: map[string]int{"AC-1": 3},
	}
	_, err := ps.Advance(StepResult{Success: false, TestACKey: "AC-1"})
	if err == nil {
		t.Error("expected error when test-fix attempts exceed 3")
	}
}

// --- SaveState / LoadState ---

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &PipelineState{
		IssueNumber:       7,
		CurrentStep:       StepImplement,
		ImplementAttempts: 1,
		TestFixAttempts:   map[string]int{"AC-1": 2, "AC-2": 0},
		Commits:           []string{"sha1", "sha2"},
		StartedAt:         time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		StepHistory:       []StepResult{{Success: true}, {Success: false}},
	}

	if err := SaveState(dir, original); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadState returned nil with no error")
	}

	if loaded.IssueNumber != original.IssueNumber {
		t.Errorf("IssueNumber: got %d, want %d", loaded.IssueNumber, original.IssueNumber)
	}
	if loaded.CurrentStep != original.CurrentStep {
		t.Errorf("CurrentStep: got %v, want %v", loaded.CurrentStep, original.CurrentStep)
	}
	if loaded.ImplementAttempts != original.ImplementAttempts {
		t.Errorf("ImplementAttempts: got %d, want %d", loaded.ImplementAttempts, original.ImplementAttempts)
	}
	if len(loaded.TestFixAttempts) != len(original.TestFixAttempts) {
		t.Errorf("TestFixAttempts length: got %d, want %d", len(loaded.TestFixAttempts), len(original.TestFixAttempts))
	}
	for k, v := range original.TestFixAttempts {
		if loaded.TestFixAttempts[k] != v {
			t.Errorf("TestFixAttempts[%q]: got %d, want %d", k, loaded.TestFixAttempts[k], v)
		}
	}
	if len(loaded.Commits) != len(original.Commits) {
		t.Errorf("Commits length: got %d, want %d", len(loaded.Commits), len(original.Commits))
	}
	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", loaded.StartedAt, original.StartedAt)
	}
	if len(loaded.StepHistory) != len(original.StepHistory) {
		t.Errorf("StepHistory length: got %d, want %d", len(loaded.StepHistory), len(original.StepHistory))
	}

	// Verify the state file exists at the right path
	statePath := filepath.Join(dir, ".themis", "state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state file not found at %s: %v", statePath, err)
	}
}

func TestLoadStateMissingFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState on missing file should return nil error, got: %v", err)
	}
	if state != nil {
		t.Errorf("LoadState on missing file should return nil state, got: %+v", state)
	}
}

func TestSaveStateWritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	ps := &PipelineState{
		IssueNumber:     1,
		CurrentStep:     StepFetch,
		TestFixAttempts: map[string]int{},
		StartedAt:       time.Now(),
	}
	if err := SaveState(dir, ps); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".themis", "state.json"))
	if err != nil {
		t.Fatalf("could not read state file: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("state.json is not valid JSON: %v", err)
	}
}

func TestLoadStateMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, err := LoadState(dir)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}
