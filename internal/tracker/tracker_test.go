package tracker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// ---------------------------------------------------------------------------
// AC2, AC6: GiteaQuerier and GitHubQuerier defined in internal/tracker
// with httptest-based assertions (issue #68).
// ---------------------------------------------------------------------------

func TestNewGiteaQuerier_ListReadyIssues_ReturnsErrorOnTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", time.Millisecond)
	_, err := q.ListReadyIssues(context.Background())
	if err == nil {
		t.Error("ListReadyIssues: expected non-nil error when server does not respond, got nil")
	}
}

func TestNewGiteaQuerier_IsOpen_ReturnsErrorOnTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", time.Millisecond)
	_, err := q.IsOpen(context.Background(), 1)
	if err == nil {
		t.Error("IsOpen: expected non-nil error when server does not respond, got nil")
	}
}

func TestNewGiteaQuerier_ListReadyIssues_MapsFieldsViaParseIssueItems(t *testing.T) {
	payload := []tracker.IssueItem{
		{
			Number: 42,
			Title:  "Implement thing",
			Body:   "- [ ] AC one",
			Labels: []tracker.IssueItemLabel{{Name: "ready-for-agent"}, {Name: "enhancement"}},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", 5*time.Second)
	result, err := q.ListReadyIssues(context.Background())
	if err != nil {
		t.Fatalf("ListReadyIssues: unexpected error: %v", err)
	}
	expected := tracker.ParseIssueItems(payload)
	if len(result) != len(expected) {
		t.Fatalf("ListReadyIssues: got %d issues, want %d", len(result), len(expected))
	}
	got, want := result[0], expected[0]
	if got.Number != want.Number {
		t.Errorf("Number: got %d, want %d", got.Number, want.Number)
	}
	if got.Title != want.Title {
		t.Errorf("Title: got %q, want %q", got.Title, want.Title)
	}
	if got.Body != want.Body {
		t.Errorf("Body: got %q, want %q", got.Body, want.Body)
	}
	if len(got.Labels) != len(want.Labels) {
		t.Fatalf("Labels: got %v (len %d), want %v", got.Labels, len(got.Labels), want.Labels)
	}
	for i := range want.Labels {
		if got.Labels[i] != want.Labels[i] {
			t.Errorf("Labels[%d]: got %q, want %q", i, got.Labels[i], want.Labels[i])
		}
	}
}

func TestNewGiteaQuerier_IsOpen_ReturnsTrueForOpenIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"state":"open"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", 5*time.Second)
	open, err := q.IsOpen(context.Background(), 1)
	if err != nil {
		t.Fatalf("IsOpen: unexpected error: %v", err)
	}
	if !open {
		t.Error("IsOpen: expected true for open issue, got false")
	}
}

func TestNewGiteaQuerier_IsOpen_ReturnsFalseForClosedIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"state":"closed"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", 5*time.Second)
	open, err := q.IsOpen(context.Background(), 1)
	if err != nil {
		t.Fatalf("IsOpen: unexpected error: %v", err)
	}
	if open {
		t.Error("IsOpen: expected false for closed issue, got true")
	}
}

func TestNewGiteaQuerier_ListReadyIssues_ReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", 5*time.Second)
	_, err := q.ListReadyIssues(context.Background())
	if err == nil {
		t.Error("ListReadyIssues: expected error on non-200 response, got nil")
	}
}

func TestNewGiteaQuerier_IsOpen_ReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	q := tracker.NewGiteaQuerier("owner", "repo", srv.URL, "", 5*time.Second)
	_, err := q.IsOpen(context.Background(), 99)
	if err == nil {
		t.Error("IsOpen: expected error on non-200 response, got nil")
	}
}

// AC2: GitHubQuerier is defined in internal/tracker.
// This compile-time assertion fails until tracker.NewGitHubQuerier is defined.
func TestNewGitHubQuerier_IsNotNil(t *testing.T) {
	q := tracker.NewGitHubQuerier()
	if q == nil {
		t.Error("NewGitHubQuerier returned nil")
	}
}

