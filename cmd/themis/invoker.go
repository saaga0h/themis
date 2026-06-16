package main

import (
	"context"

	"git.home.federation.fi/lavernea/themis/internal/agent"
)

// claudeInvoker wraps agent.ClaudeCodeInvoker for use in cmd.
type claudeInvoker struct{}

func (c *claudeInvoker) Invoke(ctx context.Context, opts agent.InvokeOptions) (*agent.InvokeResult, error) {
	return (&agent.ClaudeCodeInvoker{}).Invoke(ctx, opts)
}
