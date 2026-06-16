package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git.home.federation.fi/lavernea/themis/internal/runner"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: themis <command>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "issue":
		if err := runIssue(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runIssue(args []string) error {
	parsed, err := parseIssueArgs(args)
	if err != nil {
		return fmt.Errorf("parsing issue args: %w", err)
	}

	workDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	repoRoot := findRepoRoot(workDir)
	templateDir := filepath.Join(repoRoot, "templates")

	fetcher, err := tracker.NewFetcher(parsed.provider,
		os.Getenv("GITEA_OWNER"), os.Getenv("GITEA_REPO"),
		os.Getenv("GITEA_API_URL"), os.Getenv("GITEA_TOKEN"))
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}

	issueWriter := newIssueWriter(parsed.provider, parsed.number)

	cfg := runner.Config{
		WorkDir:     repoRoot,
		IssueNumber: parsed.number,
		Fetcher:     fetcher,
		Invoker:     &claudeInvoker{},
		IssueWriter: issueWriter,
		TemplateDir: templateDir,
	}

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	fmt.Printf("PR created: %s\n", result.PRURL)
	return nil
}

// findRepoRoot walks up from dir until it finds a .git directory.
func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
