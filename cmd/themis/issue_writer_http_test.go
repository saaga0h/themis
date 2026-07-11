package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/runner"
)

// newTestGiteaIssueWriter builds a giteaIssueWriter pointed at the given
// httptest server, with fixed owner/repo/token — no env var reads needed.
func newTestGiteaIssueWriter(srv *httptest.Server) *giteaIssueWriter {
	return &giteaIssueWriter{
		owner:   "o",
		repo:    "r",
		apiBase: srv.URL,
		token:   "test-token",
		client:  srv.Client(),
	}
}

// decodeJSONBody decodes the request body into a generic map for assertions.
func decodeJSONBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	return m
}

// ---------------------------------------------------------------------------
// AC1 — giteaIssueWriter.AddLabel
// ---------------------------------------------------------------------------

func TestGiteaIssueWriter_AddLabel_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"bug"}]`))
	})
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/issues/5/labels", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.AddLabel(t.Context(), 5, "bug")
	if err != nil {
		t.Fatalf("AddLabel: unexpected error: %v", err)
	}
	labelsField, ok := gotBody["labels"].([]interface{})
	if !ok || len(labelsField) != 1 {
		t.Fatalf("AddLabel POST body: got %v, want labels:[7]", gotBody)
	}
	if got := labelsField[0].(float64); got != 7 {
		t.Errorf("AddLabel POST body labels[0]: got %v, want 7", got)
	}
}

func TestGiteaIssueWriter_AddLabel_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"bug"}]`))
	})
	mux.HandleFunc("POST /api/v1/repos/o/r/issues/5/labels", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.AddLabel(t.Context(), 5, "bug")
	if err == nil {
		t.Fatal("AddLabel: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("AddLabel error = %q, want it to contain %q", err.Error(), "HTTP 400")
	}
}

func TestGiteaIssueWriter_AddLabel_LabelNotFound_CreatesThenAdds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	mux.HandleFunc("POST /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	})
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/issues/5/labels", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.AddLabel(t.Context(), 5, "bug")
	if err != nil {
		t.Fatalf("AddLabel: unexpected error: %v", err)
	}
	labelsField, ok := gotBody["labels"].([]interface{})
	if !ok || len(labelsField) != 1 {
		t.Fatalf("AddLabel POST body: got %v, want labels:[42]", gotBody)
	}
	if got := labelsField[0].(float64); got != 42 {
		t.Errorf("AddLabel POST body labels[0]: got %v, want 42", got)
	}
}

// ---------------------------------------------------------------------------
// AC2 — giteaIssueWriter.RemoveLabel
// ---------------------------------------------------------------------------

func TestGiteaIssueWriter_RemoveLabel_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"bug"}]`))
	})
	var gotPath string
	mux.HandleFunc("DELETE /api/v1/repos/o/r/issues/5/labels/7", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.RemoveLabel(t.Context(), 5, "bug")
	if err != nil {
		t.Fatalf("RemoveLabel: unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/labels/7") {
		t.Errorf("RemoveLabel DELETE path = %q, want it to contain %q", gotPath, "/labels/7")
	}
}

func TestGiteaIssueWriter_RemoveLabel_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"bug"}]`))
	})
	mux.HandleFunc("DELETE /api/v1/repos/o/r/issues/5/labels/7", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.RemoveLabel(t.Context(), 5, "bug")
	if err == nil {
		t.Fatal("RemoveLabel: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("RemoveLabel error = %q, want it to contain %q", err.Error(), "HTTP 500")
	}
}

// ---------------------------------------------------------------------------
// AC3 — giteaIssueWriter.Comment
// ---------------------------------------------------------------------------

func TestGiteaIssueWriter_Comment_Success(t *testing.T) {
	const wantBody = "this is a comment"
	mux := http.NewServeMux()
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.Comment(t.Context(), 5, wantBody)
	if err != nil {
		t.Fatalf("Comment: unexpected error: %v", err)
	}
	if gotBody["body"] != wantBody {
		t.Errorf("Comment POST body[\"body\"] = %v, want %q", gotBody["body"], wantBody)
	}
}

func TestGiteaIssueWriter_Comment_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/repos/o/r/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	err := g.Comment(t.Context(), 5, "body")
	if err == nil {
		t.Fatal("Comment: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("Comment error = %q, want it to contain %q", err.Error(), "HTTP 500")
	}
}

// ---------------------------------------------------------------------------
// AC4 — giteaIssueWriter.CreatePR
// ---------------------------------------------------------------------------

func TestGiteaIssueWriter_CreatePR_Success(t *testing.T) {
	mux := http.NewServeMux()
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"html_url":"https://git.example.com/pr/1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	opts := runner.PROptions{Title: "Fix bug", Body: "desc", Head: "branch-x"}
	url, err := g.CreatePR(t.Context(), opts)
	if err != nil {
		t.Fatalf("CreatePR: unexpected error: %v", err)
	}
	if url != "https://git.example.com/pr/1" {
		t.Errorf("CreatePR url = %q, want %q", url, "https://git.example.com/pr/1")
	}
	if gotBody["base"] != "main" {
		t.Errorf("CreatePR POST body[\"base\"] = %v, want %q", gotBody["base"], "main")
	}
	if gotBody["title"] != "Fix bug" {
		t.Errorf("CreatePR POST body[\"title\"] = %v, want %q (no WIP prefix)", gotBody["title"], "Fix bug")
	}
}

func TestGiteaIssueWriter_CreatePR_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	opts := runner.PROptions{Title: "Fix bug", Body: "desc", Head: "branch-x"}
	url, err := g.CreatePR(t.Context(), opts)
	if url != "" {
		t.Errorf("CreatePR url = %q, want empty on error", url)
	}
	if err == nil {
		t.Fatal("CreatePR: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 422") {
		t.Errorf("CreatePR error = %q, want it to contain %q", err.Error(), "HTTP 422")
	}
}

func TestGiteaIssueWriter_CreatePR_MissingURLInResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	opts := runner.PROptions{Title: "Fix bug", Body: "desc", Head: "branch-x"}
	url, err := g.CreatePR(t.Context(), opts)
	// NOTE: this WILL fail RED against current code — current code returns a
	// nil error unconditionally, even when html_url is absent from the
	// response. That is correct and expected; Implement must add the check.
	if err == nil {
		t.Fatal("CreatePR: expected error when response has no html_url, got nil")
	}
	if url != "" {
		t.Errorf("CreatePR url = %q, want empty when html_url missing", url)
	}
}

