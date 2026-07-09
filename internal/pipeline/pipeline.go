package pipeline

import (
	"errors"
	"fmt"
	"strings"
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
	// Completed is true when the agent signalled completion, false when it ran
	// out of turns. Progressed is true when the attempt made at least one commit.
	// Together they let the Implement green gate tell a converging run (retry may
	// finish it) from a stalled one (retrying the full budget won't help).
	Completed  bool
	Progressed bool
}

// BlockCategory classifies why a run stopped so the operator can act without
// reading the logs or having telemetry: RE-RUN (transient, or the step was
// progressing — try again or raise the budget), MANUAL (the agent tried and
// cannot converge — a human must look), CONFIG (a setup problem to fix first).
type BlockCategory string

const (
	BlockRerun  BlockCategory = "RE-RUN"
	BlockManual BlockCategory = "MANUAL"
	BlockConfig BlockCategory = "CONFIG"
)

// BlockError is a terminal pipeline error carrying an operator-facing diagnosis:
// the category, a one-line reason, and the recommended next action. The verify
// output, stack traces, and other detail stay in the log — this is the decision
// summary, enough to choose re-run vs config vs manual.
type BlockError struct {
	Category BlockCategory
	Reason   string
	Action   string
	Err      error // underlying cause, for the log and %w chains
}

func (e *BlockError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason
}

func (e *BlockError) Unwrap() error { return e.Err }

// Classify returns the operator-facing diagnosis for a terminal error. A
// *BlockError is returned as-is (the producer already classified it); any other
// error is classified heuristically so the operator still gets a category and an
// action rather than a bare message. The default is MANUAL — the safe direction,
// since an unclassified failure is one a human should look at.
func Classify(err error) *BlockError {
	var be *BlockError
	if errors.As(err, &be) {
		return be
	}
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "identity not configured") || strings.Contains(low, "token") || strings.Contains(low, "dubious ownership"):
		return &BlockError{Category: BlockConfig, Reason: err.Error(), Action: "fix the reported setup problem, then re-run", Err: err}
	case strings.Contains(low, "push") || strings.Contains(low, "connection") || strings.Contains(low, "network") || strings.Contains(low, "timeout") || strings.Contains(low, "rate limit"):
		return &BlockError{Category: BlockRerun, Reason: err.Error(), Action: "looks transient/infrastructure — re-run once it's resolved", Err: err}
	default:
		return &BlockError{Category: BlockManual, Reason: err.Error(), Action: "inspect the issue branch and run log; fix by hand (interactively with an AI is fine) or correct the issue, then re-run", Err: err}
	}
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
				return 0, &BlockError{
					Category: BlockManual,
					Reason:   fmt.Sprintf("TestRed could not produce a stable failing test for %q after 3 attempts", result.TestACKey),
					Action:   "the acceptance criterion likely needs clarification; refine the AC or write the test by hand, then re-run",
					Err:      fmt.Errorf("test-fix attempts for %q exceeded maximum of 3", result.TestACKey),
				}
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
			// No-progress stall: the agent hit the turn limit (no completion
			// signal) and made no commit this attempt, yet the suite is still red.
			// Re-running the full budget won't converge — this is the #58 signature.
			// Bail after this one round rather than grinding maxGreenGateAttempts.
			if !result.Completed && !result.Progressed {
				return 0, &BlockError{
					Category: BlockManual,
					Reason:   "Implement hit the turn limit with no commits and the verify suite still failing — the agent is not converging",
					Action:   "a re-run will hit the same wall; inspect the work-in-progress on the issue branch and the verify output in the log, then finish by hand (interactively with an AI is fine) or re-scope the issue",
					Err:      fmt.Errorf("implement stalled: turn limit reached with no commit progress and verify still failing"),
				}
			}
			if ps.ImplementAttempts >= maxGreenGateAttempts {
				// Exhausted the bounded retry. A still-progressing run (committed
				// but out of turns) may finish with a re-run or a bigger budget; a
				// completed-but-red run needs a human to resolve the failing verify.
				if !result.Completed && result.Progressed {
					return 0, &BlockError{
						Category: BlockRerun,
						Reason:   fmt.Sprintf("Implement was still making progress but exhausted %d attempts without a green suite", maxGreenGateAttempts),
						Action:   "re-run to resume from the branch, or raise --max-turns to give the step a larger budget",
						Err:      fmt.Errorf("implement attempts exceeded maximum of %d: still progressing", maxGreenGateAttempts),
					}
				}
				return 0, &BlockError{
					Category: BlockManual,
					Reason:   fmt.Sprintf("Implement completed but the verify suite would not pass after %d attempts", maxGreenGateAttempts),
					Action:   "inspect the failing verify command in the log and the issue branch; the failure needs a human fix (or an issue/AC correction)",
					Err:      fmt.Errorf("implement attempts exceeded maximum of %d: test suite still failing", maxGreenGateAttempts),
				}
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
