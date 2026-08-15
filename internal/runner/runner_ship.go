package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/issuespec"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/prompt"
	"github.com/saaga0h/themis/internal/tracker"
)

// runShipStep pushes the branch, invokes the ship agent for a PR description,
// creates the PR, and returns the Result. It also saves final pipeline state and
// removes the review-results.json artifact.
func runShipStep(ctx context.Context, cfg Config, issue *tracker.IssueData, state *pipeline.PipelineState, prof ProfileData, stepStart time.Time, log io.Writer) (*Result, error) {
	var branch string
	var branchErr error
	if cfg.Git != nil {
		branch, branchErr = cfg.Git.CurrentBranch(ctx, cfg.WorkDir)
		if branchErr != nil {
			branch = "main"
		}
	}
	acs := issuespec.ParseCheckboxes(issue.Body)
	base := issue.Ref
	if base == "" {
		base = "main"
	}

	if cfg.Git != nil {
		if branchErr == nil && branch == base {
			return nil, fmt.Errorf("current branch is the base branch — no issue branch was created")
		}
		if n, countErr := cfg.Git.CommitsAheadOfBase(ctx, cfg.WorkDir, base); countErr == nil && n == 0 {
			return nil, fmt.Errorf("no commits on branch %s — nothing to ship", branch)
		}
		if err := cfg.Git.PushBranch(ctx, cfg.WorkDir, branch); err != nil {
			return nil, fmt.Errorf("pushing branch %s: %w", branch, err)
		}
	}

	// The review gate's findings decide ready vs draft deterministically (the
	// runner owns this, not the ship agent). The verdict also seeds the fallback
	// PR body so it carries the gate result even when the ship agent is skipped.
	findings, _ := cfg.ReviewResultsLoader(ctx, cfg.WorkDir)
	draft, verdict := reviewVerdict(findings)
	prBody := buildPRBody(cfg.IssueNumber, issue.Title, acs) + verdict

	shipTmplPath := filepath.Join(cfg.TemplateDir, "ship.md")
	shipTmplContent, readErr := os.ReadFile(shipTmplPath)
	if readErr != nil {
		fmt.Fprintf(log, "warning: ship template read failed: %v — using fallback PR body\n", readErr)
	} else {
		shipArgs := buildTemplateArgs(ctx, cfg, issue, branch, "")
		filteredArgs := filterArgs(string(shipTmplContent), shipArgs)
		substituted, subErr := prompt.Substitute(string(shipTmplContent), filteredArgs)
		if subErr != nil {
			fmt.Fprintf(log, "warning: ship template substitution failed: %v — using fallback PR body\n", subErr)
		} else {
			model := modelForStep(pipeline.StepShip, prof)
			turns := turnsForStep(pipeline.StepShip, cfg.MaxTurns)
			fmt.Fprintf(log, "%s: invoking agent model=%s maxTurns=%d\n", pipeline.StepShip, model, turns)
			shipOpts := agent.InvokeOptions{
				Prompt:       substituted,
				Model:        model,
				MaxTurns:     turns,
				WorkDir:      cfg.WorkDir,
				IssueNumber:  cfg.IssueNumber,
				PipelineStep: pipeline.StepShip.String(),
			}
			if cfg.Git != nil {
				shipOpts.CommitCountFn = cfg.Git.CommitSHAs
			}
			invokeResult, invokeErr := cfg.Invoker.Invoke(ctx, shipOpts)
			if invokeErr != nil {
				fmt.Fprintf(log, "warning: ship agent invocation failed: %v; falling back to buildPRBody\n", invokeErr)
			} else if invokeResult.Stdout != "" {
				prBody = stripCodeFences(invokeResult.Stdout)
			}
		}
	}

	prURL, err := cfg.IssueWriter.CreatePR(ctx, PROptions{
		Title: fmt.Sprintf("Closes #%d — %s", cfg.IssueNumber, issue.Title),
		Body:  prBody,
		Base:  base,
		Head:  branch,
		Draft: draft,
	})
	if err != nil {
		return nil, fmt.Errorf("creating PR: %w", err)
	}
	readiness := "ready"
	if draft {
		readiness = "draft (blocking review findings — needs a maintainer before merge)"
	}
	fmt.Fprintf(log, "%s: PR URL %s [%s]\n", pipeline.StepShip, prURL, readiness)
	verdictLabel := "ready"
	if draft {
		verdictLabel = "draft"
	}
	emitStep(ctx, cfg, log, StepRecord{
		IssueNumber: cfg.IssueNumber,
		RunID:       runID(cfg.IssueNumber, state.StartedAt.Unix()),
		Stage:       pipeline.StepShip.String(),
		Outcome:     "shipped",
		Verdict:     verdictLabel,
		PRURL:       prURL,
		DurationMs:  time.Since(stepStart).Milliseconds(),
	})
	if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
		return nil, fmt.Errorf("saving final state: %w", err)
	}
	if rmErr := os.Remove(filepath.Join(cfg.WorkDir, ".themis", "review-results.json")); rmErr != nil && !os.IsNotExist(rmErr) {
		fmt.Fprintf(log, "warning: removing review-results.json: %v\n", rmErr)
	}
	fmt.Fprintf(log, "%s: done (%dms)\n", pipeline.StepShip, time.Since(stepStart).Milliseconds())
	return &Result{PRURL: prURL}, nil
}