func TestGiteaIssueWriter_CreatePR_Draft_PrependsWIPTitle(t *testing.T) {
	mux := http.NewServeMux()
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"html_url":"https://git.example.com/pr/2"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	opts := runner.PROptions{Title: "Fix bug", Draft: true}
	url, err := g.CreatePR(t.Context(), opts)
	if err != nil {
		t.Fatalf("CreatePR: unexpected error: %v", err)
	}
	if url != "https://git.example.com/pr/2" {
		t.Errorf("CreatePR url = %q, want %q", url, "https://git.example.com/pr/2")
	}
	if gotBody["title"] != "WIP: Fix bug" {
		t.Errorf("CreatePR POST body[\"title\"] = %v, want %q", gotBody["title"], "WIP: Fix bug")
	}
}

// ---------------------------------------------------------------------------
// AC6 — findOrCreateLabel
// ---------------------------------------------------------------------------

func TestFindOrCreateLabel_LabelExists(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"foo"}]`))
	})
	mux.HandleFunc("POST /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("findOrCreateLabel: POST /labels should not be called when the label already exists")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	id, err := g.findOrCreateLabel(t.Context(), "foo")
	if err != nil {
		t.Fatalf("findOrCreateLabel: unexpected error: %v", err)
	}
	if id != 7 {
		t.Errorf("findOrCreateLabel id = %d, want 7", id)
	}
}

func TestFindOrCreateLabel_LabelCreated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	var gotBody map[string]interface{}
	mux.HandleFunc("POST /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeJSONBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":9}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	id, err := g.findOrCreateLabel(t.Context(), "foo")
	if err != nil {
		t.Fatalf("findOrCreateLabel: unexpected error: %v", err)
	}
	if id != 9 {
		t.Errorf("findOrCreateLabel id = %d, want 9", id)
	}
	if gotBody["name"] != "foo" {
		t.Errorf("findOrCreateLabel POST body[\"name\"] = %v, want %q", gotBody["name"], "foo")
	}
	if gotBody["color"] != "#e11d48" {
		t.Errorf("findOrCreateLabel POST body[\"color\"] = %v, want %q", gotBody["color"], "#e11d48")
	}
}

func TestFindOrCreateLabel_ListError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("POST /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("findOrCreateLabel: POST /labels should not be called when the list request itself fails")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	id, err := g.findOrCreateLabel(t.Context(), "foo")
	if id != 0 {
		t.Errorf("findOrCreateLabel id = %d, want 0 on error", id)
	}
	if err == nil {
		t.Fatal("findOrCreateLabel: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("findOrCreateLabel error = %q, want it to contain %q", err.Error(), "HTTP 500")
	}
}

func TestFindOrCreateLabel_CreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	mux.HandleFunc("POST /api/v1/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newTestGiteaIssueWriter(srv)
	id, err := g.findOrCreateLabel(t.Context(), "foo")
	if id != 0 {
		t.Errorf("findOrCreateLabel id = %d, want 0 on error", id)
	}
	if err == nil {
		t.Fatal("findOrCreateLabel: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("findOrCreateLabel error = %q, want it to contain %q", err.Error(), "HTTP 500")
	}
}
