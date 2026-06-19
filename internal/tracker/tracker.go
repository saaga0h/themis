package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// IssueData holds the data extracted from an issue tracker.
type IssueData struct {
	Number int
	Title  string
	Body   string
	Labels []string
	URL    string
	Ref    string
}

// Fetcher retrieves a single issue from an issue tracker.
type Fetcher interface {
	Fetch(ctx context.Context, number int) (*IssueData, error)
}

var checkboxRE = regexp.MustCompile(`(?m)^- \[[ xX]\] (.+)$`)

// ParseCheckboxes extracts checkbox items from a markdown body.
// Both unchecked (- [ ]) and checked (- [x]) items are returned.
func ParseCheckboxes(body string) []string {
	matches := checkboxRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	items := make([]string, 0, len(matches))
	for _, m := range matches {
		items = append(items, strings.TrimSpace(m[1]))
	}
	return items
}

// ghIssue mirrors the JSON shape returned by `gh issue view --json`.
type ghIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// ParseGitHubJSON parses the JSON output of `gh issue view --json`.
func ParseGitHubJSON(data []byte) (*IssueData, error) {
	var gh ghIssue
	if err := json.Unmarshal(data, &gh); err != nil {
		return nil, fmt.Errorf("parsing gh JSON: %w", err)
	}
	labels := make([]string, 0, len(gh.Labels))
	for _, l := range gh.Labels {
		labels = append(labels, l.Name)
	}
	return &IssueData{
		Number: gh.Number,
		Title:  gh.Title,
		Body:   gh.Body,
		Labels: labels,
		URL:    gh.URL,
		Ref:    gh.Ref,
	}, nil
}

// GitHubFetcher fetches issues via the `gh` CLI.
type GitHubFetcher struct{}

// Fetch retrieves issue number via `gh issue view --json`.
func (g *GitHubFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "view",
		fmt.Sprintf("%d", number),
		"--json", "number,title,body,labels,state,url",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue view %d: %w", number, err)
	}
	return ParseGitHubJSON(out)
}

// GiteaFetcher fetches issues from a Gitea instance via its REST API.
type GiteaFetcher struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

// NewGiteaFetcher creates a GiteaFetcher for the given repo.
func NewGiteaFetcher(owner, repo, apiBase, token string, timeout time.Duration) *GiteaFetcher {
	return &GiteaFetcher{
		owner:   owner,
		repo:    repo,
		apiBase: strings.TrimRight(apiBase, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

// giteaIssue mirrors the Gitea API issue shape.
type giteaIssue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Ref     string `json:"ref"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// Fetch retrieves issue number from Gitea.
func (g *GiteaFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", g.apiBase, g.owner, g.repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gitea API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var gi giteaIssue
	if err := json.NewDecoder(resp.Body).Decode(&gi); err != nil {
		return nil, fmt.Errorf("decoding Gitea response: %w", err)
	}

	labels := make([]string, 0, len(gi.Labels))
	for _, l := range gi.Labels {
		labels = append(labels, l.Name)
	}
	return &IssueData{
		Number: gi.Number,
		Title:  gi.Title,
		Body:   gi.Body,
		Labels: labels,
		URL:    gi.HTMLURL,
		Ref:    gi.Ref,
	}, nil
}

// NewFetcher returns a Fetcher for the given provider.
// provider must be "github" or "gitea". owner, repo, and timeout are required for Gitea.
func NewFetcher(provider, owner, repo, apiBase, token string, timeout time.Duration) (Fetcher, error) {
	switch provider {
	case "github":
		return &GitHubFetcher{}, nil
	case "gitea":
		return NewGiteaFetcher(owner, repo, apiBase, token, timeout), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}
}
