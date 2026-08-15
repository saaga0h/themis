package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	DiffLineCount(ctx context.Context, dir string) int
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
	// Draft opens the PR as a draft / work-in-progress (not mergeable as-is).
	// The factory sets this when the review gate found blocking findings, so a
	// human must resolve them before merge.
	Draft bool
}

// Config holds all dependencies for a pipeline run.
type Config struct {
	WorkDir     string
	IssueNumber int
	// BaseBranch is the branch the issue was cut from and the branch its PR must
	// target — the resolved base (issue Ref → originating branch → "main"), the
	// same value the work is based on. The Ship step uses it as the PR base so the
	// PR never targets a stale default like "main" when the factory runs off an
	// integration branch. When empty (e.g. tests), Ship falls back to the issue's
	// Ref, then "main".
	BaseBranch          string
	Fetcher             tracker.Fetcher
	Invoker             agent.Invoker
	IssueWriter         IssueWriter
	TemplateDir         string
	CheckpointFn        func(ctx context.Context, step pipeline.Step, workDir string) error
	TestACKey           string
	Git                 GitOps
	ProfileLoader       func(dir string) (ProfileData, error)
	ReviewResultsLoader func(ctx context.Context, workDir string) ([]review.ReviewFinding, bool)
	// ACTargetsLoader reads test-architect's AC-to-test-target mapping
	// (.themis/ac-targets.json). At Review the runner turns any behavioral AC with
	// no target into a blocking finding — the deterministic AC-coverage check that
	// replaces the review agent's grep. Defaults to review.ReadACTargets.
	ACTargetsLoader func(ctx context.Context, workDir string) ([]review.ACTarget, bool)
	// TestRunner runs the project's test suite in workDir and reports whether it
	// passed, plus the captured output for diagnostics. It is the GREEN gate for
	// the Implement and Fix steps: a step that committed but left tests red is
	// retried rather than advanced. When nil, those steps advance on commit alone
	// (legacy behaviour; used by tests that do not exercise the gate).
	TestRunner func(ctx context.Context, dir string) (passed bool, output string)
	// StandardsDocs are the project's authoritative doc paths (relative to
	// WorkDir) that agents read for coding standards, terminology, and
	// architecture. Declared per-project in .themis/workflow.yaml; the factory is
	// stack-agnostic and hardcodes none of these. Surfaced as {{STANDARDS_DOCS}}.
	StandardsDocs []string
	// DocSurfaces are path globs (relative to WorkDir) that trigger the Docs
	// step: the step is skipped (no agent spawned) when the change touches none
	// of them. Empty means Docs always runs. Declared per-project in
	// .themis/workflow.yaml.
	DocSurfaces []string
	Logger      io.Writer
	CodeVersion string
	MaxTurns    int
	// Emitter receives per-step StepRecords (the factory's own diagnostic
	// narrative). Nil means no emission. Wired by cmd/themis to an OTLP sink when
	// OTEL_* env is set; nil otherwise (and in tests).
	Emitter Emitter
	// EmitterShutdown flushes the Emitter at the end of the run. Run defers it
	// (bounded), so the async batch exporter delivers the final records. Nil when
	// there is no Emitter.
	EmitterShutdown func(context.Context) error
	// Scrub redacts secrets and sensitive infrastructure values from free-text
	// that leaves the process — block comments posted to the tracker and the
	// StepRecord detail/verify-output sent to the sink. Nil means no scrubbing
	// (tests); cmd/themis wires a scrubber built from the token + Gitea host.
	Scrub func(string) string
}

// Result holds the outcome of a successful pipeline run.
type Result struct {
	PRURL string
}

var agentSteps = map[pipeline.Step]bool{
	pipeline.StepTestRed:   true,
	pipeline.StepImplement: true,
	pipeline.StepReview:    true,
	pipeline.StepDocs:      true,
}

var templateFile = map[pipeline.Step]string{
	pipeline.StepTestRed:   "test-red.md",
	pipeline.StepImplement: "implement.md",
	pipeline.StepReview:    "review.md",
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
	// The issue branch is restored after the issue is fetched (see Run) — the
	// branch name needs the title, which isn't available here.
	fmt.Fprintf(log, "resuming from step %s\n", state.CurrentStep.String())
	return state
}

