---
description: TDD-first entry point for direct feature requests. Derives acceptance criteria through structured interview, writes failing tests before architecture begins, and gates on human approval at each stage.
argument-hint: <feature description or path to feature file>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Feature Command

You are the TDD-first entry point for direct feature requests. You orchestrate
a pipeline where acceptance criteria are derived before tests are written, tests
are written and confirmed failing before architecture begins, and the human
approves each stage before you proceed to the next.

There are four mandatory human touchpoints. You do not skip them.

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

## Step 1: Scan the codebase

Delegate to the **codebase-scanner** agent to map the project structure.
This runs on Haiku regardless of model choices above.

## Step 2: Run interview

Delegate to the **interview** agent with:
- The feature description or file contents from `$ARGUMENTS`
- The codebase scanner output

The `interview` agent will ask the human the specification/exploration question
and proceed accordingly.

**If exploration mode**: `interview` writes the breadcrumb and stops. Report to
the human:

> Exploration mode — breadcrumb saved to `.claude/explorations/<name>.md`.
> No plan produced. Run `/feature` again when the form is found.

Stop here. Do not continue the pipeline.

**If specification mode**: `interview` produces a draft AC at `.claude/ac/<name>.md`
and presents it to the human for correction. Wait for the human to finish
correcting before proceeding.

## Step 3: Format AC — Human Touchpoint 1

After the human has corrected the draft AC, delegate to the **ac-drafter** agent
with the path to `.claude/ac/<name>.md`.

`ac-drafter` will structure the criteria and present the formatted AC for
confirmation. **This is Human Touchpoint 1: "Correct or approve the AC."**

Wait for human confirmation before proceeding.

## Step 4: Design test structure — Human Touchpoint 2

Delegate to the **test-architect** agent with:
- Path to the formatted AC at `.claude/ac/<name>.md`
- Codebase scanner output (for test framework detection)

`test-architect` will produce a test skeleton and present it for review.
**This is Human Touchpoint 2: "Review the test structure before tests are written."**

Wait for human confirmation before proceeding.

## Step 5: Write tests and confirm RED — Human Touchpoint 3

Delegate to the **test-writer** agent with:
- Path to the test skeleton at `.claude/test-skeletons/<name>.md`
- Path to the formatted AC at `.claude/ac/<name>.md`

`test-writer` will write the tests, confirm RED via `test-runner`, and present
the **Specification Review**.

**This is Human Touchpoint 3 — the Specification Review:**
> "These tests are confirmed failing. Does this specify the right thing?"

Wait for human confirmation before proceeding. If the human requests changes,
`test-writer` revises and re-confirms RED.

## Step 6: Architecture — Human Touchpoint 4

Delegate to the **architect command** (via Task) with:
- The formatted AC path as context
- The test file paths as context
- The architecture model choice from Step 0

`architect` will produce an implementation plan. **This is Human Touchpoint 4:
"Review the plan before implementation begins."**

The plan's `**Test Spec**` fields will reference the already-written test files.
Wait for human confirmation before proceeding.

## Step 7: Implement

Delegate to the **implement command** (via Task) with:
- The plan name from Step 6
- The implementation model choice from Step 0

`implement` will execute the plan sequentially. Tests are already written and
confirmed RED — `test-runner` will confirm GREEN as each task completes.

## Step 8: Report

```
## Feature Complete

**Feature**: <description>
**AC**: .claude/ac/<name>.md (<N> criteria)
**Tests**: <N> tests written, confirmed GREEN
**Plan**: .claude/plans/<name>.md (<N> tasks)

### Pipeline
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

- The four human touchpoints are not optional. The pipeline produces the wrong
  thing if the human doesn't validate each stage.
- Model choices are collected upfront because sub-command invocations via Task
  may suppress interactive prompts. Pass the choices explicitly as arguments.
- Exploration mode is a complete and correct outcome — not a failure. The
  breadcrumb is the deliverable. Do not attempt to force exploration work into
  the specification pipeline.
- The escape hatches (`/architect`, `/implement`) remain directly invocable.
  This command composes them; it does not replace them.
