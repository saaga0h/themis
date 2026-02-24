---
description: Run code review using specialized review agents. Supports full review, scoped review, or individual perspectives (--security, --architecture, --complexity, --conventions, --coverage, --quick).
argument-hint: [scope] [--flags]
allowed-tools: Read, Glob, Grep, Bash, Task
---

# Review Command

You orchestrate code reviews by delegating to specialized review agents. Each agent has a focused perspective and runs on the appropriate model.

## Step 0: Parse arguments

Parse `$ARGUMENTS` for:
- **Scope**: a directory path, package name, or `--last-plan` (reviews files from most recently completed plan)
- **Perspective flags**: which reviewers to run

| Flag | Agent | Model |
|---|---|---|
| `--architecture` | architecture-reviewer | sonnet |
| `--security` | security-reviewer | sonnet |
| `--complexity` | complexity-reviewer | haiku |
| `--conventions` | convention-reviewer | haiku |
| `--coverage` | coverage-reviewer | haiku |
| `--quick` | complexity + conventions | haiku |
| (no flags) | all five agents | mixed |

If `--last-plan` is specified, find the most recently modified `.md` file in `.claude/plans/` and pass it as scope context to each agent.

## Step 1: Confirm with user

Show what you're about to do:

"**Code Review**
- **Scope**: <full project | directory | last plan: plan-name>
- **Reviewers**: <list of agents to run>
- **Estimated cost**: <N sonnet + M haiku calls>

Proceed?"

Wait for confirmation.

## Step 2: Gather context

Before running reviewers:
- Read CLAUDE.md (needed by architecture-reviewer and convention-reviewer)
- If scoped to a plan, read the plan file to know which files changed
- If scoped to a directory, verify the directory exists

## Step 3: Run reviewers

Delegate to each selected review agent using Task. Pass them:
- The scope (directory, file list from plan, or "full project")
- CLAUDE.md content summary (for architecture and convention reviewers)

Run the haiku agents first (they're faster), then sonnet agents.

**Order**:
1. complexity-reviewer (haiku) — fast structural scan
2. convention-reviewer (haiku) — fast pattern check
3. coverage-reviewer (haiku) — fast coverage map
4. security-reviewer (sonnet) — deeper analysis
5. architecture-reviewer (sonnet) — deepest analysis

Collect each agent's output.

## Step 4: Synthesize

Combine all agent outputs into a unified review report:

```
## Code Review Report

**Scope**: <what was reviewed>
**Date**: <today>
**Reviewers**: <which agents ran>

---

### Overall Verdict: <READY | NEEDS WORK | SIGNIFICANT ISSUES>

### Critical Findings
<anything from any reviewer marked Critical or High severity — these block shipping>

### Architecture
<summary from architecture-reviewer, or "Not reviewed">

### Security
<summary from security-reviewer, or "Not reviewed">

### Complexity
<summary from complexity-reviewer, or "Not reviewed">

### Conventions
<summary from convention-reviewer, or "Not reviewed">

### Test Coverage
<summary from coverage-reviewer, or "Not reviewed">

### Action Items
<numbered list of things to fix, ordered by severity>
1. [CRITICAL] ...
2. [HIGH] ...
3. [MEDIUM] ...
4. [LOW] ...
```

## Step 5: Save report

Save the review report to `.claude/reviews/<scope-or-date>.md` so it's available for reference when running `/ship` or future reviews.

## Step 6: Recommendation

Based on findings, tell the user:

- **READY**: "No blocking issues found. You can run `/ship` to create a PR."
- **NEEDS WORK**: "Found N issues to address. The action items above are ordered by priority."
- **SIGNIFICANT ISSUES**: "Found critical issues that should be resolved before shipping. Consider running `/architect` to plan the fixes if they're non-trivial."

## Important

- Don't duplicate agent work — let each agent do its job and synthesize their outputs
- If an agent finds nothing, that's a good result — say "No issues found" for that section
- The review report on disk should be self-contained — readable without the conversation
- If scoped to `--last-plan`, make sure the plan exists and has completed tasks
- Keep the synthesized report concise — details are in individual agent outputs
- Don't fix anything. Review only. Fixing is `/implement`'s job.
