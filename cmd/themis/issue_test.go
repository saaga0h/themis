package main

import (
	"testing"
)

// AC: themis issue <number> CLI subcommand exists and parses issue number and --provider flag

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
