package git

import (
	"context"
	"os/exec"
	"testing"
)

// initTestRepoWithRemote creates a git repo (via initTestRepo) and adds an origin remote.
func initTestRepoWithRemote(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := initTestRepo(t)
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin %q: %v\n%s", remoteURL, err, out)
	}
	return dir
}

// AC1 + AC3: HTTPS remote → owner, repo, apiBase
func TestInferGiteaConfig_ParsesHTTPSRemote(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepoWithRemote(t, "https://gitea.example.com/owner/repo.git")

	cfg, err := InferGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("InferGiteaConfig: %v", err)
	}
	if cfg.Owner != "owner" {
		t.Errorf("Owner: got %q, want %q", cfg.Owner, "owner")
	}
	if cfg.Repo != "repo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "repo")
	}
	if cfg.APIBase != "https://gitea.example.com" {
		t.Errorf("APIBase: got %q, want %q", cfg.APIBase, "https://gitea.example.com")
	}
}

// AC2: SSH remote → owner, repo, apiBase (scheme becomes https)
func TestInferGiteaConfig_ParsesSSHRemote(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepoWithRemote(t, "ssh://git@gitea.example.com:2222/owner/repo.git")

	cfg, err := InferGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("InferGiteaConfig: %v", err)
	}
	if cfg.Owner != "owner" {
		t.Errorf("Owner: got %q, want %q", cfg.Owner, "owner")
	}
	if cfg.Repo != "repo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "repo")
	}
	if cfg.APIBase != "https://gitea.example.com" {
		t.Errorf("APIBase: got %q, want %q", cfg.APIBase, "https://gitea.example.com")
	}
}

// AC4: .git suffix is stripped from the repo name
func TestInferGiteaConfig_StripsGitSuffix(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepoWithRemote(t, "https://gitea.example.com/owner/myrepo.git")

	cfg, err := InferGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("InferGiteaConfig: %v", err)
	}
	if cfg.Repo != "myrepo" {
		t.Errorf("Repo: got %q, want %q (no .git suffix)", cfg.Repo, "myrepo")
	}
}

// AC4: also works when the remote URL has no .git suffix
func TestInferGiteaConfig_WorksWithoutGitSuffix(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepoWithRemote(t, "https://gitea.example.com/owner/myrepo")

	cfg, err := InferGiteaConfig(context.Background(), dir)
	if err != nil {
		t.Fatalf("InferGiteaConfig: %v", err)
	}
	if cfg.Repo != "myrepo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "myrepo")
	}
}

// AC6 (remote side): no origin remote → returns an error
func TestInferGiteaConfig_ReturnsErrorWhenNoRemote(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepo(t) // no remote added

	_, err := InferGiteaConfig(context.Background(), dir)
	if err == nil {
		t.Error("InferGiteaConfig must return an error when there is no origin remote")
	}
}

// AC6 (remote side): SCP-style SSH (git@host:path) is not a parseable URL → returns an error
func TestInferGiteaConfig_ReturnsErrorForUnrecognizedRemoteFormat(t *testing.T) {
	t.Setenv("GITEA_OWNER", "")
	t.Setenv("GITEA_REPO", "")
	t.Setenv("GITEA_API_URL", "")
	dir := initTestRepoWithRemote(t, "git@github.com:owner/repo.git")

	_, err := InferGiteaConfig(context.Background(), dir)
	if err == nil {
		t.Error("InferGiteaConfig must return an error for unrecognized remote format (SCP-style SSH)")
	}
}
