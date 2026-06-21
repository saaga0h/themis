package main

import (
	"testing"
)

// themis issue <number> CLI subcommand exists and parses issue number and --provider flag

func TestParseIssueArgs_ValidGitHub(t *testing.T) {
	args, err := parseIssueArgs([]string{"42", "--provider", "github"})
	if err != nil {
		t.Fatalf("parseIssueArgs error: %v", err)
	}
	if args.number != 42 {
		t.Errorf("number: got %d, want 42", args.number)
	}
	if args.provider != "github" {
		t.Errorf("provider: got %q, want %q", args.provider, "github")
	}
}

func TestParseIssueArgs_ValidGitea(t *testing.T) {
	args, err := parseIssueArgs([]string{"11", "--provider", "gitea"})
	if err != nil {
		t.Fatalf("parseIssueArgs error: %v", err)
	}
	if args.number != 11 {
		t.Errorf("number: got %d, want 11", args.number)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
}

func TestParseIssueArgs_DefaultProviderIsGitHub(t *testing.T) {
	args, err := parseIssueArgs([]string{"7"})
	if err != nil {
		t.Fatalf("parseIssueArgs error: %v", err)
	}
	if args.provider != "github" {
		t.Errorf("default provider: got %q, want %q", args.provider, "github")
	}
}

func TestParseIssueArgs_MissingNumber(t *testing.T) {
	_, err := parseIssueArgs([]string{"--provider", "github"})
	if err == nil {
		t.Error("parseIssueArgs should return error when issue number is missing")
	}
}

func TestParseIssueArgs_NonNumericNumber(t *testing.T) {
	_, err := parseIssueArgs([]string{"abc", "--provider", "github"})
	if err == nil {
		t.Error("parseIssueArgs should return error for non-numeric issue number")
	}
}

func TestParseIssueArgs_InvalidProvider(t *testing.T) {
	_, err := parseIssueArgs([]string{"42", "--provider", "unknown"})
	if err == nil {
		t.Error("parseIssueArgs should return error for unknown provider")
	}
}

// --max-turns flag and default (issue #52)

func TestParseIssueArgs_MaxTurnsFlag(t *testing.T) {
	args, err := parseIssueArgs([]string{"42", "--provider", "gitea", "--max-turns", "300"})
	if err != nil {
		t.Fatalf("parseIssueArgs error: %v", err)
	}
	if args.number != 42 {
		t.Errorf("number: got %d, want 42", args.number)
	}
	if args.provider != "gitea" {
		t.Errorf("provider: got %q, want %q", args.provider, "gitea")
	}
	if args.maxTurns != 300 {
		t.Errorf("maxTurns: got %d, want 300", args.maxTurns)
	}
}

func TestParseIssueArgs_MaxTurnsRejectsNonPositive(t *testing.T) {
	for _, v := range []string{"0", "-1"} {
		if _, err := parseIssueArgs([]string{"42", "--max-turns", v}); err == nil {
			t.Errorf("parseIssueArgs(--max-turns %s) should error on a non-positive value", v)
		}
	}
}

func TestParseIssueArgs_DefaultMaxTurns(t *testing.T) {
	args, err := parseIssueArgs([]string{"42", "--provider", "gitea"})
	if err != nil {
		t.Fatalf("parseIssueArgs error: %v", err)
	}
	if args.maxTurns != defaultMaxTurns {
		t.Errorf("maxTurns default: got %d, want defaultMaxTurns (%d)", args.maxTurns, defaultMaxTurns)
	}
}

func TestDefaultMaxTurns_Is250(t *testing.T) {
	if defaultMaxTurns != 250 {
		t.Errorf("defaultMaxTurns: got %d, want 250", defaultMaxTurns)
	}
}
