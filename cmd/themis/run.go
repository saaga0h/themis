package main

import (
	"context"
	"io"

	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

type runArgs struct {
	provider string
	dryRun   bool
}

// parseRunArgs parses [--provider github|gitea] [--dry-run] from args.
func parseRunArgs(args []string) (runArgs, error) {
	return runArgs{}, nil
}

// IssueQuerier lists issues ready for processing and checks individual issue state.
type IssueQuerier interface {
	ListReadyIssues(ctx context.Context) ([]*tracker.IssueData, error)
	IsOpen(ctx context.Context, number int) (bool, error)
}

// TurnTracker reports the fraction of agentic turns remaining (0.0–1.0).
type TurnTracker interface {
	RemainingFraction() float64
}

type loopConfig struct {
	Querier IssueQuerier
	RunFn   func(ctx context.Context, issue *tracker.IssueData) error
	DryRun  bool
	Turns   TurnTracker
	Logger  io.Writer
}

func runLoop(ctx context.Context, cfg loopConfig) error {
	return nil
}
