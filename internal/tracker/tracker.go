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

// IssueItemLabel is a label entry in a list-issues API response.
type IssueItemLabel struct {
	Name string `json:"name"`
}

// IssueItem is the raw API shape for a list-issues response.
type IssueItem struct {
	Number int              `json:"number"`
	Title  string           `json:"title"`
	Body   string           `json:"body"`
	Labels []IssueItemLabel `json:"labels"`
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
	Number int              `json:"number"`
	Title  string           `json:"title"`
	Body   string           `json:"body"`
	State  string           `json:"state"`
	URL    string           `json:"url"`
	Ref    string           `json:"ref"`
	Labels []IssueItemLabel `json:"labels"`
}

// ParseGitHubJSON parses the JSON output of `gh issue view --json`.
func ParseGitHubJSON(data []byte) (*IssueData, error) {
	var gh ghIssue
	if err := json.Unmarshal(data, &gh); err != nil {
		return nil, fmt.Errorf("parsing gh JSON: %w", err)
	}
	return &IssueData{
		Number: gh.Number,
		Title:  gh.Title,
		Body:   gh.Body,
		Labels: extractLabelNames(gh.Labels),
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
	Number  int              `json:"number"`
	Title   string           `json:"title"`
	Body    string           `json:"body"`
	HTMLURL string           `json:"html_url"`
	Ref     string           `json:"ref"`
	Labels  []IssueItemLabel `json:"labels"`
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

	return &IssueData{
		Number: gi.Number,
		Title:  gi.Title,
		Body:   gi.Body,
		Labels: extractLabelNames(gi.Labels),
		URL:    gi.HTMLURL,
		Ref:    gi.Ref,
	}, nil
}

// ParseIssueItems converts a slice of IssueItem into a slice of *IssueData.
func ParseIssueItems(items []IssueItem) []*IssueData {
	result := make([]*IssueData, 0, len(items))
	for _, item := range items {
		result = append(result, &IssueData{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			Labels: extractLabelNames(item.Labels),
		})
	}
	return result
}

func extractLabelNames(labels []IssueItemLabel) []string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
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
