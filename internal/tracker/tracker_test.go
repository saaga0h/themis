package tracker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saaga0h/themis/internal/tracker"
)

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

func TestGiteaFetcher_FetchesFromAPI(t *testing.T) {
	issue := map[string]interface{}{
		"number":   11,
		"title":    "Wire pipeline",
		"body":     "- [ ] AC item one\n- [ ] AC item two",
		"labels":   []map[string]interface{}{{"name": "ready-for-agent"}},
		"state":    "open",
		"html_url": "https://git.example.com/owner/repo/issues/11",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(issue); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "test-token", 5*time.Second)
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

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "token", 5*time.Second)
	_, err := f.Fetch(context.Background(), 999)
	if err == nil {
		t.Error("GiteaFetcher.Fetch on 404 should return error")
	}
}

func TestNewFetcher_GitHub(t *testing.T) {
	f, err := tracker.NewFetcher("github", "", "", "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("NewFetcher(github) error: %v", err)
	}
	if f == nil {
		t.Error("NewFetcher(github) returned nil")
	}
}

func TestNewFetcher_Gitea(t *testing.T) {
	f, err := tracker.NewFetcher("gitea", "owner", "repo", "https://example.com", "token", 5*time.Second)
	if err != nil {
		t.Fatalf("NewFetcher(gitea) error: %v", err)
	}
	if f == nil {
		t.Error("NewFetcher(gitea) returned nil")
	}
}

func TestNewFetcher_InvalidProvider(t *testing.T) {
	_, err := tracker.NewFetcher("invalid", "", "", "", "", 5*time.Second)
	if err == nil {
		t.Error("NewFetcher(invalid) should return error")
	}
}

func TestIssueData_HasRefField(t *testing.T) {
	issue := &tracker.IssueData{
		Number: 21,
		Title:  "Test",
		Ref:    "feature-branch",
	}
	if issue.Ref != "feature-branch" {
		t.Errorf("IssueData.Ref: got %q, want %q", issue.Ref, "feature-branch")
	}
}

func TestGiteaFetcher_PopulatesRefFromAPIResponse(t *testing.T) {
	issue := map[string]interface{}{
		"number":   21,
		"title":    "Read PR target from ref",
		"body":     "- [ ] AC",
		"labels":   []map[string]interface{}{},
		"html_url": "https://gitea.example.com/owner/repo/issues/21",
		"ref":      "feature-branch",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(issue); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "token", 5*time.Second)
	got, err := f.Fetch(context.Background(), 21)
	if err != nil {
		t.Fatalf("GiteaFetcher.Fetch error: %v", err)
	}
	if got.Ref != "feature-branch" {
		t.Errorf("GiteaFetcher.Fetch Ref: got %q, want %q", got.Ref, "feature-branch")
	}
}

func TestGiteaFetcher_RefIsEmptyWhenAbsentFromAPI(t *testing.T) {
	issue := map[string]interface{}{
		"number":   22,
		"title":    "Issue without ref",
		"body":     "",
		"labels":   []map[string]interface{}{},
		"html_url": "https://gitea.example.com/owner/repo/issues/22",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(issue); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "token", 5*time.Second)
	got, err := f.Fetch(context.Background(), 22)
	if err != nil {
		t.Fatalf("GiteaFetcher.Fetch error: %v", err)
	}
	if got.Ref != "" {
		t.Errorf("GiteaFetcher.Fetch Ref: got %q, want empty when absent", got.Ref)
	}
}

func TestParseGitHubJSON_PopulatesRefWhenPresent(t *testing.T) {
	input := `{
		"number": 21,
		"title": "Read PR target from ref",
		"body": "- [ ] AC",
		"labels": [],
		"state": "open",
		"url": "https://github.com/owner/repo/issues/21",
		"ref": "feature-branch"
	}`
	got, err := tracker.ParseGitHubJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseGitHubJSON error: %v", err)
	}
	if got.Ref != "feature-branch" {
		t.Errorf("ParseGitHubJSON Ref: got %q, want %q", got.Ref, "feature-branch")
	}
}

func TestParseGitHubJSON_RefIsEmptyWhenAbsent(t *testing.T) {
	input := `{
		"number": 22,
		"title": "Issue without ref",
		"body": "",
		"labels": [],
		"state": "open",
		"url": "https://github.com/owner/repo/issues/22"
	}`
	got, err := tracker.ParseGitHubJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseGitHubJSON error: %v", err)
	}
	if got.Ref != "" {
		t.Errorf("ParseGitHubJSON Ref: got %q, want empty when absent", got.Ref)
	}
}

