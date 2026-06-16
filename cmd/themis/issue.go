package main

type issueArgs struct {
	number   int
	provider string
}

// parseIssueArgs parses [<number> [--provider github|gitea]] from args.
func parseIssueArgs(args []string) (issueArgs, error) { return issueArgs{}, nil }
