package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/tracker"
)

// validateResumedState checks the loaded state against cfg and returns the
// state to resume from, or nil to signal a fresh start.
func validateResumedState(ctx context.Context, state *pipeline.PipelineState, cfg Config, log io.Writer) *pipeline.PipelineState {
	if state == nil {
		return nil
	}
	if state.IssueNumber != cfg.IssueNumber {
		fmt.Fprintf(log, "warning: state file is for issue #%d, not #%d — starting fresh\n", state.IssueNumber, cfg.IssueNumber)
		return nil
	}
	if cfg.CodeVersion != "" && state.CodeVersion != "" && state.CodeVersion != cfg.CodeVersion {
		fmt.Fprintf(log, "warning: state was created by version %s, current version is %s\n", state.CodeVersion, cfg.CodeVersion)
	}
	// The issue branch is restored after the issue is fetched (see Run) — the
	// branch name needs the title, which isn't available here.
	fmt.Fprintf(log, "resuming from step %s (attempt %d)\n", state.CurrentStep.String(), attemptCountForStep(state, cfg))
	return state
}

// attemptCountForStep reports how many attempts have already been spent at
// the state's current step, sourced from the existing per-step counters
// (TestFixAttempts/ImplementAttempts) — resume logging must not introduce a
// third attempt-tracking mechanism alongside those.
func attemptCountForStep(state *pipeline.PipelineState, cfg Config) int {
	switch state.CurrentStep {
	case pipeline.StepImplement:
		return state.ImplementAttempts
	case pipeline.StepTestRed:
		return state.TestFixAttempts[cfg.TestACKey]
	default:
		return 0
	}
}

// newFreshState builds the PipelineState a fresh (non-resumed) run starts
// from. Shared by the top-level fresh-start path and the branch-absent reset
// (issue #56 AC6) so both discard stale attempt counters identically.
func newFreshState(cfg Config) *pipeline.PipelineState {
	return &pipeline.PipelineState{
		IssueNumber:     cfg.IssueNumber,
		CurrentStep:     pipeline.StepFetch,
		TestFixAttempts: map[string]int{},
		StartedAt:       time.Now(),
		CodeVersion:     cfg.CodeVersion,
	}
}

// isBranchAbsentErr reports whether a checkout failure's message indicates
// the target branch/ref simply does not exist (e.g. deleted between a crash
// and a resumed restart) rather than some other failure (network, auth,
// permissions) that must still surface as an error.
func isBranchAbsentErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "did not match any file") || strings.Contains(msg, "pathspec")
}

// resumeWorkspace restores the issue branch that the Branch step (skipped on
// resume) would otherwise have checked out, resets state to a fresh start if
// that branch no longer exists, and — for a genuinely resumed run — cleans a
// working tree left dirty by a crashed step before the next agent invocation.
func resumeWorkspace(ctx context.Context, cfg Config, issue *tracker.IssueData, state *pipeline.PipelineState, resumed bool, log io.Writer) (*pipeline.PipelineState, error) {
	// On resume past Branch, the Branch step (which creates/checks out the issue
	// branch) is skipped, and the caller may have left us on the base branch (the
	// run loop checks out main before each issue). Restore the issue branch so no
	// step — Ship's push or an agent's commits — ever runs on the base branch.
	if state.CurrentStep > pipeline.StepBranch && cfg.Git != nil {
		branch := issueBranchName(cfg.IssueNumber, issue.Title)
		if cur, curErr := cfg.Git.CurrentBranch(ctx, cfg.WorkDir); curErr != nil || cur != branch {
			if coErr := cfg.Git.Checkout(ctx, cfg.WorkDir, branch); coErr != nil {
				if !isBranchAbsentErr(coErr) {
					return nil, fmt.Errorf("resuming issue #%d at step %s: cannot check out its branch %q (created by the Branch step): %w", cfg.IssueNumber, state.CurrentStep, branch, coErr)
				}
				// The issue branch itself is gone (e.g. deleted between a crash
				// and this restart) — the saved state's step/attempt counters no
				// longer describe anything recoverable, so reset to a fresh
				// start instead of failing the run outright.
				fmt.Fprintf(log, "resume: issue branch %s not found (%v) — resetting to a fresh start\n", branch, coErr)
				if rmErr := os.Remove(filepath.Join(cfg.WorkDir, ".themis", "review-results.json")); rmErr != nil && !os.IsNotExist(rmErr) {
					fmt.Fprintf(log, "warning: removing review-results.json: %v\n", rmErr)
				}
				state = newFreshState(cfg)
				resumed = false
			} else {
				fmt.Fprintf(log, "resume: checked out issue branch %s\n", branch)
			}
		}
	}

	// A crashed agent step can leave uncommitted or untracked changes behind;
	// resuming must not hand the next step's agent a workspace still carrying
	// the previous, interrupted attempt's stray changes. Fresh runs never
	// clean — there is nothing to recover from.
	if resumed && cfg.Git != nil {
		if clean, ctErr := cfg.Git.WorkingTreeClean(ctx, cfg.WorkDir); ctErr == nil && !clean {
			n, clErr := cfg.Git.CleanWorkingTree(ctx, cfg.WorkDir)
			if clErr != nil {
				return nil, fmt.Errorf("cleaning working tree on resume: %w", clErr)
			}
			fmt.Fprintf(log, "resume: dirty working tree (%d files) — cleaning\n", n)
		}
	}

	return state, nil
}