func TestGiteaFetcher_ReturnsErrorOnTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	t.Cleanup(func() {
		close(block)
		srv.Close()
	})

	f := tracker.NewGiteaFetcher("owner", "repo", srv.URL, "", time.Millisecond)
	_, err := f.Fetch(context.Background(), 1)
	if err == nil {
		t.Error("GiteaFetcher.Fetch: expected non-nil error when server does not respond, got nil")
	}
}

func TestParseIssueItems_EmptySlice(t *testing.T) {
	result := tracker.ParseIssueItems([]tracker.IssueItem{})
	if len(result) != 0 {
		t.Errorf("ParseIssueItems(empty): got len %d, want 0", len(result))
	}
}

func TestParseIssueItems_SingleItem_AllFieldsPopulated(t *testing.T) {
	items := []tracker.IssueItem{
		{
			Number: 7,
			Title:  "foo",
			Body:   "bar",
			Labels: []tracker.IssueItemLabel{{Name: "lbl1"}, {Name: "lbl2"}},
		},
	}
	result := tracker.ParseIssueItems(items)
	if len(result) != 1 {
		t.Fatalf("ParseIssueItems: got len %d, want 1", len(result))
	}
	got := result[0]
	if got.Number != 7 {
		t.Errorf("Number: got %d, want 7", got.Number)
	}
	if got.Title != "foo" {
		t.Errorf("Title: got %q, want %q", got.Title, "foo")
	}
	if got.Body != "bar" {
		t.Errorf("Body: got %q, want %q", got.Body, "bar")
	}
	if len(got.Labels) != 2 {
		t.Fatalf("Labels: got %v (len %d), want 2 items", got.Labels, len(got.Labels))
	}
	if got.Labels[0] != "lbl1" {
		t.Errorf("Labels[0]: got %q, want %q", got.Labels[0], "lbl1")
	}
	if got.Labels[1] != "lbl2" {
		t.Errorf("Labels[1]: got %q, want %q", got.Labels[1], "lbl2")
	}
}

func TestParseIssueItems_MultipleItems_PreservesOrder(t *testing.T) {
	items := []tracker.IssueItem{
		{Number: 3, Title: "first"},
		{Number: 1, Title: "second"},
	}
	result := tracker.ParseIssueItems(items)
	if len(result) != 2 {
		t.Fatalf("ParseIssueItems: got len %d, want 2", len(result))
	}
	if result[0].Number != 3 {
		t.Errorf("result[0].Number: got %d, want 3", result[0].Number)
	}
	if result[1].Number != 1 {
		t.Errorf("result[1].Number: got %d, want 1", result[1].Number)
	}
}

func TestParseIssueItems_ItemWithNoLabels_LabelsIsEmpty(t *testing.T) {
	items := []tracker.IssueItem{
		{Number: 5, Title: "no labels", Body: "body text", Labels: []tracker.IssueItemLabel{}},
	}
	result := tracker.ParseIssueItems(items)
	if len(result) != 1 {
		t.Fatalf("ParseIssueItems: got len %d, want 1", len(result))
	}
	if len(result[0].Labels) != 0 {
		t.Errorf("Labels: got %v, want empty", result[0].Labels)
	}
}

func TestParseIssueItems_ItemWithMultipleLabels(t *testing.T) {
	items := []tracker.IssueItem{
		{
			Number: 10,
			Title:  "multi-label",
			Labels: []tracker.IssueItemLabel{
				{Name: "alpha"},
				{Name: "beta"},
				{Name: "gamma"},
			},
		},
	}
	result := tracker.ParseIssueItems(items)
	if len(result) != 1 {
		t.Fatalf("ParseIssueItems: got len %d, want 1", len(result))
	}
	wantLabels := []string{"alpha", "beta", "gamma"}
	gotLabels := result[0].Labels
	if len(gotLabels) != len(wantLabels) {
		t.Fatalf("Labels: got %v (len %d), want %v (len %d)", gotLabels, len(gotLabels), wantLabels, len(wantLabels))
	}
	for i, want := range wantLabels {
		if gotLabels[i] != want {
			t.Errorf("Labels[%d]: got %q, want %q", i, gotLabels[i], want)
		}
	}
}
