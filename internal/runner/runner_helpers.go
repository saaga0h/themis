package runner

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/saaga0h/themis/internal/pipeline"
	"github.com/saaga0h/themis/internal/tracker"
)

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
	for _, f := range strings.Split(changedFiles, "\n") {
		if strings.HasSuffix(strings.TrimSpace(f), "_test.go") {
			return true
		}
	}
	return false
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

func modelForStep(step pipeline.Step, prof ProfileData) string {
	switch step {
	case pipeline.StepReview:
		return prof.ReviewModel
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
