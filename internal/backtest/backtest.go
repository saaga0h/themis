// Package backtest validates a candidate gate against merged history before it
// ships as a green-gate check — the admission-control tool from issue #111: "no
// gate ships without a passing backtest." For each known-good merged commit it
// runs a predicate (the same exit-0-means-pass shape as a check block) and reports
// which commits the predicate would have blocked. A block on known-good history is
// a *false-block* — the signal that a gate is too strict to ship as written.
//
// The git/exec I/O is injected as a Predicate, so the orchestration here is pure
// and unit-tested; cmd/backtest supplies the real git-backed predicate.
package backtest

// Commit identifies one merged-good commit under test.
type Commit struct {
	SHA     string
	Subject string
	Issue   int // parsed from the subject (#N); 0 when absent
}

// Result is the predicate outcome for one commit.
type Result struct {
	Commit  Commit
	Blocked bool  // predicate exited non-zero — a false-block on known-good history
	Err     error // predicate could not be evaluated (distinct from a clean block)
}

// Predicate reports whether a commit passes the candidate check. blocked is true
// when the check fails for that commit (would have blocked it); err is non-nil
// only when the check could not be evaluated at all.
type Predicate func(Commit) (blocked bool, err error)

// Report is the outcome across all tested commits.
type Report struct {
	Results []Result
}

// Run applies predicate to every commit in order and collects the results.
func Run(commits []Commit, predicate Predicate) Report {
	report := Report{Results: make([]Result, 0, len(commits))}
	for _, c := range commits {
		blocked, err := predicate(c)
		report.Results = append(report.Results, Result{Commit: c, Blocked: blocked, Err: err})
	}
	return report
}

// FalseBlocks counts commits the predicate would have blocked.
func (r Report) FalseBlocks() int {
	n := 0
	for _, res := range r.Results {
		if res.Blocked {
			n++
		}
	}
	return n
}

// Errors counts commits whose predicate could not be evaluated.
func (r Report) Errors() int {
	n := 0
	for _, res := range r.Results {
		if res.Err != nil {
			n++
		}
	}
	return n
}

// Total is the number of commits tested.
func (r Report) Total() int { return len(r.Results) }

// Admissible reports whether the candidate gate is safe to ship: it blocked none
// of the known-good history and every commit was evaluable. A false-block or an
// evaluation error means it is not admissible as written.
func (r Report) Admissible() bool {
	return r.FalseBlocks() == 0 && r.Errors() == 0
}
