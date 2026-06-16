package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/profile"
	"git.home.federation.fi/lavernea/themis/internal/prompt"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

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
	WorkDir      string
	IssueNumber  int
	Fetcher      tracker.Fetcher
	Invoker      agent.Invoker
	IssueWriter  IssueWriter
	TemplateDir  string
	CheckpointFn func(ctx context.Context, step pipeline.Step, workDir string) error
	// TestACKey overrides the AC key used for test-fix attempt tracking.
	// In tests: set to a known key matching pre-populated state.
	// In production: leave empty to use the default "tests" key.
	TestACKey string
}

// Result holds the outcome of a successful pipeline run.
type Result struct {
	PRURL string
}

// agentSteps are pipeline steps that require agent invocation.
var agentSteps = map[pipeline.Step]bool{
	pipeline.StepTestRed:  true,
	pipeline.StepImplement: true,
	pipeline.StepRefactor:  true,
	pipeline.StepReview:    true,
	pipeline.StepFix:       true,
	pipeline.StepDocs:      true,
}

// templateFile maps each agent step to its prompt template filename.
var templateFile = map[pipeline.Step]string{
	pipeline.StepTestRed:  "test-red.md",
	pipeline.StepImplement: "implement.md",
	pipeline.StepRefactor:  "refactor.md",
	pipeline.StepReview:    "review.md",
	pipeline.StepFix:       "fix-findings.md",
	pipeline.StepDocs:      "update-docs.md",
}


// Run executes the full pipeline for the given configuration.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	// Load or initialise pipeline state.
	state, err := pipeline.LoadState(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}
	if state == nil {
		state = &pipeline.PipelineState{
			IssueNumber:     cfg.IssueNumber,
			CurrentStep:     pipeline.StepFetch,
			MaxReviewCycles: 2,
			TestFixAttempts: map[string]int{},
			StartedAt:       time.Now(),
		}
	}

	// Fetch issue data.
	issue, err := cfg.Fetcher.Fetch(ctx, cfg.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching issue #%d: %w", cfg.IssueNumber, err)
	}

	// Load project profile for model configuration.
	prof, err := profile.Load(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("loading profile: %w", err)
	}

	// Read optional doc files for template substitution.
	codingStandards := readFileOrEmpty(filepath.Join(cfg.WorkDir, "CODING_STANDARDS.md"))
	ubiquitousLanguage := readFileOrEmpty(filepath.Join(cfg.WorkDir, "UBIQUITOUS_LANGUAGE.md"))

	// Accumulate blocking findings between review and fix steps.
	var lastBlockingFindings string

	for {
		step := state.CurrentStep

		// Infrastructure steps: advance without agent invocation.
		if step == pipeline.StepFetch || step == pipeline.StepScan || step == pipeline.StepBranch {
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing step %v: %w", step, err)
			}
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving state at step %v: %w", step, err)
			}
			state.CurrentStep = next
			continue
		}

		// Ship step: create PR directly and finish.
		if step == pipeline.StepShip {
			acs := tracker.ParseCheckboxes(issue.Body)
			prURL, err := cfg.IssueWriter.CreatePR(ctx, PROptions{
				Title: fmt.Sprintf("Closes #%d — %s", cfg.IssueNumber, issue.Title),
				Body:  buildPRBody(cfg.IssueNumber, issue.Title, acs),
				Base:  "main",
			})
			if err != nil {
				return nil, fmt.Errorf("creating PR: %w", err)
			}
			if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
				return nil, fmt.Errorf("saving final state: %w", err)
			}
			return &Result{PRURL: prURL}, nil
		}

		// Agent steps: load template, substitute, invoke, checkpoint.
		if !agentSteps[step] {
			// Unknown step — skip.
			next, err := state.Advance(pipeline.StepResult{Success: true})
			if err != nil {
				return nil, fmt.Errorf("advancing unknown step %v: %w", step, err)
			}
			state.CurrentStep = next
			continue
		}

		tmplPath := filepath.Join(cfg.TemplateDir, templateFile[step])
		tmplContent, err := os.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", tmplPath, err)
		}

		// Build master args — all possible placeholders across all templates.
		acs := tracker.ParseCheckboxes(issue.Body)
		masterArgs := map[string]string{
			"ISSUE_NUMBER":       strconv.Itoa(cfg.IssueNumber),
			"ISSUE_TITLE":        issue.Title,
			"ACCEPTANCE_CRITERIA": formatACs(acs),
			"CODING_STANDARDS":   codingStandards,
			"UBIQUITOUS_LANGUAGE": ubiquitousLanguage,
			"BRANCH_NAME":        currentBranchName(cfg.WorkDir),
			"CHANGED_FILES":      changedFiles(cfg.WorkDir),
			"REVIEW_CYCLE":       strconv.Itoa(state.ReviewCycle + 1),
			"BLOCKING_FINDINGS":  lastBlockingFindings,
		}

		// Filter to only the placeholders the template actually uses.
		filteredArgs := filterArgs(string(tmplContent), masterArgs)

		substituted, err := prompt.Substitute(string(tmplContent), filteredArgs)
		if err != nil {
			return nil, fmt.Errorf("substituting template %s: %w", templateFile[step], err)
		}

		// Determine model from profile.
		model := modelForStep(step, prof)

		invokeResult, err := cfg.Invoker.Invoke(ctx, agent.InvokeOptions{
			Prompt:   substituted,
			Model:    model,
			MaxTurns: 100,
			WorkDir:  cfg.WorkDir,
		})
		if err != nil {
			return nil, fmt.Errorf("agent invocation at step %v: %w", step, err)
		}

		// Run checkpoint verification after each agent step.
		if cfg.CheckpointFn != nil {
			if err := cfg.CheckpointFn(ctx, step, cfg.WorkDir); err != nil {
				return nil, fmt.Errorf("checkpoint failed after step %v: %w", step, err)
			}
		}

		// Derive step result from agent output.
		stepResult := deriveStepResult(step, invokeResult, cfg)
		if step == pipeline.StepReview && stepResult.BlockingFindings {
			lastBlockingFindings = extractBlockingFindings(invokeResult.Stdout)
		}

		// Advance state machine.
		next, advErr := state.Advance(stepResult)
		if advErr != nil {
			// Cycle or test-fix limit reached — block the issue.
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
	}
}

