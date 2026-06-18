package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
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
	IssueNumber  int
	PipelineStep string
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
	cmd.Env = buildCmdEnv(os.Environ(), opts.IssueNumber, opts.PipelineStep)
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

// buildOTELResourceAttributes constructs the value for OTEL_RESOURCE_ATTRIBUTES,
// merging issue.number and pipeline.step with any existing attributes from parentEnv.
func buildOTELResourceAttributes(issueNumber int, stepName string, parentEnv []string) string {
	newAttrs := fmt.Sprintf("issue.number=%d,pipeline.step=%s", issueNumber, stepName)
	for _, entry := range parentEnv {
		if strings.HasPrefix(entry, "OTEL_RESOURCE_ATTRIBUTES=") {
			existing := strings.TrimPrefix(entry, "OTEL_RESOURCE_ATTRIBUTES=")
			if existing != "" {
				return newAttrs + "," + existing
			}
		}
	}
	return newAttrs
}

// buildCmdEnv returns a copy of parentEnv with OTEL_RESOURCE_ATTRIBUTES set to
// include issue.number and pipeline.step, preserving any existing attributes.
func buildCmdEnv(parentEnv []string, issueNumber int, stepName string) []string {
	otelVal := buildOTELResourceAttributes(issueNumber, stepName, parentEnv)
	result := make([]string, 0, len(parentEnv)+1)
	replaced := false
	for _, entry := range parentEnv {
		if strings.HasPrefix(entry, "OTEL_RESOURCE_ATTRIBUTES=") {
			result = append(result, "OTEL_RESOURCE_ATTRIBUTES="+otelVal)
			replaced = true
		} else {
			result = append(result, entry)
		}
	}
	if !replaced {
		result = append(result, "OTEL_RESOURCE_ATTRIBUTES="+otelVal)
	}
	return result
}

