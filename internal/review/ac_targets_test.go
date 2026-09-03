package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A behavioral AC with no test target is flagged high; a covered behavioral AC and
// a structural AC (covered by a check block, not a test) are not.
func TestUncoveredACFindings_FlagsBehavioralWithoutTargets(t *testing.T) {
	targets := []ACTarget{
		{Criterion: "returns 400 on invalid input", Kind: "behavioral", Targets: []string{"TestInvalid400"}},
		{Criterion: "returns 200 on valid input", Kind: "behavioral", Targets: nil},
		{Criterion: "no GiteaQuerier remains in cmd", Kind: KindStructural, Targets: nil},
	}
	got := UncoveredACFindings(targets)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding (the uncovered behavioral AC), got %d: %+v", len(got), got)
	}
	if got[0].Severity != "high" {
		t.Errorf("uncovered-AC finding severity = %q, want high", got[0].Severity)
	}
	if !strings.Contains(got[0].Description, "returns 200 on valid input") {
		t.Errorf("finding should name the uncovered AC, got %q", got[0].Description)
	}
}

func TestUncoveredACFindings_AllCoveredOrStructural_NoFindings(t *testing.T) {
	targets := []ACTarget{
		{Criterion: "a", Kind: "behavioral", Targets: []string{"TestA"}},
		{Criterion: "b removed", Kind: KindStructural},
	}
	if got := UncoveredACFindings(targets); len(got) != 0 {
		t.Errorf("expected no findings, got %+v", got)
	}
}

func TestReadACTargets_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	themis := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themis, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"acs":[{"criterion":"x","kind":"behavioral","targets":["TestX"]},{"criterion":"y removed","kind":"structural","targets":[]}]}`
	if err := os.WriteFile(filepath.Join(themis, "ac-targets.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadACTargets(context.Background(), dir)
	if !ok {
		t.Fatal("expected ok reading a valid ac-targets.json")
	}
	if len(got) != 2 || got[0].Criterion != "x" || len(got[0].Targets) != 1 || got[0].Targets[0] != "TestX" || got[1].Kind != KindStructural {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestReadACTargets_AbsentIsGraceful(t *testing.T) {
	if got, ok := ReadACTargets(context.Background(), t.TempDir()); ok || got != nil {
		t.Errorf("absent artifact must yield (nil, false), got (%+v, %v)", got, ok)
	}
}
