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

	"github.com/saaga0h/themis/internal/pipeline"
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

// baseBranchForIssue resolves the branch the loop resets to before creating an
// issue's branch — so each issue is cut from the right base and does not stack on
// the previous issue's branch. Provider-agnostic: it prefers the issue's Ref (the
// target branch, which Gitea carries and the Ship step also uses for the PR base;
// GitHub issues have none, leaving it empty), then falls back to the branch the
// factory was invoked on (originBranch), and only then to "main". This removes the
// hardcoded "main" that branched issues from a stale default (see the #112 loop
// failure) and works for any provider without assuming a default-branch name.
func baseBranchForIssue(issue *tracker.IssueData, originBranch string) string {
	if r := strings.TrimPrefix(strings.TrimSpace(issue.Ref), "refs/heads/"); r != "" {
		return r
	}
	if originBranch != "" {
		return originBranch
	}
	return "main"
}

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
	var blockedRerun, blockedManual, blockedConfig int

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
			d := pipeline.Classify(err)
			fmt.Fprintf(out, "issue #%d blocked [%s]: %s\n    → %s\n", issue.Number, d.Category, d.Reason, d.Action)
			blocked++
			switch d.Category {
			case pipeline.BlockRerun:
				blockedRerun++
			case pipeline.BlockConfig:
				blockedConfig++
			default:
				blockedManual++
			}
		} else {
			processed++
		}
	}

	printRunSummary(out, runCounts{processed, blocked, blockedRerun, blockedManual, blockedConfig, skippedDep, skippedTurns}, cfg.DryRun)
	return nil
}

// runCounts tallies a run's outcomes for the end-of-run summary. The blocked
// total is broken down by category (re-run / manual / config) so the operator
// sees at a glance how many failures are worth retrying vs need a human.
type runCounts struct {
	processed, blocked                         int
	blockedRerun, blockedManual, blockedConfig int
	skippedDep, skippedTurns                   int
}

// printRunSummary writes a single end-of-run summary line to out, combining all
// non-zero counts. The processed count is always emitted first; in dry-run mode it
// is labelled "would process" since no issues were actually processed. When any
// issues blocked, the blocked count carries a per-category breakdown.
func printRunSummary(out io.Writer, c runCounts, dryRun bool) {
	processedLabel := "processed"
	if dryRun {
		processedLabel = "would process"
	}
	parts := []string{fmt.Sprintf("%d %s", c.processed, processedLabel)}
	if c.blocked > 0 {
		part := fmt.Sprintf("%d blocked", c.blocked)
		var bd []string
		if c.blockedRerun > 0 {
			bd = append(bd, fmt.Sprintf("%d re-run", c.blockedRerun))
		}
		if c.blockedManual > 0 {
			bd = append(bd, fmt.Sprintf("%d manual", c.blockedManual))
		}
		if c.blockedConfig > 0 {
			bd = append(bd, fmt.Sprintf("%d config", c.blockedConfig))
		}
		if len(bd) > 0 {
			part += " (" + strings.Join(bd, ", ") + ")"
		}
		parts = append(parts, part)
	}
	if c.skippedDep > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (dependency)", c.skippedDep))
	}
	if c.skippedTurns > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (turns)", c.skippedTurns))
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
