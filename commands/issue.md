---
description: Implement a single GitHub issue fully autonomously. Fetches the issue, scans the codebase, writes failing tests from ACs, implements until green, reviews, fixes, re-reviews until clean, updates docs, ships PR. No human touchpoints.
argument-hint: <issue-number>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, mcp
---

# Issue Command

You implement a single GitHub issue from start to PR, fully autonomously.
The issue already contains the specification — description, implementation
guidance, and verifiable Acceptance Criteria. Your job is to make every AC pass.

**You run inside a sandboxed container with `--dangerously-skip-permissions`.
Do not ask for approval on file operations or tool use. Run to completion.**

The pipeline: fetch → scan → branch from main → tests (RED) → implement (GREEN) → refactor → review → fix → re-review → docs → ship.

## Cycle limits — hard ceilings to prevent dead loops

- **Test-fix attempts**: max 3 attempts to make a failing test pass before stopping
- **Review cycles**: max 3 review → fix → re-review cycles before stopping
- **Refactor passes**: max 1 refactor pass — do not loop on refactoring

When any limit is hit: comment on the GitHub issue explaining exactly what was
tried and why it's stuck, add the `blocked` label, and stop. Do not guess,
do not try a fourth time.

---

## Step 1: Fetch and parse the issue

```bash
gh issue view $ARGUMENTS --json number,title,body,labels,state,url
```

If the issue is **closed**: stop and report — nothing to do.

If the issue is **not labeled `ready-for-agent`**: comment on the issue explaining
it was skipped, then stop.

Extract from the issue body:
- Title
- Context / description
- Acceptance Criteria (the checkbox list — these are your tests)
- Notes / constraints

Save to `.claude/issues/issue-$ARGUMENTS.md`:

```markdown
# Issue #<number>: <title>

**URL**: <url>

## Description
<context and what to build>

## Acceptance Criteria
<verbatim checkbox list from issue>

## Notes
<verbatim notes section>
```

---

## Step 2: Read the foundations

Before touching any code, read:

1. `UBIQUITOUS_LANGUAGE.md` at the repo root — canonical terminology.
   Every identifier, comment, and commit message must use these terms exactly.
   Terminology violations are review failures.

2. `CODING_STANDARDS.md` — language standards, architecture rules, reviewer
   checklist. Implement to these standards from the start.

Then delegate to **codebase-scanner** with the issue title and key terms as scope.
Read any docs the scanner resolves. Do not read beyond what it returns.

---

## Step 3: Create a branch from main

**Always branch from a fresh main.** Never branch from another feature branch.
PRs must always target main.

```bash
git fetch origin
git checkout main
git pull origin main
git checkout -b issue/$ARGUMENTS-<slug>
```

`<slug>` is a short kebab-case summary of the issue title, max 5 words.
Example: `issue/3-spec-schema-validation`

If `git pull origin main` fails (e.g. merge conflict, dirty state): stop and
comment on the issue explaining the state of the repo. Do not proceed.

---

## Step 4: Write failing tests from ACs (RED)

Each Acceptance Criteria item becomes one or more tests. Do not write
implementation code yet.

For each AC:
- Determine the appropriate test type (unit, integration — refer to CODING_STANDARDS)
- Write a test that will fail because the implementation doesn't exist yet
- The test must assert the exact behaviour the AC describes — not a proxy for it

Delegate to **test-runner** after writing all tests. Confirm they all FAIL.
If any test passes without implementation, the test is wrong — fix it.

Commit the failing tests:
```
test(<scope>): add failing tests for issue #$ARGUMENTS
```

---

## Step 5: Implement until GREEN

Implement the code needed to make the tests pass.

Work AC by AC — implement the minimum to pass each test, then move to the next.
Do not implement anything not required by an AC.

After each AC's tests pass, delegate to **test-runner** to confirm nothing
regressed.

**Test-fix limit: 3 attempts per AC.** If a test won't pass after 3 distinct
implementation attempts, stop: comment on the issue explaining which AC is
failing and what was tried, add `blocked` label, stop entirely.

When all ACs pass:
- Run the full test suite via **test-runner**
- If anything fails: apply the same 3-attempt limit per failing test

Commit the implementation:
```
feat(<scope>): implement issue #$ARGUMENTS — <title>
```

---

## Step 6: Refactor

One pass only. Review the code just written for: duplication, naming,
readability, adherence to UBIQUITOUS_LANGUAGE.md and CODING_STANDARDS.md.

Make improvements. Delegate to **test-runner** after each change.
If any test turns red: revert the change immediately — do not fix forward.
Do not loop — one refactor pass, then move on.

Commit if changes were made:
```
refactor(<scope>): clean up implementation for issue #$ARGUMENTS
```

---

## Step 7: Review → fix → re-review

**Review cycle limit: 3 cycles maximum.**

Cycle N:
1. Delegate to **review command** (via Task) with `--last-plan` flag
2. Categorise findings as blocking or non-blocking
3. If no blocking findings: proceed to Step 8
4. Fix each blocking finding — run **test-runner** after each fix to confirm GREEN
5. Increment cycle counter. If counter = 3 and blocking findings remain:
   - Comment on the issue: "Review cycle limit reached. Remaining blocking
     findings: <list>. Human review required."
   - Add `blocked` label. Stop.

Non-blocking findings are carried forward to the PR description.

---

## Step 8: Update documentation

Check if any user-facing docs need updating:
- Did the implementation add new public interfaces, types, or behaviour?
- Does the README reference anything that changed?\

If yes: delegate to **doc-updater** agent.
If no: skip.

---

## Step 9: Update the issue label

```bash
gh issue edit $ARGUMENTS --add-label "needs-review" --remove-label "ready-for-agent"
```

---

## Step 10: Ship

Delegate to **ship command** (via Task) with the branch name.

The PR must target `main` — never another feature branch.

The PR description must include:
- `Closes #$ARGUMENTS` in the first line
- Summary of what was implemented
- AC verification table:

```
| AC | Description | Test | Status |
|----|-------------|------|--------|
| 1  | <ac text>   | <test file:line> | ✓ PASS |
```

- Any non-blocking review findings noted

---

## Step 11: Final report

```
## Issue #<number> Complete

**Title**: <title>
**Branch**: issue/<number>-<slug>
**PR**: <pr url>

### AC Verification
<table>

### Review
Cycles used: <n>/3
Blocking findings resolved: <n>
Non-blocking findings: <n> (noted in PR)

### Files Changed
<list>
```

---

## Failure modes — what to do when stuck

- **Issue is ambiguous or contradictory**: comment on the GitHub issue explaining
  the ambiguity. Add `blocked` label. Stop.
- **Test-fix limit (3) reached**: comment with which AC failed and what was tried.
  Add `blocked` label. Stop.
- **Review cycle limit (3) reached**: comment with remaining blocking findings.
  Add `blocked` label. Stop.
- **Repo not in clean state**: comment explaining what was found. Stop.
- **In all cases**: do not guess, do not improvise beyond the AC scope, do not
  ship a PR with known failures. Never target a feature branch as PR base.
