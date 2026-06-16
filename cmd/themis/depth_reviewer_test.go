package main_test

import (
	"os"
	"strings"
	"testing"
)

// helpers

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel)
	if err != nil {
		t.Fatalf("required file %q not found: %v", rel, err)
	}
	return string(b)
}

// AC: agents/depth-reviewer.md exists with frontmatter name, description,
// tools (Read, Glob, Grep, Bash), model (sonnet)

func TestDepthReviewerAgentFileExists(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "name: depth-reviewer") {
		t.Error("agents/depth-reviewer.md must contain frontmatter 'name: depth-reviewer'")
	}
	if !strings.Contains(content, "model: sonnet") {
		t.Error("agents/depth-reviewer.md must specify 'model: sonnet' in frontmatter")
	}
}

func TestDepthReviewerAgentFrontmatterTools(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	for _, tool := range []string{"Read", "Glob", "Grep", "Bash"} {
		if !strings.Contains(content, tool) {
			t.Errorf("agents/depth-reviewer.md frontmatter must list tool %q", tool)
		}
	}
}

// AC: agent applies the deletion test as primary filter

func TestDepthReviewerAppliesDeletionTest(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "deletion test") {
		t.Error("agents/depth-reviewer.md must describe the deletion test")
	}
	if !strings.Contains(content, "primary filter") {
		t.Error("agents/depth-reviewer.md must describe the deletion test as primary filter")
	}
}

func TestDepthReviewerDeletionTestCoversComplexityVanishes(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "vanishes") {
		t.Error("deletion test must cover the 'complexity vanishes → shallow' case")
	}
}

func TestDepthReviewerDeletionTestCoversComplexityReappears(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "reappears") {
		t.Error("deletion test must cover the 'complexity reappears → earning its keep' case")
	}
}

// AC: agent classifies every candidate by dependency category

func TestDepthReviewerClassifiesInProcess(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "in-process") {
		t.Error("agents/depth-reviewer.md must define the 'in-process' dependency category")
	}
}

func TestDepthReviewerClassifiesLocalSubstitutable(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "local-substitutable") {
		t.Error("agents/depth-reviewer.md must define the 'local-substitutable' dependency category")
	}
}

func TestDepthReviewerClassifiesRemoteOwned(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "remote") {
		t.Error("agents/depth-reviewer.md must define the 'remote-owned' dependency category")
	}
}

func TestDepthReviewerClassifiesTrueExternal(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "true external") || !strings.Contains(content, "True external") {
		if !strings.Contains(strings.ToLower(content), "true external") {
			t.Error("agents/depth-reviewer.md must define the 'true external' dependency category")
		}
	}
}

// AC: agent ranks candidates as Strong, Worth exploring, or Speculative

func TestDepthReviewerRanksStrong(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Strong") {
		t.Error("agents/depth-reviewer.md must use 'Strong' as a ranking tier")
	}
}

func TestDepthReviewerRanksWorthExploring(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Worth exploring") {
		t.Error("agents/depth-reviewer.md must use 'Worth exploring' as a ranking tier")
	}
}

func TestDepthReviewerRanksSpeculative(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Speculative") {
		t.Error("agents/depth-reviewer.md must use 'Speculative' as a ranking tier")
	}
}

// AC: output format matches .claude/reviews/ convention

func TestDepthReviewerOutputFormatHasOverallVerdict(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Overall") {
		t.Error("agents/depth-reviewer.md output format must include an overall verdict field")
	}
}

func TestDepthReviewerOutputFormatHasCandidateCards(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	for _, field := range []string{"Files involved", "Why shallow", "Dependency category", "Deepening shape", "Leverage", "Locality"} {
		if !strings.Contains(content, field) {
			t.Errorf("agents/depth-reviewer.md output format must include candidate card field %q", field)
		}
	}
}

func TestDepthReviewerOutputFormatHasTopRecommendation(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Top recommendation") {
		t.Error("agents/depth-reviewer.md output format must include a Top recommendation section")
	}
}

// AC: out-of-scope section excludes interface proposals, small-but-earning-keep,
// two-adapter seams, generated code, non-depth concerns

func TestDepthReviewerOutOfScopeSection(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "Out of scope") && !strings.Contains(content, "out of scope") {
		t.Error("agents/depth-reviewer.md must have an 'Out of scope' section")
	}
}

func TestDepthReviewerExcludesInterfaceProposals(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "interface") {
		t.Error("agents/depth-reviewer.md out-of-scope must exclude interface proposals")
	}
}

func TestDepthReviewerExcludesTwoAdapterSeams(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "two") || !strings.Contains(content, "adapter") {
		t.Error("agents/depth-reviewer.md must exclude seams that have two real adapters")
	}
}

func TestDepthReviewerExcludesGeneratedCode(t *testing.T) {
	content := readRepoFile(t, "agents/depth-reviewer.md")
	if !strings.Contains(content, "generated code") {
		t.Error("agents/depth-reviewer.md must exclude generated code from review scope")
	}
}

// AC: commands/review.md updated with --depth flag to dispatch depth-reviewer

func TestReviewCommandHasDepthFlag(t *testing.T) {
	content := readRepoFile(t, "commands/review.md")
	if !strings.Contains(content, "--depth") {
		t.Error("commands/review.md must include a --depth flag to dispatch depth-reviewer")
	}
}

func TestReviewCommandDepthFlagDispatchesDepthReviewer(t *testing.T) {
	content := readRepoFile(t, "commands/review.md")
	if !strings.Contains(content, "depth-reviewer") {
		t.Error("commands/review.md must reference depth-reviewer when --depth is used")
	}
}
