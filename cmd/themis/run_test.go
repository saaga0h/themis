package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewGiteaQuerier_ClientTimeoutIsGiteaClientTimeout asserts that the HTTP
// client created by newGiteaQuerier uses the named constant giteaClientTimeout
// as its Timeout value (AC1, AC3).
func TestNewGiteaQuerier_ClientTimeoutIsGiteaClientTimeout(t *testing.T) {
	q := newGiteaQuerier("owner", "repo", "http://localhost", "token")
	if q.client.Timeout != giteaClientTimeout {
		t.Errorf("newGiteaQuerier client.Timeout = %v, want giteaClientTimeout (%v)", q.client.Timeout, giteaClientTimeout)
	}
}

// TestGiteaQuerier_ListReadyIssues_ReturnsErrorOnTimeout asserts that
// ListReadyIssues returns a non-nil error when the server does not respond
// within the client timeout (AC4).
func TestGiteaQuerier_ListReadyIssues_ReturnsErrorOnTimeout(t *testing.T) {
	_ = giteaClientTimeout // compile-time assertion: constant must be defined

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // block forever until cleanup
	}))
	t.Cleanup(srv.Close)

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

// TestGiteaQuerier_IsOpen_ReturnsErrorOnTimeout asserts that IsOpen returns a
// non-nil error when the server does not respond within the client timeout (AC5).
func TestGiteaQuerier_IsOpen_ReturnsErrorOnTimeout(t *testing.T) {
	_ = giteaClientTimeout // compile-time assertion: constant must be defined

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // block forever until cleanup
	}))
	t.Cleanup(srv.Close)

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
