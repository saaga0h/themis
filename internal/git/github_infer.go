package git

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GitHubPushURL derives the https push URL for the origin remote in dir, e.g.
// "https://github.com/owner/repo.git". It handles all three remote forms git
// writes — https:// , ssh:// , and the scp-style git@host:owner/repo default that
// `git clone git@github.com:...` and the GitHub UI produce — because the factory
// must push over https-with-token inside the sandbox regardless of how the user's
// origin is configured (see PushBranchWithToken). The returned URL carries no
// credentials; authentication is supplied separately.
func GitHubPushURL(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("no origin remote: %w", err)
	}
	host, owner, repo, err := parseRemoteHostOwnerRepo(strings.TrimSpace(out))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo), nil
}

// parseRemoteHostOwnerRepo extracts the host, owner, and repo from a git remote
// URL in any of the three forms git emits:
//
//	https://github.com/owner/repo.git
//	ssh://git@github.com/owner/repo.git
//	git@github.com:owner/repo.git        (scp-style — the common default)
//
// The ".git" suffix is optional. A URL that names fewer than two path segments,
// or an unrecognized shape, is an error.
func parseRemoteHostOwnerRepo(remoteURL string) (host, owner, repo string, err error) {
	var hostPart, pathPart string
	switch {
	case strings.Contains(remoteURL, "://"):
		u, perr := url.Parse(remoteURL)
		if perr != nil {
			return "", "", "", fmt.Errorf("parsing remote URL %q: %w", remoteURL, perr)
		}
		hostPart, pathPart = u.Hostname(), u.Path
	case isSCPRemote(remoteURL):
		// [user@]host:owner/repo — split on the first colon; the host may carry a
		// leading "user@" (typically "git@") to strip.
		i := strings.IndexByte(remoteURL, ':')
		hostPart, pathPart = remoteURL[:i], remoteURL[i+1:]
		if at := strings.LastIndexByte(hostPart, '@'); at >= 0 {
			hostPart = hostPart[at+1:]
		}
	default:
		return "", "", "", fmt.Errorf("unrecognized remote URL format %q", remoteURL)
	}

	parts := strings.Split(strings.Trim(pathPart, "/"), "/")
	if hostPart == "" || len(parts) < 2 || parts[0] == "" {
		return "", "", "", fmt.Errorf("cannot parse host/owner/repo from remote %q", remoteURL)
	}
	owner = parts[len(parts)-2]
	repo = strings.TrimSuffix(parts[len(parts)-1], ".git")
	if repo == "" {
		return "", "", "", fmt.Errorf("cannot parse repo from remote %q", remoteURL)
	}
	return hostPart, owner, repo, nil
}

// isSCPRemote reports whether s is an scp-style remote (host:path) rather than a
// URL or a bare path: it has a colon whose left side names a host, not a Windows
// drive or a "://" scheme, and no slash before that colon.
func isSCPRemote(s string) bool {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return false
	}
	return !strings.ContainsAny(s[:i], "/")
}
