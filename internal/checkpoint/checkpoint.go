// Package checkpoint verifies that a pipeline step left the repository in the
// expected state: a clean working tree and, for committing steps, a commit
// carrying the step's conventional-commit prefix.
package checkpoint

import (
	"context"
	"fmt"

	"github.com/saaga0h/themis/internal/git"
)

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
