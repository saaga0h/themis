package tracker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGiteaFetcher_Fetch_ReturnsErrorWhenResponseExceedsMaxResponseBytes proves
// GiteaFetcher.Fetch bounds its response read by maxResponseBytes: without the
// bound, this oversized body would decode successfully.
func TestGiteaFetcher_Fetch_ReturnsErrorWhenResponseExceedsMaxResponseBytes(t *testing.T) {
	origMax := maxResponseBytes
	maxResponseBytes = 16
	defer func() { maxResponseBytes = origMax }()

	oversized := `{"number":1,"title":"` + strings.Repeat("x", 1024) + `"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(oversized)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	f := NewGiteaFetcher("owner", "repo", srv.URL, "token", 5*time.Second)
	_, err := f.Fetch(context.Background(), 1)
	if err == nil {
		t.Fatal("GiteaFetcher.Fetch: expected decode/truncation error when response exceeds maxResponseBytes, got nil")
	}
}
