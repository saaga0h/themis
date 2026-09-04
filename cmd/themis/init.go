package main

import (
	"fmt"
	"path/filepath"

	"github.com/saaga0h/themis/internal/workflow"
)

// runInit scaffolds the deterministic per-project config a factory run needs:
// .themis/workflow.yaml, a Containerfile (agent layer pre-filled, toolchain TODO),
// and a .env.example. Non-destructive by default — an existing file is reported
// as skipped — unless --force overwrites. Judgment settings (real verify commands,
// the toolchain, contracts) stay TODOs the docs explain; init sets up the frame,
// not the guesses.
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

	scaffolds := []struct {
		name  string
		write func(string, bool) (bool, error)
	}{
		{".themis/workflow.yaml", workflow.WriteSkeleton},
		{"Containerfile", workflow.WriteContainerfile},
		{".env.example", workflow.WriteEnvExample},
	}
	for _, s := range scaffolds {
		created, werr := s.write(workDir, force)
		if werr != nil {
			return fmt.Errorf("scaffolding %s: %w", s.name, werr)
		}
		if created {
			fmt.Printf("created %s\n", s.name)
		} else {
			fmt.Printf("skipped %s (already exists; use --force to overwrite)\n", s.name)
		}
	}

	// Keep .env (secrets) and the factory's per-run artifacts out of the repo.
	// Append-only: preserves any existing .gitignore.
	if changed, err := workflow.EnsureGitignore(workDir); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	} else if changed {
		fmt.Println("updated .gitignore (.env + factory artifacts)")
	} else {
		fmt.Println("skipped .gitignore (.env + factory artifacts already ignored)")
	}

	fmt.Println()
	fmt.Println("Next steps (see docs/getting-started):")
	fmt.Println("  1. .themis/workflow.yaml — set your real verify commands, point docs at your")
	fmt.Println("     standards/glossary, and set image: to the tag you will build.")
	fmt.Println("  2. Containerfile — install your TOOLCHAIN in the TODO section: the image runs")
	fmt.Println("     your workflow.yaml verify commands, so it needs the tools those call (your")
	fmt.Println("     compiler, test runner, linters). The TODO explains it with per-stack examples.")
	fmt.Println("     (themis + Claude Code are built in automatically; override THEMIS_REPO for")
	fmt.Println("     your own git host.) Then build — -t is a lowercase tag YOU choose (not the")
	fmt.Println("     filename), set as image: in workflow.yaml:")
	fmt.Println("       podman build -t themis-myproject:latest .")
	fmt.Println("       # Docker: docker build -f Containerfile -t themis-myproject:latest .")
	fmt.Println("  3. Contracts — create CODING_STANDARDS.md and UBIQUITOUS_LANGUAGE.md (run the")
	fmt.Println("     contract-drafter skill, or write them by hand); workflow.yaml docs: points at them.")
	fmt.Println("  4. .env — copy from .env.example; fill CLAUDE_CODE_OAUTH_TOKEN and your provider token.")
	fmt.Println("  5. themis skills install — install the interactive skills into .claude.")
	fmt.Println("  6. Label an issue ready-for-agent, then run: themis run   (or themis issue <n>).")
	return nil
}
