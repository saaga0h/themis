package runner

import (
	"context"

	"git.home.federation.fi/lavernea/themis/internal/agent"
	"git.home.federation.fi/lavernea/themis/internal/pipeline"
	"git.home.federation.fi/lavernea/themis/internal/tracker"
)

// IssueWriter handles issue tracker write operations.
type IssueWriter interface {
	AddLabel(ctx context.Context, number int, label string) error
	RemoveLabel(ctx context.Context, number int, label string) error
	Comment(ctx context.Context, number int, body string) error
	CreatePR(ctx context.Context, opts PROptions) (string, error)
}

// PROptions holds the parameters for creating a pull request.
type PROptions struct {
	Title  string
	Body   string
	Base   string
	Head   string
}

// Config holds all dependencies for a pipeline run.
type Config struct {
	WorkDir      string
	IssueNumber  int
	Fetcher      tracker.Fetcher
	Invoker      agent.Invoker
	IssueWriter  IssueWriter
	TemplateDir  string
	CheckpointFn func(ctx context.Context, step pipeline.Step, workDir string) error
	TestACKey    string
}

// Result holds the outcome of a successful pipeline run.
type Result struct {
	PRURL string
}

// Run executes the full pipeline for the given configuration.
func Run(ctx context.Context, cfg Config) (*Result, error) { return nil, nil }
