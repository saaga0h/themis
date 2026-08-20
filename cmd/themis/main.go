package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/checkpoint"
	"github.com/saaga0h/themis/internal/git"
	"github.com/saaga0h/themis/internal/issuespec"
	"github.com/saaga0h/themis/internal/profile"
	"github.com/saaga0h/themis/internal/review"
	"github.com/saaga0h/themis/internal/runner"
	"github.com/saaga0h/themis/internal/scrub"
	"github.com/saaga0h/themis/internal/tracker"
	"github.com/saaga0h/themis/internal/workflow"
)

// newScrubber builds the redactor for text the factory publishes (block comments,
// telemetry detail): the Gitea token plus the Gitea host derived from apiBase, so
// a leaked git/API error carries neither the credential nor the internal host.
// Empty inputs are ignored, so this is safe for the github provider too.
func newScrubber(apiBase string) func(string) string {
	host := ""
	if apiBase != "" {
		if u, err := url.Parse(apiBase); err == nil {
			host = u.Hostname()
		}
	}
	return scrub.New(os.Getenv("GITEA_TOKEN"), host)
}

// factorySecretEnv names the orchestration credentials the factory holds but
// green-gate verify commands — builds, tests, greps, and issue-declared check
// blocks — must never see. Those commands run arbitrary shell and their output is
// published (tracker comments, telemetry), so a command that echoes or dumps env
// would leak a secret. GITEA_TOKEN/GITHUB_TOKEN authenticate git push and the
// tracker API; CLAUDE_CODE_OAUTH_TOKEN is held only to forward to the Claude Code
// subprocess (internal/agent) and is read by no factory Go code.
var factorySecretEnv = map[string]bool{
	"GITEA_TOKEN":             true,
	"GITHUB_TOKEN":            true,
	"CLAUDE_CODE_OAUTH_TOKEN": true,
}