// ---------------------------------------------------------------------------
// Issue #98: extract issue-declared `check` blocks into the green gate.
// ParseCheckBlocks, IsDestructiveAC, and ValidateDestructiveChecks are stubs
// today (see internal/tracker/tracker.go) — these tests specify the real
// behaviour and fail against the stubs.
// ---------------------------------------------------------------------------

func TestParseCheckBlocks_ExtractsFencedCommand(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] No Foo remains\n\n```check\n! grep -rq 'type Foo' pkg/\n```\n"
	got := tracker.ParseCheckBlocks(body)
	want := []string{"! grep -rq 'type Foo' pkg/"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v", len(got), got, len(want), want)
	}
	if got[0] != want[0] {
		t.Errorf("ParseCheckBlocks[0]: got %q, want %q", got[0], want[0])
	}
}

func TestParseCheckBlocks_NoBlocksReturnsEmpty(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] Something behavioural\n\n" +
		"```\nplain fenced block, no tag\n```\n\n" +
		"```text\nsome other tagged fence\n```\n"
	got := tracker.ParseCheckBlocks(body)
	if len(got) != 0 {
		t.Errorf("ParseCheckBlocks (no check fences) = %v, want empty", got)
	}
}

func TestParseCheckBlocks_MultipleBlocksPreserveOrder(t *testing.T) {
	body := "```check\nfirst command\n```\n\nSome prose in between the two check blocks.\n\n```check\nsecond command\n```\n"
	got := tracker.ParseCheckBlocks(body)
	want := []string{"first command", "second command"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseCheckBlocks_IgnoresNonCheckFences(t *testing.T) {
	body := "```go\nfunc main() {}\n```\n\n```\nuntagged fence, not a check\n```\n\n```check\nreal check command\n```\n"
	got := tracker.ParseCheckBlocks(body)
	want := []string{"real check command"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v (must ignore go/untagged fences)", len(got), got, len(want), want)
	}
	if got[0] != want[0] {
		t.Errorf("ParseCheckBlocks[0]: got %q, want %q", got[0], want[0])
	}
}

func TestIsDestructiveAC_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		ac   string
		want bool
	}{
		{"no struct remains", "No GiteaQuerier struct remains in cmd/themis", true},
		{"old thing no longer exists", "the old GiteaFetcher no longer exists in cmd/themis", true},
		{"removed hardcoded config", "Removed hardcoded Finna config from GetSources handler", true},
		{"plain behavioural AC", "CreateItem returns 400 for invalid itemType values", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tracker.IsDestructiveAC(tt.ac)
			if got != tt.want {
				t.Errorf("IsDestructiveAC(%q) = %v, want %v", tt.ac, got, tt.want)
			}
		})
	}
}

func TestValidateDestructiveChecks_ErrorsWhenNoCheckBlockPresent(t *testing.T) {
	const offendingAC = "No GiteaQuerier struct remains in cmd/themis"
	body := "## Acceptance Criteria\n- [ ] " + offendingAC + "\n"
	err := tracker.ValidateDestructiveChecks(body)
	if err == nil {
		t.Fatal("ValidateDestructiveChecks: expected error when a destructive AC has no check block, got nil")
	}
	if !strings.Contains(err.Error(), offendingAC) {
		t.Errorf("ValidateDestructiveChecks error must name the offending AC %q, got %q", offendingAC, err.Error())
	}
}

func TestValidateDestructiveChecks_PassesWhenCheckBlockPresent(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] No GiteaQuerier struct remains in cmd/themis\n\n" +
		"```check\n! grep -rq 'type GiteaQuerier' cmd/themis/\n```\n"
	if err := tracker.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (destructive AC is paired with a check block)", err)
	}
}

func TestValidateDestructiveChecks_PassesWhenNoDestructiveACs(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] CreateItem returns 400 for invalid itemType values\n"
	if err := tracker.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (no destructive ACs present)", err)
	}
}
