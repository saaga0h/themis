package checkpoint

import (
	"context"
	"fmt"
	"strings"

	"git.home.federation.fi/lavernea/themis/internal/git"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
)

var stepPrefix = map[pipeline.Step]string{
	pipeline.StepTestRed:  "test(",
	pipeline.StepImplement: "feat(",
	pipeline.StepRefactor:  "refactor(",
	pipeline.StepFix:       "fix(",
	pipeline.StepDocs:      "docs(",
}

var optionalCommit = map[pipeline.Step]bool{
	pipeline.StepRefactor: true,
	pipeline.StepDocs:     true,
}

// NewStepCheckpoint returns a checkpoint function that verifies each pipeline
// step produced the correct commit type and left a clean working tree.
//
// Instead of tracking a "before" snapshot, it checks whether a commit with the
// expected prefix exists anywhere on the branch (relative to the base). This
// makes resume work correctly: if a previous attempt already committed, the
// checkpoint passes without requiring a new commit.
func NewStepCheckpoint(ctx context.Context, dir string) (func(context.Context, pipeline.Step, string) error, error) {
	return func(ctx context.Context, step pipeline.Step, workDir string) error {
		if err := VerifyCleanWorkingTree(ctx, workDir); err != nil {
			return err
		}

		prefix, hasPrefix := stepPrefix[step]
		if !hasPrefix {
			return nil
		}

		if optionalCommit[step] {
			return nil
		}

		// Check if any commit on the branch has the expected prefix.
		// BranchCommitLog returns "git log --oneline" relative to the base branch.
		log := git.BranchCommitLog(ctx, workDir)
		if log == "" {
			return fmt.Errorf("step %v requires a commit with prefix %q but no commits found on branch", step, prefix)
		}

		for _, line := range strings.Split(log, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// git log --oneline format: <sha> <message>
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 && strings.HasPrefix(parts[1], prefix) {
				return nil
			}
		}

		return fmt.Errorf("step %v requires a commit with prefix %q but none found on branch", step, prefix)
	}, nil
}