// verifyEnv returns the parent environment with factorySecretEnv removed. It is a
// denylist, not an allowlist: everything else (PATH, HOME, GOCACHE, GOPATH, proxy
// vars, …) is preserved so stack-agnostic builds and tests still run. Stripping at
// the source is the right control — the scrubber cannot be relied on for the OAuth
// token (the factory does not even own it), so no verify subprocess is handed the
// credentials in the first place.
func verifyEnv() []string {
	parent := os.Environ()
	filtered := make([]string, 0, len(parent))
	for _, kv := range parent {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if factorySecretEnv[name] {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// verifyRunner returns a runner.Config.TestRunner that runs the project's
// declared verify commands (the Green Gate) in order, each via `bash -c`, in
// dir. The first non-zero exit fails the gate. With no commands declared it
// no-ops to passing — the warning is emitted once at config time, not here.
// The factory stays stack-agnostic: these commands come from
// .themis/workflow.yaml, never hardcoded. Each command runs with the factory's
// orchestration secrets stripped from its environment (see verifyEnv).
func verifyRunner(verify []string, baseBranch string) func(ctx context.Context, dir string) (bool, string) {
	return func(ctx context.Context, dir string) (bool, string) {
		if len(verify) == 0 {
			return true, "no verify commands declared; green gate is a no-op"
		}
		// Resolve the base once so footprint/diff checks compare against it as $BASE:
		// the merge-base with the issue's OWN base branch, not an arbitrary remote
		// ref. Empty when unresolvable — such checks fail safe / pass. Same BASE
		// contract cmd/backtest uses, so a footprint check runs identically here and
		// under backtest.
		base := git.MergeBaseWith(ctx, dir, baseBranch)
		var out strings.Builder
		for _, cmd := range verify {
			fmt.Fprintf(&out, "$ %s\n", cmd)
			c := exec.CommandContext(ctx, "bash", "-c", cmd)
			c.Dir = dir
			c.Env = append(verifyEnv(), "BASE="+base)
			o, err := c.CombinedOutput()
			out.Write(o)
			if err != nil {
				fmt.Fprintf(&out, "\nverify command failed (%s): %v\n", cmd, err)
				return false, out.String()
			}
		}
		return true, out.String()
	}
}

// cmdGitOps wraps internal/git functions and satisfies runner.GitOps.
// pushURL and token, when set (gitea), make PushBranch authenticate with the
// factory's GITEA_TOKEN over https — so the target repo needs no push
// credentials in its git remote (no SSH key, no token baked into .git/config).
type cmdGitOps struct {
	pushURL string
	token   string
}

// newCmdGitOps builds the GitOps for a provider. For gitea it derives the https
// push URL from the resolved config and takes the token from GITEA_TOKEN, so
// pushes authenticate the same way the API does. For other providers (or when the
// URL can't be derived) it leaves them empty and PushBranch falls back to pushing
// to origin with whatever auth the remote is configured for.
func newCmdGitOps(provider, owner, repo, apiBase string) *cmdGitOps {
	g := &cmdGitOps{}
	if provider == "gitea" {
		if pushURL, err := git.GiteaPushURL(apiBase, owner, repo); err == nil {
			g.pushURL = pushURL
			g.token = os.Getenv("GITEA_TOKEN")
		}
	}
	return g
}

func (g *cmdGitOps) CheckoutNewBranch(ctx context.Context, dir, name string) error {
	return git.CheckoutNewBranch(ctx, dir, name)
}

func (g *cmdGitOps) Checkout(ctx context.Context, dir, name string) error {
	return git.Checkout(ctx, dir, name)
}

func (g *cmdGitOps) PushBranch(ctx context.Context, dir, branch string) error {
	if g.pushURL != "" && g.token != "" {
		return git.PushBranchWithToken(ctx, dir, g.pushURL, branch, g.token)
	}
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

func (g *cmdGitOps) DiffLineCount(ctx context.Context, dir string) int {
	return git.DiffLineCount(ctx, dir)
}

func (g *cmdGitOps) CommitSHAs(ctx context.Context, dir string) ([]string, error) {
	return git.CommitsBefore(ctx, dir)
}

func (g *cmdGitOps) WorkingTreeClean(ctx context.Context, dir string) (bool, error) {
	return git.WorkingTreeClean(ctx, dir)
}

func (g *cmdGitOps) CleanWorkingTree(ctx context.Context, dir string) (int, error) {
	return git.CleanWorkingTree(ctx, dir)
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
	if err := git.CheckIdentity(context.Background(), repoRoot); err != nil {
		return err
	}
	templateDir, cleanup, err := resolveTemplateDir(parsed.templates)
	if err != nil {
		return fmt.Errorf("resolving templates: %w", err)
	}
	defer cleanup()

	var giteaOwner, giteaRepo, giteaAPIBase string
	if parsed.provider == "gitea" {
		giteaOwner, giteaRepo, giteaAPIBase, err = tracker.ResolveGiteaConfig(context.Background(), repoRoot)
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
	gitOps := newCmdGitOps(parsed.provider, giteaOwner, giteaRepo, giteaAPIBase)

	cfg, err := newIssueConfig(context.Background(), parsed.number, repoRoot, templateDir, fetcher, issueWriter, gitOps, parsed.maxTurns)
	if err != nil {
		return fmt.Errorf("creating issue config: %w", err)
	}
	cfg.Scrub = newScrubber(giteaAPIBase)

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	fmt.Printf("PR created: %s\n", result.PRURL)
	return nil
}

func newIssueConfig(ctx context.Context, issueNumber int, workDir, tmplDir string, fetcher tracker.Fetcher, issueWriter runner.IssueWriter, gitOps runner.GitOps, maxTurns int) (runner.Config, error) {
	checkpointFn, err := checkpoint.NewStepCheckpoint(ctx, workDir)
	if err != nil {
		return runner.Config{}, fmt.Errorf("creating checkpoint: %w", err)
	}
	desc, err := workflow.Load(workDir)
	if err != nil {
		return runner.Config{}, fmt.Errorf("loading workflow descriptor: %w", err)
	}
	if len(desc.Verify) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no verify commands declared in .themis/workflow.yaml; green gate will be a no-op\n")
	}
	issue, err := fetcher.Fetch(ctx, issueNumber)
	if err != nil {
		return runner.Config{}, fmt.Errorf("fetching issue #%d: %w", issueNumber, err)
	}
	if err := issuespec.ValidateDestructiveChecks(issue.Body); err != nil {
		return runner.Config{}, fmt.Errorf("issue #%d: %w", issueNumber, err)
	}
	verify := append(append([]string{}, desc.Verify...), issuespec.ParseCheckBlocks(issue.Body)...)
	// Footprint gate (#111): translate the issue's declared change surface into a
	// check appended to the green gate for this run. Empty when the issue declares
	// no footprint or declares `wide` — no gate in those cases.
	if fpCheck := issuespec.ParseFootprint(issue.Body).CheckCommand(desc.FootprintExempt); fpCheck != "" {
		verify = append(verify, fpCheck)
	}
	// Export-budget gate (#111): a declared ```exports block bounds a package's
	// exported surface. A tree check (go doc), so no $BASE needed. Empty when the
	// issue declares no budget — no gate.
	if exCheck := issuespec.ExportCheckCommand(issuespec.ParseExports(issue.Body)); exCheck != "" {
		verify = append(verify, exCheck)
	}
	// The issue's base branch — what a footprint/diff check diffs against as $BASE.
	// The issue's Ref (Gitea) when set, else the current branch, which at config
	// time (before the Branch step) is the base the issue is cut from.
	curBranch, _ := git.CurrentBranch(ctx, workDir)
	baseBranch := baseBranchForIssue(issue, curBranch)
	// Diagnostic emitter: ships the factory's own per-step narrative to the OTLP
	// collector when OTEL_* env is set (same gating as Claude Code's telemetry);
	// nil otherwise. Run defers EmitterShutdown to flush the batch on exit.
	emitter, emitterShutdown, _ := newOTELEmitter(ctx)
	return runner.Config{
		WorkDir:             workDir,
		IssueNumber:         issueNumber,
		BaseBranch:          baseBranch,
		Fetcher:             fetcher,
		Invoker:             &agent.ClaudeCodeInvoker{},
		IssueWriter:         issueWriter,
		TemplateDir:         tmplDir,
		CheckpointFn:        checkpointFn,
		CodeVersion:         version,
		MaxTurns:            maxTurns,
		Git:                 gitOps,
		ProfileLoader:       profileLoader,
		ReviewResultsLoader: review.ReadReviewResults,
		TestRunner:          verifyRunner(verify, baseBranch),
		StandardsDocs:       desc.StandardsDocs(),
		DocSurfaces:         desc.Docs.Surfaces,
		Emitter:             emitter,
		EmitterShutdown:     emitterShutdown,
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
	if err := git.CheckIdentity(context.Background(), repoRoot); err != nil {
		return err
	}
	templateDir, cleanup, err := resolveTemplateDir(parsed.templates)
	if err != nil {
		return fmt.Errorf("resolving templates: %w", err)
	}
	defer cleanup()

	var querier IssueQuerier
	var giteaOwner, giteaRepo, giteaAPIBase string

	switch parsed.provider {
	case "gitea":
		giteaOwner, giteaRepo, giteaAPIBase, err = tracker.ResolveGiteaConfig(context.Background(), repoRoot)
		if err != nil {
			return fmt.Errorf("resolving Gitea config: %w", err)
		}
		querier = tracker.NewGiteaQuerier(giteaOwner, giteaRepo, giteaAPIBase, os.Getenv("GITEA_TOKEN"), giteaClientTimeout)
	case "github":
		querier = tracker.NewGitHubQuerier()
	}

	var fetcher tracker.Fetcher
	switch parsed.provider {
	case "gitea":
		fetcher = tracker.NewGiteaFetcher(giteaOwner, giteaRepo, giteaAPIBase, os.Getenv("GITEA_TOKEN"), giteaClientTimeout)
	default:
		fetcher = &tracker.GitHubFetcher{}
	}

	// The branch the factory was invoked on, captured once before any issue runs —
	// the base the loop falls back to for issues without a Ref (e.g. GitHub). Empty
	// if it can't be determined; baseBranchForIssue then defaults to "main".
	originBranch, _ := git.CurrentBranch(context.Background(), repoRoot)

	cfg := loopConfig{
		Querier: querier,
		RunFn: func(ctx context.Context, issue *tracker.IssueData) error {
			base := baseBranchForIssue(issue, originBranch)
			if err := git.Checkout(ctx, repoRoot, base); err != nil {
				return fmt.Errorf("checkout base %q before issue #%d: %w", base, issue.Number, err)
			}
			issueWriter := newIssueWriter(parsed.provider, giteaOwner, giteaRepo, giteaAPIBase)
			gitOps := newCmdGitOps(parsed.provider, giteaOwner, giteaRepo, giteaAPIBase)
			issueCfg, err := newIssueConfig(ctx, issue.Number, repoRoot, templateDir, fetcher, issueWriter, gitOps, parsed.maxTurns)
			if err != nil {
				return fmt.Errorf("creating config for issue #%d: %w", issue.Number, err)
			}
			issueCfg.Scrub = newScrubber(giteaAPIBase)
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
