package main

import (
	"fmt"
	"strconv"
)

type issueArgs struct {
	number   int
	provider string
	maxTurns int
}

// parseIssueArgs parses [<number> [--provider github|gitea] [--max-turns N]] from args.
func parseIssueArgs(args []string) (issueArgs, error) {
	if len(args) == 0 || (len(args) > 0 && len(args[0]) > 0 && args[0][0] == '-') {
		return issueArgs{}, fmt.Errorf("issue number is required")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return issueArgs{}, fmt.Errorf("issue number must be an integer, got %q", args[0])
	}

	provider := "github"
	maxTurns := defaultMaxTurns
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			if i+1 >= len(args) {
				return issueArgs{}, fmt.Errorf("--provider requires a value")
			}
			i++
			provider = args[i]
		case "--max-turns":
			if i+1 >= len(args) {
				return issueArgs{}, fmt.Errorf("--max-turns requires a value")
			}
			i++
			maxTurns, err = strconv.Atoi(args[i])
			if err != nil {
				return issueArgs{}, fmt.Errorf("--max-turns must be an integer, got %q", args[i])
			}
			if maxTurns <= 0 {
				return issueArgs{}, fmt.Errorf("--max-turns must be a positive integer, got %d", maxTurns)
			}
		}
	}

	switch provider {
	case "github", "gitea":
	default:
		return issueArgs{}, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}

	return issueArgs{number: n, provider: provider, maxTurns: maxTurns}, nil
}
