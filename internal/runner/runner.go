package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/prompt"
	"github.com/saaga0h/themis/internal/review"
	"github.com/saaga0h/themis/internal/tracker"
)

// DefaultMaxTurns is the default per-agent turn limit used when no --max-turns
// value is supplied on the CLI. It is the single source of truth for the default.
const DefaultMaxTurns = 250

// ProfileData holds the subset of profile fields the runner needs.
// Concrete values are injected via Config.ProfileLoader; cmd/themis/ translates
// profile.Profile into this struct so the runner does not import internal/profile.
type ProfileData struct {
	ImplementModel string
	ReviewModel    string
}

// GitOps groups the git operations that the runner requires. Concrete
// implementations live in cmd/themis/; tests substitute fakes.
type GitOps interface {
	CheckoutNewBranch(ctx context.Context, dir, name string) error
	Checkout(ctx context.Context, dir, name string) error
	PushBranch(ctx context.Context, dir, branch string) error
	CurrentBranch(ctx context.Context, dir string) (string, error)
	BranchCommitLog(ctx context.Context, dir string) string
	CommitsAheadOfBase(ctx context.Context, dir, base string) (int, error)
	ChangedFiles(ctx context.Context, dir string) string
	CommitSHAs(ctx context.Context, dir string) ([]string, error)
}

// IssueWriter handles issue tracker write operations.
type IssueWriter interface {
	AddLabel(ctx context.Context, number int, label string) error
	RemoveLabel(ctx context.Context, number int, label string) error
	Comment(ctx context.Context, number int, body string) error
	CreatePR(ctx context.Context, opts PROptions) (string, error)
}

// PROptions holds the parameters for creating a pull request.
type PROptions struct {
	Title string
	Body  string
	Base  string
	Head  string
}

// Config holds all dependencies for a pipeline run.
type Config struct {
	WorkDir              string
	IssueNumber          int
	Fetcher              tracker.Fetcher
	Invoker              agent.Invoker
	IssueWriter          IssueWriter
	TemplateDir          string
	CheckpointFn         func(ctx context.Context, step pipeline.Step, workDir string) error
	TestACKey            string
	Git                  GitOps
	ProfileLoader        func(dir string) (ProfileData, error)
	ReviewResultsLoader  func(ctx context.Context, workDir string) ([]review.ReviewFinding, bool)
	Logger               io.Writer
	CodeVersion          string
	MaxTurns             int
}

// Result holds the outcome of a successful pipeline run.
type Result struct {
	PRURL string
}

var agentSteps = map[pipeline.Step]bool{
	pipeline.StepTestRed:   true,
	pipeline.StepImplement: true,
	pipeline.StepRefactor:  true,
	pipeline.StepReview:    true,
	pipeline.StepFix:       true,
	pipeline.StepDocs:      true,
}

var templateFile = map[pipeline.Step]string{
	pipeline.StepTestRed:   "test-red.md",
	pipeline.StepImplement: "implement.md",
	pipeline.StepRefactor:  "refactor.md",
	pipeline.StepReview:    "review.md",
	pipeline.StepFix:       "fix-findings.md",
	pipeline.StepDocs:      "update-docs.md",
}

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
	if state.CurrentStep > pipeline.StepBranch && cfg.Git != nil {
		if branch, brErr := cfg.Git.CurrentBranch(ctx, cfg.WorkDir); brErr == nil {
			if !strings.Contains(branch, strconv.Itoa(cfg.IssueNumber)) {
				fmt.Fprintf(log, "warning: current branch %q does not contain issue number %d\n", branch, cfg.IssueNumber)
			}
		}
	}
	fmt.Fprintf(log, "resuming from step %s\n", state.CurrentStep.String())
	return state
}

