package agent

import "context"

type InvokeResult struct {
	ExitCode    int
	Stdout      string
	CommitsMade []string
	TestsPassed bool
	Completed   bool
}

type InvokeOptions struct {
	Prompt       string
	Model        string
	MaxTurns     int
	WorkDir      string
	AllowedTools []string
}

type Invoker interface {
	Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error)
}

type ClaudeCodeInvoker struct{}

func (c *ClaudeCodeInvoker) Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error) {
	panic("not implemented")
}

func (c *ClaudeCodeInvoker) buildArgs(opts InvokeOptions) []string {
	panic("not implemented")
}

func containsCompletionMarker(output string) bool {
	panic("not implemented")
}
