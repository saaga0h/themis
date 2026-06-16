package tracker

import "context"

// IssueData holds the data extracted from an issue tracker.
type IssueData struct {
	Number int
	Title  string
	Body   string
	Labels []string
	URL    string
}

// Fetcher retrieves a single issue from an issue tracker.
type Fetcher interface {
	Fetch(ctx context.Context, number int) (*IssueData, error)
}

// ParseCheckboxes extracts checkbox items from a markdown body.
// Both unchecked (- [ ]) and checked (- [x]) items are returned.
func ParseCheckboxes(body string) []string { return nil }

// ParseGitHubJSON parses the JSON output of `gh issue view --json`.
func ParseGitHubJSON(data []byte) (*IssueData, error) { return nil, nil }

// NewFetcher returns a Fetcher for the given provider.
// provider must be "github" or "gitea".
func NewFetcher(provider, apiBase, token string) (Fetcher, error) { return nil, nil }

// GiteaFetcher fetches issues from a Gitea instance via its REST API.
type GiteaFetcher struct{}

// NewGiteaFetcher creates a GiteaFetcher for the given repo.
func NewGiteaFetcher(owner, repo, apiBase, token string) *GiteaFetcher { return &GiteaFetcher{} }

// Fetch retrieves issue number from Gitea.
func (g *GiteaFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) { return nil, nil }
