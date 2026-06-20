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
	pipeline.StepRefactor:  30,
	pipeline.StepReview:    80,
	pipeline.StepFix:       60,
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
	case pipeline.StepRefactor, pipeline.StepDocs:
		// Refactor and Docs are either a fast "nothing to do" exit or mechanical
		// cleanup — neither needs a frontier model.
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
	reviewCycle int,
	lastBlockingFindings string,
) map[string]string {
	acs := tracker.ParseCheckboxes(issue.Body)
	acList := formatACs(acs)
	var commitLog string
	var changedFilesResult string
	var diffLines int
	if cfg.Git != nil {
		commitLog = cfg.Git.BranchCommitLog(ctx, cfg.WorkDir)
		changedFilesResult = cfg.Git.ChangedFiles(ctx, cfg.WorkDir)
		diffLines = cfg.Git.DiffLineCount(ctx, cfg.WorkDir)
	}
	return map[string]string{
		"ISSUE_NUMBER":        strconv.Itoa(cfg.IssueNumber),
		"ISSUE_TITLE":         issue.Title,
		"ACCEPTANCE_CRITERIA": acList,
		"AC_STATUS":           acList,
		"BRANCH_NAME":         branchName,
		"CHANGED_FILES":       changedFilesResult,
		"TEST_FILES":          filterTestFiles(changedFilesResult),
		"DIFF_LINES":          strconv.Itoa(diffLines),
		"REVIEW_CYCLE":        strconv.Itoa(reviewCycle + 1),
		"BLOCKING_FINDINGS":   lastBlockingFindings,
		"PIPELINE_SHAPE":      pipelineShape(commitLog),
		"COMMIT_LOG":          commitLog,
	}
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
