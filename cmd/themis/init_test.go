package main

import (
	"strings"
	"testing"
)

func TestGuidance_GiteaMentionsMCPNotGh(t *testing.T) {
	g := guidance("gitea", "go", "themis-x:latest")
	if !strings.Contains(g, "Gitea MCP") {
		t.Errorf("gitea guidance must mention the Gitea MCP; got:\n%s", g)
	}
	if strings.Contains(g, "gh ") || strings.Contains(g, "gh label") {
		t.Errorf("gitea guidance must not mention gh; got:\n%s", g)
	}
}

func TestGuidance_GithubMentionsGhAndLabels(t *testing.T) {
	g := guidance("github", "go", "themis-x:latest")
	if !strings.Contains(g, "gh auth login") {
		t.Errorf("github guidance must tell the user to authenticate gh; got:\n%s", g)
	}
	if !strings.Contains(g, "gh label create ready-for-agent") {
		t.Errorf("github guidance must include the label-create command; got:\n%s", g)
	}
	if strings.Contains(g, "Gitea MCP") {
		t.Errorf("github guidance must not mention the Gitea MCP; got:\n%s", g)
	}
}

func TestGuidance_OtherPointsAtReferenceDocs(t *testing.T) {
	for _, prov := range []string{"github", "gitea"} {
		g := guidance(prov, "other", "themis-x:latest")
		if !strings.Contains(g, "docs/configuration-reference.md") {
			t.Errorf("other guidance (%s) must point at the reference docs; got:\n%s", prov, g)
		}
	}
}

func TestGuidance_AlwaysHasBaseSteps(t *testing.T) {
	for _, prov := range []string{"github", "gitea", ""} {
		g := guidance(prov, "go", "themis-x:latest")
		if !strings.Contains(g, "podman build -t themis-x:latest") {
			t.Errorf("guidance(%q) missing build-image step; got:\n%s", prov, g)
		}
		if !strings.Contains(g, ".env") || !strings.Contains(g, "CLAUDE_CODE_OAUTH_TOKEN") {
			t.Errorf("guidance(%q) missing credentials step; got:\n%s", prov, g)
		}
	}
}

func TestSkillsChoice(t *testing.T) {
	cases := []struct {
		idx     int
		install bool
		global  bool
	}{
		{0, true, false},
		{1, true, true},
		{2, false, false},
		{99, false, false},
	}
	for _, c := range cases {
		got := skillsChoice(c.idx)
		if got.install != c.install || got.global != c.global {
			t.Errorf("skillsChoice(%d) = %+v, want {install:%v global:%v}", c.idx, got, c.install, c.global)
		}
	}
}

func TestDecideInitMode(t *testing.T) {
	cases := []struct {
		contentSet, tty bool
		want            initMode
	}{
		{true, true, modeHeadless},
		{true, false, modeHeadless},
		{false, true, modeInteractive},
		{false, false, modeErrNoTTY},
	}
	for _, c := range cases {
		if got := decideInitMode(c.contentSet, c.tty); got != c.want {
			t.Errorf("decideInitMode(content=%v, tty=%v) = %d, want %d", c.contentSet, c.tty, got, c.want)
		}
	}
}

func TestDefaultImageTag(t *testing.T) {
	cases := []struct {
		workDir string
		want    string
	}{
		{"/home/me/MyProj", "themis-myproj:latest"},
		{"/home/me/bar baz", "themis-bar-baz:latest"},
		{"/home/me/already-lower", "themis-already-lower:latest"},
	}
	for _, c := range cases {
		if got := defaultImageTag(c.workDir); got != c.want {
			t.Errorf("defaultImageTag(%q) = %q, want %q", c.workDir, got, c.want)
		}
	}
}
