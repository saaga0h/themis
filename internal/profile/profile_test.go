package profile

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Load on missing file returns default ---

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error for missing profile, got: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil default profile")
	}

	// Default review agents: sonnet/haiku mix
	if p.Review.Agents.Security != "sonnet" {
		t.Errorf("default security agent: got %q, want %q", p.Review.Agents.Security, "sonnet")
	}
	if p.Review.Agents.Architecture != "sonnet" {
		t.Errorf("default architecture agent: got %q, want %q", p.Review.Agents.Architecture, "sonnet")
	}
	if p.Review.Agents.Complexity != "haiku" {
		t.Errorf("default complexity agent: got %q, want %q", p.Review.Agents.Complexity, "haiku")
	}
	if p.Review.Agents.Conventions != "haiku" {
		t.Errorf("default conventions agent: got %q, want %q", p.Review.Agents.Conventions, "haiku")
	}
	if p.Review.Agents.Coverage != "haiku" {
		t.Errorf("default coverage agent: got %q, want %q", p.Review.Agents.Coverage, "haiku")
	}
	if p.Review.Agents.Depth != "sonnet" {
		t.Errorf("default depth agent: got %q, want %q", p.Review.Agents.Depth, "sonnet")
	}

	// Default round3: auto
	if p.Review.Round3 != "auto" {
		t.Errorf("default round3: got %q, want %q", p.Review.Round3, "auto")
	}

	// Default test_fix_attempts: 3
	if p.Implement.TestFixAttempts != 3 {
		t.Errorf("default test_fix_attempts: got %d, want 3", p.Implement.TestFixAttempts)
	}

	// Default docs and refactor enabled
	if p.Docs.Enabled == nil || !*p.Docs.Enabled {
		t.Error("default docs.enabled should be true")
	}
	if p.Refactor.Enabled == nil || !*p.Refactor.Enabled {
		t.Error("default refactor.enabled should be true")
	}
}

// --- Load a valid profile file ---

func TestLoadValidProfile(t *testing.T) {
	dir := t.TempDir()
	content := `
review:
  agents:
    security: opus
    architecture: sonnet
    complexity: haiku
    conventions: haiku
    coverage: haiku
    numerical: skip
    depth: sonnet
  round3: always
  round3_surfaces:
    - auth
    - credentials

implement:
  model: sonnet
  test_fix_attempts: 2

refactor:
  enabled: false

docs:
  enabled: true
  skip_tiers: [3]

blocking:
  includes:
    - security
    - compile-failure
`
	writeProfile(t, dir, content)

	p, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Review.Agents.Security != "opus" {
		t.Errorf("security agent: got %q, want opus", p.Review.Agents.Security)
	}
	if p.Review.Agents.Numerical != "skip" {
		t.Errorf("numerical agent: got %q, want skip", p.Review.Agents.Numerical)
	}
	if p.Review.Round3 != "always" {
		t.Errorf("round3: got %q, want always", p.Review.Round3)
	}
	if len(p.Review.Round3Surfaces) != 2 {
		t.Errorf("round3_surfaces: got %d items, want 2", len(p.Review.Round3Surfaces))
	}
	if p.Implement.TestFixAttempts != 2 {
		t.Errorf("test_fix_attempts: got %d, want 2", p.Implement.TestFixAttempts)
	}
	if p.Refactor.Enabled == nil || *p.Refactor.Enabled {
		t.Error("refactor.enabled: got true, want false")
	}
	if len(p.Blocking.Includes) != 2 {
		t.Errorf("blocking.includes: got %d items, want 2", len(p.Blocking.Includes))
	}
}

// --- skip agent ---

func TestLoadSkipAgent(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, `
review:
  agents:
    numerical: skip
`)
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Review.Agents.Numerical != "skip" {
		t.Errorf("expected numerical agent to be skip, got %q", p.Review.Agents.Numerical)
	}
}

// --- Model override ---

func TestLoadModelOverride(t *testing.T) {
	for _, model := range []string{"haiku", "sonnet", "opus"} {
		dir := t.TempDir()
		writeProfile(t, dir, "review:\n  agents:\n    security: "+model+"\n")
		p, err := Load(dir)
		if err != nil {
			t.Fatalf("model %q: unexpected error: %v", model, err)
		}
		if p.Review.Agents.Security != model {
			t.Errorf("model %q: got %q", model, p.Review.Agents.Security)
		}
	}
}

// --- round3 values ---

func TestLoadRound3Values(t *testing.T) {
	for _, r3 := range []string{"auto", "always", "never"} {
		dir := t.TempDir()
		writeProfile(t, dir, "review:\n  round3: "+r3+"\n")
		p, err := Load(dir)
		if err != nil {
			t.Fatalf("round3 %q: unexpected error: %v", r3, err)
		}
		if p.Review.Round3 != r3 {
			t.Errorf("round3 %q: got %q", r3, p.Review.Round3)
		}
	}
}

// --- Invalid YAML ---

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "review: [not a map\n")
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadInvalidModelNameReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "review:\n  agents:\n    security: gpt4\n")
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid model name, got nil")
	}
}

func TestLoadInvalidRound3ValueReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "review:\n  round3: maybe\n")
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid round3 value, got nil")
	}
}

func TestLoadPartialProfileEnabledDefaultsToTrue(t *testing.T) {
	// A profile that has a refactor/docs section but omits enabled should default to true.
	dir := t.TempDir()
	writeProfile(t, dir, "refactor: {}\ndocs: {}\n")
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Refactor.Enabled == nil || !*p.Refactor.Enabled {
		t.Error("partial refactor section: expected enabled to default to true")
	}
	if p.Docs.Enabled == nil || !*p.Docs.Enabled {
		t.Error("partial docs section: expected enabled to default to true")
	}
}

func TestLoadUnknownFieldReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "unknown_field: true\n")
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for unknown field, got nil")
	}
}

// --- Round-trip ---

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	content := `
review:
  agents:
    security: opus
    architecture: sonnet
    complexity: haiku
    conventions: haiku
    coverage: haiku
    numerical: skip
    depth: sonnet
  round3: auto
  round3_surfaces:
    - auth

implement:
  model: sonnet
  test_fix_attempts: 3

refactor:
  enabled: true

docs:
  enabled: true
  skip_tiers: []

blocking:
  includes:
    - security
`
	writeProfile(t, dir, content)

	p1, err := Load(dir)
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	// Save and reload
	dir2 := t.TempDir()
	if err := Save(dir2, p1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	p2, err := Load(dir2)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if p2.Review.Agents.Security != p1.Review.Agents.Security {
		t.Errorf("round-trip security agent: %q vs %q", p2.Review.Agents.Security, p1.Review.Agents.Security)
	}
	if p2.Review.Round3 != p1.Review.Round3 {
		t.Errorf("round-trip round3: %q vs %q", p2.Review.Round3, p1.Review.Round3)
	}
	if p2.Implement.TestFixAttempts != p1.Implement.TestFixAttempts {
		t.Errorf("round-trip test_fix_attempts: %d vs %d", p2.Implement.TestFixAttempts, p1.Implement.TestFixAttempts)
	}
	if p1.Refactor.Enabled == nil || p2.Refactor.Enabled == nil || *p2.Refactor.Enabled != *p1.Refactor.Enabled {
		t.Errorf("round-trip refactor.enabled mismatch")
	}
}

// helpers

func writeProfile(t *testing.T, dir, content string) {
	t.Helper()
	themisDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themisDir, 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themisDir, "profile.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
