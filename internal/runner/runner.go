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
	"git.home.federation.fi/lavernea/themis/internal/git"
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
	TestACKey    string
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

// Run executes the full pipeline for the given configuration.
func Run(ctx context.Context, cfg Config) (*Result, error) {
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

	issue, err := cfg.Fetcher.Fetch(ctx, cfg.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching issue #%d: %w", cfg.IssueNumber, err)
	}

	prof, err := profile.Load(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("loading profile: %w", err)
	}

	codingStandards := readFileOrEmpty(filepath.Join(cfg.WorkDir, "CODING_STANDARDS.md"))
	ubiquitousLanguage := readFileOrEmpty(filepath.Join(cfg.WorkDir, "UBIQUITOUS_LANGUAGE.md"))

	var lastBlockingFindings string

	for {
		step := state.CurrentStep

		// Fetch step: git fetch origin.
		if step == pipeline.StepFetch {
			if err := git.Fetch(ctx, cfg.WorkDir); err != nil {
				fmt.Fprintf(os.Stderr, "warning: git fetch failed: %v\n", err)
			}
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
			continue
		}

		// Branch step: create issue branch, or checkout if it already exists.
		if step == pipeline.StepBranch {
			branchName := fmt.Sprintf("issue/%d-%s", cfg.IssueNumber, slugify(issue.Title))
			if err := git.CheckoutNewBranch(ctx, cfg.WorkDir, branchName); err != nil {
				// Branch may exist from a previous run — try checking it out.
				if checkoutErr := git.Checkout(ctx, cfg.WorkDir, branchName); checkoutErr != nil {
					return nil, fmt.Errorf("branch %s: create failed (%v), checkout failed (%v)", branchName, err, checkoutErr)
				}
				fmt.Fprintf(os.Stderr, "note: branch %s already exists, checked out existing\n", branchName)
			}
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

		// Ship step: push branch and create PR.
		if step == pipeline.StepShip {
			branch := currentBranchName(cfg.WorkDir)
			if err := git.PushBranch(ctx, cfg.WorkDir, branch); err != nil {
				return nil, fmt.Errorf("pushing branch %s: %w", branch, err)
			}
			acs := tracker.ParseCheckboxes(issue.Body)
			prURL, err := cfg.IssueWriter.CreatePR(ctx, PROptions{
				Title: fmt.Sprintf("Closes #%d — %s", cfg.IssueNumber, issue.Title),
				Body:  buildPRBody(cfg.IssueNumber, issue.Title, acs),
				Base:  "themis-2.0",
				Head:  branch,
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

		acs := tracker.ParseCheckboxes(issue.Body)
		masterArgs := map[string]string{
			"ISSUE_NUMBER":        strconv.Itoa(cfg.IssueNumber),
			"ISSUE_TITLE":         issue.Title,
			"ACCEPTANCE_CRITERIA": formatACs(acs),
			"CODING_STANDARDS":    codingStandards,
			"UBIQUITOUS_LANGUAGE": ubiquitousLanguage,
			"BRANCH_NAME":         currentBranchName(cfg.WorkDir),
			"CHANGED_FILES":       changedFiles(cfg.WorkDir),
			"REVIEW_CYCLE":        strconv.Itoa(state.ReviewCycle + 1),
			"BLOCKING_FINDINGS":   lastBlockingFindings,
		}

		filteredArgs := filterArgs(string(tmplContent), masterArgs)

		substituted, err := prompt.Substitute(string(tmplContent), filteredArgs)
		if err != nil {
			return nil, fmt.Errorf("substituting template %s: %w", templateFile[step], err)
		}

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

		if cfg.CheckpointFn != nil {
			if err := cfg.CheckpointFn(ctx, step, cfg.WorkDir); err != nil {
				return nil, fmt.Errorf("checkpoint failed after step %v: %w", step, err)
			}
		}

		stepResult := deriveStepResult(step, invokeResult, cfg)
		if step == pipeline.StepReview && stepResult.BlockingFindings {
			lastBlockingFindings = extractBlockingFindings(invokeResult.Stdout)
		}

		next, advErr := state.Advance(stepResult)
		if advErr != nil {
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

func hasBlockingFindings(output string) bool {
	if blockingLineRE.MatchString(output) {
		return true
	}
	upper := strings.ToUpper(output)
	return strings.Contains(upper, "BLOCKING_FINDINGS: YES")
}

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

func modelForStep(step pipeline.Step, prof *profile.Profile) string {
	switch step {
	case pipeline.StepReview:
		return prof.Review.Agents.Security
	default:
		return prof.Implement.Model
	}
}

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
	return ""
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}
