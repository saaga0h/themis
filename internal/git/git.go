package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// CommitsBefore returns all commit SHAs currently reachable from HEAD in dir.
func CommitsBefore(ctx context.Context, dir string) ([]string, error) {
	out, err := runGit(ctx, dir, "log", "--format=%H")
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseLines(out), nil
}

// CommitsAfter returns SHAs added since the before snapshot.
func CommitsAfter(ctx context.Context, dir string, before []string) ([]string, error) {
	current, err := CommitsBefore(ctx, dir)
	if err != nil {
		return nil, err
	}
	beforeSet := make(map[string]bool, len(before))
	for _, sha := range before {
		beforeSet[sha] = true
	}
	var added []string
	for _, sha := range current {
		if !beforeSet[sha] {
			added = append(added, sha)
		}
	}
	return added, nil
}

// WorkingTreeClean reports whether the working tree has no uncommitted changes.
func WorkingTreeClean(ctx context.Context, dir string) (bool, error) {
	out, err := runGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(out) == "", nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// LastCommitMessage returns the subject line of the most recent commit.
func LastCommitMessage(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "log", "-1", "--format=%s")
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CheckoutNewBranch creates and checks out a new branch from the current HEAD.
func CheckoutNewBranch(ctx context.Context, dir, name string) error {
	_, err := runGit(ctx, dir, "checkout", "-b", name)
	if err != nil {
		return fmt.Errorf("git checkout -b %s: %w", name, err)
	}
	return nil
}

// Checkout switches to an existing branch.
func Checkout(ctx context.Context, dir, name string) error {
	_, err := runGit(ctx, dir, "checkout", name)
	if err != nil {
		return fmt.Errorf("git checkout %s: %w", name, err)
	}
	return nil
}

// Fetch runs git fetch origin in the given directory.
func Fetch(ctx context.Context, dir string) error {
	_, err := runGit(ctx, dir, "fetch", "origin")
	if err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

// ChangedFiles returns newline-separated file paths changed on HEAD relative to
// the nearest remote tracking branch, using git diff --name-only against the
// merge-base. Returns empty string when no remote tracking branch exists or any
// git command fails.
func ChangedFiles(ctx context.Context, dir string) string {
	base := branchMergeBase(ctx, dir)
	if base == "" {
		return ""
	}
	out, err := runGit(ctx, dir, "diff", "--name-only", base)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// branchMergeBase returns the merge-base SHA between HEAD and the nearest remote
// tracking branch. Returns empty string if no remote exists or any git command fails.
func branchMergeBase(ctx context.Context, dir string) string {
	refs, err := runGit(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/")
	if err != nil || strings.TrimSpace(refs) == "" {
		return ""
	}
	for _, ref := range parseLines(refs) {
		if strings.Contains(ref, "/HEAD") {
			continue
		}
		mergeBase, err := runGit(ctx, dir, "merge-base", "HEAD", ref)
		if err != nil {
			continue
		}
		return strings.TrimSpace(mergeBase)
	}
	return ""
}

// BranchCommitLog returns git log --oneline output for commits on the current
// branch relative to the nearest remote tracking branch. Returns empty string
// when no remote exists or any git command fails.
func BranchCommitLog(ctx context.Context, dir string) string {
	base := branchMergeBase(ctx, dir)
	if base == "" {
		return ""
	}
	out, err := runGit(ctx, dir, "log", "--oneline", base+"..HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// CommitsAheadOfBase counts commits reachable from HEAD but not from the base branch.
// It tries origin/<base> first, then <base> directly.
// Returns an error if neither ref can be resolved.
func CommitsAheadOfBase(ctx context.Context, dir, base string) (int, error) {
	for _, ref := range []string{"origin/" + base, base} {
		out, err := runGit(ctx, dir, "rev-list", "--count", ref+"..HEAD")
		if err != nil {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			continue
		}
		return n, nil
	}
	return 0, fmt.Errorf("could not count commits ahead of %s", base)
}

// DiffLineCount returns the total number of changed lines (insertions plus
// deletions) on the current branch relative to the nearest remote tracking
// branch. Returns 0 when no remote exists or any git command fails. Binary-file
// rows (which git reports as "-") are skipped.
func DiffLineCount(ctx context.Context, dir string) int {
	base := branchMergeBase(ctx, dir)
	if base == "" {
		return 0
	}
	out, err := runGit(ctx, dir, "diff", "--numstat", base)
	if err != nil {
		return 0
	}
	total := 0
	for _, line := range parseLines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		added, errA := strconv.Atoi(fields[0])
		deleted, errD := strconv.Atoi(fields[1])
		if errA != nil || errD != nil {
			continue
		}
		total += added + deleted
	}
	return total
}

// PushBranch pushes the current branch to origin.
func PushBranch(ctx context.Context, dir, branch string) error {
	_, err := runGit(ctx, dir, "push", "-u", "origin", branch)
	if err != nil {
		return fmt.Errorf("git push origin %s: %w", branch, err)
	}
	return nil
}

// giteaTokenAuthHeader builds the HTTP Basic-auth header value that authenticates
// a git-over-HTTPS request to Gitea with a personal access token. Gitea accepts a
// token as the Basic-auth username (the `https://<token>@host/...` form), so the
// credential is base64("<token>:").
func giteaTokenAuthHeader(token string) string {
	return "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(token+":"))
}

// PushBranchWithToken pushes branch to an explicit https remote URL, authenticating
// with a Gitea token. The token is passed as an HTTP Authorization header injected
// through git's environment config (GIT_CONFIG_*), so it never appears in the
// remote URL, the process arguments, or any on-disk git config — and a push error
// cannot echo it. This lets the factory push with the same GITEA_TOKEN it uses for
// the API, requiring no credentials stored in any repo's remote (SSH keys or a
// token baked into .git/config).
func PushBranchWithToken(ctx context.Context, dir, remoteURL, branch, token string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("dir must be an absolute path, got %q", dir)
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "push", remoteURL, branch)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.extraheader",
		"GIT_CONFIG_VALUE_0="+giteaTokenAuthHeader(token),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// stderr may name the remote URL (which holds no token — auth is in the
		// header) but never the credential, so it is safe to surface.
		return fmt.Errorf("git push (token auth) %s: %w: %s", branch, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// CheckIdentity verifies that a git author identity is configured — user.name and
// user.email both resolve to non-empty values in dir. It returns an actionable
// error when either is missing so the factory can fail fast before the first
// commit-producing step, rather than letting `git commit` fail deep in the
// pipeline and surface as a misleading "uncommitted changes" checkpoint error.
func CheckIdentity(ctx context.Context, dir string) error {
	// `git config user.name` exits non-zero when unset; treat that as empty.
	name, _ := runGit(ctx, dir, "config", "user.name")
	email, _ := runGit(ctx, dir, "config", "user.email")
	return identityError(name, email)
}

// identityError reports whether a resolved (name, email) pair is a usable git
// identity, returning a fix-it error when either side is blank.
func identityError(name, email string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" {
		return fmt.Errorf("git identity not configured: user.name and user.email must be set — " +
			"configure them in the repo's .git/config, pass GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL/" +
			"GIT_COMMITTER_NAME/GIT_COMMITTER_EMAIL, or bake a default into the sandbox image")
	}
	return nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("dir must be an absolute path, got %q", dir)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func parseLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
