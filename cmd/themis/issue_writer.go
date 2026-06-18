package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.home.federation.fi/lavernea/themis/internal/runner"
)

// ghIssueWriter implements runner.IssueWriter using the gh CLI (GitHub).
type ghIssueWriter struct {
	number int
}

func (g *ghIssueWriter) AddLabel(ctx context.Context, number int, label string) error {
	return runGH(ctx, "issue", "edit", fmt.Sprintf("%d", number), "--add-label", label)
}

func (g *ghIssueWriter) RemoveLabel(ctx context.Context, number int, label string) error {
	return runGH(ctx, "issue", "edit", fmt.Sprintf("%d", number), "--remove-label", label)
}

func (g *ghIssueWriter) Comment(ctx context.Context, number int, body string) error {
	return runGH(ctx, "issue", "comment", fmt.Sprintf("%d", number), "--body", body)
}

func (g *ghIssueWriter) CreatePR(ctx context.Context, opts runner.PROptions) (string, error) {
	base := opts.Base
	if base == "" {
		base = "main"
	}
	out, err := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--base", base,
	).Output()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGH(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh %v: %w\n%s", args, err, out)
	}
	return nil
}

// giteaIssueWriter implements runner.IssueWriter using the Gitea REST API.
type giteaIssueWriter struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

func (g *giteaIssueWriter) AddLabel(ctx context.Context, number int, label string) error {
	labelID, err := g.findOrCreateLabel(ctx, label)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]interface{}{"labels": []int{labelID}})
	return g.do(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/labels", g.owner, g.repo, number),
		body, nil)
}

func (g *giteaIssueWriter) RemoveLabel(ctx context.Context, number int, label string) error {
	labelID, err := g.findOrCreateLabel(ctx, label)
	if err != nil {
		return err
	}
	return g.do(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%d", g.owner, g.repo, number, labelID),
		nil, nil)
}

func (g *giteaIssueWriter) Comment(ctx context.Context, number int, body string) error {
	payload, _ := json.Marshal(map[string]string{"body": body})
	return g.do(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", g.owner, g.repo, number),
		payload, nil)
}

func (g *giteaIssueWriter) CreatePR(ctx context.Context, opts runner.PROptions) (string, error) {
	base := opts.Base
	if base == "" {
		base = "main"
	}
	payload, _ := json.Marshal(map[string]string{
		"title": opts.Title,
		"body":  opts.Body,
		"base":  base,
		"head":  opts.Head,
	})
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := g.do(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/pulls", g.owner, g.repo),
		payload, &result); err != nil {
		return "", err
	}
	return result.HTMLURL, nil
}

func (g *giteaIssueWriter) findOrCreateLabel(ctx context.Context, name string) (int, error) {
	var labels []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := g.do(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/labels", g.owner, g.repo),
		nil, &labels); err != nil {
		return 0, err
	}
	for _, l := range labels {
		if l.Name == name {
			return l.ID, nil
		}
	}
	// Create the label.
	payload, _ := json.Marshal(map[string]string{"name": name, "color": "#e11d48"})
	var created struct {
		ID int `json:"id"`
	}
	if err := g.do(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/labels", g.owner, g.repo),
		payload, &created); err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (g *giteaIssueWriter) do(ctx context.Context, method, path string, body []byte, out interface{}) error {
	url := strings.TrimRight(g.apiBase, "/") + "/api/v1" + path
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d", method, url, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// newIssueWriter returns an IssueWriter for the given provider.
// owner, repo, and apiBase are only used for the gitea provider.
func newIssueWriter(provider, owner, repo, apiBase string) runner.IssueWriter {
	switch provider {
	case "gitea":
		return &giteaIssueWriter{
			owner:   owner,
			repo:    repo,
			apiBase: apiBase,
			token:   os.Getenv("GITEA_TOKEN"),
			client:  &http.Client{Timeout: 30 * time.Second},
		}
	default:
		return &ghIssueWriter{}
	}
}
