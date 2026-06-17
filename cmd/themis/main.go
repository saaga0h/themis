package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/checkpoint"
	"git.home.federation.fi/lavernea/themis/internal/git"
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

	cfg, err := newIssueConfig(context.Background(), parsed.number, repoRoot, templateDir, fetcher, issueWriter)
	if err != nil {
		return fmt.Errorf("creating issue config: %w", err)
	}

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	fmt.Printf("PR created: %s\n", result.PRURL)
	return nil
}

func newIssueConfig(ctx context.Context, issueNumber int, workDir, tmplDir string, fetcher tracker.Fetcher, issueWriter runner.IssueWriter) (runner.Config, error) {
	checkpointFn, err := checkpoint.NewStepCheckpoint(ctx, workDir)
	if err != nil {
		return runner.Config{}, fmt.Errorf("creating checkpoint: %w", err)
	}
	return runner.Config{
		WorkDir:      workDir,
		IssueNumber:  issueNumber,
		Fetcher:      fetcher,
		Invoker:      &agent.ClaudeCodeInvoker{},
		IssueWriter:  issueWriter,
		TemplateDir:  tmplDir,
		CheckpointFn: checkpointFn,
		GitBranchFn: func(ctx context.Context, wd, branch string) error {
			if err := git.CheckoutNewBranch(ctx, wd, branch); err != nil {
				if checkoutErr := git.Checkout(ctx, wd, branch); checkoutErr != nil {
					return fmt.Errorf("create failed (%v), checkout failed (%v)", err, checkoutErr)
				}
				fmt.Fprintf(os.Stderr, "note: branch %s already exists, checked out existing\n", branch)
			}
			return nil
		},
		GitPushFn: git.PushBranch,
	}, nil
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
