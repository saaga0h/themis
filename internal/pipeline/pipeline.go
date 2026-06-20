package pipeline

import (
	"errors"
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
	StepRefactor
	StepReview
	StepFix
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

type Round3Trigger int

const (
	TriggerNone Round3Trigger = iota
	TriggerSecurity
	TriggerPublicAPI
	TriggerNumerical
	TriggerContextArtifact
)

type StepResult struct {
	Success          bool
	BlockingFindings bool
	Round3Trigger    Round3Trigger
	TestACKey        string
}

type PipelineState struct {
	IssueNumber       int
	CurrentStep       Step
	ReviewCycle       int
	MaxReviewCycles   int
	TestFixAttempts   map[string]int
	ImplementAttempts int
	FixAttempts       int
	Commits           []string
	StartedAt         time.Time
	StepHistory       []StepResult
	CodeVersion       string
}

// maxGreenGateAttempts bounds how many times Implement or Fix may re-run when the
// test suite is still red after the step completes. It mirrors the TestRed
// retry ceiling: enough room to recover from a truncated run, low enough that a
// genuinely stuck step blocks the issue for a human instead of looping forever.
const maxGreenGateAttempts = 3

// Advance computes the next pipeline step given the result of the current step.
// It encodes review-cycle limits and round-3 gate logic deterministically.
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
		// Implement is "done" only when the suite is green (Success). A red or
		// truncated run re-runs Implement, bounded by maxGreenGateAttempts, so a
		// turn-limited implementation is caught here rather than wasting a review
		// cycle downstream.
		if !result.Success {
			if ps.ImplementAttempts >= maxGreenGateAttempts {
				return 0, fmt.Errorf("implement attempts exceeded maximum of %d: test suite still failing", maxGreenGateAttempts)
			}
			ps.ImplementAttempts++
			return StepImplement, nil
		}
		ps.recordStep(result)
		ps.CurrentStep = StepRefactor
		return StepRefactor, nil

	case StepFix:
		// Fix carries the same green gate as Implement: re-running blocking-finding
		// fixes must leave the suite green before the next review, otherwise Review
		// burns a cycle on code that no longer compiles or passes.
		if !result.Success {
			if ps.FixAttempts >= maxGreenGateAttempts {
				return 0, fmt.Errorf("fix attempts exceeded maximum of %d: test suite still failing", maxGreenGateAttempts)
			}
			ps.FixAttempts++
			return StepFix, nil
		}
		ps.recordStep(result)
		ps.CurrentStep = StepReview
		return StepReview, nil

	case StepReview:
		if result.BlockingFindings {
			if err := ps.checkReviewCycleLimit(result.Round3Trigger); err != nil {
				return 0, err
			}
			ps.ReviewCycle++
			ps.recordStep(result)
			ps.CurrentStep = StepFix
			return StepFix, nil
		}
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

func (ps *PipelineState) maxReviewCycles() int {
	if ps.MaxReviewCycles == 0 {
		return 2
	}
	return ps.MaxReviewCycles
}

func (ps *PipelineState) checkReviewCycleLimit(trigger Round3Trigger) error {
	if ps.ReviewCycle >= 3 {
		return errors.New("review cycle 3 exhausted: blocking findings remain after maximum cycles")
	}
	if ps.ReviewCycle >= ps.maxReviewCycles() && trigger == TriggerNone {
		return fmt.Errorf("review cycle limit %d reached with blocking findings and no round-3 trigger", ps.maxReviewCycles())
	}
	return nil
}

func (ps *PipelineState) recordStep(result StepResult) {
	ps.StepHistory = append(ps.StepHistory, result)
}

// linearNext returns the next step in the default linear sequence.
func linearNext(s Step) (Step, error) {
	switch s {
	case StepFetch:
		return StepScan, nil
	case StepScan:
		return StepBranch, nil
	case StepBranch:
		return StepTestRed, nil
	case StepImplement:
		return StepRefactor, nil
	case StepRefactor:
		return StepReview, nil
	case StepFix:
		return StepReview, nil
	case StepDocs:
		return StepShip, nil
	default:
		return 0, fmt.Errorf("no linear transition defined for step %v", s)
	}
}
