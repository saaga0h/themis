---
description: Implement a single issue fully autonomously. Fetches the issue, scans the codebase, writes failing tests from ACs, implements until green, reviews, fixes, re-reviews until clean, updates docs, ships PR. No human touchpoints.
argument-hint: <issue-number> [--provider github|gitea]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, mcp
---

# Issue Command

You implement a single issue from start to PR, fully autonomously.
The issue already contains the specification — description, implementation
guidance, and verifiable Acceptance Criteria. Your job is to make every AC pass.

**You run inside a sandboxed container with `--dangerously-skip-permissions`.
Do not ask for approval on file operations or tool use. Run to completion.**

The pipeline: fetch → scan → branch from main → tests (RED) → implement (GREEN) → refactor → review → fix → re-review → docs → ship.

**Each step produces exactly one commit. Do not combine steps. Do not skip commits.
The commit history must show the pipeline stages clearly.**

## Cycle limits — hard ceilings to prevent dead loops

- **Test-fix attempts**: max 3 attempts to make a failing test pass before stopping
- **Review cycles**: max 2 cycles by default; max 3 only with a context change (see Step 7)
- **Refactor passes**: max 1 refactor pass — do not loop on refactoring

When any limit is hit: comment on the issue explaining exactly what was
tried and why it's stuck, add the `blocked` label, and stop. Do not guess,
do not try a fourth time.

---

## Step 0: Parse arguments

Extract from `$ARGUMENTS`:
- Issue number (required, first positional argument)
- `--provider github|gitea` — which issue tracker to use (default: `github`)

Set `ISSUE_NUMBER` and `PROVIDER` for use throughout.

---

## Step 1: Fetch and parse the issue

**If provider = github:**
```bash
gh issue view $ISSUE_NUMBER --json number,title,body,labels,state,url
```

**If provider = gitea:**
Use the `gitea:issue_read` MCP tool with `method: get`, the repo `owner` and `repo`
from the git remote, and `index: $ISSUE_NUMBER`.

If the issue is **closed**: stop and report — nothing to do.

If the issue is **not labeled `ready-for-agent`**: comment on the issue explaining
it was skipped, then stop.

Extract from the issue body:
- Title
- Context / description
- Acceptance Criteria (the checkbox list — these are your tests)
- Notes / constraints

