package checkpoint

import (
	"context"
	"fmt"

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
func NewStepCheckpoint(ctx context.Context, dir string) (func(context.Context, pipeline.Step, string) error, error) {
	before, err := git.CommitsBefore(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("capturing initial commit state: %w", err)
	}

	return func(ctx context.Context, step pipeline.Step, workDir string) error {
		if err := VerifyCleanWorkingTree(ctx, workDir); err != nil {
			return err
		}

		newCommits, err := git.CommitsAfter(ctx, workDir, before)
		if err != nil {
			return fmt.Errorf("checking new commits: %w", err)
		}

		prefix, hasPrefix := stepPrefix[step]
		if len(newCommits) == 0 {
			if hasPrefix && !optionalCommit[step] {
				return fmt.Errorf("step %v requires a new commit with prefix %q but no new commit was found", step, prefix)
			}
		} else if hasPrefix {
			if err := VerifyCommitPrefix(ctx, workDir, prefix); err != nil {
				return err
			}
		}

		// Advance the snapshot so the next step only sees commits made during that step.
		current, err := git.CommitsBefore(ctx, workDir)
		if err != nil {
			return fmt.Errorf("updating commit snapshot: %w", err)
		}
		before = current

		return nil
	}, nil
}