// Run executes the full pipeline for the given configuration.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	log := cfg.Logger
	if log == nil {
		log = os.Stderr
	}

	loaded, err := pipeline.LoadState(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	state := validateResumedState(ctx, loaded, cfg, log)
	if state == nil {
		fmt.Fprintf(log, "fresh start\n")
		if rmErr := os.Remove(filepath.Join(cfg.WorkDir, ".themis", "review-results.json")); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(log, "warning: removing review-results.json: %v\n", rmErr)
		}
		state = &pipeline.PipelineState{
			IssueNumber:     cfg.IssueNumber,
			CurrentStep:     pipeline.StepFetch,
			MaxReviewCycles: 2,
			TestFixAttempts: map[string]int{},
			StartedAt:       time.Now(),
			CodeVersion:     cfg.CodeVersion,
		}
	}

	initialReviewCycle := state.ReviewCycle

	issue, err := cfg.Fetcher.Fetch(ctx, cfg.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching issue #%d: %w", cfg.IssueNumber, err)
	}

	var prof ProfileData
	if cfg.ProfileLoader != nil {
		var err error
		prof, err = cfg.ProfileLoader(cfg.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("loading profile: %w", err)
		}
	}
	if prof.ImplementModel == "" {
		prof.ImplementModel = "sonnet"
	}
	if prof.ReviewModel == "" {
		prof.ReviewModel = "sonnet"
	}

	var lastBlockingFindings string

	for {
		step := state.CurrentStep
		stepStart := time.Now()
		fmt.Fprintf(log, "%s: start\n", step)

		// Fetch step: auto-advance (fetch is a best-effort pre-run sync handled by the caller).
		if step == pipeline.StepFetch {
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing step %v: %w", step, err)
			}
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving state at step %v: %w", step, err)
			}
			state.CurrentStep = next
			fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
			continue
		}

		// Scan step: auto-advance.
		if step == pipeline.StepScan {
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing step %v: %w", step, err)
			}
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving state at step %v: %w", step, err)
			}
			state.CurrentStep = next
			fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
			continue
		}

		// Branch step: create issue branch, or checkout if it already exists.
		if step == pipeline.StepBranch {
			branchName := fmt.Sprintf("issue/%d-%s", cfg.IssueNumber, slugify(issue.Title))
			if cfg.Git != nil {
				if err := cfg.Git.CheckoutNewBranch(ctx, cfg.WorkDir, branchName); err != nil {
					if checkoutErr := cfg.Git.Checkout(ctx, cfg.WorkDir, branchName); checkoutErr != nil {
						return nil, fmt.Errorf("branch %s: create failed (%v), checkout failed (%v)", branchName, err, checkoutErr)
					}
					fmt.Fprintf(log, "note: branch %s already exists, checked out existing\n", branchName)
				}
			}
			// Seed review-results.json with empty findings so that Review is
			// non-blocking unless the review agent itself writes blocking findings.
			// This only runs on fresh pipeline starts (resumed runs skip Branch).
			themisDir := filepath.Join(cfg.WorkDir, ".themis")
			if mkErr := os.MkdirAll(themisDir, 0o755); mkErr != nil {
				return nil, fmt.Errorf("creating .themis directory: %w", mkErr)
			}
			if wfErr := os.WriteFile(filepath.Join(themisDir, "review-results.json"),
				[]byte(`{"findings":[]}`), 0o644); wfErr != nil {
				return nil, fmt.Errorf("seeding review-results.json: %w", wfErr)
			}
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing step %v: %w", step, err)
			}
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving state at step %v: %w", step, err)
			}
			state.CurrentStep = next
			fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
			continue
		}

		// Ship step: push branch, invoke agent for PR body, create PR.
		if step == pipeline.StepShip {
			var branch string
			var branchErr error
			if cfg.Git != nil {
				branch, branchErr = cfg.Git.CurrentBranch(ctx, cfg.WorkDir)
				if branchErr != nil {
					branch = "main"
				}
			}
			acs := tracker.ParseCheckboxes(issue.Body)
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

			prBody := buildPRBody(cfg.IssueNumber, issue.Title, acs)

			shipTmplPath := filepath.Join(cfg.TemplateDir, "ship.md")
			shipTmplContent, readErr := os.ReadFile(shipTmplPath)
			if readErr != nil {
				fmt.Fprintf(log, "warning: ship template read failed: %v — using fallback PR body\n", readErr)
			} else {
				shipArgs := buildTemplateArgs(ctx, cfg, issue, branch, state.ReviewCycle, lastBlockingFindings)
				filteredArgs := filterArgs(string(shipTmplContent), shipArgs)
				substituted, subErr := prompt.Substitute(string(shipTmplContent), filteredArgs)
				if subErr != nil {
					fmt.Fprintf(log, "warning: ship template substitution failed: %v — using fallback PR body\n", subErr)
				} else {
					model := modelForStep(step, prof)
					fmt.Fprintf(log, "%s: invoking agent model=%s maxTurns=%d\n", step, model, cfg.MaxTurns)
					shipOpts := agent.InvokeOptions{
						Prompt:       substituted,
						Model:        model,
						MaxTurns:     cfg.MaxTurns,
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
			})
			if err != nil {
				return nil, fmt.Errorf("creating PR: %w", err)
			}
			fmt.Fprintf(log, "%s: PR URL %s\n", step, prURL)
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving final state: %w", err)
			}
			if rmErr := os.Remove(filepath.Join(cfg.WorkDir, ".themis", "review-results.json")); rmErr != nil && !os.IsNotExist(rmErr) {
				fmt.Fprintf(log, "warning: removing review-results.json: %v\n", rmErr)
			}
			fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
			return &Result{PRURL: prURL}, nil
		}

		// Agent steps: load template, substitute, invoke, checkpoint.
		if !agentSteps[step] {
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing unknown step %v: %w", step, err)
			}
			state.CurrentStep = next
			fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
			continue
		}

		tmplPath := filepath.Join(cfg.TemplateDir, templateFile[step])
		tmplContent, err := os.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", tmplPath, err)
		}

		branchName := "main"
		if cfg.Git != nil {
			if b, brErr := cfg.Git.CurrentBranch(ctx, cfg.WorkDir); brErr == nil {
				branchName = b
			}
		}

		allArgs := buildTemplateArgs(ctx, cfg, issue, branchName, state.ReviewCycle, lastBlockingFindings)

		filteredArgs := filterArgs(string(tmplContent), allArgs)

		substituted, err := prompt.Substitute(string(tmplContent), filteredArgs)
		if err != nil {
			return nil, fmt.Errorf("substituting template %s: %w", templateFile[step], err)
		}

		model := modelForStep(step, prof)
		fmt.Fprintf(log, "%s: invoking agent model=%s maxTurns=%d\n", step, model, cfg.MaxTurns)

		opts := agent.InvokeOptions{
			Prompt:       substituted,
			Model:        model,
			MaxTurns:     cfg.MaxTurns,
			WorkDir:      cfg.WorkDir,
			IssueNumber:  cfg.IssueNumber,
			PipelineStep: step.String(),
		}
		if cfg.Git != nil {
			opts.CommitCountFn = cfg.Git.CommitSHAs
		}
		invokeResult, err := cfg.Invoker.Invoke(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("agent invocation at step %v: %w", step, err)
		}

		completionStatus := "completed"
		if !invokeResult.Completed {
			completionStatus = "not completed"
		}
		fmt.Fprintf(log, "%s: agent result: %d commits, %s\n", step, len(invokeResult.CommitsMade), completionStatus)

		if cfg.CheckpointFn != nil {
			if chkErr := cfg.CheckpointFn(ctx, step, cfg.WorkDir); chkErr != nil {
				fmt.Fprintf(log, "%s: checkpoint failed: %v\n", step, chkErr)
				return nil, fmt.Errorf("checkpoint failed after step %v: %w", step, chkErr)
			}
			fmt.Fprintf(log, "%s: checkpoint pass\n", step)
		}

		stepResult := deriveStepResult(ctx, step, invokeResult, cfg)
		if step == pipeline.StepReview {
			loadResults := cfg.ReviewResultsLoader
			if loadResults == nil {
				loadResults = review.ReadReviewResults
			}
			findings, found := loadResults(ctx, cfg.WorkDir)
			if !found {
				fmt.Fprintf(log, "warning: review-results.json not found after review step — treating as blocking\n")
			} else {
				blocking, nonBlocking := review.CountFindingsBySeverity(findings)
				fmt.Fprintf(log, "%s: review findings: %d blocking, %d non-blocking\n", step, blocking, nonBlocking)
				if stepResult.BlockingFindings {
					lastBlockingFindings = review.FormatBlockingFindings(findings)
				}
			}
		}

		next, advErr := state.Advance(stepResult)
		if advErr != nil {
			if strings.Contains(advErr.Error(), "review cycle") && state.ReviewCycle > initialReviewCycle {
				// Cycle limit hit during this run — code works, review has unresolved opinions.
				// Continue to ship so the human can decide via the PR.
				fmt.Fprintf(log, "%s: %v — continuing to ship with unresolved findings\n", step, advErr)
				state.CurrentStep = pipeline.StepDocs
				if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
					return nil, fmt.Errorf("saving state after cycle limit: %w", err)
				}
				fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
				continue
			}
			blockErr := blockIssue(ctx, cfg, advErr)
			if blockErr != nil {
				return nil, fmt.Errorf("blocking issue after %v: %w", advErr, blockErr)
			}
			return nil, advErr
		}

		if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
			return nil, fmt.Errorf("saving state after step %v: %w", step, err)
		}

		state.CurrentStep = next
		fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
	}
}

