# Plan: TDD-First Workflow — Agents, Commands, and Plan Format
## Created: 2026-04-14
## Complexity: opus
## Recommended implementation model: sonnet

## Context

Themis currently treats tests as post-implementation verification: `test-runner`
confirms GREEN after `/implement` completes. This plan inverts that: tests define
the observable contract before architecture begins, and architect is constrained by
what the tests already call.

The change is made through four new agents (`interview`, `ac-drafter`,
`test-architect`, `test-writer`), two new commands (`/feature`, `/concept`), a
breaking change to the architect plan template (`**Test**` → `**Test Spec**`), and
a new storage convention for exploration-mode breadcrumbs.

The existing commands (`/architect`, `/implement`, `/review`, `/ship`) are unchanged
and remain directly invocable as escape hatches.

Key decisions baked into this plan:
- `/concept` detects whether its input is already a grounded intent document
  (presence of `## Status: grounded` marker) and skips `intent-bridge` if so
- The human gate after test files is named "Specification Review" explicitly
- Exploration mode writes a breadcrumb only — no stub plan, because a plan
  contradicts the point of exploration
- `**Test**` → `**Test Spec**` in architect's plan template is a clean break;
  old plans are already implemented or rewriteable

## Prerequisites

- [x] All existing agents and commands are passing (no broken files)
- [x] `agents/` and `commands/` directories exist at project root

## Tasks

### 1. Create `agents/interview.md`

