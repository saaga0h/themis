package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CommitsBefore returns all commit SHAs currently reachable from HEAD in dir.
func CommitsBefore(dir string) ([]string, error) {
	out, err := runGit(dir, "log", "--format=%H")
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseLines(out), nil
}

// CommitsAfter returns SHAs added since the before snapshot by diffing against
// the current log.
func CommitsAfter(dir string, before []string) ([]string, error) {
	current, err := CommitsBefore(dir)
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
func WorkingTreeClean(dir string) (bool, error) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(out) == "", nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
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
