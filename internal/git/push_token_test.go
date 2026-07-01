package git

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGiteaPushURL(t *testing.T) {
	cases := []struct {
		name, apiBase, owner, repo, want string
	}{
		{"host only", "https://git.example", "the-collective", "heimdall", "https://git.example/the-collective/heimdall.git"},
		{"api path stripped", "https://git.example/api/v1", "o", "r", "https://git.example/o/r.git"},
	}
	for _, c := range cases {
		got, err := GiteaPushURL(c.apiBase, c.owner, c.repo)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: GiteaPushURL(%q,%q,%q) = %q, want %q", c.name, c.apiBase, c.owner, c.repo, got, c.want)
		}
	}
}

func TestGiteaPushURL_Invalid(t *testing.T) {
	if _, err := GiteaPushURL("://bad", "o", "r"); err == nil {
		t.Error("expected an error for an unparseable api base")
	}
}

// The auth header must carry the token as the Basic-auth username (Gitea's
// token-as-username form) and must never expose the token in cleartext.
func TestGiteaTokenAuthHeader(t *testing.T) {
	h := giteaTokenAuthHeader("secrettoken")
	const prefix = "Authorization: Basic "
	if !strings.HasPrefix(h, prefix) {
		t.Fatalf("header %q missing prefix %q", h, prefix)
	}
	if strings.Contains(h, "secrettoken") {
		t.Errorf("header must not contain the raw token: %q", h)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, prefix))
	if err != nil {
		t.Fatalf("credential is not valid base64: %v", err)
	}
	if string(decoded) != "secrettoken:" {
		t.Errorf("decoded credential = %q, want %q", decoded, "secrettoken:")
	}
}
