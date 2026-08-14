package backtest

import (
	"errors"
	"testing"
)

func commits(n int) []Commit {
	cs := make([]Commit, n)
	for i := range cs {
		cs[i] = Commit{SHA: string(rune('a' + i)), Subject: "feat: x", Issue: i + 1}
	}
	return cs
}

// A predicate that passes every commit is admissible — zero false-blocks, zero
// errors — which is the shape a shippable gate must have.
func TestRun_AllPass_Admissible(t *testing.T) {
	rep := Run(commits(3), func(Commit) (bool, error) { return false, nil })
	if rep.Total() != 3 {
		t.Fatalf("Total = %d, want 3", rep.Total())
	}
	if rep.FalseBlocks() != 0 || rep.Errors() != 0 {
		t.Errorf("expected 0 false-blocks/0 errors, got %d/%d", rep.FalseBlocks(), rep.Errors())
	}
	if !rep.Admissible() {
		t.Error("a predicate that passes all known-good history must be admissible")
	}
}

// A single false-block on known-good history disqualifies the gate.
func TestRun_OneFalseBlock_NotAdmissible(t *testing.T) {
	rep := Run(commits(3), func(c Commit) (bool, error) { return c.Issue == 2, nil })
	if rep.FalseBlocks() != 1 {
		t.Errorf("FalseBlocks = %d, want 1", rep.FalseBlocks())
	}
	if rep.Admissible() {
		t.Error("a gate that blocks any known-good commit must NOT be admissible")
	}
	// The blocked result must be the one flagged.
	for _, res := range rep.Results {
		if res.Commit.Issue == 2 && !res.Blocked {
			t.Error("commit #2 should be recorded as blocked")
		}
	}
}

// An evaluation error is not a clean block, but it still makes the gate
// inadmissible — an un-evaluable candidate can't be trusted to ship.
func TestRun_EvaluationError_NotAdmissible(t *testing.T) {
	boom := errors.New("git failed")
	rep := Run(commits(2), func(c Commit) (bool, error) {
		if c.Issue == 1 {
			return false, boom
		}
		return false, nil
	})
	if rep.Errors() != 1 {
		t.Errorf("Errors = %d, want 1", rep.Errors())
	}
	if rep.FalseBlocks() != 0 {
		t.Errorf("an evaluation error must not be counted as a false-block; got %d", rep.FalseBlocks())
	}
	if rep.Admissible() {
		t.Error("a candidate with an un-evaluable commit must not be admissible")
	}
}

// Results preserve input order and count, so the report maps 1:1 to the history.
func TestRun_PreservesOrderAndCount(t *testing.T) {
	in := commits(4)
	rep := Run(in, func(Commit) (bool, error) { return false, nil })
	if len(rep.Results) != len(in) {
		t.Fatalf("Results len = %d, want %d", len(rep.Results), len(in))
	}
	for i, res := range rep.Results {
		if res.Commit.SHA != in[i].SHA {
			t.Errorf("result %d SHA = %q, want %q (order not preserved)", i, res.Commit.SHA, in[i].SHA)
		}
	}
}

// An empty history is vacuously admissible (nothing to block).
func TestRun_EmptyHistory_Admissible(t *testing.T) {
	rep := Run(nil, func(Commit) (bool, error) { return true, nil })
	if rep.Total() != 0 || !rep.Admissible() {
		t.Errorf("empty history: Total=%d Admissible=%v, want 0/true", rep.Total(), rep.Admissible())
	}
}
