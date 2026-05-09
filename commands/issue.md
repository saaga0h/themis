---
description: Implement a GitHub issue end-to-end. Fetches the issue, scans the codebase, architects a plan, implements it, reviews, and ships a PR. Designed for well-formed issues with explicit Acceptance Criteria — skips interview and AC derivation.
argument-hint: <issue-number>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, mcp
---

# Issue Command

You implement a single GitHub issue from start to PR. Issues worked on via this
command are expected to be well-formed — they contain a description, explicit
implementation guidance, and verifiable Acceptance Criteria. The interview and
AC derivation phases from `/concept` and `/feature` are skipped because the issue
already encodes that work.

The pipeline: fetch → scan → architect → implement → review → ship.

You run inside a sandboxed container with `--dangerously-skip-permissions`. Do not
ask for approval on individual file operations — you have blanket authorization
within the sandbox. The human reviews the PR, not individual edits.

## Step 0: Validate input and ask model choices upfront

`$ARGUMENTS` must be a GitHub issue number. If missing or not a number, stop:

> Usage: `/issue <number>` — e.g. `/issue 3`

Then ask:

> **Model choices** — needed for two steps:
>
> 1. **Architecture model** (for the implementation plan)
>    - Opus — new system design, cross-domain reasoning, unfamiliar territory
>    - Sonnet — feature addition, well-understood domain (default for Hestia)
>    - Haiku — simple mechanical change with clear patterns
>
> 2. **Implementation model** (for executing the plan)
>    - Sonnet — moderate complexity, needs judgment (default for Hestia)
>    - Haiku — straightforward, following clear patterns
>    - Opus — complex logic, deep reasoning required

Wait for both answers before proceeding.

## Step 1: Fetch the issue

Fetch full issue content via GitHub MCP or CLI:

```bash
gh issue view $ARGUMENTS --json number,title,body,labels,state,url
```

If the issue is closed, stop:

> Issue #$ARGUMENTS is already closed. Nothing to do.

If the issue does not have the `ready-for-agent` label, warn but do not stop:

> Warning: issue #$ARGUMENTS is not labeled `ready-for-agent`. Proceeding anyway.

Extract from the issue body:
- **Title**: the issue title
- **Description / Context**: the "Context" or "What to build" section
- **Acceptance Criteria**: the checkbox list under "Acceptance Criteria"
- **Notes**: anything under "Notes" or "Important"

Save the extracted content to `.claude/issues/issue-$ARGUMENTS.md`:

```markdown
# Issue #<number>: <title>

**URL**: <url>

## Description
<context and what to build sections>

## Acceptance Criteria
<checkbox list, verbatim from issue>

## Notes
<notes section, verbatim from issue>
```

## Step 2: Scan the codebase

Delegate to the **codebase-scanner** agent. Pass the issue title and key terms
as the caller's scope so it can resolve relevant docs from `docs/content-plan.md`
if it exists.

Read `UBIQUITOUS_LANGUAGE.md` if it exists at the repo root — this is the
canonical terminology document and takes precedence over any conflicting naming
in the codebase. All code, comments, and commit messages must use terms from it.

Read `CODING_STANDARDS.md` or `.sandcastle/CODING_STANDARDS.md` if either
exists — this defines language-specific standards the implementation must follow.

If the scanner returns a `## Relevant Docs` section with paths: read those files.
Do not speculatively read docs beyond what the scanner resolved.

## Step 3: Create a branch

Create a dedicated branch for this issue:

```bash
git checkout -b issue/$ARGUMENTS-<slug>
```

Where `<slug>` is a short kebab-case summary of the issue title (max 5 words).

Example: `issue/3-spec-schema-validation`

## Step 4: Architect the implementation plan

Delegate to the **architect command** (via Task) with:
- The issue file path `.claude/issues/issue-$ARGUMENTS.md` as context
- The codebase scanner output as context
- The architecture model choice from Step 0
- Instruction to treat the issue's Acceptance Criteria as the definition of done

The plan must include a `## Acceptance Criteria` section that maps each issue AC
to the planned test or verification approach.

**Human Touchpoint**: After `architect` produces the plan, present it:

> Plan ready for issue #$ARGUMENTS. Review `.claude/plans/<name>.md`.
> Approve to proceed with implementation, or tell me what to change.

Wait for human confirmation before proceeding.

## Step 5: Implement

Delegate to the **implement command** (via Task) with:
- The plan name from Step 4
- The implementation model choice from Step 0

`implement` will execute tasks sequentially, run tests after each, and verify
against the AC file.

## Step 6: Review

After implementation completes, delegate to the **review command** (via Task)
with `--last-plan` to run all reviewers against the changes.

If the review surfaces blocking issues (security vulnerabilities, broken
abstractions, missing test coverage on ACs), fix them before proceeding.
Non-blocking findings are noted in the review report and included in the PR.

## Step 7: Update the issue label

Mark the issue as implemented by adding the `needs-review` label:

```bash
gh issue edit $ARGUMENTS --add-label "needs-review" --remove-label "ready-for-agent"
```

## Step 8: Ship

Delegate to the **ship command** (via Task) with the plan name.

`ship` will compose the PR description from the plan, review report, and git diff,
show it to the human for confirmation, then create the PR.

The PR description must include:
- Reference to the issue: `Closes #$ARGUMENTS`
- Summary of what was implemented
- AC verification table from the implement step
- Any review findings and how they were addressed

## Step 9: Final report

```
## Issue #<number> Complete

**Issue**: <title>
**URL**: <issue url>
**Branch**: issue/<number>-<slug>
**PR**: <pr url>

### Pipeline
- [x] Issue fetched and parsed
- [x] Codebase scanned
- [x] Branch created: issue/<number>-<slug>
- [x] Plan architected and approved
- [x] Implementation complete
- [x] Review passed
- [x] Labels updated (needs-review)
- [x] PR created: <url>

### Acceptance Criteria
<AC verification table from implement step>
```

## Important

- Read `UBIQUITOUS_LANGUAGE.md` before writing any code. Terminology violations
  are review failures — wrong names will be caught and must be fixed.
- Read `CODING_STANDARDS.md` before writing any code. Language standards are
  enforced by the reviewer.
- The issue's Acceptance Criteria are the definition of done. Every AC must have
  a corresponding test that passes before the PR is created.
- Do not implement anything beyond what the issue describes. Scope creep is a
  review failure.
- Commit early and often on the feature branch. Use conventional commit format:
  `type(scope): description (#issue-number)`
- If the implementation reveals that the issue is unclear or contradictory, stop
  and comment on the GitHub issue explaining the ambiguity rather than guessing.
- This command is designed to run unattended within a sandbox. The only human
  touchpoint is plan approval in Step 4. Everything else runs to completion.
