package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saaga0h/themis/internal/git"
	"github.com/saaga0h/themis/internal/preset"
	"github.com/saaga0h/themis/internal/tui"
	"github.com/saaga0h/themis/internal/workflow"
)

// runInit scaffolds the deterministic per-project config a factory run needs:
// .themis/workflow.yaml, a Containerfile, and a .env.example, plus .gitignore
// entries for secrets/artifacts.
//
// It produces a WORKING config or it refuses — it never writes a
// plausible-but-broken skeleton by default. Dispatch:
//   - any content flag (--language/--image/--provider/--runtime/--stack) → headless
//   - no content flags + a terminal → the interactive wizard
//   - no content flags + no terminal → refuse (pass --language)
//
// The only intentionally-incomplete path is an explicit "other" language
// (skeleton + a pointer to the reference docs).
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

	workDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	switch decideInitMode(contentSet, stdinIsTTY()) {
	case modeInteractive:
		return runInteractiveInit(workDir, *force)
	case modeErrNoTTY:
		return fmt.Errorf("themis init: no terminal — pass --language <%s|other> for a headless init. See docs/configuration-reference.md",
			strings.Join(preset.Languages(), "|"))
	}

	// Headless: resolve flags into a config.
	lang := strings.ToLower(strings.TrimSpace(*language))
	if lang == "" {
		return fmt.Errorf("themis init: --language is required for a headless init (e.g. --language go, or --language other). Known: %s",
			strings.Join(preset.Languages(), ", "))
	}
	if _, known := preset.Get(lang); !known && lang != "other" {
		return fmt.Errorf("themis init: unknown language %q. Choose one of: %s, other",
			lang, strings.Join(preset.Languages(), ", "))
	}
	if *provider != "" && *provider != "github" && *provider != "gitea" {
		return fmt.Errorf("themis init: --provider %q must be \"github\" or \"gitea\"", *provider)
	}
	if *runtime != "" && *runtime != "podman" && *runtime != "docker" {
		return fmt.Errorf("themis init: --runtime %q must be \"podman\" or \"docker\"", *runtime)
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
	if err := writeProjectFiles(workDir, cfg, *force); err != nil {
		return err
	}
	fmt.Print(guidance(effectiveProvider(workDir, cfg.Provider), cfg.Language, cfg.Image))
	fmt.Println("\nInstall the interactive skills:  themis skills install [--global]")
	return nil
}

// initMode is the resolved dispatch for `themis init`.
type initMode int

const (
	modeHeadless initMode = iota
	modeInteractive
	modeErrNoTTY
)

// decideInitMode is the pure dispatch decision: a content flag means headless; no
// content flags run the wizard on a terminal, or refuse without one.
func decideInitMode(contentSet, tty bool) initMode {
	switch {
	case contentSet:
		return modeHeadless
	case tty:
		return modeInteractive
	default:
		return modeErrNoTTY
	}
}

// runInteractiveInit walks the wizard, then writes the config through the same
// core the headless path uses.
func runInteractiveInit(workDir string, force bool) error {
	langs := make([]tui.LangOption, 0)
	for _, id := range preset.Languages() {
		p, _ := preset.Get(id)
		langs = append(langs, tui.LangOption{ID: id, Display: p.Display})
	}
	providerDef := git.InferProvider(context.Background(), workDir)
	if providerDef == "" {
		providerDef = "github"
	}

	form := tui.InitForm{
		Languages:    langs,
		ImageDefault: defaultImageTag(workDir),
		ProviderDef:  providerDef,
		ImageExists:  imageExists,
	}
	p := tui.New(os.Stdin, os.Stdout)
	ans, err := form.Run(p)
	if err != nil {
		// No usable input (e.g. stdin is /dev/null, which reads as a char device
		// but is not an interactive terminal): treat it as the no-terminal case
		// and refuse with the actionable message rather than a bare EOF.
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("themis init: no input — pass --language <%s|other> for a headless init, or run in a terminal. See docs/configuration-reference.md",
				strings.Join(preset.Languages(), "|"))
		}
		return fmt.Errorf("themis init: %w", err)
	}
	// Interactive confirms the provider explicitly, so it is written uncommented.
	cfg := workflow.InitConfig{
		Language: ans.Language,
		Image:    ans.Image,
		Provider: ans.Provider,
	}
	if err := writeProjectFiles(workDir, cfg, force); err != nil {
		return err
	}

	// Offer to install the interactive skills — the one thing we cannot detect
	// (whether they are already installed). Skip covers "already installed".
	if idx, serr := p.Select("Install the interactive skills (grill-me, issue-writer, review, …)?",
		[]string{"Into this project (./.claude)", "Globally (~/.claude)", "Skip (already installed)"}, 0); serr == nil {
		if act := skillsChoice(idx); act.install {
			if e := installSkills(act.global); e != nil {
				fmt.Fprintf(os.Stderr, "skills install failed: %v\n", e)
			}
		}
	}

	fmt.Print(guidance(ans.Provider, cfg.Language, cfg.Image))
	return nil
}

