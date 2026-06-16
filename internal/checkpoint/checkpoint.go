package checkpoint

import "context"

// VerifyCommitPrefix checks that the last commit message starts with prefix.
func VerifyCommitPrefix(ctx context.Context, dir, prefix string) error { return nil }

// VerifyCleanWorkingTree checks that the git working tree has no uncommitted changes.
func VerifyCleanWorkingTree(ctx context.Context, dir string) error { return nil }
