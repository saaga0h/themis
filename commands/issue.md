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

### What counts as blocking

Only fix findings that are **strictly blocking**. The threshold is high:

- **Security vulnerability** — exploitable in Hestia's threat model (not theoretical)
- **AC not covered** — a specified behaviour has no test and no implementation
- **Compile failure** — the code does not build
- **Abstraction boundary violated** — directly contradicts CODING_STANDARDS.md rules
- **Data loss or corruption** — incorrect state transitions, lost writes

Everything else is **non-blocking** regardless of how the reviewer words it:
- Style preferences
- "Consider" or "could be improved" suggestions
- Performance concerns without a concrete benchmark
- Missing features beyond the AC scope
- Redundant code that doesn't affect correctness
- Medium-severity findings that require future issues to address properly

**Do not fix non-blocking findings.** Note them in the PR description. Do not
let reviewer suggestions expand the scope of the implementation. The human
reviewer decides what warrants a follow-up issue.

### Cycle procedure

Cycle N:
1. Delegate to **review command** (via Task) with `--last-plan` flag
2. Categorise each finding strictly: blocking (per above) or non-blocking
3. If no blocking findings: proceed to Step 8
4. Fix each blocking finding — run **test-runner** after each fix to confirm GREEN
5. Increment cycle counter. If counter = 3 and blocking findings remain:
   - Comment on the issue: "Review cycle limit reached. Remaining blocking
     findings: <list>. Human review required."
   - Add `blocked` label. Stop.

Non-blocking findings are collected and included in the PR description as a
"Review Notes" section so the human reviewer can decide what to act on.

---

## Step 8: Update documentation

Documentation must not drift from the code. After review passes, update docs
scoped to what this issue changed — not a full audit, just the diff.

### 8a: Determine what changed

```bash
git diff main...HEAD --name-only
```

Group the changed files by package/subsystem. This is your scope.

### 8b: Scan docs for drift in scope

Delegate to **doc-scanner** with explicit scope: the changed packages only.
Instruct it to check:
- Do any existing docs describe interfaces, types, or behaviour that changed?
- Were new public interfaces, types, or packages added that have no doc coverage?
- Does `docs/content-plan.md` reference all relevant docs?

If doc-scanner finds nothing that needs updating: skip to Step 9.

### 8c: Update drifted docs

For each doc that needs updating, delegate to **doc-writer** with:
- The target file path
- The specific drift (what changed in code, what the doc currently says)
- Instruction to update only the drifted sections — do not rewrite the whole doc

Rules:
- New package added → update `ARCHITECTURE.md` component inventory and
  create `docs/subsystems/<name>/README.md` if the package is substantial
- New public interface or type → update the relevant subsystem README
- Existing behaviour changed → update any doc that describes that behaviour
- New doc created → update `docs/content-plan.md` with the new entry

Do not create Tier 3 module docs per issue — those are a `/document` judgment call.
Do not rewrite CONCEPTS.md per issue — that is a human decision via `/document`.

### 8d: Commit doc updates

If any docs were updated:
```
docs(<scope>): update documentation for issue #$ARGUMENTS
```

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

- **Review Notes** section listing all non-blocking findings for the human reviewer
- Documentation changes made (if any)

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
Non-blocking findings: <n> (noted in PR — human reviewer to decide)

### Documentation
<list of docs updated, or "no changes needed">

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
