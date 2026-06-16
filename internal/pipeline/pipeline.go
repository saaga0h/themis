package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	IssueNumber     int
	CurrentStep     Step
	ReviewCycle     int
	MaxReviewCycles int
	TestFixAttempts map[string]int
	Commits         []string
	StartedAt       time.Time
	StepHistory     []StepResult
}

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

const stateFile = ".themis/state.json"

// SaveState writes state to <dir>/.themis/state.json, creating directories as needed.
func SaveState(dir string, state *PipelineState) error {
	statePath := filepath.Join(dir, stateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	return os.WriteFile(statePath, data, 0o644)
}

// LoadState reads state from <dir>/.themis/state.json.
// Returns nil, nil when the file does not exist (start fresh).
func LoadState(dir string) (*PipelineState, error) {
	statePath := filepath.Join(dir, stateFile)
	data, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	var state PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &state, nil
}
