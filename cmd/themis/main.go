package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/checkpoint"
	"github.com/saaga0h/themis/internal/git"
	"github.com/saaga0h/themis/internal/profile"
	"github.com/saaga0h/themis/internal/runner"
	"github.com/saaga0h/themis/internal/tracker"
)

// cmdGitOps wraps internal/git functions and satisfies runner.GitOps.
type cmdGitOps struct{}

func (g *cmdGitOps) CheckoutNewBranch(ctx context.Context, dir, name string) error {
	return git.CheckoutNewBranch(ctx, dir, name)
}

func (g *cmdGitOps) Checkout(ctx context.Context, dir, name string) error {
	return git.Checkout(ctx, dir, name)
}

func (g *cmdGitOps) PushBranch(ctx context.Context, dir, branch string) error {
	return git.PushBranch(ctx, dir, branch)
}

func (g *cmdGitOps) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return git.CurrentBranch(ctx, dir)
}

func (g *cmdGitOps) BranchCommitLog(ctx context.Context, dir string) string {
	return git.BranchCommitLog(ctx, dir)
}

func (g *cmdGitOps) CommitsAheadOfBase(ctx context.Context, dir, base string) (int, error) {
	return git.CommitsAheadOfBase(ctx, dir, base)
}

func (g *cmdGitOps) ChangedFiles(ctx context.Context, dir string) string {
	return git.ChangedFiles(ctx, dir)
}

// profileLoader wraps profile.Load with the signature expected by runner.Config.ProfileLoader.
func profileLoader(dir string) (runner.ProfileData, error) {
	p, err := profile.Load(dir)
	if err != nil {
		return runner.ProfileData{}, err
	}
	return runner.ProfileData{
		ImplementModel: p.Implement.Model,
		ReviewModel:    p.Review.Agents.Security,
	}, nil
}

const version = "0.1.0"

// defaultMaxTurns is the CLI default for --max-turns, sourced from the runner
// so the runner remains the single source of truth for the value.
const defaultMaxTurns = runner.DefaultMaxTurns

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
	case "run":
		if err := runRun(os.Args[2:]); err != nil {
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

	var giteaOwner, giteaRepo, giteaAPIBase string
	if parsed.provider == "gitea" {
		giteaOwner, giteaRepo, giteaAPIBase, err = resolveGiteaConfig(context.Background(), repoRoot)
		if err != nil {
			return fmt.Errorf("resolving Gitea config: %w", err)
		}
	}

	fetcher, err := tracker.NewFetcher(parsed.provider,
		giteaOwner, giteaRepo, giteaAPIBase, os.Getenv("GITEA_TOKEN"), giteaClientTimeout)
	if err != nil {
		return fmt.Errorf("creating fetcher: %w", err)
	}

	issueWriter := newIssueWriter(parsed.provider, giteaOwner, giteaRepo, giteaAPIBase)

	cfg, err := newIssueConfig(context.Background(), parsed.number, repoRoot, templateDir, fetcher, issueWriter, parsed.maxTurns)
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

func newIssueConfig(ctx context.Context, issueNumber int, workDir, tmplDir string, fetcher tracker.Fetcher, issueWriter runner.IssueWriter, maxTurns int) (runner.Config, error) {
	checkpointFn, err := checkpoint.NewStepCheckpoint(ctx, workDir)
	if err != nil {
		return runner.Config{}, fmt.Errorf("creating checkpoint: %w", err)
	}
	return runner.Config{
		WorkDir:       workDir,
		IssueNumber:   issueNumber,
		Fetcher:       fetcher,
		Invoker:       &agent.ClaudeCodeInvoker{},
		IssueWriter:   issueWriter,
		TemplateDir:   tmplDir,
		CheckpointFn:  checkpointFn,
		CodeVersion:   version,
		MaxTurns:      maxTurns,
		Git:           &cmdGitOps{},
		ProfileLoader: profileLoader,
	}, nil
}

func runRun(args []string) error {
	parsed, err := parseRunArgs(args)
	if err != nil {
		return fmt.Errorf("parsing run args: %w", err)
	}

	workDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	repoRoot := findRepoRoot(workDir)
	templateDir := filepath.Join(repoRoot, "templates")

	var querier IssueQuerier
	var giteaOwner, giteaRepo, giteaAPIBase string

	switch parsed.provider {
	case "gitea":
		giteaOwner, giteaRepo, giteaAPIBase, err = resolveGiteaConfig(context.Background(), repoRoot)
		if err != nil {
			return fmt.Errorf("resolving Gitea config: %w", err)
		}
		querier = newGiteaQuerier(giteaOwner, giteaRepo, giteaAPIBase, os.Getenv("GITEA_TOKEN"))
	case "github":
		querier = &GitHubQuerier{}
	}

	var fetcher tracker.Fetcher
	switch parsed.provider {
	case "gitea":
		fetcher = tracker.NewGiteaFetcher(giteaOwner, giteaRepo, giteaAPIBase, os.Getenv("GITEA_TOKEN"), giteaClientTimeout)
	default:
		fetcher = &tracker.GitHubFetcher{}
	}

	cfg := loopConfig{
		Querier: querier,
		RunFn: func(ctx context.Context, issue *tracker.IssueData) error {
			if err := git.Checkout(ctx, repoRoot, "main"); err != nil {
				return fmt.Errorf("checkout main before issue #%d: %w", issue.Number, err)
			}
			issueWriter := newIssueWriter(parsed.provider, giteaOwner, giteaRepo, giteaAPIBase)
			issueCfg, err := newIssueConfig(ctx, issue.Number, repoRoot, templateDir, fetcher, issueWriter, parsed.maxTurns)
			if err != nil {
				return fmt.Errorf("creating config for issue #%d: %w", issue.Number, err)
			}
			result, err := runner.Run(ctx, issueCfg)
			if err != nil {
				return err
			}
			fmt.Printf("PR created for issue #%d: %s\n", issue.Number, result.PRURL)
			return nil
		},
		DryRun: parsed.dryRun,
		Turns:  &envTurns{},
		Logger: os.Stderr,
	}

	return runLoop(context.Background(), cfg)
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
