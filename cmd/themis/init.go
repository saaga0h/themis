package main

import (
	"fmt"
	"path/filepath"

	"github.com/saaga0h/themis/internal/workflow"
)

// runInit scaffolds .themis/workflow.yaml in the current directory. It is
// non-destructive by default — an existing file is reported as skipped —
// unless --force is passed, in which case it is overwritten with the
// skeleton. See internal/workflow.WriteSkeleton for the skeleton content.
func runInit(args []string) error {
	force := false
	for _, a := range args {
		if a == "--force" {
			force = true
		}
	}

	workDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	created, err := workflow.WriteSkeleton(workDir, force)
	if err != nil {
		return fmt.Errorf("scaffolding .themis/workflow.yaml: %w", err)
	}

	if !created {
		fmt.Println("skipped: .themis/workflow.yaml already exists (use --force to overwrite)")
		return nil
	}

	fmt.Println("created .themis/workflow.yaml")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .themis/workflow.yaml — set stack, replace the default verify")
	fmt.Println("     command with your project's real build/lint/test commands, and point")
	fmt.Println("     docs at your standards/glossary/architecture files.")
	fmt.Println("  2. Run themis issue <number> or themis run.")
	return nil
}
