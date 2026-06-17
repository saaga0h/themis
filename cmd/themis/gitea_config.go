package main

import (
	"context"
	"fmt"
	"os"

	"git.home.federation.fi/lavernea/themis/internal/git"
)

// resolveGiteaConfig determines owner, repo, and apiBase by inferring from the
// git remote first, then letting environment variables override individual fields.
// Returns an error only when neither source provides a value.
func resolveGiteaConfig(ctx context.Context, dir string) (owner, repo, apiBase string, err error) {
	inferred, inferErr := git.InferGiteaConfig(ctx, dir)
	if inferErr == nil {
		owner = inferred.Owner
		repo = inferred.Repo
		apiBase = inferred.APIBase
	}

	if v := os.Getenv("GITEA_OWNER"); v != "" {
		owner = v
	}
	if v := os.Getenv("GITEA_REPO"); v != "" {
		repo = v
	}
	if v := os.Getenv("GITEA_API_URL"); v != "" {
		apiBase = v
	}

	if owner == "" || repo == "" || apiBase == "" {
		if inferErr != nil {
			return "", "", "", fmt.Errorf("cannot determine Gitea config: remote inference failed (%v) and GITEA_OWNER/GITEA_REPO/GITEA_API_URL are not set", inferErr)
		}
		return "", "", "", fmt.Errorf("cannot determine Gitea config: set GITEA_OWNER, GITEA_REPO, and GITEA_API_URL")
	}

	return owner, repo, apiBase, nil
}
