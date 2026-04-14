# Intent: TDD Workflow — /concept and /feature Commands
## Date: 2026-04-14
## Status: draft — for /intent-bridge

## Core Intent
Themis should support test-driven development as a first-class workflow, not as a
post-implementation verification step. This means two new top-level commands —
`/concept` and `/feature` — that orchestrate existing agents plus new ones through
a pipeline where tests are written before implementation, acceptance criteria are
derived through structured interview rather than assumed, and the distinction between
specification (you know what correct means) and exploration (you're finding out) is
made explicit before any test or plan is written. The existing commands — `/architect`,
`/implement`, `/review`, `/ship` — remain unchanged and are called as sub-agents
within the new pipeline, preserving direct invocation as an escape hatch.

## Context
Themis currently treats tests as verification: `test-runner` runs after each
implementation task and confirms GREEN. This works, but it places tests downstream
of design — the interface is shaped by implementation decisions, not by what tests
need to call. TDD inverts this: tests define the observable contract, architect is
constrained by what the tests already call, implement makes failing tests pass.

The harder problem is acceptance criteria. A human saying "it should work" is not
a testable specification. An AI filling that gap with assumptions produces tests
that are formally correct but specify the wrong thing — the most expensive possible
failure, because everything passes and the wrong thing gets built. The workflow
needs a structured interview that earns each question (only ask when the answer
changes what tests get written), surfaces load-bearing implications explicitly
(showing working, not just asking), and produces a draft AC the human corrects
rather than a questionnaire the human answers exhaustively.

Two further complications crystallized in conversation:

**Greenfield** breaks the assumption that the codebase answers questions. There are
no existing patterns, no attachment points. The interview must cover foundational
decisions that brownfield projects have already made implicitly.

**Hypothetical/experimental work** breaks TDD entirely. When the form isn't found
yet — when the point of building is to discover whether the hypothesis holds — writing
acceptance criteria creates false certainty and slows iteration. Exploration mode
defers tests until the form stabilizes, leaving a breadcrumb to re-enter the TDD
pipeline when it does.

The two entry points reflect where work originates: concepts developed in webUI
conversation (Path A, `/concept`) and direct feature requests or scaffolding in
Claude Code (Path B, `/feature`). Both converge on the same internal pipeline.

## What This Is Not

- Not a replacement for `/architect`, `/implement`, `/review`, `/ship` — those
  remain as commands and as callable sub-agents
- Not an attempt to automate the decision of when something is ready for TDD —
  that remains a human judgment, surfaced by the mode detection question
- Not a solution for hypothetical/experimental work — exploration mode explicitly
  defers TDD and this is correct behavior, not a gap to fill
- Not a change to how `test-runner`, `coverage-reviewer`, or other existing agents
  work internally
- Not automatic — every command has explicit human touchpoints before proceeding;
  nothing runs fully autonomously

## Open Questions

### Needs a decision
- What is `/concept` called when the intent doc doesn't come from the webUI
  intent-doc skill but from some other source (hand-written, existing file)?
  Does `/concept` accept any grounded-intent document, or only intent-doc output?

- Should exploration mode produce anything beyond a breadcrumb marker, or is a
  lightweight note in `.claude/explorations/` sufficient? Does it need a plan
  skeleton the human can return to?

- When `/feature` scans a greenfield project and finds nothing, how much does the
  interview change? Is there a greenfield interview variant, or does the interview
  sub-agent detect this from scanner output and adapt?

- The human touchpoint after test files ("does this specify the right thing?") is
  the most important gate. Should the command explicitly name this as a
  specification review, distinct from a code review, so the human knows what
  they're being asked to evaluate?

### Needs an experiment
- How many interview questions is "as few as necessary" in practice? The principle
  is clear (earn each question, stop when answers stop changing tests) but the
  calibration needs real usage to validate. Three feels too few for novel territory;
  ten may be too many for simple features.

- Draft AC with explicit assumptions — does showing assumptions actually reduce
  correction effort compared to asking questions? Hypothesis is yes, but this is
  an empirical claim.

### Probably answered in the codebase
- Whether `intent-bridge` needs modification to serve as a sub-agent called by
  `/concept` rather than a standalone command, or whether it can be called as-is
- How existing plan format in `/architect` needs to change to reflect test
  constraints (the `**Test**` field becoming a `**Test Spec**` with a file reference)

## New Agents Required

Listed here for `/intent-bridge` to find attachment points:

- **`interview`** (Sonnet) — structured AC derivation through earned questions and
  explicit assumption surfacing; detects specification vs exploration mode
- **`ac-drafter`** (Sonnet) — produces draft acceptance criteria document with all
  assumptions stated explicitly; human corrects the draft
- **`test-architect`** (Sonnet) — decides test type (unit/integration/contract),
  boundary, and what NOT to test; produces test skeleton structure
- **`test-writer`** (Sonnet) — writes compilable, failing tests from test-architect
  output; confirms RED before handing to architect

## Intent in One Sentence

Themis should support a TDD-first workflow through two new commands — `/concept`
(from webUI intent doc) and `/feature` (direct prompt or file) — where acceptance
criteria are derived through structured interview, tests are written and confirmed
failing before architecture begins, and the distinction between specification and
exploration is made explicit so the pipeline is only applied where it adds value.
