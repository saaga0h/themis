// Command backtest validates a candidate gate against merged history before it is
// shipped as a green-gate check (issue #111 — admission control: "no gate ships
// without a passing backtest"). It lists the merged feat commits reachable from
// HEAD and, for each, runs a predicate command with the same exit-0-means-pass
// contract as a check block. A predicate that exits non-zero on a known-good
// commit is a *false-block* — the signal the candidate gate is too strict.
//
// The predicate runs via `bash -c` with COMMIT (the commit under test) and BASE
// (its first parent) exported, so a git-ref predicate can inspect either the tree
// (`git grep -q PATTERN $COMMIT`) or the diff (`git diff --name-only $BASE $COMMIT`)
// without mutating the working tree.
//
// Usage:
//
//	backtest [--range REV..REV] [--last N] [-v] --check 'SHELL'
//
// Exit code is 0 when the candidate is admissible (zero false-blocks, zero
// evaluation errors), 1 otherwise — so it is usable as a gate on gates in CI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/saaga0h/themis/internal/backtest"
)

var issueRE = regexp.MustCompile(`#(\d+)`)

func main() {
	check := flag.String("check", "", "candidate predicate, run via `bash -c`; exit 0 = the commit passes (required)")
	revRange := flag.String("range", "", "git revision range to test (default: feat commits reachable from HEAD)")
	last := flag.Int("last", 0, "test only the most recent N matching commits (0 = all)")
	verbose := flag.Bool("v", false, "list every commit, not just false-blocks")
	flag.Parse()

	if strings.TrimSpace(*check) == "" {
		fmt.Fprintln(os.Stderr, "backtest: --check is required")
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	commits, err := listFeatCommits(ctx, ".", *revRange, *last)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backtest: listing history: %v\n", err)
		os.Exit(2)
	}
	if len(commits) == 0 {
		fmt.Fprintln(os.Stderr, "backtest: no feat commits matched — nothing to test")
		os.Exit(2)
	}

	report := backtest.Run(commits, checkPredicate(ctx, ".", *check))
	printReport(report, *verbose)

	if !report.Admissible() {
		os.Exit(1)
	}
}

// listFeatCommits returns the merged feat commits to test, newest first. Without a
// range it takes feat( commits reachable from HEAD; --last bounds the count.
func listFeatCommits(ctx context.Context, dir, revRange string, last int) ([]backtest.Commit, error) {
	args := []string{"-C", dir, "log", "--format=%H%x1f%s", "--grep=^feat", "--extended-regexp"}
	if revRange != "" {
		args = append(args, revRange)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var commits []backtest.Commit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		sha, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue
		}
		commits = append(commits, backtest.Commit{SHA: sha, Subject: subject, Issue: parseIssue(subject)})
		if last > 0 && len(commits) >= last {
			break
		}
	}
	return commits, nil
}

func parseIssue(subject string) int {
	if m := issueRE.FindStringSubmatch(subject); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// checkPredicate runs the candidate command for a commit via `bash -c`, exporting
// COMMIT and BASE. A non-zero exit means the predicate would block that commit; a
// failure to launch the shell is an evaluation error (distinct from a block).
func checkPredicate(ctx context.Context, dir, check string) backtest.Predicate {
	return func(c backtest.Commit) (bool, error) {
		cmd := exec.CommandContext(ctx, "bash", "-c", check)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "COMMIT="+c.SHA, "BASE="+c.SHA+"^")
		err := cmd.Run()
		if err == nil {
			return false, nil // exit 0 → passes
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return true, nil // non-zero exit → would block
		}
		return false, fmt.Errorf("could not evaluate check: %w", err)
	}
}

func printReport(r backtest.Report, verbose bool) {
	for _, res := range r.Results {
		switch {
		case res.Err != nil:
			fmt.Printf("ERROR %s %s — %v\n", short(res.Commit.SHA), label(res.Commit), res.Err)
		case res.Blocked:
			fmt.Printf("BLOCK %s %s\n", short(res.Commit.SHA), label(res.Commit))
		case verbose:
			fmt.Printf("pass  %s %s\n", short(res.Commit.SHA), label(res.Commit))
		}
	}
	fmt.Printf("\nbacktest: would block %d of %d merged-good commits", r.FalseBlocks(), r.Total())
	if r.Errors() > 0 {
		fmt.Printf(", %d not evaluable", r.Errors())
	}
	fmt.Println()
	if r.Admissible() {
		fmt.Println("ADMISSIBLE — the candidate blocks none of known-good history.")
	} else {
		fmt.Println("NOT ADMISSIBLE — resolve or exempt each false-block before shipping this as a gate.")
	}
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func label(c backtest.Commit) string {
	if c.Issue > 0 {
		return fmt.Sprintf("#%d %s", c.Issue, c.Subject)
	}
	return c.Subject
}
