package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/saaga0h/themis/internal/tracker"
)

type runArgs struct {
	provider  string
	dryRun    bool
	maxTurns  int
	templates string
}

// parseRunArgs parses [--provider github|gitea] [--dry-run] [--max-turns N] [--templates DIR] from args.
func parseRunArgs(args []string) (runArgs, error) {
	provider := "github"
	dryRun := false
	maxTurns := defaultMaxTurns
	templates := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--provider requires a value")
			}
			i++
			provider = args[i]
		case "--templates":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--templates requires a value")
			}
			i++
			templates = args[i]
		case "--dry-run":
			dryRun = true
		case "--max-turns":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--max-turns requires a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return runArgs{}, fmt.Errorf("--max-turns must be an integer, got %q", args[i])
			}
			if n <= 0 {
				return runArgs{}, fmt.Errorf("--max-turns must be a positive integer, got %d", n)
			}
			maxTurns = n
		default:
			return runArgs{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	switch provider {
	case "github", "gitea":
	default:
		return runArgs{}, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}

	return runArgs{provider: provider, dryRun: dryRun, maxTurns: maxTurns, templates: templates}, nil
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

var dependsOnRE = regexp.MustCompile(`(?i)depends on #(\d+)`)

func runLoop(ctx context.Context, cfg loopConfig) error {
	out := cfg.Logger
	if out == nil {
		out = io.Discard
	}

	issues, err := cfg.Querier.ListReadyIssues(ctx)
	if err != nil {
		return fmt.Errorf("listing ready issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Fprintf(out, "no ready-for-agent issues found\n")
		return nil
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})

	var processed, blocked, skippedDep, skippedTurns int

	for i, issue := range issues {
		if m := dependsOnRE.FindStringSubmatch(issue.Body); m != nil {
			depNum, _ := strconv.Atoi(m[1])
			open, err := cfg.Querier.IsOpen(ctx, depNum)
			if err != nil {
				fmt.Fprintf(out, "skipping issue #%d: dependency check error for #%d: %v\n", issue.Number, depNum, err)
				skippedDep++
				continue
			}
			if open {
				fmt.Fprintf(out, "skipping issue #%d: depends on open issue #%d\n", issue.Number, depNum)
				skippedDep++
				continue
			}
		}

		if cfg.Turns.RemainingFraction() < 0.10 {
			fmt.Fprintf(out, "insufficient turns remaining\n")
			skippedTurns = len(issues) - i
			break
		}

		if cfg.DryRun {
			fmt.Fprintf(out, "would process issue #%d: %s\n", issue.Number, issue.Title)
			processed++
			continue
		}

		if err := cfg.RunFn(ctx, issue); err != nil {
			fmt.Fprintf(out, "issue #%d blocked: %v\n", issue.Number, err)
			blocked++
		} else {
			processed++
		}
	}

	printRunSummary(out, processed, blocked, skippedDep, skippedTurns, cfg.DryRun)
	return nil
}

// printRunSummary writes a single end-of-run summary line to out, combining all
// non-zero counts. The processed count is always emitted first; in dry-run mode it
// is labelled "would process" since no issues were actually processed.
func printRunSummary(out io.Writer, processed, blocked, skippedDep, skippedTurns int, dryRun bool) {
	processedLabel := "processed"
	if dryRun {
		processedLabel = "would process"
	}
	parts := []string{fmt.Sprintf("%d %s", processed, processedLabel)}
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", blocked))
	}
	if skippedDep > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (dependency)", skippedDep))
	}
	if skippedTurns > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (turns)", skippedTurns))
	}
	fmt.Fprintf(out, "run summary: %s\n", strings.Join(parts, ", "))
}

// envTurns reads the remaining turn fraction from THEMIS_TURNS_REMAINING_FRACTION.
// When unset, it returns 1.0 so the loop runs unrestricted.
type envTurns struct{}

func (e *envTurns) RemainingFraction() float64 {
	s := os.Getenv("THEMIS_TURNS_REMAINING_FRACTION")
	if s == "" {
		return 1.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 1.0
	}
	if f > 1.0 {
		return 1.0
	}
	return f
}

// giteaClientTimeout bounds the Gitea HTTP clients constructed in main.go
// (issue writer and tracker queriers). The querier/fetcher implementations now
// live in internal/tracker; this consumer-side constant is passed into their
// constructors.
const giteaClientTimeout = 30 * time.Second
