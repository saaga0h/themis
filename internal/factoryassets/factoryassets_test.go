package factoryassets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	themis "github.com/saaga0h/themis"
	"github.com/saaga0h/themis/internal/factoryassets"
)

// Materialize writes exactly the manifest'd factory-internal skills (as trees)
// and agents (as files) into the target .claude dir, plus settings.json.
func TestMaterialize_WritesManifestAssetsAndSettings(t *testing.T) {
	claude := filepath.Join(t.TempDir(), ".claude")
	if err := factoryassets.Materialize(themis.Assets, claude); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	for _, p := range []string{
		"skills/test-red/SKILL.md",
		"skills/pr-composition/SKILL.md",
		"agents/test-architect.md",
		"agents/test-writer.md",
		"agents/test-runner.md",
		"agents/security-reviewer.md",
	} {
		if _, err := os.Stat(filepath.Join(claude, p)); err != nil {
			t.Errorf("expected %s materialized: %v", p, err)
		}
	}

	// Interactive-only assets must NOT be materialized by a factory run.
	if _, err := os.Stat(filepath.Join(claude, "skills", "grill-me")); err == nil {
		t.Error("grill-me (interactive) should not be materialized into the factory .claude")
	}

	data, err := os.ReadFile(filepath.Join(claude, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json: %v", err)
	}
	if !strings.Contains(string(data), `"disableBundledSkills": true`) {
		t.Errorf("settings.json missing disableBundledSkills: %s", data)
	}
}

// InstallInteractive writes the named interactive skills, all commands, and every
// agent except the pipeline-only ones — and none of the factory-internal skills.
func TestInstallInteractive_WritesInteractiveSetNotPipelineOnly(t *testing.T) {
	claude := filepath.Join(t.TempDir(), ".claude")
	if err := factoryassets.InstallInteractive(themis.Assets, claude); err != nil {
		t.Fatalf("InstallInteractive: %v", err)
	}

	for _, s := range []string{"grill-me", "split-walker", "issue-writer", "contract-drafter", "review-walker", "pr-review"} {
		if _, err := os.Stat(filepath.Join(claude, "skills", s, "SKILL.md")); err != nil {
			t.Errorf("expected interactive skill %s: %v", s, err)
		}
	}
	for _, c := range []string{"review.md", "document.md", "context.md"} {
		if _, err := os.Stat(filepath.Join(claude, "commands", c)); err != nil {
			t.Errorf("expected command %s: %v", c, err)
		}
	}
	// An interactive (review-panel) agent is installed; a pipeline-only one is not.
	if _, err := os.Stat(filepath.Join(claude, "agents", "architecture-reviewer.md")); err != nil {
		t.Errorf("expected interactive agent architecture-reviewer: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claude, "agents", "test-architect.md")); err == nil {
		t.Error("pipeline-only agent test-architect must not be installed interactively")
	}
	// Factory-internal skills are not part of the interactive install.
	if _, err := os.Stat(filepath.Join(claude, "skills", "test-red")); err == nil {
		t.Error("factory-internal skill test-red must not be in the interactive install")
	}
}
