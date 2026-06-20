package profile

import (
	"reflect"
	"testing"
)

// Tests for issue #67: dead per-reviewer model fields and BlockingConfig.Includes
// must be removed from the profile schema. These tests currently FAIL because
// the fields still exist; they pass once the implementation removes them.

// --- AC1: AgentConfig must not define per-reviewer model fields ---

// TestAgentConfigHasNoPerReviewerModelFields asserts that AgentConfig contains
// no Architecture, Complexity, Conventions, Coverage, Numerical, or Depth fields.
// Only Security is a live field (read by profileLoader at runtime).
func TestAgentConfigHasNoPerReviewerModelFields(t *testing.T) {
	agentType := reflect.TypeOf(AgentConfig{})
	for _, field := range []string{"Architecture", "Complexity", "Conventions", "Coverage", "Numerical", "Depth"} {
		if _, ok := agentType.FieldByName(field); ok {
			t.Errorf("AgentConfig must not have field %q (dead config, issue #67)", field)
		}
	}
}

// TestLoadRejectsPerReviewerModelFields asserts that loading a profile YAML that
// sets any removed per-reviewer key returns an error. The decoder uses
// KnownFields(true), so unknown struct fields produce a parse error.
func TestLoadRejectsPerReviewerModelFields(t *testing.T) {
	for _, yamlKey := range []string{"architecture", "complexity", "conventions", "coverage", "numerical", "depth"} {
		t.Run(yamlKey, func(t *testing.T) {
			dir := t.TempDir()
			writeProfile(t, dir, "review:\n  agents:\n    "+yamlKey+": sonnet\n")
			_, err := Load(dir)
			if err == nil {
				t.Errorf("Load with removed agent field %q: expected error, got nil", yamlKey)
			}
		})
	}
}

// --- AC2: BlockingConfig must not have an Includes field ---

// TestBlockingConfigHasNoIncludesField asserts that BlockingConfig contains no
// Includes field. Blocking is determined by finding severity, not by this list.
func TestBlockingConfigHasNoIncludesField(t *testing.T) {
	if _, ok := reflect.TypeOf(BlockingConfig{}).FieldByName("Includes"); ok {
		t.Error(`BlockingConfig must not have field "Includes" (dead config, issue #67)`)
	}
}

// TestLoadRejectsBlockingIncludes asserts that loading a profile YAML with a
// blocking.includes key returns an error (unknown field via KnownFields).
func TestLoadRejectsBlockingIncludes(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "blocking:\n  includes:\n    - security\n")
	_, err := Load(dir)
	if err == nil {
		t.Error("Load with blocking.includes: expected error (removed field), got nil")
	}
}
