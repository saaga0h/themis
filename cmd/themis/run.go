package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

type runArgs struct {
	provider string
	dryRun   bool
	maxTurns int
}

// parseRunArgs parses [--provider github|gitea] [--dry-run] [--max-turns N] from args.
func parseRunArgs(args []string) (runArgs, error) {
	provider := "github"
	dryRun := false
	maxTurns := defaultMaxTurns

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--provider requires a value")
			}
			i++
			provider = args[i]
		case "--dry-run":
			dryRun = true
		case "--max-turns":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--max-turns requires a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return runArgs{}, fmt.Errorf("--max-turns must be an integer, got %q", args[i])
			}
			maxTurns = n
		default:
			return runArgs{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	switch provider {
	case "github", "gitea":
	default:
		return runArgs{}, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}

	return runArgs{provider: provider, dryRun: dryRun, maxTurns: maxTurns}, nil
}

// IssueQuerier lists issues ready for processing and checks individual issue state.
type IssueQuerier interface {
	ListReadyIssues(ctx context.Context) ([]*tracker.IssueData, error)
	IsOpen(ctx context.Context, number int) (bool, error)
}

// TurnTracker reports the fraction of agentic turns remaining (0.0–1.0).
type TurnTracker interface {
	RemainingFraction() float64
}

type loopConfig struct {
	Querier IssueQuerier
	RunFn   func(ctx context.Context, issue *tracker.IssueData) error
	DryRun  bool
	Turns   TurnTracker
	Logger  io.Writer
}

var dependsOnRE = regexp.MustCompile(`(?i)depends on #(\d+)`)

type issueItem struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func toIssueDataList(items []issueItem) []*tracker.IssueData {
	result := make([]*tracker.IssueData, 0, len(items))
	for _, item := range items {
		labels := make([]string, 0, len(item.Labels))
		for _, l := range item.Labels {
			labels = append(labels, l.Name)
		}
		result = append(result, &tracker.IssueData{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			Labels: labels,
		})
	}
	return result
}

func runLoop(ctx context.Context, cfg loopConfig) error {
	out := cfg.Logger
	if out == nil {
		out = io.Discard
	}

	issues, err := cfg.Querier.ListReadyIssues(ctx)
	if err != nil {
		return fmt.Errorf("listing ready issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Fprintf(out, "no ready-for-agent issues found\n")
		return nil
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})

	var processed, blocked, skippedDep, skippedTurns int

	for i, issue := range issues {
		if m := dependsOnRE.FindStringSubmatch(issue.Body); m != nil {
			depNum, _ := strconv.Atoi(m[1])
			open, err := cfg.Querier.IsOpen(ctx, depNum)
			if err != nil {
				fmt.Fprintf(out, "skipping issue #%d: dependency check error for #%d: %v\n", issue.Number, depNum, err)
				skippedDep++
				continue
			}
			if open {
				fmt.Fprintf(out, "skipping issue #%d: depends on open issue #%d\n", issue.Number, depNum)
				skippedDep++
				continue
			}
		}

		if cfg.Turns.RemainingFraction() < 0.10 {
			fmt.Fprintf(out, "insufficient turns remaining\n")
			skippedTurns = len(issues) - i
			break
		}

		if cfg.DryRun {
			fmt.Fprintf(out, "would process issue #%d: %s\n", issue.Number, issue.Title)
			processed++
			continue
		}

		if err := cfg.RunFn(ctx, issue); err != nil {
			fmt.Fprintf(out, "issue #%d blocked: %v\n", issue.Number, err)
			blocked++
		} else {
			processed++
		}
	}

	printRunSummary(out, processed, blocked, skippedDep, skippedTurns, cfg.DryRun)
	return nil
}

// printRunSummary writes a single end-of-run summary line to out, combining all
// non-zero counts. The processed count is always emitted first; in dry-run mode it
// is labelled "would process" since no issues were actually processed.
func printRunSummary(out io.Writer, processed, blocked, skippedDep, skippedTurns int, dryRun bool) {
	processedLabel := "processed"
	if dryRun {
		processedLabel = "would process"
	}
	parts := []string{fmt.Sprintf("%d %s", processed, processedLabel)}
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", blocked))
	}
	if skippedDep > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (dependency)", skippedDep))
	}
	if skippedTurns > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (turns)", skippedTurns))
	}
	fmt.Fprintf(out, "run summary: %s\n", strings.Join(parts, ", "))
}

// envTurns reads the remaining turn fraction from THEMIS_TURNS_REMAINING_FRACTION.
// When unset, it returns 1.0 so the loop runs unrestricted.
type envTurns struct{}

func (e *envTurns) RemainingFraction() float64 {
	s := os.Getenv("THEMIS_TURNS_REMAINING_FRACTION")
	if s == "" {
		return 1.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 1.0
	}
	if f > 1.0 {
		return 1.0
	}
	return f
}

// GiteaQuerier implements IssueQuerier against the Gitea REST API.
type GiteaQuerier struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

func newGiteaQuerier(owner, repo, apiBase, token string) *GiteaQuerier {
	return &GiteaQuerier{
		owner:   owner,
		repo:    repo,
		apiBase: strings.TrimRight(apiBase, "/"),
		token:   token,
		client:  &http.Client{Timeout: giteaClientTimeout},
	}
}

func (q *GiteaQuerier) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if q.token != "" {
		req.Header.Set("Authorization", "token "+q.token)
	}
	return q.client.Do(req)
}

const (
	giteaClientTimeout = 30 * time.Second
	giteaPageSize      = 50
)

func (q *GiteaQuerier) ListReadyIssues(ctx context.Context) ([]*tracker.IssueData, error) {
	var all []issueItem
	for page := 1; ; page++ {
		pageURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues?state=open&type=issues&limit=%d&page=%d&labels=ready-for-agent",
			q.apiBase, q.owner, q.repo, giteaPageSize, page)
		resp, err := q.get(ctx, pageURL)
		if err != nil {
			return nil, fmt.Errorf("listing issues page %d: %w", page, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("gitea API returned %d", resp.StatusCode)
		}
		var items []issueItem
		decodeErr := json.NewDecoder(resp.Body).Decode(&items)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding issues page %d: %w", page, decodeErr)
		}
		all = append(all, items...)
		if len(items) < giteaPageSize {
			break
		}
	}
	return toIssueDataList(all), nil
}

func (q *GiteaQuerier) IsOpen(ctx context.Context, number int) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", q.apiBase, q.owner, q.repo, number)
	resp, err := q.get(ctx, url)
	if err != nil {
		return false, fmt.Errorf("checking issue #%d: %w", number, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("gitea API returned %d for issue #%d", resp.StatusCode, number)
	}
	var issue struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return false, fmt.Errorf("decoding issue #%d: %w", number, err)
	}
	return issue.State == "open", nil
}

// GitHubQuerier implements IssueQuerier using the gh CLI.
type GitHubQuerier struct{}

func (q *GitHubQuerier) ListReadyIssues(ctx context.Context) ([]*tracker.IssueData, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "list",
		"--label", "ready-for-agent",
		"--state", "open",
		"--json", "number,title,body,labels",
		"--limit", "1000",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	var items []issueItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	return toIssueDataList(items), nil
}

func (q *GitHubQuerier) IsOpen(ctx context.Context, number int) (bool, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "view",
		fmt.Sprintf("%d", number),
		"--json", "state",
	).Output()
	if err != nil {
		return false, fmt.Errorf("gh issue view %d: %w", number, err)
	}
	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return false, fmt.Errorf("parsing gh output: %w", err)
	}
	return strings.ToLower(result.State) == "open", nil
}
