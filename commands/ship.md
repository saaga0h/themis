---
description: Create a PR/MR for completed work. Reads plan, review report, and git diff to compose the PR description. Uses GitHub/GitLab MCP if available, falls back to CLI.
argument-hint: [plan-name] [--skip-review] [--no-confirm] [--pr-sections <path>]
allowed-tools: Read, Glob, Grep, Bash, Task, mcp
---

# Ship Command

You create pull requests / merge requests for completed work. You compose the PR description from available artifacts (plans, reviews) and the actual git diff.

## Step 0: Pre-flight checks

Before anything else, verify:

1. **Git state**: Are there uncommitted changes? If yes, ask the user if they want to commit first.
2. **Branch**: Are we on a feature branch or main/master? If on main, warn the user — they probably want a branch.
3. **Plan**: If `$ARGUMENTS` is provided (excluding flags), read `.claude/plans/$ARGUMENTS.md`. If not, check for the most recently modified plan with completed tasks.
4. **Review**: Check `.claude/reviews/` for a recent review report.
   - If a report exists: proceed.
   - If `--skip-review` is in `$ARGUMENTS`: ask the user to state the reason for skipping, then proceed.
   - If neither: **STOP** — do not proceed:

     ```
     No review report found in .claude/reviews/.
     Run /review before shipping.

     To override: re-run /ship --skip-review and state your reason.
     ```

## Step 1: Gather context

Collect from available sources:
- **Plan file**: what was intended, tasks completed, decisions made
- **Review report**: any findings, overall verdict
- **Git diff**: `git diff main...HEAD` (or appropriate base branch) — what actually changed
- **Git log**: commits on this branch — `git log main..HEAD --oneline`

Do NOT read CLAUDE.md for PR composition — it contains operational constraints, not project descriptions. The plan and git diff tell the story.

## Step 2: Compose PR description

Write the PR description with this structure:

```markdown
## Summary
<1-2 sentences: what this PR does and why>

## Changes
<grouped list of what changed, derived from git diff and plan>

## Context
<why these changes were made — from plan rationale>

## Testing
<what was tested — from plan test criteria and review report>

## Review Notes
<every non-blocking review finding, for the human to triage — or supplied via --pr-sections; see below>
```

Keep it concise. The PR description should help a human reviewer understand what happened and why, not repeat every line of the diff.

**Caller-supplied sections (`--pr-sections <path>`).** A caller may hand you
pre-composed body sections rather than have you derive them. If `--pr-sections
<path>` is in `$ARGUMENTS`, read that file and splice its contents **verbatim**
into the PR body, immediately after the Testing section, in place of the Review
Notes block above. The factory always supplies this — its file holds the AC
verification table and the full Review Notes list, both built by `pr-composer` in a
fresh context precisely so this command never has to compose them from a deep
context. A standalone caller may point `--pr-sections` at any file of pre-built
sections, or omit it.

**When `--pr-sections` is omitted** (a manual ship), compose the Review Notes
yourself from the `/review` findings that were addressed or flagged as known. Do
not narrow a factory-style Review Notes list to only addressed findings — the
unaddressed non-blocking ones are the point of the handoff.

## Step 3: Confirm or create directly

**If `--no-confirm` is in `$ARGUMENTS`**: skip straight to Step 4 — create the PR immediately without showing a preview or waiting. This flag is set by `/issue` when running in the autonomous factory pipeline.

**Otherwise**: display the composed PR before creating it:

"**Ready to create PR**
- **Branch**: <branch name>
- **Base**: <target branch>
- **Title**: <derived from plan name or summary>
- **Files changed**: <count>

<show composed PR description>

Create this PR?"

Wait for confirmation. The user may want to edit the title or description.

## Step 4: Create the PR

Try in order:
1. **GitHub MCP** (if available): Use the GitHub MCP tools to create the PR
2. **GitLab MCP** (if available): Use the GitLab MCP tools to create the MR
3. **GitHub CLI**: `gh pr create --title "..." --body "..." --base main`
4. **GitLab CLI**: `glab mr create --title "..." --description "..." --target-branch main`
5. **Manual**: If none available, output the PR description and tell the user to create it manually

After creation, report the PR URL.

## Step 5: Post-ship

After the PR is created:

1. Note the PR URL in the plan file (append to the end)
2. Tell the user: "PR created: <url>"

Do NOT:
- Merge the PR (that's a human decision)
- Delete the branch
- Close the plan file

## Important

- The PR title should be clear and concise — not a plan file name
- If the git diff is very large, summarize by area rather than listing every file
- If there's no plan file, compose from git diff alone — plans are recommended but not required
- If the user hasn't pushed the branch yet, push it: `git push -u origin <branch>`
- Respect the project's PR conventions if documented in CLAUDE.md