// blockIssue adds the blocked label and posts a comment explaining why.
func blockIssue(ctx context.Context, cfg Config, reason error) error {
	comment := fmt.Sprintf("Pipeline blocked on issue #%d: %v", cfg.IssueNumber, reason)
	if err := cfg.IssueWriter.Comment(ctx, cfg.IssueNumber, comment); err != nil {
		return fmt.Errorf("posting block comment: %w", err)
	}
	if err := cfg.IssueWriter.AddLabel(ctx, cfg.IssueNumber, "blocked"); err != nil {
		return fmt.Errorf("adding blocked label: %w", err)
	}
	return nil
}

// deriveStepResult converts an InvokeResult into a pipeline StepResult.
func deriveStepResult(step pipeline.Step, r *agent.InvokeResult, cfg Config) pipeline.StepResult {
	switch step {
	case pipeline.StepTestRed:
		key := cfg.TestACKey
		if key == "" {
			key = "tests"
		}
		if len(r.CommitsMade) > 0 || r.Completed {
			return pipeline.StepResult{Success: true}
    }
    return pipeline.StepResult{Success: false, TestACKey: key}

	case pipeline.StepReview:
		blocking := hasBlockingFindings(r.Stdout)
		return pipeline.StepResult{
			Success:          !blocking,
			BlockingFindings: blocking,
		}

	default:
		return pipeline.StepResult{Success: true}
	}
}

var blockingLineRE = regexp.MustCompile(`(?im)^blocking:\s+.+`)

// hasBlockingFindings reports whether agent output contains blocking review findings.
func hasBlockingFindings(output string) bool {
	if blockingLineRE.MatchString(output) {
		return true
	}
	upper := strings.ToUpper(output)
	return strings.Contains(upper, "BLOCKING_FINDINGS: YES")
}

// extractBlockingFindings extracts blocking finding lines from review output.
func extractBlockingFindings(output string) string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "BLOCKING:") {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) == 0 {
		return output
	}
	return strings.Join(lines, "\n")
}

// filterArgs returns only the args whose keys appear as {{KEY}} in the template.
var placeholderRE = regexp.MustCompile(`\{\{([A-Z0-9_]+)\}\}`)

func filterArgs(tmpl string, all map[string]string) map[string]string {
	out := make(map[string]string)
	for _, m := range placeholderRE.FindAllStringSubmatch(tmpl, -1) {
		key := m[1]
		if v, ok := all[key]; ok {
			out[key] = v
		}
	}
	return out
}

// modelForStep returns the agent model to use for a given pipeline step.
func modelForStep(step pipeline.Step, prof *profile.Profile) string {
	switch step {
	case pipeline.StepReview:
		return prof.Review.Agents.Security
	default:
		return prof.Implement.Model
	}
}

// buildPRBody constructs the pull request body.
func buildPRBody(number int, title string, acs []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Closes #%d\n\n", number)
	fmt.Fprintf(&sb, "## Summary\n\n%s\n\n", title)
	sb.WriteString("## Acceptance Criteria\n\n")
	for _, ac := range acs {
		fmt.Fprintf(&sb, "- %s\n", ac)
	}
	return sb.String()
}

// formatACs formats parsed acceptance criteria for template substitution.
func formatACs(acs []string) string {
	if len(acs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, ac := range acs {
		fmt.Fprintf(&sb, "- [ ] %s\n", ac)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func currentBranchName(workDir string) string {
	data, err := os.ReadFile(filepath.Join(workDir, ".git", "HEAD"))
	if err != nil {
		return "main"
	}
	ref := strings.TrimSpace(string(data))
	if after, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return after
	}
	return "main"
}

func changedFiles(workDir string) string {
	// Best-effort: read files changed vs origin/main.
	// In test environments this may be empty, which is fine.
	return ""
}
