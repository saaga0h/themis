package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/saaga0h/themis/internal/preset"
	"github.com/saaga0h/themis/internal/workflow"
)

// runInit scaffolds the deterministic per-project config a factory run needs:
// .themis/workflow.yaml, a Containerfile, and a .env.example, plus .gitignore
// entries for secrets/artifacts.
//
// It produces a WORKING config or it refuses — it never writes a
// plausible-but-broken skeleton by default. A content flag (--language/--image/
// --provider/--runtime/--stack) selects the headless path: --language is the
// floor, and a known language composes a runnable config from its preset. The
// only intentionally-incomplete path is an explicit --language other (skeleton +
// a pointer to the reference docs). With no content flags, init errors — the
// interactive form is wired in a later slice (#146); until then, pass --language.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we format our own errors
	var (
		language = fs.String("language", "", "project language: "+strings.Join(preset.Languages(), ", ")+", or other")
		image    = fs.String("image", "", "sandbox image tag (default themis-<dir>:latest)")
		provider = fs.String("provider", "", "issue tracker: github or gitea")
		runtime  = fs.String("runtime", "", "container runtime: podman or docker")
		stack    = fs.String("stack", "", "informational stack label (defaults from language)")
		force    = fs.Bool("force", false, "overwrite existing files")
	)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("themis init: %w", err)
	}

	// A content flag means "headless — I know what I want"; --force alone does not.
	contentSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "language", "image", "provider", "runtime", "stack":
			contentSet = true
		}
	})

	if !contentSet {
		return fmt.Errorf("themis init: pass --language <%s|other> for a headless init, or run interactive init in a terminal (coming soon). See docs/configuration-reference.md",
			strings.Join(preset.Languages(), "|"))
	}

	lang := strings.ToLower(strings.TrimSpace(*language))
	if lang == "" {
		return fmt.Errorf("themis init: --language is required for a headless init (e.g. --language go, or --language other). Known: %s",
			strings.Join(preset.Languages(), ", "))
	}
	_, known := preset.Get(lang)
	if !known && lang != "other" {
		return fmt.Errorf("themis init: unknown language %q. Choose one of: %s, other",
			lang, strings.Join(preset.Languages(), ", "))
	}

	if *provider != "" && *provider != "github" && *provider != "gitea" {
		return fmt.Errorf("themis init: --provider %q must be \"github\" or \"gitea\"", *provider)
	}
	if *runtime != "" && *runtime != "podman" && *runtime != "docker" {
		return fmt.Errorf("themis init: --runtime %q must be \"podman\" or \"docker\"", *runtime)
	}

	workDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	img := *image
	if img == "" {
		img = defaultImageTag(workDir)
	}

	cfg := workflow.InitConfig{
		Language: lang,
		Stack:    *stack,
		Image:    img,
		Provider: *provider,
		Runtime:  *runtime,
	}

	scaffolds := []struct {
		name  string
		write func() (bool, error)
	}{
		{".themis/workflow.yaml", func() (bool, error) { return workflow.WriteWorkflow(workDir, cfg, *force) }},
		{"Containerfile", func() (bool, error) { return workflow.WriteContainerfileFor(workDir, cfg, *force) }},
		{".env.example", func() (bool, error) { return workflow.WriteEnvExample(workDir, *force) }},
	}
	for _, s := range scaffolds {
		created, werr := s.write()
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
	if changed, err := workflow.EnsureGitignore(workDir); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	} else if changed {
		fmt.Println("updated .gitignore (.env + factory artifacts)")
	} else {
		fmt.Println("skipped .gitignore (.env + factory artifacts already ignored)")
	}

	printNextSteps(lang == "other", cfg)
	return nil
}

// defaultImageTag derives the default sandbox image tag from the project
// directory name: themis-<dir>:latest, lowercased (image tags must be lowercase).
func defaultImageTag(workDir string) string {
	base := strings.ToLower(filepath.Base(workDir))
	base = strings.ReplaceAll(base, " ", "-")
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "project"
	}
	return "themis-" + base + ":latest"
}

// printNextSteps prints post-init guidance. This is the minimal #145 version;
// #147 replaces it with fully provider/language-conditional guidance.
func printNextSteps(other bool, cfg workflow.InitConfig) {
	fmt.Println()
	if other {
		fmt.Println("Note: --language other wrote an intentionally incomplete config —")
		fmt.Println("  set your verify commands in .themis/workflow.yaml and install your")
		fmt.Println("  toolchain in the Containerfile TODO. See docs/configuration-reference.md.")
		fmt.Println()
	}
	fmt.Println("Next steps (see docs/getting-started):")
	fmt.Printf("  1. Build the sandbox image:  podman build -t %s .\n", cfg.Image)
	fmt.Printf("     # Docker: docker build -f Containerfile -t %s .\n", cfg.Image)
	fmt.Println("  2. .env — copy from .env.example; fill CLAUDE_CODE_OAUTH_TOKEN and your provider token.")
	fmt.Println("  3. Contracts — create CODING_STANDARDS.md and UBIQUITOUS_LANGUAGE.md (contract-drafter skill or by hand).")
	fmt.Println("  4. themis skills install — install the interactive skills into .claude.")
	fmt.Println("  5. Label an issue ready-for-agent, then run: themis run   (or themis issue <n>).")
}
