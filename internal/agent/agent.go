package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"git.home.federation.fi/lavernea/themis/internal/git"
)

// InvokeResult holds the structured output of an agent invocation.
type InvokeResult struct {
	ExitCode    int
	Stdout      string
	CommitsMade []string
	TestsPassed bool
	Completed   bool
}

// InvokeOptions configures an agent invocation.
type InvokeOptions struct {
	Prompt       string
	Model        string
	MaxTurns     int
	WorkDir      string
	AllowedTools []string
}

// Invoker is the interface for spawning an agent with a prompt and getting a result.
type Invoker interface {
	Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error)
}

// ClaudeCodeInvoker implements Invoker by spawning the claude CLI.
type ClaudeCodeInvoker struct{}

// Invoke spawns claude with the given options, feeds the prompt via stdin,
// captures stdout, and returns a structured result.
func (c *ClaudeCodeInvoker) Invoke(ctx context.Context, opts InvokeOptions) (*InvokeResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	workDir := opts.WorkDir
	before, err := git.CommitsBefore(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("snapshotting commits: %w", err)
	}

	args := c.buildArgs(opts)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(opts.Prompt)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Non-zero exit or context cancellation.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("agent killed by context: %w", ctx.Err())
		}
		return nil, fmt.Errorf("claude exited with error: %w", err)
	}

	output := stdout.String()
	newCommits, err := git.CommitsAfter(ctx, workDir, before)
	if err != nil {
		return nil, fmt.Errorf("detecting new commits: %w", err)
	}

	return &InvokeResult{
		ExitCode:    0,
		Stdout:      output,
		CommitsMade: newCommits,
		TestsPassed: containsTestPass(output),
		Completed:   containsCompletionMarker(output),
	}, nil
}

func (c *ClaudeCodeInvoker) buildArgs(opts InvokeOptions) []string {
	args := []string{
		"--print",
		"--verbose",
		"--dangerously-skip-permissions",
		"--max-turns", strconv.Itoa(opts.MaxTurns),
		"--model", opts.Model,
	}
	return args
}

// completionMarkers are strings in agent output that signal the agent finished.
var completionMarkers = []string{
	"COMPLETED",
	"Task complete",
	"All ACs pass",
	"Implementation complete",
}

func containsCompletionMarker(output string) bool {
	upper := strings.ToUpper(output)
	for _, marker := range completionMarkers {
		if strings.Contains(upper, strings.ToUpper(marker)) {
			return true
		}
	}
	return false
}

func containsTestPass(output string) bool {
	return strings.Contains(output, "PASS") || strings.Contains(output, "ok ")
}

