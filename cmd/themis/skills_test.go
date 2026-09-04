package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSkills_GlobalInstallsToHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)        // os.UserHomeDir on Unix
	t.Setenv("USERPROFILE", tmp) // os.UserHomeDir on Windows

	if err := runSkills([]string{"install", "--global"}); err != nil {
		t.Fatalf("runSkills install --global: %v", err)
	}
	for _, p := range []string{
		filepath.Join(".claude", "skills", "grill-me", "SKILL.md"),
		filepath.Join(".claude", "commands", "review.md"),
		filepath.Join(".claude", "agents", "architecture-reviewer.md"),
	} {
		if _, err := os.Stat(filepath.Join(tmp, p)); err != nil {
			t.Errorf("expected %s installed under home: %v", p, err)
		}
	}
}

func TestRunSkills_BadUsage(t *testing.T) {
	if err := runSkills(nil); err == nil {
		t.Error("expected usage error for no args")
	}
	if err := runSkills([]string{"install", "--bogus"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}
