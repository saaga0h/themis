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

const querierPageSize = 50

// GiteaQuerier implements an issue querier against the Gitea REST API.
type GiteaQuerier struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

// NewGiteaQuerier creates a GiteaQuerier for the given repo with the specified HTTP timeout.
func NewGiteaQuerier(owner, repo, apiBase, token string, timeout time.Duration) *GiteaQuerier {
	return &GiteaQuerier{
		owner:   owner,
		repo:    repo,
		apiBase: strings.TrimRight(apiBase, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
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

// ListReadyIssues returns all open issues labelled ready-for-agent.
func (q *GiteaQuerier) ListReadyIssues(ctx context.Context) ([]*IssueData, error) {
	var all []IssueItem
	for page := 1; ; page++ {
		pageURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues?state=open&type=issues&limit=%d&page=%d&labels=ready-for-agent",
			q.apiBase, q.owner, q.repo, querierPageSize, page)
		resp, err := q.get(ctx, pageURL)
		if err != nil {
			return nil, fmt.Errorf("listing issues page %d: %w", page, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("gitea API returned %d", resp.StatusCode)
		}
		var items []IssueItem
		decodeErr := json.NewDecoder(resp.Body).Decode(&items)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding issues page %d: %w", page, decodeErr)
		}
		all = append(all, items...)
		if len(items) < querierPageSize {
			break
		}
	}
	return ParseIssueItems(all), nil
}

// IsOpen reports whether issue number is in the open state.
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

// GitHubQuerier implements an issue querier using the gh CLI.
type GitHubQuerier struct{}

// NewGitHubQuerier creates a GitHubQuerier.
func NewGitHubQuerier() *GitHubQuerier {
	return &GitHubQuerier{}
}

// ListReadyIssues returns all open issues labelled ready-for-agent via the gh CLI.
func (q *GitHubQuerier) ListReadyIssues(ctx context.Context) ([]*IssueData, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "list",
		"--label", "ready-for-agent",
		"--state", "open",
		"--json", "number,title,body,labels",
		"--limit", "1000",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	var items []IssueItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	return ParseIssueItems(items), nil
}

// IsOpen reports whether issue number is in the open state via the gh CLI.
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