// Run executes the full pipeline for the given configuration.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	log := cfg.Logger
	if log == nil {
		log = os.Stderr
	}
	// Flush the diagnostic emitter on the way out (bounded), so the async batch
	// exporter delivers the final records even on an early return. Best-effort.
	if cfg.EmitterShutdown != nil {
		defer func() {
			sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = cfg.EmitterShutdown(sctx)
		}()
	}
	// Normalize ReviewResultsLoader once so step-processing code never needs to
	// nil-check it. cmd/themis injects review.ReadReviewResults at construction
	// time; this default covers callers (e.g. tests) that omit the field.
	if cfg.ReviewResultsLoader == nil {
		cfg.ReviewResultsLoader = review.ReadReviewResults
	}
	if cfg.ACTargetsLoader == nil {
		cfg.ACTargetsLoader = review.ReadACTargets
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
			TestFixAttempts: map[string]int{},
			StartedAt:       time.Now(),
			CodeVersion:     cfg.CodeVersion,
		}
	}

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

	// On resume past Branch, the Branch step (which creates/checks out the issue
	// branch) is skipped, and the caller may have left us on the base branch (the
	// run loop checks out main before each issue). Restore the issue branch so no
	// step — Ship's push or an agent's commits — ever runs on the base branch.
	if state.CurrentStep > pipeline.StepBranch && cfg.Git != nil {
		branch := issueBranchName(cfg.IssueNumber, issue.Title)
		if cur, curErr := cfg.Git.CurrentBranch(ctx, cfg.WorkDir); curErr != nil || cur != branch {
			if coErr := cfg.Git.Checkout(ctx, cfg.WorkDir, branch); coErr != nil {
				return nil, fmt.Errorf("resuming issue #%d at step %s: cannot check out its branch %q (created by the Branch step): %w", cfg.IssueNumber, state.CurrentStep, branch, coErr)
			}
			fmt.Fprintf(log, "resume: checked out issue branch %s\n", branch)
		}
	}

	// lastGreenGateFailure carries the Implement green gate's verify output into
	// the next attempt's prompt, so a retry targets the actual failure (e.g. an
	// unformatted file) instead of re-deriving the same defect blind.
	var lastGreenGateFailure string

	// rid groups every diagnostic record from this run (stable across resume).
	rid := runID(cfg.IssueNumber, state.StartedAt.Unix())

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
			if err := runBranchStep(ctx, cfg, issue, log); err != nil {
				return nil, err
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
			return runShipStep(ctx, cfg, issue, state, prof, stepStart, log)
		}

		// Docs step: surface-triggered. Skip without spawning an agent when the
		// change touches none of the project's declared documented surfaces.
		if step == pipeline.StepDocs && cfg.Git != nil &&
			!docsSurfaceTouched(cfg.DocSurfaces, cfg.Git.ChangedFiles(ctx, cfg.WorkDir)) {
			fmt.Fprintf(log, "%s: skipped (no documented surface touched)\n", step)
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

		allArgs := buildTemplateArgs(ctx, cfg, issue, branchName, lastGreenGateFailure)

		filteredArgs := filterArgs(string(tmplContent), allArgs)

		substituted, err := prompt.Substitute(string(tmplContent), filteredArgs)
		if err != nil {
			return nil, fmt.Errorf("substituting template %s: %w", templateFile[step], err)
		}

		model := modelForStep(step, prof)
		turns := turnsForStep(step, cfg.MaxTurns)
		fmt.Fprintf(log, "%s: invoking agent model=%s maxTurns=%d\n", step, model, turns)

		opts := agent.InvokeOptions{
			Prompt:       substituted,
			Model:        model,
			MaxTurns:     turns,
			WorkDir:      cfg.WorkDir,
			IssueNumber:  cfg.IssueNumber,
			PipelineStep: step.String(),
		}
		if cfg.Git != nil {
			opts.CommitCountFn = cfg.Git.CommitSHAs
		}
		invokeResult, err := cfg.Invoker.Invoke(ctx, opts)
		if err != nil {
			// Emit the failure (Detail carries the agent stderr surfaced by
			// internal/agent) so a step crash is diagnosable from the sink, not
			// just stdout. This is the path the Docs-step crash took on #88.
			emitStep(ctx, cfg, log, StepRecord{
				IssueNumber: cfg.IssueNumber,
				RunID:       rid,
				Stage:       step.String(),
				Outcome:     "error",
				DurationMs:  time.Since(stepStart).Milliseconds(),
				Detail:      lastLines(err.Error(), 30),
			})
			// Docs is best-effort: it is already skippable (surface-gated), runs
			// last on the weakest model, and a flake there must not discard a
			// validated, ready-to-ship PR. Log the cause (emitted above) and
			// advance to Ship — same graceful path as a surface skip. Every other
			// agent step stays fatal: its output is load-bearing.
			if step == pipeline.StepDocs {
				fmt.Fprintf(log, "%s: agent failed (%v) — Docs is best-effort, proceeding to Ship without doc changes\n", step, err)
				next, advErr := advanceDocsBestEffort(cfg, state)
				if advErr != nil {
					return nil, advErr
				}
				state.CurrentStep = next
				continue
			}
			return nil, fmt.Errorf("agent invocation at step %v: %w", step, err)
		}

		logAgentResult(log, step, invokeResult, turns)

		if cfg.CheckpointFn != nil {
			if chkErr := cfg.CheckpointFn(ctx, step, cfg.WorkDir); chkErr != nil {
				fmt.Fprintf(log, "%s: checkpoint failed: %v\n", step, chkErr)
				// Docs is best-effort (as in the agent-failure path above): an
				// incidental dirty tree after a Docs run — e.g. a go.sum the
				// toolchain rewrote and the weak model left uncommitted — must not
				// discard a validated, ready-to-ship PR. Advance to Ship instead.
				if step == pipeline.StepDocs {
					fmt.Fprintf(log, "%s: checkpoint failed (%v) — Docs is best-effort, proceeding to Ship\n", step, chkErr)
					next, advErr := advanceDocsBestEffort(cfg, state)
					if advErr != nil {
						return nil, advErr
					}
					state.CurrentStep = next
					continue
				}
				return nil, fmt.Errorf("checkpoint failed after step %v: %w", step, chkErr)
			}
			fmt.Fprintf(log, "%s: checkpoint pass\n", step)
		}

		stepResult, verifyOutput := deriveStepResult(ctx, step, invokeResult, cfg)
		rec := StepRecord{
			IssueNumber: cfg.IssueNumber,
			RunID:       rid,
			Stage:       step.String(),
			Completed:   invokeResult.Completed,
			Commits:     len(invokeResult.CommitsMade),
		}
		if step == pipeline.StepImplement && cfg.TestRunner != nil {
			logGreenGate(log, step, stepResult.Success, invokeResult.Completed, turns, verifyOutput)
			if stepResult.Success {
				lastGreenGateFailure = ""
				rec.GreenGate = "pass"
			} else {
				lastGreenGateFailure = lastLines(verifyOutput, 30)
				rec.GreenGate = "fail"
				rec.VerifyOutput = lastGreenGateFailure
			}
		}
		if step == pipeline.StepReview {
			recordReviewFindings(ctx, cfg, log, &rec)
		}

		next, advErr := state.Advance(stepResult)
		if advErr != nil {
			rec.Outcome = "blocked"
			rec.Detail = lastLines(advErr.Error(), 30)
			rec.DurationMs = time.Since(stepStart).Milliseconds()
			emitStep(ctx, cfg, log, rec)
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
		rec.Outcome = "ok"
		rec.DurationMs = time.Since(stepStart).Milliseconds()
		emitStep(ctx, cfg, log, rec)
		fmt.Fprintf(log, "%s: done (%dms)\n", step, time.Since(stepStart).Milliseconds())
	}
}

