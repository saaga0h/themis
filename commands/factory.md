---
description: Autonomous software factory loop. Fetches open ready-for-agent issues in order, runs /issue on each, continues until the backlog is empty or a blocking failure occurs. Designed to run unattended inside a sandbox.
argument-hint: [--provider github|gitea] [--max <n>] [--dry-run]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, mcp
---

# Factory Command

You are the autonomous software factory loop. You process open issues
labeled `ready-for-agent` one at a time, in order, until the backlog is empty
or something requires human attention.

**You run inside a sandboxed container with `--dangerously-skip-permissions`
and `--max-turns`. Do not ask for approval. Run to completion. Stop only when
the backlog is empty, a blocking failure occurs, or turns are exhausted.**

---

## Step 0: Parse arguments

- `--provider github|gitea` — which issue tracker to use (default: `github`)
- `--max <n>` — process at most N issues this run (default: unlimited)
- `--dry-run` — fetch and list issues that would be processed, but do nothing

Set `PROVIDER`, `MAX`, and `DRY_RUN` for use throughout.

---

## Step 1: Fetch the backlog

**If provider = github:**
```bash
gh issue list \
  --label ready-for-agent \
  --state open \
  --json number,title,labels \
  --jq 'sort_by(.number) | .[]'
```

**If provider = gitea:**
Use the `gitea:issue_read` MCP tool with `method: list`, the repo `owner` and
`repo` from the git remote, `type: issues`, `state: open`, and
`labels: ready-for-agent`.

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
2. `CODING_STANDARDS.md` — standards and reviewer checklist

These do not need to be re-read between issues.

---

## Step 3: Process issues in order

For each issue in the backlog (respecting `--max` if set):

### 3a: Check remaining turns

Before starting each issue, assess whether enough turns remain to complete it.
Each issue typically requires 50-150 turns. If you are approaching the
`--max-turns` limit (within ~50 turns), stop the loop cleanly:

```
Turn limit approaching — stopping factory after <n> issues.
Remaining backlog: #X, #Y, #Z
Re-run /factory to continue.
```

Do not start an issue you cannot finish — a half-implemented issue is worse
than an unstarted one.

### 3b: Check for blockers

Before starting an issue, check if it references other open issues as
dependencies. Look for patterns like "depends on #N", "blocked by #N",
or "needs #N" in the issue body.

If a referenced issue is still open: skip this issue and continue to the next.
Log the skip:

```
Skipping #<number> — blocked by #<dependency> (still open)
```

### 3c: Run /issue

Delegate to the **issue command** (via Task) with the issue number and provider:

```
/issue <number> --provider $PROVIDER
```

### 3d: Handle the outcome

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

Comment on the issue:

**If provider = github:**
```bash
gh issue comment $NUMBER --body "Factory run failed unexpectedly on this issue.
Error: <details>
The factory has stopped. Human attention required."
```

**If provider = gitea:**
Use `gitea:issue_write` MCP tool with `method: create_comment`.

Add the `blocked` label. Stop the factory loop.

---

## Step 4: Final report

After the loop completes (backlog empty, --max reached, turn limit approached,
or stopped on failure):

```
## Factory Run Complete

**Provider**: <github|gitea>
**Issues processed**: <n>
**PRs created**: <n>
**Skipped (blocked by dependency)**: <n>
**Stopped on**: <issue number and reason, or "backlog empty">

### Results
✓ #1 — <title> — PR: <url>
✓ #2 — <title> — PR: <url>
✗ #3 — <title> — blocked (see issue comments)

### Remaining backlog
#4 — <title>
#5 — <title>
(re-run /factory to continue)
```

---

## Design principles

**One issue at a time.** The factory does not parallelize. Each `/issue` run
branches from fresh main and targets main. Issues that can safely run in
parallel are identified by the human before labeling — if two issues are
labeled `ready-for-agent` together, the human has verified they touch different
parts of the codebase and their PRs can be merged independently.

**Always branch from main.** `/issue` fetches and pulls main before creating
its branch. This ensures PRs are always based on the latest merged work and
never accidentally stack on another feature branch.

**Stop on unexpected failure.** A crash means something is wrong the human
needs to understand before more code is written.

**Stop on blocked.** A `blocked` label means `/issue` hit its cycle limits or
found something unresolvable. The human needs to intervene.

**Turn-aware.** The factory checks remaining turns before starting each issue
and stops cleanly rather than abandoning a half-implemented issue mid-run.

**Idempotent re-runs.** Issues that completed (no longer `ready-for-agent`)
or are blocked (`blocked` label) will not appear on re-run. The factory
picks up where it can safely continue.

**The human decides what runs in parallel.** The factory never reasons about
which issues are safe to run concurrently — that judgment belongs to the human
who understands the codebase and the dependency graph. The factory's job is
to execute reliably, not to orchestrate parallelism.