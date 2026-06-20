package pipeline

import (
	"fmt"
	"time"
)

type Step int

const (
	StepFetch Step = iota
	StepScan
	StepBranch
	StepTestRed
	StepImplement
	StepRefactor // retained for state-file/resume compatibility; no longer reached
	StepReview
	StepFix // retained for state-file/resume compatibility; no longer reached
	StepDocs
	StepShip
)

func (s Step) String() string {
	names := [...]string{"Fetch", "Scan", "Branch", "TestRed", "Implement", "Refactor", "Review", "Fix", "Docs", "Ship"}
	if int(s) < len(names) {
		return names[s]
	}
	return fmt.Sprintf("Step(%d)", int(s))
}

type StepResult struct {
	Success   bool
	TestACKey string
}

type PipelineState struct {
	IssueNumber       int
	CurrentStep       Step
	TestFixAttempts   map[string]int
	ImplementAttempts int
	Commits           []string
	StartedAt         time.Time
	StepHistory       []StepResult
	CodeVersion       string
}

// maxGreenGateAttempts bounds how many times Implement may re-run when the test
// suite is still red after the step completes. It mirrors the TestRed retry
// ceiling: enough room to recover from a truncated run, low enough that a
// genuinely stuck step blocks the issue for a human instead of looping forever.
const maxGreenGateAttempts = 3

// Advance computes the next pipeline step given the result of the current step.
//
// The pipeline is linear and never loops on review: TestRed → Implement →
// Review → Docs → Ship. Review is a single-pass annotation + gate whose findings
// inform the PR verdict at ship time, not control flow — so there is no Fix or
// Refactor step and no review-cycle machinery. The only retries are the bounded
// green gates on TestRed (failing tests must exist) and Implement (tests pass).
func (ps *PipelineState) Advance(result StepResult) (Step, error) {
	switch ps.CurrentStep {
	case StepTestRed:
		if !result.Success && result.TestACKey != "" {
			if ps.TestFixAttempts[result.TestACKey] >= 3 {
				return 0, fmt.Errorf("test-fix attempts for %q exceeded maximum of 3", result.TestACKey)
			}
			ps.TestFixAttempts[result.TestACKey]++
			return StepTestRed, nil
		}
		ps.recordStep(result)
		ps.CurrentStep = StepImplement
		return StepImplement, nil

	case StepImplement:
		// Green gate: Implement is "done" only when the suite passes. A red or
		// truncated run re-runs Implement, bounded by maxGreenGateAttempts, so a
		// turn-limited implementation is caught here rather than shipped broken.
		if !result.Success {
			if ps.ImplementAttempts >= maxGreenGateAttempts {
				return 0, fmt.Errorf("implement attempts exceeded maximum of %d: test suite still failing", maxGreenGateAttempts)
			}
			ps.ImplementAttempts++
			return StepImplement, nil
		}
		ps.recordStep(result)
		ps.CurrentStep = StepReview
		return StepReview, nil

	case StepReview:
		// Review never loops and never routes to a fix step. Its findings are
		// recorded for the PR verdict (composed at ship), not used for control
		// flow. Always proceed to Docs.
		ps.recordStep(result)
		ps.CurrentStep = StepDocs
		return StepDocs, nil

	default:
		next, err := linearNext(ps.CurrentStep)
		if err != nil {
			return 0, err
		}
		ps.recordStep(result)
		ps.CurrentStep = next
		return next, nil
	}
}

func (ps *PipelineState) recordStep(result StepResult) {
	ps.StepHistory = append(ps.StepHistory, result)
}

// linearNext returns the next step in the default linear sequence, for steps
// without explicit handling in Advance (Implement and Review are explicit).
func linearNext(s Step) (Step, error) {
	switch s {
	case StepFetch:
		return StepScan, nil
	case StepScan:
		return StepBranch, nil
	case StepBranch:
		return StepTestRed, nil
	case StepDocs:
		return StepShip, nil
	default:
		return 0, fmt.Errorf("no linear transition defined for step %v", s)
	}
}
