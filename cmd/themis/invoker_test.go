package main

import (
	"context"
	"testing"

	"git.home.federation.fi/lavernea/themis/internal/agent"
)

// claudeInvoker wrapper in invoker.go is removed; newIssueConfig wires *agent.ClaudeCodeInvoker directly.
// This test fails before the implementation because newIssueConfig currently uses &claudeInvoker{}.
func TestNewIssueConfig_UsesClaudeCodeInvokerDirectly(t *testing.T) {
	dir := initGitRepoCheckpointTest(t)
	cfg, err := newIssueConfig(context.Background(), 42, dir, dir, &stubMainFetcher{}, &stubMainIssueWriter{}, defaultMaxTurns)
	if err != nil {
		t.Fatalf("newIssueConfig: %v", err)
	}
	if _, ok := cfg.Invoker.(*agent.ClaudeCodeInvoker); !ok {
		t.Errorf("Invoker must be *agent.ClaudeCodeInvoker, got %T — remove the claudeInvoker wrapper", cfg.Invoker)
	}
}
