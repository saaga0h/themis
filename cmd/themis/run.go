package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

type runArgs struct {
	provider string
	dryRun   bool
}

// parseRunArgs parses [--provider github|gitea] [--dry-run] from args.
func parseRunArgs(args []string) (runArgs, error) {
	provider := "github"
	dryRun := false

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
		default:
			return runArgs{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	switch provider {
	case "github", "gitea":
	default:
		return runArgs{}, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}

	return runArgs{provider: provider, dryRun: dryRun}, nil
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

func runLoop(ctx context.Context, cfg loopConfig) error {
	out := cfg.Logger
	if out == nil {
		out = io.Discard
	}

	issues, err := cfg.Querier.ListReadyIssues(ctx)
	if err != nil {
		return fmt.Errorf("listing ready issues: %w", err)
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})

	for _, issue := range issues {
		if cfg.Turns.RemainingFraction() < 0.10 {
			fmt.Fprintf(out, "insufficient turns remaining\n")
			return nil
		}

		if m := dependsOnRE.FindStringSubmatch(issue.Body); m != nil {
			depNum, _ := strconv.Atoi(m[1])
			open, err := cfg.Querier.IsOpen(ctx, depNum)
			if err != nil {
				return fmt.Errorf("checking dependency #%d for issue #%d: %w", depNum, issue.Number, err)
			}
			if open {
				fmt.Fprintf(out, "skipping issue #%d: depends on open issue #%d\n", issue.Number, depNum)
				continue
			}
		}

		if cfg.DryRun {
			fmt.Fprintf(out, "would process issue #%d: %s\n", issue.Number, issue.Title)
			continue
		}

		if err := cfg.RunFn(ctx, issue); err != nil {
			fmt.Fprintf(out, "issue #%d blocked: %v\n", issue.Number, err)
		}
	}

	return nil
}

// unlimitedTurns is used in production where turn-budget tracking is not available.
type unlimitedTurns struct{}

func (u *unlimitedTurns) RemainingFraction() float64 { return 1.0 }

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
		client:  &http.Client{},
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

func (q *GiteaQuerier) ListReadyIssues(ctx context.Context) ([]*tracker.IssueData, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues?state=open&type=issues&limit=50&labels=ready-for-agent",
		q.apiBase, q.owner, q.repo)
	resp, err := q.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("listing issues: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gitea API returned %d", resp.StatusCode)
	}
	var items []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decoding issues: %w", err)
	}
	issues := make([]*tracker.IssueData, 0, len(items))
	for _, item := range items {
		labels := make([]string, 0, len(item.Labels))
		for _, l := range item.Labels {
			labels = append(labels, l.Name)
		}
		issues = append(issues, &tracker.IssueData{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			Labels: labels,
		})
	}
	return issues, nil
}

func (q *GiteaQuerier) IsOpen(ctx context.Context, number int) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", q.apiBase, q.owner, q.repo, number)
	resp, err := q.get(ctx, url)
	if err != nil {
		return false, fmt.Errorf("checking issue #%d: %w", number, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Gitea API returned %d for issue #%d", resp.StatusCode, number)
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
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	var items []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	issues := make([]*tracker.IssueData, 0, len(items))
	for _, item := range items {
		labels := make([]string, 0, len(item.Labels))
		for _, l := range item.Labels {
			labels = append(labels, l.Name)
		}
		issues = append(issues, &tracker.IssueData{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			Labels: labels,
		})
	}
	return issues, nil
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
