package prompt

import (
	"strings"
	"testing"
)

func TestSubstituteBasic(t *testing.T) {
	result, err := Substitute("Hello, {{NAME}}!", map[string]string{"NAME": "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("got %q, want %q", result, "Hello, World!")
	}
}

func TestSubstituteMultiplePlaceholders(t *testing.T) {
	tmpl := "Issue #{{ISSUE_NUMBER}}\n\n{{ACCEPTANCE_CRITERIA}}\n\n{{CODING_STANDARDS}}"
	args := map[string]string{
		"ISSUE_NUMBER":        "42",
		"ACCEPTANCE_CRITERIA": "- [ ] thing works",
		"CODING_STANDARDS":    "Use stdlib only.",
	}
	result, err := Substitute(tmpl, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Issue #42") {
		t.Error("expected issue number substitution")
	}
	if !strings.Contains(result, "thing works") {
		t.Error("expected AC substitution")
	}
	if !strings.Contains(result, "Use stdlib only.") {
		t.Error("expected coding standards substitution")
	}
}

func TestSubstituteSamePlaceholderMultipleTimes(t *testing.T) {
	tmpl := "{{X}} and {{X}} again"
	result, err := Substitute(tmpl, map[string]string{"X": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello and hello again" {
		t.Errorf("got %q, want %q", result, "hello and hello again")
	}
}

func TestSubstituteEmptyValue(t *testing.T) {
	result, err := Substitute("prefix{{EMPTY}}suffix", map[string]string{"EMPTY": ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "prefixsuffix" {
		t.Errorf("got %q, want %q", result, "prefixsuffix")
	}
}

func TestSubstituteNoPlaceholders(t *testing.T) {
	result, err := Substitute("no placeholders here", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "no placeholders here" {
		t.Errorf("got %q, want %q", result, "no placeholders here")
	}
}

func TestSubstituteMissingArgReturnsError(t *testing.T) {
	_, err := Substitute("Hello {{NAME}} and {{MISSING}}", map[string]string{"NAME": "World"})
	if err == nil {
		t.Error("expected error for missing arg, got nil")
	}
	if !strings.Contains(err.Error(), "MISSING") {
		t.Errorf("error should mention the missing key, got: %v", err)
	}
}

func TestSubstituteUnusedArgReturnsError(t *testing.T) {
	_, err := Substitute("Hello {{NAME}}", map[string]string{"NAME": "World", "TYPO": "unused"})
	if err == nil {
		t.Error("expected error for unused arg, got nil")
	}
	if !strings.Contains(err.Error(), "TYPO") {
		t.Errorf("error should mention the unused key, got: %v", err)
	}
}

func TestSubstituteNestedBraces(t *testing.T) {
	// Single braces should be left alone; only {{...}} is substituted
	tmpl := "value={not_a_placeholder} and {{KEY}}"
	result, err := Substitute(tmpl, map[string]string{"KEY": "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "{not_a_placeholder}") {
		t.Errorf("single-brace content should pass through unchanged, got: %s", result)
	}
	if !strings.Contains(result, "ok") {
		t.Errorf("double-brace placeholder should be substituted, got: %s", result)
	}
}

func TestSubstituteEmptyTemplate(t *testing.T) {
	result, err := Substitute("", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("got %q, want empty string", result)
	}
}

func TestSubstituteLowercasePlaceholderPassesThrough(t *testing.T) {
	// Keys must be uppercase. Lowercase placeholders are not recognised;
	// they pass through unchanged without error.
	result, err := Substitute("{{UPPER}} and {{lower}}", map[string]string{"UPPER": "yes"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "{{lower}}") {
		t.Errorf("lowercase placeholder should pass through unchanged, got: %s", result)
	}
	if !strings.Contains(result, "yes") {
		t.Errorf("uppercase placeholder should be substituted, got: %s", result)
	}
}

func TestSubstituteAllPlaceholdersReplaced(t *testing.T) {
	tmpl := "{{A}}{{B}}{{C}}"
	args := map[string]string{"A": "1", "B": "2", "C": "3"}
	result, err := Substitute(tmpl, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "123" {
		t.Errorf("got %q, want %q", result, "123")
	}
}
