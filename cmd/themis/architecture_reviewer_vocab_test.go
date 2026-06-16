package main_test

import (
	"os"
	"strings"
	"testing"
)

func readAgentFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel)
	if err != nil {
		t.Fatalf("required file %q not found: %v", rel, err)
	}
	return string(b)
}

// AC: agents/architecture-reviewer.md uses "seam" instead of "boundary" throughout

func TestArchitectureReviewerUsesSeamNotBoundary(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if strings.Contains(content, "boundary") || strings.Contains(content, "Boundary") {
		t.Error("agents/architecture-reviewer.md must not use 'boundary'/'Boundary' — use 'seam' instead")
	}
}

func TestArchitectureReviewerUsesSeamVocabulary(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if !strings.Contains(content, "seam") {
		t.Error("agents/architecture-reviewer.md must use 'seam' vocabulary from the deepening framework")
	}
}

func TestArchitectureReviewerDescriptionUsesSeam(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	// The description frontmatter must reference seam violations, not boundary violations
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "description:") {
			if strings.Contains(strings.ToLower(line), "boundary") {
				t.Errorf("frontmatter description must not use 'boundary': got %q", line)
			}
			if !strings.Contains(strings.ToLower(line), "seam") {
				t.Errorf("frontmatter description must use 'seam': got %q", line)
			}
		}
	}
}

// AC: agents/architecture-reviewer.md uses "module" instead of "component" throughout

func TestArchitectureReviewerUsesModuleNotComponent(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if strings.Contains(content, "component") || strings.Contains(content, "Component") {
		t.Error("agents/architecture-reviewer.md must not use 'component'/'Component' — use 'module' instead")
	}
}

func TestArchitectureReviewerUsesModuleVocabulary(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if !strings.Contains(content, "module") {
		t.Error("agents/architecture-reviewer.md must use 'module' vocabulary from the deepening framework")
	}
}

// AC: section headers updated where needed

func TestArchitectureReviewerSeamViolationsHeader(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	// Must not have "Boundary Violations" header (old vocabulary)
	if strings.Contains(content, "Boundary Violations") || strings.Contains(content, "boundary violations") {
		t.Error("section header 'Boundary Violations' must be replaced with seam-based equivalent")
	}
	// Must have a seam-based header instead
	if !strings.Contains(content, "Seam") && !strings.Contains(content, "seam") {
		t.Error("agents/architecture-reviewer.md must have a seam-based section header")
	}
}

// AC: no functional change — review logic stays the same

func TestArchitectureReviewerResponsibilityDriftPreserved(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if !strings.Contains(content, "Responsibility drift") && !strings.Contains(content, "responsibility drift") {
		t.Error("agents/architecture-reviewer.md must still check for responsibility drift (functional content preserved)")
	}
}

func TestArchitectureReviewerStructuralCoherencePreserved(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if !strings.Contains(content, "Structural coherence") && !strings.Contains(content, "structural coherence") {
		t.Error("agents/architecture-reviewer.md must still check for structural coherence (functional content preserved)")
	}
}

func TestArchitectureReviewerConstraintViolationsPreserved(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	if !strings.Contains(content, "Constraint violations") && !strings.Contains(content, "Constraint Violations") {
		t.Error("agents/architecture-reviewer.md must still check for constraint violations (functional content preserved)")
	}
}

func TestArchitectureReviewerOutputFormatPreserved(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	// Output format must still include Alignment verdict
	if !strings.Contains(content, "CONSISTENT") {
		t.Error("agents/architecture-reviewer.md output format must preserve CONSISTENT verdict option")
	}
	if !strings.Contains(content, "Recommendations") {
		t.Error("agents/architecture-reviewer.md output format must preserve Recommendations section")
	}
}

// AC: architecture-reviewer and depth-reviewer use consistent terms side by side

func TestArchitectureAndDepthReviewerShareSeamVocabulary(t *testing.T) {
	arch := readAgentFile(t, "agents/architecture-reviewer.md")
	depth := readAgentFile(t, "agents/depth-reviewer.md")

	archHasSeam := strings.Contains(arch, "seam")
	depthHasSeam := strings.Contains(depth, "seam")

	if !archHasSeam || !depthHasSeam {
		t.Errorf("both reviewers must use 'seam': architecture-reviewer has seam=%v, depth-reviewer has seam=%v",
			archHasSeam, depthHasSeam)
	}
}

func TestArchitectureAndDepthReviewerShareModuleVocabulary(t *testing.T) {
	arch := readAgentFile(t, "agents/architecture-reviewer.md")
	depth := readAgentFile(t, "agents/depth-reviewer.md")

	archHasModule := strings.Contains(arch, "module")
	depthHasModule := strings.Contains(depth, "module")

	if !archHasModule || !depthHasModule {
		t.Errorf("both reviewers must use 'module': architecture-reviewer has module=%v, depth-reviewer has module=%v",
			archHasModule, depthHasModule)
	}
}

func TestArchitectureReviewerNeitherBoundaryNorComponent(t *testing.T) {
	content := readAgentFile(t, "agents/architecture-reviewer.md")
	hasBoundary := strings.Contains(strings.ToLower(content), "boundary")
	hasComponent := strings.Contains(strings.ToLower(content), "component")
	if hasBoundary || hasComponent {
		t.Errorf("agents/architecture-reviewer.md must not use old vocabulary: boundary=%v component=%v",
			hasBoundary, hasComponent)
	}
}
