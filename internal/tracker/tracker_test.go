package tracker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// AC: AC parser extracts checkbox items from issue body into a structured list of strings

func TestParseCheckboxes_ExtractsBothCheckedAndUnchecked(t *testing.T) {
	body := "## Summary\n\nSome text.\n\n## Acceptance Criteria\n\n- [ ] First item\n- [x] Second item already done\n- [ ] Third item\n\n## Notes\n\nSome notes."
	got := tracker.ParseCheckboxes(body)
	want := []string{"First item", "Second item already done", "Third item"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckboxes: got %d items %v, want %d %v", len(got), got, len(want), want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("item[%d]: got %q, want %q", i, g, want[i])
		}
	}
}

func TestParseCheckboxes_EmptyBody(t *testing.T) {
	got := tracker.ParseCheckboxes("")
	if len(got) != 0 {
		t.Errorf("ParseCheckboxes(\"\") = %v, want empty", got)
	}
}

func TestParseCheckboxes_NoCheckboxes(t *testing.T) {
	body := "## Description\n\nJust some text without checkboxes."
	got := tracker.ParseCheckboxes(body)
	if len(got) != 0 {
		t.Errorf("ParseCheckboxes (no checkboxes) = %v, want empty", got)
	}
}

// AC: Issue tracker integration fetches issue data from GitHub via gh issue view --json

func TestParseGitHubJSON_ExtractsIssueData(t *testing.T) {
	input := `{
		"number": 42,
		"title": "Test Issue",
		"body": "## AC\n- [ ] First AC\n- [x] Second AC",
		"labels": [{"name": "ready-for-agent"}, {"name": "foundation"}],
		"state": "open",
		"url": "https://github.com/owner/repo/issues/42"
	}`
	got, err := tracker.ParseGitHubJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseGitHubJSON error: %v", err)
	}
	if got.Number != 42 {
		t.Errorf("Number: got %d, want 42", got.Number)
	}
	if got.Title != "Test Issue" {
		t.Errorf("Title: got %q, want %q", got.Title, "Test Issue")
	}
	if len(got.Labels) != 2 {
		t.Errorf("Labels: got %v, want 2 items", got.Labels)
	}
	if got.URL != "https://github.com/owner/repo/issues/42" {
		t.Errorf("URL: got %q", got.URL)
	}
}

func TestParseGitHubJSON_InvalidJSON(t *testing.T) {
	_, err := tracker.ParseGitHubJSON([]byte("not json"))
	if err == nil {
		t.Error("ParseGitHubJSON(invalid JSON) should return error")
	}
}

// AC: Issue tracker integration fetches issue data from Gitea API

func TestGiteaFetcher_FetchesFromAPI(t *testing.T) {
	issue := map[string]interface{}{
		"number": 11,
		"title":  "Wire pipeline",
		"body":   "- [ ] AC item one\n- [ ] AC item two",
		"labels": []map[string]interface{}{{"name": "ready-for-agent"}},
		"state":  "open",
		"html_url": "https://git.example.com/owner/repo/issues/11",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(issue); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "test-token")
	ctx := context.Background()
	got, err := f.Fetch(ctx, 11)
	if err != nil {
		t.Fatalf("GiteaFetcher.Fetch error: %v", err)
	}
	if got.Number != 11 {
		t.Errorf("Number: got %d, want 11", got.Number)
	}
	if got.Title != "Wire pipeline" {
		t.Errorf("Title: got %q, want %q", got.Title, "Wire pipeline")
	}
	acs := tracker.ParseCheckboxes(got.Body)
	if len(acs) != 2 {
		t.Errorf("AC count: got %d, want 2", len(acs))
	}
}

func TestGiteaFetcher_ReturnsErrorOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "token")
	_, err := f.Fetch(context.Background(), 999)
	if err == nil {
		t.Error("GiteaFetcher.Fetch on 404 should return error")
	}
}

// NewFetcher factory

func TestNewFetcher_GitHub(t *testing.T) {
	f, err := tracker.NewFetcher("github", "", "", "", "")
	if err != nil {
		t.Fatalf("NewFetcher(github) error: %v", err)
	}
	if f == nil {
		t.Error("NewFetcher(github) returned nil")
	}
}

func TestNewFetcher_Gitea(t *testing.T) {
	f, err := tracker.NewFetcher("gitea", "owner", "repo", "https://example.com", "token")
	if err != nil {
		t.Fatalf("NewFetcher(gitea) error: %v", err)
	}
	if f == nil {
		t.Error("NewFetcher(gitea) returned nil")
	}
}

func TestNewFetcher_InvalidProvider(t *testing.T) {
	_, err := tracker.NewFetcher("invalid", "", "", "", "")
	if err == nil {
		t.Error("NewFetcher(invalid) should return error")
	}
}
