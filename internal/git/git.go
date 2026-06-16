package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
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
