package pipeline

import "time"

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

func (ps *PipelineState) Advance(result StepResult) (Step, error) {
	panic("not implemented")
}

func SaveState(dir string, state *PipelineState) error {
	panic("not implemented")
}

func LoadState(dir string) (*PipelineState, error) {
	panic("not implemented")
}
