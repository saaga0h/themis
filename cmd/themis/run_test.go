package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

func TestNewGiteaQuerier_ClientTimeoutIsGiteaClientTimeout(t *testing.T) {
	q := newGiteaQuerier("owner", "repo", "http://localhost", "token")
	if q.client.Timeout != giteaClientTimeout {
		t.Errorf("newGiteaQuerier client.Timeout = %v, want giteaClientTimeout (%v)", q.client.Timeout, giteaClientTimeout)
	}
}

func TestGiteaQuerier_ListReadyIssues_ReturnsErrorOnTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // block forever until cleanup
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	q := &GiteaQuerier{
		owner:   "owner",
		repo:    "repo",
		apiBase: srv.URL,
		token:   "",
		client:  &http.Client{Timeout: time.Millisecond},
	}

	_, err := q.ListReadyIssues(context.Background())
	if err == nil {
		t.Error("ListReadyIssues: expected non-nil error when server does not respond, got nil")
	}
}

func TestGiteaQuerier_IsOpen_ReturnsErrorOnTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // block forever until cleanup
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	q := &GiteaQuerier{
		owner:   "owner",
		repo:    "repo",
		apiBase: srv.URL,
		token:   "",
		client:  &http.Client{Timeout: time.Millisecond},
	}

	_, err := q.IsOpen(context.Background(), 1)
	if err == nil {
		t.Error("IsOpen: expected non-nil error when server does not respond, got nil")
	}
}

// TestGiteaQuerier_ListReadyIssues_MapsFieldsViaParseIssueItems uses
// tracker.IssueItem and tracker.ParseIssueItems directly — this test fails to
// compile if those types are removed, and asserts output matches ParseIssueItems.
func TestGiteaQuerier_ListReadyIssues_MapsFieldsViaParseIssueItems(t *testing.T) {
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

	q := &GiteaQuerier{
		owner:   "owner",
		repo:    "repo",
		apiBase: srv.URL,
		token:   "",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	result, err := q.ListReadyIssues(context.Background())
	if err != nil {
		t.Fatalf("ListReadyIssues: unexpected error: %v", err)
	}

	expected := tracker.ParseIssueItems(payload)
	if len(result) != len(expected) {
		t.Fatalf("ListReadyIssues: got %d issues, want %d", len(result), len(expected))
	}
	got := result[0]
	want := expected[0]
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