func deriveStepResult(ctx context.Context, step pipeline.Step, r *agent.InvokeResult, cfg Config) pipeline.StepResult {
	switch step {
	case pipeline.StepTestRed:
		key := cfg.TestACKey
		if key == "" {
			key = "tests"
		}
		if len(r.CommitsMade) > 0 || r.Completed {
			return pipeline.StepResult{Success: true}
		}
		// A resumed or rebased run can reach TestRed with the failing tests
		// already committed on the branch. The agent then correctly makes no new
		// commit, but TestRed's goal — failing tests present before Implement —
		// is already met. Treat that as success rather than scoring a failed
		// attempt, which would otherwise stall the pipeline on the retry ceiling.
		if cfg.Git != nil && branchHasTestFiles(cfg.Git.ChangedFiles(ctx, cfg.WorkDir)) {
			return pipeline.StepResult{Success: true}
		}
		return pipeline.StepResult{Success: false, TestACKey: key}

	case pipeline.StepReview:
		loadResults := cfg.ReviewResultsLoader
		if loadResults == nil {
			loadResults = review.ReadReviewResults
		}
		findings, found := loadResults(ctx, cfg.WorkDir)
		if !found {
			// Missing JSON is the fail-safe: the review step produced no
			// structured result, so treat it as blocking regardless of stdout.
			return pipeline.StepResult{Success: false, BlockingFindings: true}
		}
		blocking := review.DetermineBlockingStatus(findings)
		return pipeline.StepResult{
			Success:          !blocking,
			BlockingFindings: blocking,
		}

	default:
		return pipeline.StepResult{Success: true}
	}
}