// writeProjectFiles scaffolds the three files + .gitignore for cfg. Shared by the
// headless and interactive paths so they cannot diverge. Guidance is printed by
// the caller (it differs: interactive prompts for skills first).
func writeProjectFiles(workDir string, cfg workflow.InitConfig, force bool) error {
	scaffolds := []struct {
		name  string
		write func() (bool, error)
	}{
		{".themis/workflow.yaml", func() (bool, error) { return workflow.WriteWorkflow(workDir, cfg, force) }},
		{"Containerfile", func() (bool, error) { return workflow.WriteContainerfileFor(workDir, cfg, force) }},
		{".env.example", func() (bool, error) { return workflow.WriteEnvExample(workDir, force) }},
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

	if changed, err := workflow.EnsureGitignore(workDir); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	} else if changed {
		fmt.Println("updated .gitignore (.env + factory artifacts)")
	} else {
		fmt.Println("skipped .gitignore (.env + factory artifacts already ignored)")
	}
	return nil
}

// effectiveProvider is the provider used for guidance: the configured value if
// set, else what the git remote implies, else github (the default).
func effectiveProvider(workDir, configured string) string {
	if configured != "" {
		return configured
	}
	if inferred := git.InferProvider(context.Background(), workDir); inferred != "" {
		return inferred
	}
	return "github"
}

// skillsAction is the resolved skills-install choice.
type skillsAction struct {
	install bool
	global  bool
}

// skillsChoice maps the completion prompt's selected index to an action:
// 0 = into the project, 1 = globally, anything else = skip.
func skillsChoice(idx int) skillsAction {
	switch idx {
	case 0:
		return skillsAction{install: true, global: false}
	case 1:
		return skillsAction{install: true, global: true}
	default:
		return skillsAction{install: false}
	}
}

// guidance returns the post-init next steps, conditional on provider and
// language. Shown after both the headless and interactive paths.
func guidance(provider, language, image string) string {
	var b strings.Builder
	if language == "other" {
		b.WriteString("Note: language 'other' wrote an intentionally incomplete config —\n")
		b.WriteString("  set your verify commands in .themis/workflow.yaml and install your\n")
		b.WriteString("  toolchain in the Containerfile TODO. See docs/configuration-reference.md.\n\n")
	}

	var steps []string
	steps = append(steps,
		fmt.Sprintf("Build the sandbox image:  podman build -t %s .\n     # Docker: docker build -f Containerfile -t %s .", image, image))
	steps = append(steps,
		"Credentials — copy .env.example to .env; set CLAUDE_CODE_OAUTH_TOKEN and your provider token.")
	if provider == "gitea" {
		steps = append(steps,
			"Gitea — connect a Gitea MCP server for interactive issue authoring; the factory\n     labels are auto-created. See docs/providers.md.")
	} else {
		steps = append(steps,
			"GitHub — install and authenticate the gh CLI (gh auth login), then create the\n     factory labels:\n       gh label create ready-for-agent\n       gh label create needs-review")
	}
	steps = append(steps,
		"Contracts — create CODING_STANDARDS.md and UBIQUITOUS_LANGUAGE.md (contract-drafter skill or by hand).")
	steps = append(steps,
		"Label an issue ready-for-agent, then run: themis run   (or themis issue <n>).")

	b.WriteString("Next steps (see docs/getting-started):\n")
	for i, s := range steps {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
	}
	return b.String()
}

// stdinIsTTY reports whether stdin is an interactive terminal (a character
// device), using only the standard library so it works on every OS/shell.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// imageExists reports whether an image tag already exists in the local runtime.
// Best-effort: it tries podman then docker, and reports false if neither is
// installed (the wizard simply won't flag a conflict).
func imageExists(tag string) bool {
	if _, err := exec.LookPath("podman"); err == nil {
		if exec.Command("podman", "image", "exists", tag).Run() == nil {
			return true
		}
	}
	if _, err := exec.LookPath("docker"); err == nil {
		if exec.Command("docker", "image", "inspect", tag).Run() == nil {
			return true
		}
	}
	return false
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
