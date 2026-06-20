package runner

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/tracker"
)

// deriveStepResult maps an agent invocation outcome onto a pipeline.StepResult,
// applying the per-step success contract: TestRed wants failing tests present,
// and Implement gates on a green suite (the TestRunner). Other steps (Review,
// Docs) never gate the pipeline — they always succeed.
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

	case pipeline.StepImplement:
		// GREEN gate: Implement is "done" only when the suite passes. Without a
		// TestRunner configured, fall back to the legacy commit-only contract.
		if cfg.TestRunner == nil {
			return pipeline.StepResult{Success: true}
		}
		passed, _ := cfg.TestRunner(ctx, cfg.WorkDir)
		return pipeline.StepResult{Success: passed}

	default:
		return pipeline.StepResult{Success: true}
	}
}

// logGreenGate reports the outcome of the Implement/Fix green gate, distinguishing
// a stuck implementation (completed but red) from a truncated one (no completion
// signal, likely out of turns) so the operator can tell a budget problem from a
// capability problem.
func logGreenGate(log io.Writer, step pipeline.Step, passed, completed bool, turns int) {
	switch {
	case passed:
		fmt.Fprintf(log, "%s: green gate passed (test suite passes)\n", step)
	case completed:
		fmt.Fprintf(log, "%s: green gate failed — tests red though agent signalled completion (implementation may be stuck), retrying\n", step)
	default:
		fmt.Fprintf(log, "%s: green gate failed — tests red and no completion signal (likely hit the %d-turn limit), retrying\n", step, turns)
	}
}

// logAgentResult records the commit count and completion status of an agent
// step, and warns when the step ended without signalling completion — meaning it
// ran to its turn limit rather than reaching its terminal state.
func logAgentResult(log io.Writer, step pipeline.Step, r *agent.InvokeResult, turns int) {
	status := "completed"
	if !r.Completed {
		status = "not completed"
	}
	fmt.Fprintf(log, "%s: agent result: %d commits, %s\n", step, len(r.CommitsMade), status)
	if !r.Completed {
		fmt.Fprintf(log, "%s: warning: step did not signal completion (hit turn limit at %d turns)\n", step, turns)
	}
}

var placeholderRE = regexp.MustCompile(`\{\{([A-Z0-9_]+)\}\}`)

var conventionalPrefixRE = regexp.MustCompile(`^[0-9a-f]+\s+([a-z]+)[\(:]`)

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

// branchHasTestFiles reports whether the branch's changed-files list (relative to
// the base) contains at least one Go test file. Used to recognise that TestRed's
// failing tests are already present on a resumed or rebased branch.
func branchHasTestFiles(changedFiles string) bool {
	return filterTestFiles(changedFiles) != ""
}

// filterTestFiles returns the *_test.go entries from a changed-files list, one
// per line. TestRed commits only test files, so for the Implement step this is
// the set of tests that step must make pass.
func filterTestFiles(changedFiles string) string {
	var out []string
	for _, f := range strings.Split(changedFiles, "\n") {
		if strings.HasSuffix(strings.TrimSpace(f), "_test.go") {
			out = append(out, f)
		}
	}
	return strings.Join(out, "\n")
}

func pipelineShape(commitLog string) string {
	if commitLog == "" {
		return ""
	}
	seen := make(map[string]bool)
	var prefixes []string
	for _, line := range strings.Split(commitLog, "\n") {
		if m := conventionalPrefixRE.FindStringSubmatch(line); m != nil {
			p := m[1]
			if !seen[p] {
				seen[p] = true
				prefixes = append(prefixes, p)
			}
		}
	}
	return strings.Join(prefixes, ", ")
}

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

// defaultStepTurns caps how many turns each pipeline step may use. These are
// per-step ceilings: a step uses its default when that default is lower than the
// global --max-turns value, so a smaller --max-turns still lowers every step
// while the defaults keep simple steps from running to a large global limit.
var defaultStepTurns = map[pipeline.Step]int{
	pipeline.StepTestRed:   80,
	pipeline.StepImplement: 120,
	pipeline.StepReview:    80,
	pipeline.StepDocs:      40,
	pipeline.StepShip:      60,
}

// turnsForStep returns the per-step turn budget: the step's default when it is
// lower than maxTurns, otherwise the global maxTurns ceiling. maxTurns therefore
// acts as a hard ceiling — passing a value below every default lowers all steps.
func turnsForStep(step pipeline.Step, maxTurns int) int {
	if stepLimit, ok := defaultStepTurns[step]; ok && stepLimit < maxTurns {
		return stepLimit
	}
	return maxTurns
}

func modelForStep(step pipeline.Step, prof ProfileData) string {
	switch step {
	case pipeline.StepReview:
		return prof.ReviewModel
	case pipeline.StepDocs:
		// Docs is either a fast "nothing to do" exit or mechanical cleanup —
		// it doesn't need a frontier model.
		return "haiku"
	default:
		return prof.ImplementModel
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

func buildTemplateArgs(
	ctx context.Context,
	cfg Config,
	issue *tracker.IssueData,
	branchName string,
) map[string]string {
	acs := tracker.ParseCheckboxes(issue.Body)
	acList := formatACs(acs)
	var commitLog string
	var changedFilesResult string
	if cfg.Git != nil {
		commitLog = cfg.Git.BranchCommitLog(ctx, cfg.WorkDir)
		changedFilesResult = cfg.Git.ChangedFiles(ctx, cfg.WorkDir)
	}
	return map[string]string{
		"ISSUE_NUMBER":        strconv.Itoa(cfg.IssueNumber),
		"ISSUE_TITLE":         issue.Title,
		"ACCEPTANCE_CRITERIA": acList,
		"AC_STATUS":           acList,
		"BRANCH_NAME":         branchName,
		"CHANGED_FILES":       changedFilesResult,
		"TEST_FILES":          filterTestFiles(changedFilesResult),
		"STANDARDS_DOCS":      formatStandardsDocs(cfg.StandardsDocs),
		"PIPELINE_SHAPE":      pipelineShape(commitLog),
		"COMMIT_LOG":          commitLog,
	}
}

// formatStandardsDocs renders the project's declared authoritative docs as a
// bullet list for the {{STANDARDS_DOCS}} placeholder. When a project declares
// none, it says so plainly rather than implying a (Go-specific) default.
func formatStandardsDocs(docs []string) string {
	if len(docs) == 0 {
		return "(none declared for this project — follow the conventions visible in the surrounding code)"
	}
	var sb strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&sb, "- `%s`\n", d)
	}
	return strings.TrimRight(sb.String(), "\n")
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

// stripCodeFences removes leading/trailing code fence markers from agent output.
// Claude Code's --print mode sometimes wraps markdown responses in ```...``` blocks.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if idx := strings.Index(s, "\n"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
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
