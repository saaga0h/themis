package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

// themis run --provider gitea subcommand exists (not "unknown command")

func TestRunSubcommandIsRecognized(t *testing.T) {
	// Without a live tracker, the command will fail — but it must NOT print
	// "unknown command: run", which would mean the subcommand is not wired up.
	cmd := exec.Command(binary, "run", "--provider", "gitea", "--dry-run")
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "unknown command") {
		t.Errorf("themis run must be a recognized subcommand; got: %s", string(out))
	}
}
