package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	return dir
}

func initGitRepoWithGiteaRemote(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := newTestGitRepo(t)
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "README.md")
	gitCmd(t, dir, "commit", "-m", "chore: initial commit")
	gitCmd(t, dir, "remote", "add", "origin", remoteURL)
	return dir
}

// AC8: resolveGiteaConfig infers owner, repo, apiBase from the git remote URL
// when no GITEA_* env vars are set.
func TestResolveGiteaConfig_InfersFromHTTPSRemote(t *testing.T) {
	dir := initGitRepoWithGiteaRemote(t, "https://gitea.example.com/owner/repo.git")
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")

	owner, repo, apiBase, err := resolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveGiteaConfig: %v", err)
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

// AC5: env vars override values inferred from the git remote
func TestResolveGiteaConfig_EnvVarsOverrideInferred(t *testing.T) {
	dir := initGitRepoWithGiteaRemote(t, "https://gitea.example.com/remote-owner/remote-repo.git")
	t.Setenv("GITEA_OWNER", "env-owner")
	t.Setenv("GITEA_REPO", "env-repo")
	t.Setenv("GITEA_API_URL", "https://env.gitea.example.com")

	owner, repo, apiBase, err := resolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveGiteaConfig: %v", err)
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

// AC5: only the set env var wins; unset fields fall through to the inferred value
func TestResolveGiteaConfig_PartialEnvVarOverride(t *testing.T) {
	dir := initGitRepoWithGiteaRemote(t, "https://gitea.example.com/remote-owner/remote-repo.git")
	t.Setenv("GITEA_OWNER", "env-owner")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")

	owner, repo, apiBase, err := resolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveGiteaConfig: %v", err)
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

// AC6: no origin remote → falls back to env vars rather than returning an error
func TestResolveGiteaConfig_FallsBackToEnvVarsWhenNoRemote(t *testing.T) {
	dir := newTestGitRepo(t)
	t.Setenv("GITEA_OWNER", "fallback-owner")
	t.Setenv("GITEA_REPO", "fallback-repo")
	t.Setenv("GITEA_API_URL", "https://fallback.gitea.example.com")

	owner, repo, apiBase, err := resolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveGiteaConfig should succeed via env var fallback when no remote: %v", err)
	}
	if owner != "fallback-owner" {
		t.Errorf("owner: got %q, want fallback-owner", owner)
	}
	if repo != "fallback-repo" {
		t.Errorf("repo: got %q, want fallback-repo", repo)
	}
	if apiBase != "https://fallback.gitea.example.com" {
		t.Errorf("apiBase: got %q, want https://fallback.gitea.example.com", apiBase)
	}
}

// AC6: unrecognized remote format → falls back to env vars rather than returning an error
func TestResolveGiteaConfig_FallsBackToEnvVarsForUnrecognizedRemote(t *testing.T) {
	dir := initGitRepoWithGiteaRemote(t, "git@github.com:owner/repo.git") // SCP-style, not a URL
	t.Setenv("GITEA_OWNER", "fallback-owner")
	t.Setenv("GITEA_REPO", "fallback-repo")
	t.Setenv("GITEA_API_URL", "https://fallback.gitea.example.com")

	owner, repo, apiBase, err := resolveGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveGiteaConfig should succeed via env var fallback for unrecognized remote: %v", err)
	}
	if owner != "fallback-owner" {
		t.Errorf("owner: got %q, want fallback-owner", owner)
	}
	if repo != "fallback-repo" {
		t.Errorf("repo: got %q, want fallback-repo", repo)
	}
	if apiBase != "https://fallback.gitea.example.com" {
		t.Errorf("apiBase: got %q, want https://fallback.gitea.example.com", apiBase)
	}
}

// AC7: neither remote nor env vars → returns a clear error
func TestResolveGiteaConfig_ReturnsErrorWhenNeitherSourceWorks(t *testing.T) {
	dir := newTestGitRepo(t)
	// no remote, no env vars
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")

	_, _, _, err := resolveGiteaConfig(context.Background(), dir)
	if err == nil {
		t.Error("resolveGiteaConfig must return an error when neither remote nor env vars provide a value")
	}
}
