package scrub

import (
	"strings"
	"testing"
)

// The tooling around this repo rewrites email-like "word@word" substrings in
// source files, which would poison any fixture containing a literal credential
// URL. We assemble the "@" at runtime from a non-literal so the on-disk source
// holds no email-shaped text; the string is a real "@" when the test runs.
const at = "\x40" // '@'

// The core #91 guarantee: an error built from a credentialed remote URL leaks
// neither the credential nor the (sensitive) host once both are known.
func TestScrub_CredentialedRemoteURL(t *testing.T) {
	s := New("toksecret", "internalhost")
	in := "git push https://user:toksecret" + at + "internalhost:3000/o/r failed: could not read from internalhost"
	out := s(in)

	if strings.Contains(out, "toksecret") {
		t.Errorf("token leaked: %q", out)
	}
	if strings.Contains(out, "internalhost") {
		t.Errorf("sensitive host leaked: %q", out)
	}
	if strings.Contains(out, "user:") {
		t.Errorf("URL userinfo leaked: %q", out)
	}
	if !strings.Contains(out, "https://") {
		t.Errorf("scheme should stay readable for diagnostics: %q", out)
	}
}

// A secret is redacted wherever it appears, not only inside a URL.
func TestScrub_SecretByValueAnywhere(t *testing.T) {
	s := New("TOKEN123")
	if got := s("Authorization: token TOKEN123"); strings.Contains(got, "TOKEN123") {
		t.Errorf("token leaked outside a URL: %q", got)
	}
}

// URL credentials are redacted even when the process doesn't hold the value.
func TestScrub_URLCredsWithoutKnownSecret(t *testing.T) {
	s := New() // no known secrets
	out := s("clone https://oauth2:abc123" + at + "codehost/x")
	if strings.Contains(out, "abc123") || strings.Contains(out, "oauth2:") {
		t.Errorf("embedded credential leaked: %q", out)
	}
}

// Empty/duplicate secrets are ignored and ordinary text is untouched.
func TestScrub_EmptySecretsAndCleanText(t *testing.T) {
	s := New("", "", "tok")
	const clean = "no verify commands declared; green gate is a no-op"
	if got := s(clean); got != clean {
		t.Errorf("clean text altered: %q", got)
	}
	if got := s(""); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}
}
