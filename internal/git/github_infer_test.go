package git

import "testing"

// parseRemoteHostOwnerRepo is a pure string parser — these cases feed it literals
// and assert the split. It runs no git and touches no network, so the owner/repo
// names are deliberately synthetic placeholders, not real repositories.
func TestParseRemoteHostOwnerRepo(t *testing.T) {
	cases := []struct {
		name, remote, host, owner, repo string
	}{
		{"https with .git", "https://github.com/acme/widget.git", "github.com", "acme", "widget"},
		{"https no .git", "https://github.com/acme/widget", "github.com", "acme", "widget"},
		{"scp default", "git@github.com:acme/widget.git", "github.com", "acme", "widget"},
		{"scp no user", "github.com:acme/widget.git", "github.com", "acme", "widget"},
		{"ssh url", "ssh://git@github.com/acme/widget.git", "github.com", "acme", "widget"},
		{"enterprise host", "git@ghe.example.com:team/svc.git", "ghe.example.com", "team", "svc"},
	}
	for _, c := range cases {
		host, owner, repo, err := parseRemoteHostOwnerRepo(c.remote)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if host != c.host || owner != c.owner || repo != c.repo {
			t.Errorf("%s: parse(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.name, c.remote, host, owner, repo, c.host, c.owner, c.repo)
		}
	}
}

func TestParseRemoteHostOwnerRepo_Invalid(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-remote",
		"https://github.com/owneronly",
		"git@github.com:owneronly",
		"https://github.com/",
	} {
		if _, _, _, err := parseRemoteHostOwnerRepo(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}
