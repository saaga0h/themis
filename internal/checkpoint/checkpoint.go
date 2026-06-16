package checkpoint

import (
	"context"
	"fmt"
	"strings"

	"git.home.federation.fi/lavernea/themis/internal/git"
)

// VerifyCommitPrefix checks that the last commit message starts with prefix.
func VerifyCommitPrefix(ctx context.Context, dir, prefix string) error {
	msg, err := git.LastCommitMessage(ctx, dir)
	if err != nil {
		return fmt.Errorf("reading last commit message: %w", err)
	}
	if !strings.HasPrefix(msg, prefix) {
		return fmt.Errorf("last commit %q does not start with %q", msg, prefix)
	}
	return nil
}

// VerifyCleanWorkingTree checks that the git working tree has no uncommitted changes.
func VerifyCleanWorkingTree(ctx context.Context, dir string) error {
	clean, err := git.WorkingTreeClean(ctx, dir)
	if err != nil {
		return fmt.Errorf("checking working tree: %w", err)
	}
	if !clean {
		return fmt.Errorf("working tree has uncommitted changes")
	}
	return nil
}
