package git

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GiteaConfig holds the inferred Gitea repository information.
type GiteaConfig struct {
	Owner   string
	Repo    string
	APIBase string
}

// InferGiteaConfig parses the git origin remote URL in dir and returns the
// owner, repo name, and API base URL. Supports https:// and ssh:// formats.
// SCP-style SSH remotes (git@host:path) are not supported and return an error.
func InferGiteaConfig(ctx context.Context, dir string) (*GiteaConfig, error) {
	out, err := runGit(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("no origin remote: %w", err)
	}
	remoteURL := strings.TrimSpace(out)

	u, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("parsing remote URL %q: %w", remoteURL, err)
	}

	if u.Scheme != "https" && u.Scheme != "ssh" {
		return nil, fmt.Errorf("unrecognized remote URL format %q (expected https:// or ssh://)", remoteURL)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("cannot parse owner/repo from remote URL path %q", u.Path)
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	apiBase := "https://" + u.Hostname()

	return &GiteaConfig{
		Owner:   owner,
		Repo:    repo,
		APIBase: apiBase,
	}, nil
}
