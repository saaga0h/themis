// Package labels defines the issue-tracker label names the factory acts on — the
// ready-for-agent selection label, needs-review, and blocked — as the single
// source of truth shared across the tracker and the runner.
package labels

const (
	ReadyForAgent = "ready-for-agent"
	NeedsReview   = "needs-review"
	Blocked       = "blocked"
)
