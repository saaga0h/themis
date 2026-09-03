package checkpoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/saaga0h/themis/internal/git"
	"github.com/saaga0h/themis/internal/pipeline"
)

var stepPrefix = map[pipeline.Step]string{
	pipeline.StepTestRed:   "test(",
	pipeline.StepImplement: "feat(",
	pipeline.StepRefactor:  "refactor(",
	pipeline.StepFix:       "fix(",
	pipeline.StepDocs:      "docs(",
}

var optionalCommit = map[pipeline.Step]bool{
	pipeline.StepRefactor: true,
	pipeline.StepDocs:     true,
}

// NewStepCheckpoint returns a checkpoint function that verifies each committing
// pipeline step produced the correct commit type and left a clean working tree.
//
// The clean-tree guard is scoped to steps that are expected to commit (those in
// stepPrefix). Non-committing steps — Fetch, Scan, Branch, Review, Ship — produce
// no commit, so a dirty tree there is leftover tooling output (e.g. a binary from
// a `go build`/`go test` the Review agent ran to verify the code), not work the
// step failed to save. Enforcing clean-tree on them blocks the run for something
// no step is responsible for committing.
//
// Instead of tracking a "before" snapshot, it checks whether a commit with the
// expected prefix exists anywhere on the branch (relative to the base). This
// makes resume work correctly: if a previous attempt already committed, the
// checkpoint passes without requiring a new commit.
func NewStepCheckpoint(ctx context.Context, dir string) (func(context.Context, pipeline.Step, string) error, error) {
	return func(ctx context.Context, step pipeline.Step, workDir string) error {
		prefix, hasPrefix := stepPrefix[step]
		if !hasPrefix {
			// Non-committing step: nothing to verify (see doc comment above).
			return nil
		}

		// Committing step: its work must be saved — a clean tree (nothing left
		// uncommitted) and, for required-commit steps, the expected commit type.
		if err := VerifyCleanWorkingTree(ctx, workDir); err != nil {
			return err
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
