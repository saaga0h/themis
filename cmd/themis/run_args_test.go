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

func TestParseRunArgs_ProviderGitHub(t *testing.T) {
	args, err := parseRunArgs([]string{"--provider", "github"})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "github" {
		t.Errorf("provider: got %q, want %q", args.provider, "github")
	}
}

func TestParseRunArgs_DefaultProviderIsGitHub(t *testing.T) {
	args, err := parseRunArgs([]string{})
	if err != nil {
		t.Fatalf("parseRunArgs error: %v", err)
	}
	if args.provider != "github" {
		t.Errorf("default provider: got %q, want %q", args.provider, "github")
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
