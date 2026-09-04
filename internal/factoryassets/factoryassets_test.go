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