Save to `.claude/issues/issue-$ISSUE_NUMBER.md`:

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
git checkout -b issue/$ISSUE_NUMBER-<slug>
```

`<slug>` is a short kebab-case summary of the issue title, max 5 words.
Example: `issue/3-spec-schema-validation`

If `git pull origin main` fails (e.g. merge conflict, dirty state): stop and
comment on the issue explaining the state of the repo. Do not proceed.

---

## Step 4: Write failing tests from ACs (RED)

**This step writes ONLY test files. Do NOT create or modify any implementation files.**
**Do NOT think about the implementation yet. Focus only on what the ACs require.**

Each Acceptance Criteria item becomes one or more tests. For each AC:
- Determine the appropriate test type (unit, integration — refer to CODING_STANDARDS)
- Write a test that asserts the exact behaviour the AC describes — not a proxy for it
- If the test needs minimal type stubs to compile, create the absolute minimum
  (empty struct, interface with no methods) — not the real implementation

### 4a: Verify no implementation files were created

After writing all test files, run:
```bash
git diff --name-only | grep -v _test.go | grep -v .claude/ | grep -v doc.go
```
If this command produces ANY output, you have created implementation files.
**Delete them now.** Only `*_test.go` files and minimal stubs (empty types in
existing files) are permitted at this point.

### 4b: Run tests — they MUST fail

Delegate to **test-runner**. Tests must fail or fail to compile.
If any test passes without real implementation, the test is wrong — fix it.

### 4c: Commit the failing tests

```
test(<scope>): add failing tests for issue #$ISSUE_NUMBER
```

### 4d: CHECKPOINT — verify commit and clean working tree

```bash
git log --oneline -1
git status --porcelain
```

The most recent commit MUST start with `test(`.
`git status` MUST show a clean working tree (no modified files, no untracked files
except in `.claude/`).

If the working tree is not clean, you wrote implementation files alongside tests.
**This is a violation.** Run `git checkout -- .` to discard uncommitted changes,
then proceed to Step 5.

Do not proceed to Step 5 until both checks pass.

---

## Step 5: Implement until GREEN

**Verify Step 4's commit exists before writing any implementation:**
```bash
git log --oneline -1 | grep "^[a-f0-9]* test("
```
If this produces no output, STOP — go back to Step 4 and commit the tests first.

Now implement the code needed to make the tests pass.

Work AC by AC — implement the minimum to pass each test, then move to the next.
Do not implement anything not required by an AC.

**Do NOT modify test files during implementation.** If a test is wrong, that
is a signal that the AC needs clarification — comment on the issue and add
`blocked` label, do not silently fix the test to match your implementation.

After each AC's tests pass, delegate to **test-runner** to confirm nothing
regressed.

**Test-fix limit: 3 attempts per AC.** If a test won't pass after 3 distinct
implementation attempts, stop: comment on the issue explaining which AC is
failing and what was tried, add `blocked` label, stop entirely.

When all ACs pass:
- Run the full test suite via **test-runner**
- If anything fails: apply the same 3-attempt limit per failing test

**Commit the implementation:**
```
feat(<scope>): implement issue #$ISSUE_NUMBER — <title>
```

---

## Step 6: Refactor

One pass only. Review the code just written for: duplication, naming,
readability, adherence to UBIQUITOUS_LANGUAGE.md and CODING_STANDARDS.md.

Make improvements. Delegate to **test-runner** after each change.
If any test turns red: revert the change immediately — do not fix forward.
Do not loop — one refactor pass, then move on.

**Commit if changes were made:**
```
refactor(<scope>): clean up implementation for issue #$ISSUE_NUMBER
```

If no refactoring was needed, skip this commit — do not create an empty commit.

---

## Step 7: Review → fix → re-review

**Default review cycle limit: 2 cycles.**
**Round 3 is only permitted when a context change justifies it** — see below.

### What counts as blocking

Only fix findings that are **strictly blocking**. The threshold is high:

- **Security vulnerability** — exploitable in the project's threat model (not theoretical)
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
5. **Commit the fixes for this cycle:**
   ```
   fix(<scope>): resolve review findings cycle N for issue #$ISSUE_NUMBER
   ```
6. Increment cycle counter

### Round 3 gate

After cycle 2, if blocking findings still remain, **do not automatically enter
round 3**. Instead, evaluate whether a context change is available:

**Permitted round 3 triggers** (at least one must apply):
- The issue touches a **security-sensitive surface** (auth, credentials, network
  policy, sandboxing, data persistence) — invoke the security reviewer again with
  explicit focus on the remaining findings
- The change modifies a **public API surface** — invoke the architecture reviewer
  with the full interface diff as additional context
- The change involves **numerical correctness** (floating point, linear algebra,
  statistical computation) — invoke the complexity reviewer with tolerance and
  reference expectations as context
- A **new context artifact** is available that wasn't present in cycle 2 (e.g.
  a related issue's resolution just landed on main, a doc was updated)

If none of these triggers apply after cycle 2: do not enter round 3.
Comment on the issue: "Review cycle limit reached after 2 cycles. No round 3
trigger applies. Remaining blocking findings: <list>. Human review required."
Add `blocked` label. Stop.

If a trigger applies: enter round 3 with the trigger explicitly noted in the
review invocation as additional context. If round 3 still leaves blocking
findings unresolved: comment with remaining findings, add `blocked` label, stop.

**Rationale**: LLM self-review past round 2 without new information produces
diminishing returns and, for security-sensitive code, can introduce new
vulnerabilities. Round 3 must add context, not just retry.

---

## Step 8: Update documentation

**This step is NOT optional. Always execute it regardless of remaining turns.**

Documentation must not drift from the code. After review passes, update docs
scoped to what this issue changed — not a full audit, just the diff.

### 8a: Determine what changed

```bash
git diff main...HEAD --name-only
```

Group the changed files by package/subsystem. This is your scope.

### 8b: Check for required doc updates

For each changed package, check directly (these are file-existence checks,
not a doc-scanner delegation):

1. Does `docs/subsystems/<package-name>/README.md` exist? If the package is
   new and substantial (new types, interfaces, or functions), it MUST be created.
2. Does `ARCHITECTURE.md` list this package in its component inventory?
   If not, it MUST be updated.
3. Does `docs/content-plan.md` reference any new docs? If not, update it.

### 8c: Scan existing docs for drift

Delegate to **doc-scanner** with explicit scope: the changed packages only.
Instruct it to check:
- Do any existing docs describe interfaces, types, or behaviour that changed?
- Were new public interfaces, types, or packages added that have no doc coverage?

### 8d: Update drifted docs

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

Do not create Tier 3 module docs per issue — those are a `/document` judgement call.
Do not rewrite CONCEPTS.md per issue — that is a human decision via `/document`.

### 8e: Commit doc updates

**If any docs were updated, this commit is mandatory:**
```
docs(<scope>): update documentation for issue #$ISSUE_NUMBER
```

**CHECKPOINT — verify doc commit if docs were changed:**
```bash
git diff --cached --name-only | grep -E "^(docs/|ARCHITECTURE\.md|CONCEPTS\.md)" || echo "no doc files staged"
```

---

## Step 9: Update the issue label

**If provider = github:**
```bash
gh issue edit $ISSUE_NUMBER --add-label "needs-review" --remove-label "ready-for-agent"
```

**If provider = gitea:**
Use `gitea:issue_write` MCP tool with `method: replace_labels` to swap
`ready-for-agent` for `needs-review`.

---

## Step 10: Ship

Delegate to **ship command** (via Task) with the branch name and `--no-confirm` flag.

```
/ship <branch-name> --no-confirm
```

The `--no-confirm` flag skips the PR preview and confirmation step — the factory
is autonomous, there is no human to confirm. The PR is created immediately.

The PR must target `main` — never another feature branch.

The PR description must include:
- `Closes #$ISSUE_NUMBER` in the first line
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

### Commits
<list each commit with its message — should show the pipeline stages>

### AC Verification
<table>

### Review
Cycles used: <n>/2 (or <n>/3 if round 3 trigger applied)
Round 3 trigger: <trigger name or "N/A">
Blocking findings resolved: <n>
Non-blocking findings: <n> (noted in PR — human reviewer to decide)

### Documentation
<list of docs updated, or "no changes needed">

### Files Changed
<list>
```

---

## Failure modes — what to do when stuck

- **Issue is ambiguous or contradictory**: comment on the issue explaining
  the ambiguity. Add `blocked` label. Stop.
- **Test-fix limit (3) reached**: comment with which AC failed and what was tried.
  Add `blocked` label. Stop.
- **Review cycle limit reached, no round 3 trigger**: comment with remaining
  blocking findings and which triggers were evaluated. Add `blocked` label. Stop.
- **Round 3 exhausted**: comment with remaining findings. Add `blocked` label. Stop.
- **Repo not in clean state**: comment explaining what was found. Stop.
- **In all cases**: do not guess, do not improvise beyond the AC scope, do not
  ship a PR with known failures. Never target a feature branch as PR base.

---

## Expected commit history

A correctly executed issue produces this commit sequence:

```
test(<scope>):     add failing tests for issue #N
feat(<scope>):     implement issue #N — <title>
refactor(<scope>): clean up implementation for issue #N     (if needed)
fix(<scope>):      resolve review findings cycle 1 for #N   (if needed)
fix(<scope>):      resolve review findings cycle 2 for #N   (if needed)
docs(<scope>):     update documentation for issue #N        (if docs changed)
```

If your commit history does not match this pattern, you have skipped steps.
Go back and fix the history before shipping.