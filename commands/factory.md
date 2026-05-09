---
description: Autonomous software factory loop. Fetches open ready-for-agent issues in order, runs /issue on each, continues until the backlog is empty or a blocking failure occurs. Designed to run unattended inside a sandbox.
argument-hint: [--max <n>] [--dry-run]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, mcp
---

# Factory Command

You are the autonomous software factory loop. You process open GitHub issues
labeled `ready-for-agent` one at a time, in order, until the backlog is empty
or something requires human attention.

**You run inside a sandboxed container with `--dangerously-skip-permissions`.
Do not ask for approval. Run to completion. Stop only when the backlog is empty
or a blocking failure occurs.**

---

## Step 0: Parse arguments

- `--max <n>` — process at most N issues this run (default: unlimited)
- `--dry-run` — fetch and list issues that would be processed, but do nothing

---

## Step 1: Fetch the backlog

```bash
gh issue list \
  --label ready-for-agent \
  --state open \
  --json number,title,labels \
  --jq 'sort_by(.number) | .[]'
```

If no issues: report and stop.

```
Backlog is empty — nothing to do.
```

If `--dry-run`: list the issues and stop.

```
Issues that would be processed (in order):
#1 — <title>
#2 — <title>
#3 — <title>

Run /factory to process them.
```

---

## Step 2: Read foundations once

Before processing any issue, read these once — they apply to all issues:

1. `UBIQUITOUS_LANGUAGE.md` — canonical terminology, applies to all code
2. `.sandcastle/CODING_STANDARDS.md` or `CODING_STANDARDS.md` — standards and
   reviewer checklist

These do not need to be re-read between issues.

---

## Step 3: Process issues in order

For each issue in the backlog (respecting `--max` if set):

### 3a: Check for blockers

Before starting an issue, check if it references other open issues as
dependencies. Look for patterns like "depends on #N", "blocked by #N",
or "needs #N" in the issue body.

If a referenced issue is still open: skip this issue and continue to the next.
Log the skip:

```
Skipping #<number> — blocked by #<dependency> (still open)
```

### 3b: Run /issue

Delegate to the **issue command** (via Task) with the issue number.

### 3c: Handle the outcome

**Success** — `/issue` completed and created a PR:

```
✓ Issue #<number> — PR created: <url>
```

Continue to the next issue.

**Blocked** — `/issue` added the `blocked` label and stopped:

```
✗ Issue #<number> — blocked, needs human attention
  See issue comments for details.
```

Stop the factory loop. Do not continue to the next issue — a blocked issue
means something unexpected was found that the human needs to see before
the factory continues.

**Unexpected failure** — `/issue` crashed or produced no PR:

```
✗ Issue #<number> — unexpected failure
  <error details>
```

Comment on the GitHub issue:

```bash
gh issue comment $NUMBER --body "Factory run failed unexpectedly on this issue.
Error: <details>
The factory has stopped. Human attention required."
```

Add the `blocked` label. Stop the factory loop.

---

## Step 4: Final report

After the loop completes (backlog empty, --max reached, or stopped on failure):

```
## Factory Run Complete

**Issues processed**: <n>
**PRs created**: <n>
**Skipped (blocked by dependency)**: <n>
**Stopped on**: <issue number and reason, or "backlog empty">

### Results
✓ #1 — <title> — PR: <url>
✓ #2 — <title> — PR: <url>
✗ #3 — <title> — blocked (see issue comments)
```

---

## Design principles

**One issue at a time.** The factory does not parallelize. Issues may depend
on each other and parallel execution would cause conflicts. Sequential is
correct here.

**Stop on unexpected failure.** A crash or unresolvable blocker means something
is wrong that the human needs to understand before more code is written. Do not
skip and continue — stop and report.

**Stop on blocked.** A `blocked` label means `/issue` found something it could
not resolve autonomously. The human needs to see that before more issues are
processed, because the blocker might affect subsequent issues too.

**Dependency ordering.** Issues are processed lowest-number-first by default.
Dependency checks skip issues whose dependencies are still open, but the
human is responsible for writing issues in a resolvable order. The factory
does not try to reorder or resolve dependency graphs beyond simple open/closed
checks.

**Idempotent re-runs.** If the factory is stopped mid-run and re-run, it will
re-fetch the backlog. Issues that were completed (no longer labeled
`ready-for-agent`) will not appear. Issues that were blocked (labeled `blocked`)
will not appear either — they require human intervention before being relabeled.
The factory picks up where it can safely continue.