- **File**: `agents/interview.md`
- **Action**: Write a new Sonnet agent that performs structured AC derivation.
  The agent receives a grounded intent document path and codebase scanner output.
  It must:
  - Detect specification mode vs. exploration mode by asking one explicit question:
    "Do you know what correct looks like, or are you finding out?" Surface the
    distinction clearly — if exploration, write a breadcrumb to
    `.claude/explorations/<name>.md` (hypothesis + what is known + "re-enter with
    `/feature` when the form is found") and stop. Do not produce AC for exploration.
  - For specification mode: read the grounded intent, scan the codebase for
    greenfield signals (no existing test framework, no patterns, no attachment
    points), and produce a **draft AC document** with all assumptions stated
    explicitly — not a questionnaire. The draft is the output; the human corrects it.
  - After presenting the draft, ask only questions whose answers would change a
    stated assumption. Stop asking when no remaining assumption is in question.
  - Output: draft AC written to `.claude/ac/<name>.md`, or breadcrumb written to
    `.claude/explorations/<name>.md` for exploration mode.
- **Pattern**: Follow the agent frontmatter format from `agents/architecture-reviewer.md`
  (YAML frontmatter with `name`, `description`, `tools`, `model: sonnet`)
- **Test Spec**: Verify the file exists at `agents/interview.md`, has valid YAML
  frontmatter with `model: sonnet`, contains sections for specification mode,
  exploration mode, greenfield detection, and draft AC output. Read the file after
  writing.
- **Notes**: The exploration mode exit is permanent within a single invocation —
  the agent writes the breadcrumb and stops. Re-entry requires the human to run
  `/feature` again. Make this explicit in the breadcrumb text. Greenfield detection
  is inferred from scanner output: no test framework found, no existing patterns,
  no attachment points listed in the intent doc.

### 2. Create `agents/ac-drafter.md`

- **File**: `agents/ac-drafter.md`
- **Action**: Write a new Sonnet agent that takes a draft AC (written by `interview`
  or corrected by the human) and produces a formal acceptance criteria document
  suitable for `test-architect` to consume. It must:
  - Read the draft AC from `.claude/ac/<name>.md`
  - Structure it as numbered acceptance criteria, each with: criterion text, what
    constitutes passing, what constitutes failing, and any stated assumption that
    underpins it
  - Output: formatted AC written back to `.claude/ac/<name>.md` (overwrites draft)
  - Present the formatted AC to the human for final confirmation before handing off
- **Pattern**: `agents/architecture-reviewer.md` for frontmatter; content structure
  follows the input→process→output pattern used by `agents/test-runner.md`
- **Test Spec**: Verify the file exists at `agents/ac-drafter.md`, has valid YAML
  frontmatter with `model: sonnet`, describes the input format, output format, and
  human confirmation gate.
- **Notes**: This agent is intentionally narrow — it formats and structures, it does
  not generate new criteria. Generation happens in `interview`. Separation matters
  because `interview` is interactive (may loop on questions) while `ac-drafter` is
  a one-shot transformation.

### 3. Create `agents/test-architect.md`

- **File**: `agents/test-architect.md`
- **Action**: Write a new Sonnet agent that reads a formatted AC document and
  produces a test skeleton. It must:
  - Read the AC from `.claude/ac/<name>.md`
  - For each criterion: decide test type (unit/integration/contract), identify the
    boundary under test, and specify what NOT to test (mock boundaries, out-of-scope
    behaviors)
  - Produce a test skeleton: file paths, function/method signatures, assertion intent
    in comments — no implementation, no passing assertions
  - Output: test skeleton written to `.claude/test-skeletons/<name>.md`
  - Present skeleton to human for review before handing to `test-writer`
- **Pattern**: `agents/architecture-reviewer.md` for frontmatter
- **Test Spec**: Verify the file exists at `agents/test-architect.md`, has valid
  YAML frontmatter with `model: sonnet`, describes test type decision criteria,
  skeleton output format, and the human review gate.
- **Notes**: "What NOT to test" is as important as what to test — the agent must
  state mock boundaries and out-of-scope behaviors explicitly so `test-writer`
  doesn't over-specify. Integration tests that require live infrastructure should
  be flagged with a note that RED confirmation may require environment setup.

### 4. Create `agents/test-writer.md`

- **File**: `agents/test-writer.md`
- **Action**: Write a new Sonnet agent that writes compilable, failing tests from
  a test skeleton. It must:
  - Read the test skeleton from `.claude/test-skeletons/<name>.md`
  - Write actual test files at the paths specified in the skeleton
  - Delegate to `test-runner` to confirm RED — all written tests must fail
  - If any test passes before implementation (i.e., cannot be made to fail), flag
    it as a specification ambiguity signal and report it to the human before
    continuing
  - Present the written test files and RED confirmation to the human as a
    **Specification Review**: "These tests specify the contract your implementation
    must satisfy. Does this specify the right thing?"
  - Wait for human confirmation before signaling ready for `/architect`
  - Output: test files on disk, RED confirmed, human has approved the spec
- **Pattern**: `agents/architecture-reviewer.md` for frontmatter; delegation to
  `test-runner` follows the pattern used in `commands/implement.md`
- **Test Spec**: Verify the file exists at `agents/test-writer.md`, has valid YAML
  frontmatter with `model: sonnet`, contains the RED confirmation step, the
  "Specification Review" named gate, and the spec ambiguity signal behavior.
- **Notes**: The "Specification Review" naming is intentional — the human is being
  asked whether the tests specify the right thing, not whether the code is correct.
  These are different questions and the human needs to know which one they're
  answering. Integration tests flagged by `test-architect` as requiring live
  infrastructure should be noted explicitly so the human knows RED confirmation
  may be partial.

### 5. Update `commands/architect.md` — plan template field rename

- **File**: `commands/architect.md`
- **Action**: In the plan template (the markdown code block starting at "```markdown"),
  change the task field from:
  ```
  - **Test**: <how to verify this task>
  ```
  to:
  ```
  - **Test Spec**: <path to pre-written test file, or inline check if no test exists>
  ```
  Also update the "Plan quality checklist" — the item about tasks being verifiable
  should reference `**Test Spec**` not `**Test**`.
- **Pattern**: Edit in place; the rest of the file is unchanged.
- **Test Spec**: Read `commands/architect.md` after editing and verify the string
  `**Test**:` no longer appears in the plan template block, and `**Test Spec**:`
  does. Verify no other content changed.
- **Notes**: This is the clean break. The inline check fallback (`or inline check
  if no test exists`) allows `/architect` to remain usable as a standalone command
  for work that doesn't go through the TDD pipeline. Without this escape, `/architect`
  called directly would always require a pre-existing test file, which breaks the
  escape-hatch guarantee.

### 6. Create `commands/feature.md`

- **File**: `commands/feature.md`
- **Action**: Write the `/feature` command. This is the simpler TDD entry point —
  no `intent-bridge` call. It orchestrates: `codebase-scanner` → `interview` →
  human correction of AC draft → `ac-drafter` → `test-architect` → `test-writer`
  → Specification Review gate → `/architect` → human plan review → `/implement`.

  The command must:
  - Accept `$ARGUMENTS` as either a feature description (string) or a path to a
    feature file
  - Delegate `codebase-scanner` to map the project (Haiku — fast)
  - Delegate `interview` with the feature description and scanner output; if
    `interview` routes to exploration mode, the command stops (breadcrumb is written
    by `interview`, command reports "Exploration mode — breadcrumb saved, no plan
    produced")
  - After `interview` produces draft AC: present it to the human for correction,
    then delegate `ac-drafter` to format the corrected draft
  - Delegate `test-architect` with the formatted AC; present skeleton to human
  - Delegate `test-writer` with the skeleton; present Specification Review gate
  - After human approves spec: ask for model choice for `/architect` ("Opus /
    Sonnet / Haiku — this is passed to architect"), then delegate `/architect` with
    the grounded AC and test spec as context
  - After architect produces plan: present to human for review; on approval,
    ask for model choice for `/implement`, then delegate `/implement`
  - Report final status

  Human touchpoints (must pause and wait):
  1. After `interview` draft AC — "correct or approve"
  2. After `test-architect` skeleton — "review before writing tests"
  3. Specification Review after `test-writer` — "does this specify the right thing?"
  4. After `architect` plan — "review before implementing"
  All four are named explicitly in the command text.

- **Pattern**: `commands/intent-bridge.md` for overall command structure (YAML
  frontmatter, step-numbered prose, explicit human gates). Delegation pattern
  from `commands/review.md` (Task-based agent delegation).
- **Test Spec**: Verify the file exists at `commands/feature.md`, has valid YAML
  frontmatter with `description` and `argument-hint`, contains all four named human
  touchpoints, and the exploration mode exit is explicit.
- **Notes**: The model choice questions for `/architect` and `/implement` are asked
  by `/feature` upfront and passed as arguments — not left to the sub-commands to
  ask interactively, since sub-agent Task invocation may suppress interactive prompts.
  This resolves the architectural tension flagged in the intent doc.

### 7. Create `commands/concept.md`

- **File**: `commands/concept.md`
- **Action**: Write the `/concept` command. Entry point for webUI-originated work.
  Identical to `/feature` from `interview` onward, but prepends an `intent-bridge`
  step (or skips it).

  The command must:
  - Accept `$ARGUMENTS` as a path to an intent document
  - Read the first 20 lines of the document to detect whether it is already grounded:
    if it contains `## Status: grounded`, skip `intent-bridge` and proceed to
    `interview` with the document as input
  - If not grounded: delegate `intent-bridge` with the document path; infer the
    output path as `.claude/plans/intent-<source-basename>.md`; read the grounded
    document from that path
  - From `interview` onward: identical pipeline to `/feature` (same four human
    touchpoints, same exploration mode exit, same model-choice questions upfront)

- **Pattern**: `commands/feature.md` (just written) for pipeline structure;
  `commands/intent-bridge.md` for the grounded-detection pattern
- **Test Spec**: Verify the file exists at `commands/concept.md`, has valid YAML
  frontmatter, contains the grounded-detection logic (`## Status: grounded` check),
  and the skip/call branch for `intent-bridge`.
- **Notes**: The grounded detection keeps `/concept` flexible — a hand-written intent
  document that follows the grounded format doesn't need to re-run `intent-bridge`.
  The distinction between `/concept` and `/feature` is now purely "does work start
  from an intent document" vs. "does work start from a direct prompt." Both converge
  on the same pipeline.

### 8. Update `README.md` — new commands and TDD workflow

- **File**: `README.md`
- **Action**: Delegate to `doc-updater` agent to update the README. Changes needed:
  - Add `/concept` and `/feature` to the Commands table
  - Add the four new agents to the Agents table
  - Add a TDD workflow section showing the full pipeline (parallel to the existing
    workflow examples for `/architect`, `/implement`, `/review`, `/ship`)
  - Update the "Evolving this" checklist: mark TDD workflow items as done, add
    `/concept` and `/feature` as completed
  - Update `install.sh` echo line: add `/concept` and `/feature` to the "Commands
    available" output
- **Pattern**: Follow existing README style exactly — `doc-updater` derives this
  from the file itself
- **Test Spec**: Read `README.md` after update and verify the Commands table
  contains `/concept` and `/feature`, the Agents table contains all four new agents,
  and a TDD workflow section exists.
- **Notes**: Delegate to `doc-updater` rather than editing directly — it follows
  existing style without being told to. The `install.sh` echo change is small enough
  to do inline during this task rather than a separate task.

### 9. Update `install.sh` — new commands listed in output

- **File**: `install.sh`
- **Action**: Update the final echo block to include the new commands:
  Change:
  ```
  echo "Commands available: /architect, /implement, /review, /ship"
  ```
  to:
  ```
  echo "Commands available: /architect, /implement, /review, /ship, /concept, /feature"
  ```
  Also update the Quick start section to mention `/concept` and `/feature` with
  brief one-line descriptions.
- **Pattern**: Edit in place; `install.sh` installs all `agents/*.md` and
  `commands/*.md` via glob — new files are picked up automatically, no loop changes
  needed.
- **Test Spec**: Read `install.sh` after editing and verify `/concept` and `/feature`
  appear in the echo output. Verify the glob loops are unchanged.
- **Notes**: The install loops already handle new files via `agents/*.md` and
  `commands/*.md` globs. No structural change to `install.sh` is needed — only the
  documentation echo lines.

## Completion Criteria

- [x] `agents/interview.md` exists with `model: sonnet`, specification/exploration
      branching, greenfield detection, and draft AC output
- [x] `agents/ac-drafter.md` exists with `model: sonnet`, structured AC formatting,
      and human confirmation gate
- [x] `agents/test-architect.md` exists with `model: sonnet`, test type decisions,
      skeleton output, and "what NOT to test" section
- [x] `agents/test-writer.md` exists with `model: sonnet`, RED confirmation via
      `test-runner`, named "Specification Review" gate, and spec ambiguity signal
- [x] `commands/architect.md` has `**Test Spec**` in plan template, `**Test**` is gone
- [x] `commands/feature.md` exists with all four named human touchpoints and
      exploration mode exit
- [x] `commands/concept.md` exists with grounded-detection logic and same pipeline
      as `/feature` from `interview` onward
- [x] `README.md` reflects all new agents and commands
- [x] `install.sh` echo output lists `/concept` and `/feature`
- [x] A fresh read of each new agent/command file is coherent and self-contained
      (a session with no prior context could execute it)

## New Constraints or Gotchas

- New storage paths used by the TDD pipeline (in target projects, not in Themis
  itself): `.claude/ac/<name>.md`, `.claude/test-skeletons/<name>.md`,
  `.claude/explorations/<name>.md`. These are created by the agents at runtime —
  no setup needed in Themis.
- `/concept` and `/feature` ask for model choices upfront and pass them as arguments
  to `/architect` and `/implement`. This is intentional: sub-agent Task invocation
  may suppress interactive prompts in the sub-commands. If a target project's
  `/architect` or `/implement` still prompts interactively when called as a
  sub-agent, the upfront questions become redundant but harmless.
- The `**Test Spec**` field in architect plans has an escape: "or inline check if
  no test exists." This preserves `/architect` as a standalone command for work
  that bypasses the TDD pipeline.