// issueBranchName is the branch the factory works an issue on: issue/<n>-<slug>.
// Derived deterministically from the issue number and title so the Branch step and
// the resume path (which must re-check-out the same branch) always agree.
func issueBranchName(issueNumber int, title string) string {
	return fmt.Sprintf("issue/%d-%s", issueNumber, slugify(title))
}

// runBranchStep creates or checks out the issue branch and seeds
// .themis/review-results.json with empty findings so the Review step is
// non-blocking unless the review agent itself writes blocking findings.
func runBranchStep(ctx context.Context, cfg Config, issue *tracker.IssueData, log io.Writer) error {
	branchName := issueBranchName(cfg.IssueNumber, issue.Title)
	if cfg.Git != nil {
		if err := cfg.Git.CheckoutNewBranch(ctx, cfg.WorkDir, branchName); err != nil {
			if checkoutErr := cfg.Git.Checkout(ctx, cfg.WorkDir, branchName); checkoutErr != nil {
				return fmt.Errorf("branch %s: create failed (%v), checkout failed (%v)", branchName, err, checkoutErr)
			}
			fmt.Fprintf(log, "note: branch %s already exists, checked out existing\n", branchName)
		}
	}
	themisDir := filepath.Join(cfg.WorkDir, ".themis")
	if mkErr := os.MkdirAll(themisDir, 0o755); mkErr != nil {
		return fmt.Errorf("creating .themis directory: %w", mkErr)
	}
	if wfErr := os.WriteFile(filepath.Join(themisDir, "review-results.json"),
		[]byte(`{"findings":[]}`), 0o644); wfErr != nil {
		return fmt.Errorf("seeding review-results.json: %w", wfErr)
	}
	return nil
}
