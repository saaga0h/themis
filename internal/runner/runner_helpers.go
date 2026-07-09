package runner

import (
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/labels"
	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/review"
	"github.com/saaga0h/themis/internal/tracker"
)

// reviewVerdict summarises the review gate for the PR body and decides whether
// the PR opens as a draft. Any blocking finding (critical/high) makes it a draft
// for a maintainer to resolve before merge; everything else is a non-blocking
// note. This is the deterministic ready/draft signal — the runner owns it rather
// than trusting the ship agent's prose.
func reviewVerdict(findings []review.ReviewFinding) (draft bool, section string) {
	blocking, nonBlocking := review.CountFindingsBySeverity(findings)
	var sb strings.Builder
	sb.WriteString("\n## Review verdict\n\n")
	if blocking == 0 {
		sb.WriteString("Security & AC gate: **clean** — no blocking findings.\n")
	} else {
		fmt.Fprintf(&sb, "Security & AC gate: **%d blocking finding(s)** — opened as a draft for a maintainer to resolve before merge:\n\n", blocking)
		sb.WriteString(review.FormatBlockingFindings(findings))
	}
	if nonBlocking > 0 {
		sb.WriteString("\n### Reviewer observations (not addressed — for maintainer triage)\n\n")
		for _, f := range findings {
			switch f.Severity {
			case "critical", "high", review.BlockingThreshold:
				continue
			}
			fmt.Fprintf(&sb, "- %s", f.Description)
			if f.File != "" {
				fmt.Fprintf(&sb, " (%s", f.File)
				if f.Line > 0 {
					fmt.Fprintf(&sb, ":%d", f.Line)
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
	}
	return blocking > 0, sb.String()
}

// deriveStepResult maps an agent invocation outcome onto a pipeline.StepResult,
// applying the per-step success contract: TestRed wants failing tests present,
// and Implement gates on a green suite (the TestRunner). Other steps (Review,
// Docs) never gate the pipeline — they always succeed.
// deriveStepResult also returns the verify output for the Implement green gate
// (empty for other steps), so the runner can surface which verify command failed
// and feed that output back into the retry prompt.
func deriveStepResult(ctx context.Context, step pipeline.Step, r *agent.InvokeResult, cfg Config) (pipeline.StepResult, string) {
	switch step {
	case pipeline.StepTestRed:
		key := cfg.TestACKey
		if key == "" {
			key = "tests"
		}
		if len(r.CommitsMade) > 0 || r.Completed {
			return pipeline.StepResult{Success: true}, ""
		}
		// A resumed or rebased run can reach TestRed with the failing tests
		// already committed on the branch. The agent then correctly makes no new
		// commit, but TestRed's goal — failing tests present before Implement —
		// is already met. Treat that as success rather than scoring a failed
		// attempt, which would otherwise stall the pipeline on the retry ceiling.
		if cfg.Git != nil && branchHasTestFiles(cfg.Git.ChangedFiles(ctx, cfg.WorkDir)) {
			return pipeline.StepResult{Success: true}, ""
		}
		return pipeline.StepResult{Success: false, TestACKey: key}, ""

	case pipeline.StepImplement:
		// GREEN gate: Implement is "done" only when the project's verify suite
		// passes. Without a TestRunner configured, fall back to the legacy
		// commit-only contract.
		if cfg.TestRunner == nil {
			return pipeline.StepResult{Success: true}, ""
		}
		passed, output := cfg.TestRunner(ctx, cfg.WorkDir)
		return pipeline.StepResult{
			Success:    passed,
			Completed:  r.Completed,
			Progressed: len(r.CommitsMade) > 0,
		}, output

	default:
		return pipeline.StepResult{Success: true}, ""
	}
}

// logGreenGate reports the outcome of the Implement/Fix green gate, distinguishing
// a stuck implementation (completed but red) from a truncated one (no completion
// signal, likely out of turns) so the operator can tell a budget problem from a
// capability problem.
func logGreenGate(log io.Writer, step pipeline.Step, passed, completed bool, turns int, output string) {
	if passed {
		fmt.Fprintf(log, "%s: green gate passed (verify suite passes)\n", step)
		return
	}
	// The verify suite is build/format/lint/test — not just "tests". Report that
	// plainly and echo the output (which names the failing command), so neither
	// the operator nor the retry is misled into thinking it was a test failure.
	if completed {
		fmt.Fprintf(log, "%s: green gate failed — verify suite did not pass though the agent signalled completion (e.g. a missed format/vet step); retrying with the failure output\n", step)
	} else {
		fmt.Fprintf(log, "%s: green gate failed — verify suite did not pass and no completion signal (likely hit the %d-turn limit); retrying\n", step, turns)
	}
	if out := lastLines(strings.TrimSpace(output), 20); out != "" {
		fmt.Fprintf(log, "%s: verify output (tail):\n%s\n", step, out)
	}
}

// lastLines returns the final n lines of s (all of s when it has fewer), used to
// cap verify output echoed to the log and fed into the retry prompt.
func lastLines(s string, n int) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
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
	d := pipeline.Classify(reason)
	comment := fmt.Sprintf("Pipeline blocked on issue #%d\n\n**%s** — %s\n\n**What to do:** %s",
		cfg.IssueNumber, d.Category, d.Reason, d.Action)
	if cfg.Scrub != nil {
		comment = cfg.Scrub(comment)
	}
	if err := cfg.IssueWriter.Comment(ctx, cfg.IssueNumber, comment); err != nil {
		return fmt.Errorf("posting block comment: %w", err)
	}
	if err := cfg.IssueWriter.AddLabel(ctx, cfg.IssueNumber, labels.Blocked); err != nil {
		return fmt.Errorf("adding blocked label: %w", err)
	}
	return nil
}

// advanceDocsBestEffort moves the pipeline past a non-fatal Docs failure — an
// agent flake or a dirty-tree checkpoint — to the next step (Ship), the same
// graceful path as a surface skip. Docs is skippable, runs last on the weakest
// model, and commits nothing load-bearing, so a hiccup there must never discard a
// validated, ready-to-ship PR. It forces the success transition and persists
// state; the caller sets state.CurrentStep to the returned step.
func advanceDocsBestEffort(cfg Config, state *pipeline.PipelineState) (pipeline.Step, error) {
	next, err := state.Advance(pipeline.StepResult{Success: true})
	if err != nil {
		return 0, fmt.Errorf("advancing past non-fatal Docs failure: %w", err)
	}
	if err := pipeline.SaveState(cfg.WorkDir, state); err != nil {
		return 0, fmt.Errorf("saving state past non-fatal Docs failure: %w", err)
	}
	return next, nil
}

// docsSurfaceTouched reports whether the Docs step should run: true when no
// surfaces are declared (Docs always runs), or when a changed file matches a
// declared surface glob. A surface ending in "/" matches files under that
// directory; otherwise it is matched as a path glob (and as an exact path).
func docsSurfaceTouched(surfaces []string, changedFiles string) bool {
	if len(surfaces) == 0 {
		return true
	}
	for _, f := range strings.Split(changedFiles, "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		for _, s := range surfaces {
			if strings.HasSuffix(s, "/") {
				if strings.HasPrefix(f, s) {
					return true
				}
				continue
			}
			if f == s {
				return true
			}
			if ok, _ := path.Match(s, f); ok {
				return true
			}
		}
	}
	return false
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
	// Docs is a minimal, issue-scoped pass, but 20 turns proved too tight — a
	// real doc pass (locate the surfaces, read them, edit) hit the ceiling and
	// aborted best-effort without writing anything. 80 leaves room to finish
	// while still capping it well below implement-scale work (issue #75's over-run).
	pipeline.StepDocs: 80,
	pipeline.StepShip: 60,
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
	greenGateFailure string,
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
		"GREEN_GATE_FAILURE":  greenGateFailure,
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
