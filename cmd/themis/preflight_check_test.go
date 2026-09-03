package main

import (
	"errors"
	"strings"
	"testing"
)

// fakeLookPath resolves only the named commands; everything else is "not found".
func fakeLookPath(available ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, c := range available {
		set[c] = true
	}
	return func(cmd string) (string, error) {
		if set[cmd] {
			return "/usr/bin/" + cmd, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

func TestPreflightCheckRunnability_BlocksOnMissingBinary(t *testing.T) {
	checks := []string{"nomad job validate spec.nomad"}
	err := preflightCheckRunnability(56, checks, fakeLookPath("grep"))
	if err == nil {
		t.Fatal("expected a block when a check needs a missing binary, got nil")
	}
	msg := err.Error()
	// The message must be a self-contained, actionable diagnostic.
	for _, want := range []string{"nomad", "#56", "sandbox image", "operator step", "nomad job validate spec.nomad"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should contain %q; got:\n%s", want, msg)
		}
	}
}

func TestPreflightCheckRunnability_PassesWhenToolsAvailable(t *testing.T) {
	checks := []string{"! grep -rq X ./...", "test -f spec && nomad validate spec"}
	if err := preflightCheckRunnability(1, checks, fakeLookPath("grep", "nomad")); err != nil {
		t.Errorf("expected nil when every command resolves, got %v", err)
	}
}

func TestPreflightCheckRunnability_BuiltinsOnlyNeverLooksUp(t *testing.T) {
	checks := []string{"test ! -e foo.go && test ! -e bar.go"}
	called := false
	lp := func(cmd string) (string, error) { called = true; return "", errors.New("nope") }
	if err := preflightCheckRunnability(1, checks, lp); err != nil {
		t.Errorf("expected nil for a builtins-only check, got %v", err)
	}
	if called {
		t.Error("lookPath must not be called for a check with no external commands")
	}
}

func TestPreflightCheckRunnability_NoChecks(t *testing.T) {
	if err := preflightCheckRunnability(1, nil, fakeLookPath()); err != nil {
		t.Errorf("expected nil when there are no checks, got %v", err)
	}
}
