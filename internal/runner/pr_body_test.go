package runner

import "testing"

func TestExtractPRBody_BetweenMarkers(t *testing.T) {
	stdout := "Build succeeded and all tests pass. Here's the PR body:\n\n" +
		prBodyStartMarker + "\nCloses #2\n\n**Status:** All ACs passed.\n" + prBodyEndMarker +
		"\n\nSTEP COMPLETE\n"
	got := extractPRBody(stdout)
	want := "Closes #2\n\n**Status:** All ACs passed."
	if got != want {
		t.Errorf("extractPRBody between markers:\n got: %q\nwant: %q", got, want)
	}
}

func TestExtractPRBody_StripsPreambleAndCompletion(t *testing.T) {
	// The leak we're fixing: preamble before the body and STEP COMPLETE after it
	// must not appear in the result.
	got := extractPRBody("Here's the PR body:\n\n" + prBodyStartMarker + "\nCloses #2\n" + prBodyEndMarker + "\nSTEP COMPLETE")
	if got != "Closes #2" {
		t.Errorf("got %q, want %q", got, "Closes #2")
	}
	if containsAny(got, "Here's the PR body", "STEP COMPLETE") {
		t.Errorf("agent framing leaked into body: %q", got)
	}
}

func TestExtractPRBody_FallbackWithoutMarkers(t *testing.T) {
	// No markers (older/misbehaving agent) — still strip a trailing STEP COMPLETE.
	got := extractPRBody("Closes #2\n\nBody text.\n\nSTEP COMPLETE")
	want := "Closes #2\n\nBody text."
	if got != want {
		t.Errorf("fallback:\n got: %q\nwant: %q", got, want)
	}
}

func TestExtractPRBody_FallbackStripsCodeFence(t *testing.T) {
	got := extractPRBody("```markdown\nCloses #2\n```\nSTEP COMPLETE")
	if got != "Closes #2" {
		t.Errorf("got %q, want %q", got, "Closes #2")
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
