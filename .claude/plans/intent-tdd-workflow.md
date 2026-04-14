# Intent: TDD-First Workflow — /concept and /feature Commands
## Created: 2026-04-14
## Source: intent-tdd-workflow.md
## Status: grounded — ready for /architect

## Core Intent

Themis should support test-driven development as the default entry point for
implementation work. Two new top-level commands — `/concept` (from a web
conversation intent document) and `/feature` (direct prompt or file) — orchestrate
existing agents plus four new ones through a pipeline where acceptance criteria are
derived through structured interview, tests are written and confirmed failing before
architecture begins, and the distinction between specification (you know what correct
means) and exploration (you're finding out) is made explicit before any test or plan
is written. The existing `/architect`, `/implement`, `/review`, `/ship` commands
remain callable both directly and as sub-agents within the new pipeline.

The plan format produced by `/architect` changes to reflect TDD: the `**Test**`
field (currently "how to verify this task") becomes `**Test Spec**` with a reference
to a pre-written, already-RED test file. Old plans are either already implemented
or can be rewritten; this is a deliberate shift in how development is done, not a
backwards-compatible extension.

## Codebase Attachment Points

**`commands/intent-bridge.md`** — The most structurally similar command to what
`/concept` needs to do. `/concept` calls `intent-bridge` as a sub-agent to ground
the webUI document before interview begins. `intent-bridge` already writes to
`.claude/plans/intent-*.md`; `/concept` reads that output and passes it downstream.

**`commands/architect.md`** — Called as a sub-agent after tests are confirmed RED.
The plan template's `**Test**` field changes to `**Test Spec**: <path to test file>`
so architect is constrained by what tests already call rather than specifying tests
as future verification work.

**`commands/implement.md`** — Called as a sub-agent after architect produces the
plan. The RED-confirmation gate sits between `test-writer` and `/implement`; once
the human confirms the tests specify the right thing, `/implement` runs as normal.
No changes needed to `implement.md` itself — the pre-written tests make its
`test-runner` step confirm GREEN instead of writing new tests.

**`agents/test-runner.md`** — Already exists; used in a new role: confirming RED
before implementation begins, not only GREEN after. No changes to the agent itself —
only the calling context changes.

**`agents/codebase-scanner.md`** — Called at the start of `/feature` (and
transitively through `intent-bridge` in `/concept`) to establish project context
before interview. Already pattern-matched for this use.

**Plan file convention** — Plans live in `.claude/plans/<name>.md` within the
project using Themis. The grounded intent documents from `intent-bridge` already
use `intent-<name>.md`. The new pipeline produces a TDD plan at
`.claude/plans/tdd-<name>.md` (or reuses the existing plan naming convention with
architect — decision needed, see Open Questions).

## What Already Exists

- `test-runner` agent — runs tests, reports RED/GREEN, does not fix
- `intent-bridge` command — translates web conversation intent docs into grounded
  architect-ready documents; directly callable as sub-agent from `/concept`
- `codebase-scanner` agent — maps project before reasoning begins
- `/architect` command — produces implementation plan; already receives intent
  documents as input (that's the documented use case of `intent-bridge`)
- `/implement` command — executes plans sequentially, runs `test-runner` after
  each task; already handles plan resumption via checkboxes
- Plan format in `architect.md` — has a `**Test**` field that becomes `**Test Spec**`

## What Doesn't Exist Yet

**Four new agents** (all Sonnet — require judgment):

- **`agents/interview.md`** — Structured AC derivation. Scans codebase and the
  grounded intent, surfaces a draft AC with explicit assumptions, asks only questions
  whose answers change what tests get written. Detects specification vs. exploration
  mode and routes accordingly.

- **`agents/ac-drafter.md`** — Produces a draft acceptance criteria document with
  all assumptions stated explicitly. Human corrects the draft rather than answering
  a questionnaire. Output is the input to `test-architect`.

- **`agents/test-architect.md`** — Decides test type (unit/integration/contract),
  boundary, and what NOT to test. Produces a test skeleton structure (file paths,
  function signatures, assertion intent) without writing implementation. Output is
  the input to `test-writer`.

- **`agents/test-writer.md`** — Writes compilable, failing tests from
  `test-architect` output. Calls `test-runner` to confirm RED before handing off.
  Reports any tests that can't be made to fail (spec ambiguity signal).

**Two new commands:**

- **`commands/concept.md`** — Entry point for webUI-originated work. Calls
  `intent-bridge` → `interview` → `ac-drafter` → human correction → `test-architect`
  → `test-writer` → RED confirmation → `/architect` → human review → `/implement`.

- **`commands/feature.md`** — Entry point for direct Claude Code prompts or feature
  files. Skips `intent-bridge`, starts with `codebase-scanner` + `interview`.
  Otherwise same pipeline as `/concept` from `interview` onward.

**One new storage convention:**

- **`.claude/explorations/<name>.md`** — Breadcrumb written when `interview` detects
  exploration mode. Contains the hypothesis, what was tried, and a pointer to
  re-enter the TDD pipeline when the form stabilizes. No tests written, no plan
  produced.

**Plan template change:**

- In `commands/architect.md`, the `**Test**: <how to verify this task>` field in
  the plan template changes to `**Test Spec**: <path to pre-written test file>`.
  This is a deliberate breaking change; old plans are already implemented or
  rewriteable.

## Architectural Tensions

**Commands calling commands.** `/concept` and `/feature` would be the first commands
in Themis to call other commands (`intent-bridge`, `/architect`, `/implement`) as
sub-agents via Task. This is architecturally valid in Claude Code but untested in
this framework. The question is whether commands-as-sub-agents behave identically to
direct invocation (model selection prompts, file writes, human touchpoints). If
sub-agent invocation suppresses interactive prompts, the pipeline breaks at model
selection and human confirmation steps.

**Human touchpoints inside an orchestration.** The existing commands all have
explicit human gates before writing files. The new pipeline has multiple such
gates (after AC draft, after test files, after architect plan). Orchestrating these
inside a single `/concept` or `/feature` call requires the command to pause, surface
the gate, and wait — not hand off silently. The framework has no established pattern
for multi-gate orchestration within a single command invocation.

**`intent-bridge` as sub-agent vs. standalone.** Currently `intent-bridge` is a
standalone command that writes its output to disk and presents a summary. As a
sub-agent called from `/concept`, it needs to return the path of the grounded intent
file so the pipeline can continue. This may require a small behavioral adjustment
or the caller can infer the path from the known naming convention.

**Greenfield detection.** `codebase-scanner` returns facts; it doesn't signal
"this is greenfield." `interview` needs to infer this from scanner output (no
existing patterns, no test framework, no attachment points) and adapt its question
set. This coupling between scanner output interpretation and interview behavior is
implicit — it works but should be stated explicitly in `interview`'s prompt.

## Open Questions

### Need a decision before implementation

- **Does `/concept` accept any grounded-intent document or only webUI intent-doc
  skill output?** — Affects whether `/concept` always calls `intent-bridge` or
  accepts a pre-grounded document and skips to interview. If it accepts any
  grounded intent document, the distinction between `/concept` and `/feature`
  narrows to "has an intent document" vs. "doesn't."

- **Does the "specification review" gate have a name?** — The human touchpoint
  after test files ("does this specify the right thing?") is qualitatively different
  from a code review. Naming it explicitly (e.g., "Specification Review") changes
  what the human knows they're being asked to evaluate. This is a UX decision with
  no code implication but affects the prompt language in `test-writer` and the
  command flow.

- **What does exploration mode produce beyond a breadcrumb?** — A plain `.md` in
  `.claude/explorations/` is sufficient for the intent. Should it also include a
  stub plan the human can return to, or is the breadcrumb the full output? Affects
  what `interview` writes when it detects exploration mode.

### Codebase already implies an answer

- **Can `intent-bridge` be called as a sub-agent without modification?** → The
  command writes to a predictable path (`intent-<name>.md`) and presents a summary.
  The calling command can infer the output path from the source document name. No
  modification required — the caller reads the written file after `intent-bridge`
  completes.

- **Does `/implement` need changes?** → No. Pre-written RED tests mean `test-runner`
  inside `/implement` finds tests to run. The `**Test Spec**` field in the plan gives
  it the file reference. `/implement` behavior is unchanged.

- **Does `test-runner` need changes?** → No. RED is just a failing test. The agent
  already reports pass/fail without knowing the intent. The calling context (is this
  a pre-implementation RED check or a post-implementation GREEN check) is irrelevant
  to the agent.

- **Where do plans land?** → `.claude/plans/` in the target project, same as all
  other plans. The TDD pipeline produces one grounded intent file
  (`intent-<name>.md`) and one implementation plan (`<name>.md`). Same convention,
  no new location needed.

### Need an experiment to answer

- **How many interview questions is "as few as necessary" in practice?** — The
  principle (earn each question, stop when answers stop changing tests) is clear.
  Calibration requires real usage. Starting point: draft AC with stated assumptions
  first, then ask only questions that would change an assumption. Track how often
  the human corrects vs. accepts the draft.

- **Does showing assumptions reduce correction effort vs. asking questions?** —
  The `ac-drafter` approach (draft + correct) vs. a questionnaire (answer + derive)
  is an empirical claim. Implement draft-first; measure correction rate in practice.

## Risks and Unknowns

- **Commands-as-sub-agents suppress interactive prompts.** If Task invocation of
  `/architect` or `/implement` skips the model-selection question, the pipeline
  silently uses a default model the user didn't choose. Mitigation: test this
  explicitly before building the full pipeline; `/concept` may need to ask for model
  choices upfront and pass them as arguments rather than relying on sub-command
  prompts.

- **RED confirmation is fragile for some test types.** Integration tests against
  external dependencies (databases, APIs) may not be runnable in a RED state if the
  infrastructure isn't set up. `test-writer` needs to handle this gracefully —
  either by writing unit-level tests that can always be RED, or by flagging
  integration tests that require a live environment for RED confirmation.

- **`interview` scope is very large.** Specification vs. exploration detection,
  earned questions, assumption surfacing, greenfield adaptation, and AC draft
  production could all be one agent or could be split. If `interview` is too large,
  it will produce mediocre output across all dimensions. The split into `interview`
  + `ac-drafter` in the intent document addresses this — preserve it.

- **Exploration mode is a permanent exit from the pipeline.** Once `interview`
  routes to exploration, there's no automated re-entry. The breadcrumb in
  `.claude/explorations/` is a human-read document, not a machine-readable trigger.
  This is correct behavior per the intent, but it means re-entry requires the human
  to explicitly run `/feature` again. Make this clear in the exploration breadcrumb.

## Suggested Scope for Architect

**Minimal viable implementation:**
1. Four new agents (`interview`, `ac-drafter`, `test-architect`, `test-writer`)
2. One new command (`/feature`) — the simpler entry point, no `intent-bridge` call
3. Plan template change in `architect.md` (`**Test**` → `**Test Spec**`)
4. Exploration mode breadcrumb convention (`.claude/explorations/`)

This is enough to validate the TDD pipeline end-to-end with a direct feature request.
`/concept` can follow once `/feature` is proven.

**Full implementation adds:**
5. `/concept` command — calls `intent-bridge`, then same pipeline as `/feature`
6. README updates — new commands, updated workflow diagram, TDD philosophy section
7. `install.sh` updates — new agents and commands included

**Defer:**
- Plan composition (parent plans with child plans) — listed in README as a future
  item; TDD pipeline doesn't require it
- Hooks for auto-running `test-runner` — listed in README as a future item;
  orthogonal to this intent
- Skills versions of the new commands — README lists skills as a future direction;
  out of scope here

## Constraints from Intent

- `/architect`, `/implement`, `/review`, `/ship` must remain callable directly
  as escape hatches — they are not replaced, only composed
- Exploration mode must explicitly defer TDD rather than attempting to fit
  experimental work into the pipeline — this is correct behavior, not a gap
- Every gate (after AC draft, after test files, after architect plan) must be
  an explicit human touchpoint — nothing runs fully autonomously through the
  full pipeline
- `test-runner`, `coverage-reviewer`, and other existing agents must not change
  internally — only their calling context changes
