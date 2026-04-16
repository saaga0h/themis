---
description: TDD-first entry point for webUI-originated work. Accepts an intent document (raw or already grounded), runs intent-bridge if needed, then follows the same pipeline as /feature from interview onward.
argument-hint: <path to intent document>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Concept Command

You are the TDD-first entry point for work that originates from a web conversation
intent document. You accept either a raw intent document or one that has already
been grounded by `intent-bridge`. If the document is already grounded, you skip
`intent-bridge`. From `interview` onward, the pipeline is identical to `/feature`.

## Step 0: Ask for model choices upfront

Before any work begins, ask:

> **Model choices** — I'll need these for two steps later:
>
> 1. **Architecture model** (for `/architect` when designing the implementation plan)
>    - Opus — new system design, cross-domain reasoning, major refactoring
>    - Sonnet — feature addition, moderate refactoring, well-understood domain
>    - Haiku — simple, mechanical changes with clear patterns
>
> 2. **Implementation model** (for `/implement` when executing the plan)
>    - Sonnet — moderate complexity, needs some judgment
>    - Haiku — straightforward, following clear patterns
>    - Opus — complex logic, needs deep reasoning

Wait for both answers before proceeding.

## Step 1: Detect whether intent document is already grounded

Read the first 30 lines of the document at `$ARGUMENTS`.

Check for the presence of `## Status: grounded` anywhere in those lines.

**If grounded**: the document has already been processed by `intent-bridge`.
Use it directly as the grounded intent document. Skip Step 2.

**If not grounded**: it is a raw intent document from a web conversation.
Proceed to Step 2.

## Step 2: Ground the intent document (if needed)

Delegate to the **intent-bridge command** (via Task) with the document path.

`intent-bridge` will scan the codebase, map the intent to existing code, surface
open questions, and write the grounded document to
`.claude/plans/intent-<source-basename>.md`.

After `intent-bridge` completes, read the grounded document from that path.
Use it as input to `interview` in Step 3.

## Step 3: Scan the codebase

Delegate to the **codebase-scanner** agent to map the project structure.
This runs on Haiku regardless of model choices above.

(If `intent-bridge` ran in Step 2, it already called `codebase-scanner`
internally. Run it again here for a fresh snapshot — `intent-bridge` may have
been called earlier in a different session.)

Pass the intent document's title or key concepts as the caller's scope so the scanner
can resolve relevant docs from `docs/content-plan.md` if it exists.

If the scanner returns a `## Relevant Docs` section with paths: read those files now,
before interview. Do not read docs beyond what the scanner resolved.

## Step 4: Run interview

Delegate to the **interview** agent with:
- The grounded intent document (path)
- The codebase scanner output

The `interview` agent will ask the human the specification/exploration question
and proceed accordingly.

**If exploration mode**: `interview` writes the breadcrumb and stops. Report:

> Exploration mode — breadcrumb saved to `.claude/explorations/<name>.md`.
> No plan produced. Run `/feature` again when the form is found.

Stop here.

**If specification mode**: `interview` produces a draft AC at `.claude/ac/<name>.md`
and presents it to the human for correction. Wait for the human to finish
correcting before proceeding.

## Step 5: Format AC — Human Touchpoint 1

Delegate to the **ac-drafter** agent with the path to `.claude/ac/<name>.md`.

`ac-drafter` structures the criteria and presents the formatted AC for confirmation.
**Human Touchpoint 1: "Correct or approve the AC."**

Wait for human confirmation before proceeding.

## Step 6: Design test structure — Human Touchpoint 2

Delegate to the **test-architect** agent with:
- Path to the formatted AC at `.claude/ac/<name>.md`
- Codebase scanner output

`test-architect` produces a test skeleton and presents it for review.
**Human Touchpoint 2: "Review the test structure before tests are written."**

Wait for human confirmation before proceeding.

## Step 7: Write tests and confirm RED — Human Touchpoint 3

Delegate to the **test-writer** agent with:
- Path to the test skeleton at `.claude/test-skeletons/<name>.md`
- Path to the formatted AC at `.claude/ac/<name>.md`

`test-writer` writes the tests, confirms RED via `test-runner`, and presents
the **Specification Review**.

**Human Touchpoint 3 — the Specification Review:**
> "These tests are confirmed failing. Does this specify the right thing?"

Wait for human confirmation. If the human requests changes, `test-writer` revises
and re-confirms RED.

## Step 8: Architecture — Human Touchpoint 4

Delegate to the **architect command** (via Task) with:
- The formatted AC path as context
- The test file paths as context
- The architecture model choice from Step 0

`architect` produces an implementation plan.
**Human Touchpoint 4: "Review the plan before implementation begins."**

Wait for human confirmation before proceeding.

## Step 9: Implement

Delegate to the **implement command** (via Task) with:
- The plan name from Step 8
- The implementation model choice from Step 0

## Step 10: Report

```
## Concept Complete

**Source**: <path to original intent document>
**Grounded intent**: .claude/plans/intent-<name>.md
**AC**: .claude/ac/<name>.md (<N> criteria)
**Tests**: <N> tests written, confirmed GREEN
**Plan**: .claude/plans/<name>.md (<N> tasks)

### Pipeline
- [x] Intent document grounded (or was already grounded — skipped intent-bridge)
- [x] Interview — specification mode confirmed
- [x] AC drafted and approved (Human Touchpoint 1)
- [x] Test structure reviewed (Human Touchpoint 2)
- [x] Specification Review passed (Human Touchpoint 3)
- [x] Plan reviewed and approved (Human Touchpoint 4)
- [x] Implementation complete

### Files Changed
<list>
```

## Important

- The grounded-detection check (`## Status: grounded`) lets `/concept` accept
  hand-written intent documents that follow the grounded format — they don't need
  to re-run `intent-bridge`. This keeps `/concept` flexible without requiring a
  separate command for pre-grounded documents.
- The distinction between `/concept` and `/feature` is purely the starting point:
  `/concept` starts from an intent document, `/feature` starts from a direct
  description. Both converge on the same pipeline from `interview` onward.
- Model choices are collected upfront because sub-command Task invocations may
  suppress interactive prompts. Pass them explicitly.
- Exploration mode is a complete and correct outcome. Do not force it into the
  specification pipeline.
