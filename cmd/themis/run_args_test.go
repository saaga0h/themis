package main

import "testing"

// themis run --provider gitea subcommand exists and supports --provider and --dry-run flags

func TestParseRunArgs_ProviderGitea(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
}

func TestParseRunArgs_TemplatesFlag(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea", "--templates", "/tmp/tpl"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.templates != "/tmp/tpl" {
		t.Errorf("templates: got %q, want %q", args.templates, "/tmp/tpl")
	}
}

func TestParseRunArgs_ProviderGitHub(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "github"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "github" {
		t.Errorf("provider: got %q, want %q", args.provider, "github")
	}
}

func TestParseRunArgs_DefaultProviderIsUnset(t *testing.T) {
	// No --provider leaves it unset; resolveProvider applies workflow.yaml then the
	// github default.
	args, err := parseRunArgs([]string{})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "" {
		t.Errorf("default provider: got %q, want unset (\"\")", args.provider)
	}
}

func TestResolveProvider(t *testing.T) {
	cases := []struct{ flag, cfg, inferred, want string }{
		{"", "", "", "github"},                 // nothing → default
		{"", "", "gitea", "gitea"},             // inferred from the git remote
		{"", "", "github", "github"},           // inferred github
		{"", "gitea", "github", "gitea"},       // config beats inference
		{"github", "gitea", "gitea", "github"}, // flag beats all
	}
	for _, c := range cases {
		if got := resolveProvider(c.flag, c.cfg, c.inferred); got != c.want {
			t.Errorf("resolveProvider(%q,%q,%q) = %q, want %q", c.flag, c.cfg, c.inferred, got, c.want)
		}
	}
}

func TestParseRunArgs_DryRunFlag(t *testing.T) {
	args, err := parseRunArgs([]string{"--dry-run"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if !args.dryRun {
		t.Error("parseRunArgs should set dryRun=true when --dry-run is given")
	}
}

func TestParseRunArgs_DryRunWithProvider(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea", "--dry-run"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
	if !args.dryRun {
		t.Error("dryRun should be true")
	}
}

func TestParseRunArgs_DryRunDefaultsToFalse(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
	if args.dryRun {
		t.Error("dryRun should default to false when --dry-run is not given")
	}
}

func TestParseRunArgs_InvalidProvider(t *testing.T) {
	_, err := parseRunArgs([]string{"--provider", "unknown"})
	if err == nil {
		t.Error("parseRunArgs should return error for unknown provider")
	}
}

func TestParseRunArgs_MissingProviderValue(t *testing.T) {
	_, err := parseRunArgs([]string{"--provider"})
	if err == nil {
		t.Error("parseRunArgs should return error when --provider has no value")
	}
}

// --max-turns flag and default (issue #52)

func TestParseRunArgs_MaxTurnsFlag(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea", "--max-turns", "300"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
	if args.maxTurns != 300 {
		t.Errorf("maxTurns: got %d, want 300", args.maxTurns)
	}
}

func TestParseRunArgs_MaxTurnsRejectsNonPositive(t *testing.T) {
	for _, v := range []string{"0", "-1"} {
		if _, err := parseRunArgs([]string{"--provider", "gitea", "--max-turns", v}); err == nil {
			t.Errorf("parseRunArgs(--max-turns %s) should error on a non-positive value", v)
		}
	}
}

func TestParseRunArgs_DefaultMaxTurns(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "gitea"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.maxTurns != defaultMaxTurns {
		t.Errorf("maxTurns default: got %d, want defaultMaxTurns (%d)", args.maxTurns, defaultMaxTurns)
	}
}
