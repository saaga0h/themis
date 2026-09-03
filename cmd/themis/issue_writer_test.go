package main

import (
	"testing"
)

// TestNewIssueWriter_GiteaClientTimeoutIsGiteaClientTimeout asserts that the
// HTTP client created by newIssueWriter for the "gitea" provider uses the named
// constant giteaClientTimeout as its Timeout value (AC2, AC3).
func TestNewIssueWriter_GiteaClientTimeoutIsGiteaClientTimeout(t *testing.T) {
	w, ok := newIssueWriter("gitea", "owner", "repo", "http://localhost").(*giteaIssueWriter)
	if !ok {
		t.Fatal("newIssueWriter(\"gitea\", ...) did not return *giteaIssueWriter")
	}
	if w.client.Timeout != giteaClientTimeout {
		t.Errorf("giteaIssueWriter client.Timeout = %v, want giteaClientTimeout (%v)", w.client.Timeout, giteaClientTimeout)
	}
}
