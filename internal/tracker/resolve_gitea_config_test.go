package tracker_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/saaga0h/themis/internal/tracker"
)

func resolveGiteaConfigGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func resolveGiteaConfigNewTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolveGiteaConfigGitCmd(t, dir, "init", "--initial-branch=main")
	resolveGiteaConfigGitCmd(t, dir, "config", "user.email", "test@test.com")
	resolveGiteaConfigGitCmd(t, dir, "config", "user.name", "Test")
	return dir
}

func resolveGiteaConfigInitGitRepoWithGiteaRemote(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := resolveGiteaConfigNewTestGitRepo(t)
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolveGiteaConfigGitCmd(t, dir, "add", "README.md")
	resolveGiteaConfigGitCmd(t, dir, "commit", "-m", "chore: initial commit")
	resolveGiteaConfigGitCmd(t, dir, "remote", "add", "origin", remoteURL)
	return dir
}

func TestResolveGiteaConfig_InfersFromHTTPSRemote(t *testing.T) {
	dir := resolveGiteaConfigInitGitRepoWithGiteaRemote(t, "https://gitea.example.com/owner/repo.git")
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")

	owner, repo, apiBase, err := tracker.ResolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("ResolveGiteaConfig: %v", err)
	}
	if owner != "owner" {
		t.Errorf("owner: got %q, want %q", owner, "owner")
	}
	if repo != "repo" {
		t.Errorf("repo: got %q, want %q", repo, "repo")
	}
	if apiBase != "https://gitea.example.com" {
		t.Errorf("apiBase: got %q, want %q", apiBase, "https://gitea.example.com")
	}
}

func TestResolveGiteaConfig_EnvVarsOverrideInferred(t *testing.T) {
	dir := resolveGiteaConfigInitGitRepoWithGiteaRemote(t, "https://gitea.example.com/remote-owner/remote-repo.git")
	t.Setenv("GITEA_OWNER", "env-owner")
	t.Setenv("GITEA_REPO", "env-repo")
	t.Setenv("GITEA_API_URL", "https://env.gitea.example.com")

	owner, repo, apiBase, err := tracker.ResolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("ResolveGiteaConfig: %v", err)
	}
	if owner != "env-owner" {
		t.Errorf("owner: got %q, want env-owner (env var must override inferred)", owner)
	}
	if repo != "env-repo" {
		t.Errorf("repo: got %q, want env-repo (env var must override inferred)", repo)
	}
	if apiBase != "https://env.gitea.example.com" {
		t.Errorf("apiBase: got %q, want https://env.gitea.example.com (env var must override inferred)", apiBase)
	}
}

func TestResolveGiteaConfig_PartialEnvVarOverride(t *testing.T) {
	dir := resolveGiteaConfigInitGitRepoWithGiteaRemote(t, "https://gitea.example.com/remote-owner/remote-repo.git")
	t.Setenv("GITEA_OWNER", "env-owner")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")

	owner, repo, apiBase, err := tracker.ResolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("ResolveGiteaConfig: %v", err)
	}
	if owner != "env-owner" {
		t.Errorf("owner: got %q, want env-owner", owner)
	}
	if repo != "remote-repo" {
		t.Errorf("repo: got %q, want remote-repo (inferred from remote)", repo)
	}
	if apiBase != "https://gitea.example.com" {
		t.Errorf("apiBase: got %q, want https://gitea.example.com (inferred from remote)", apiBase)
	}
}
