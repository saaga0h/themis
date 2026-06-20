# Factory Pipeline Redesign (v2) — Agnostic Factory, Project Declares the Flow

Status: **proposal** (not yet implemented)
Supersedes: v1 of this document
Author: drafted with Claude Code, 2026-06-20
Scope: the **autonomous factory** pipeline (`themis issue` / `themis run`). The
interactive `/review` command for human use is explicitly **out of scope and
unchanged**.

> **v2 in one line:** the factory becomes *stack-agnostic* — it carries only
> universal *law*; everything specific to a project (commands, standards, flow)
> is *declared by the project* via config + docs. Review stops being an auto-fix
> loop and becomes a single-pass, ground-truth gate plus human-facing
> annotation. Hazardous PRs flow to an interactive remediation session, never an
> auto-fix loop.

---

## 1. Why

A trivial validation issue (#75: reject non-positive `--max-turns`) consumed
~22% of a Max 5× session window. Review was ~75% of that, and the
review→fix→review loop never converged.

**The diagnostic tell:** Fix only touches *blocking* findings, so the 5
non-blocking findings from Review #1 were on code nobody edited — yet Review #2
reported **4**. The finding count changed on unchanged code. Not "round 1 missed
some," not "Fix regressed" — **resampling**.

**Root cause:** open-ended LLM review is a **sampler, not a checker**. A test
suite is a checker (fixed assertions, ground truth, same verdict every run), so
"fix until green" converges. "Review this for problems" draws a fresh *sample*
from a huge space of plausible criticisms each run. Wired into an auto-fix loop
it chases a moving target — code ends up *different* each round, not reliably
*better*.

Two further problems surfaced while diagnosing:
- The factory is **implicitly Go-only** (`go test ./...`, a hardcoded Go
  standards digest, Go-flavored templates). The real target set is Go, Go+UI,
  node.js, Julia, … which share "almost nothing."
- **Docs are an input, not just an output.** The agents read project standards
  to do the work; stale standards → wrong code. v1 under-valued this.

---

## 2. The two-layer architecture (the spine of v2)

Be precise about what is universal vs per-project:

### 2a. Factory law — universal, lives in the binary, never per-project
These hold regardless of stack because they are about *convergence and
judgment*, not language:

- **L1** A gate you iterate against must measure a **fixed, satisfiable target.**
  Tests qualify; security-with-a-locus and AC-coverage qualify; "is this code
  good?" does not.
- **L2** An autonomous **code-modifying** stage must be reliably right; an
  **annotating** stage only needs to be useful to a human. Review can't be
  reliably right (L1) → review is **annotation + objective gate**, never a loop.
- **L3** **Quality by construction over inspection.** Mechanical rules are
  enforced by linters in the gate and by standards the Implement step follows —
  not policed after the fact.
- **L4** Fix systemic issues at the altitude where the system is visible — a
  human triaging recurring notes across many PRs — not in-scope whack-a-mole.
- **L5** The factory **never auto-merges and never auto-fixes**; it **always
  emits a PR** (ready or draft) annotated with a verdict and what it noticed but
  deliberately did not change. The **human PR review is the comprehensive gate**;
  hazardous output goes to an **interactive remediation session** (§8), never an
  auto-fix loop.
- **L6** The pipeline **skeleton** is universal:
  `Test → Implement → Green Gate → Review → [Docs] → Ship`.

### 2b. Project declaration — per-repo, lives in config + docs
Everything that differs between Go, Go+UI, node, Julia. The factory **reads**
this; it hardcodes none of it:

- the **verify toolchain** (build / test / lint / format / typecheck commands);
- the **authoritative docs** the agents read (standards, conventions,
  architecture, domain glossary);
- **stack conventions** (commit prefixes, test-file naming);
- **optional-stage toggles** (e.g. is Docs off / always / surface-triggered).

> Litmus test for any line of factory code or any template: *"Would this be the
> same for a Julia repo and a React repo?"* If no, it belongs in 2b, not 2a.

---

## 3. The project descriptor (the seam)

A per-repo descriptor is how "the project tells the factory how its stuff flows."
Extend the existing `internal/profile` or add `.themis/workflow.yaml`. Every
`verify` entry is simply *a command that must exit 0*.

```yaml
# .themis/workflow.yaml  (illustrative)
stack: go                       # informational label only

verify:                         # Green Gate runs these in order; any non-zero = not green
  - go build ./...
  - test -z "$(gofmt -l .)"     # formatting check, expressed as exit-0
  - go vet ./...
  - go test ./...
  # - golangci-lint run         # add when available in the sandbox

docs:                           # authoritative INPUTS the agents read (kept lean)
  standards:    CODING_STANDARDS.md
  glossary:     UBIQUITOUS_LANGUAGE.md
  architecture: docs/architecture.md

conventions:
  commit_prefixes: { test: "test(", implement: "feat(", fix: "fix(", docs: "docs(" }

stages:
  docs: surface-triggered       # off | always | surface-triggered

models:
  implement: sonnet
  security:  sonnet
  ac_check:  haiku
  docs:      haiku
```

**Practice it here first.** themis itself gets the descriptor above. Its Go-ness
becomes *"what themis declares,"* not *"what the factory hardcodes."* Adding node
or Julia later is "write another descriptor," not "fork the factory."

---

## 4. New pipeline shape

```
Test → Implement → Green Gate → Review (single pass) → [Docs?] → Ship
```

Removed: the **Fix loop**, the **Refactor** stage, the **5-agent review panel**,
and the review-cycle / round-3 / blocking-threshold machinery.

---

## 5. Stage specifications

### 5.1 Test (RED) — unchanged
Turns ACs into an executable contract; makes "done" objective. Keep, invest.

### 5.2 Implement (GREEN) — enhanced, absorbs Refactor
- Implement writes **clean, properly-structured code the first time**, following
  **the standards declared by the project** (read fresh from the descriptor's
  `docs.standards`, *not* a hardcoded digest — see §10, T2 reversal).
- **Refactor is folded in here.** No separate blind refactor pass.
- Done-ness is verified by the Green Gate, not by self-report.

### 5.3 Green Gate — the mechanical floor, project-defined
Runs the descriptor's `verify` commands in order (§3). All exit 0 → GREEN,
advance. Any fail → existing bounded retry (`maxGreenGateAttempts`, currently 3)
re-runs Implement; exhausting it ships a **draft PR** carrying the failing-gate
verdict (§8) — not a dead-end block. The completion-vs-turn-limit diagnostic
logging stays.

> This replaces convention/complexity/style review for every stack — whatever
> linters the project declares enforce mechanical quality deterministically and
> for ~free (L3). The runner stays agnostic; it runs *declared* commands, never
> `go test` by name.
>
> **UI caveat (relevant later, not now):** for UI stacks, `verify` (typecheck +
> lint + component tests + build) is a *weaker* signal — it can't see "looks
> right." The human gate (L5) carries more weight there; attaching a
> build/preview artifact is a future adaptation, not part of this basics pass.

### 5.4 Review — single-pass annotation + objective gate
Runs **once. Never loops. Never edits code.** Exactly two passes, both with
ground truth:

- **Security pass** (model per descriptor, default sonnet): concrete,
  locus-bound vulnerabilities in the branch diff only. A named risk at a named
  line, not a style opinion.
- **AC-satisfaction pass** (default haiku): a checklist against the issue — for
  each AC, confirm a corresponding passing test/behavior. Output: ACs with **no**
  passing coverage. (Tests derive from ACs, so the Green Gate already proves
  most of this; this pass catches TestRed *under-covering* the ACs.)

Output → `.themis/review-results.json`, two buckets only:
- **BLOCKING → escalate (no merge):** a confirmed security vuln with a locus, or
  an AC with no passing test. → §8.
- **NOTES → PR body (non-blocking):** everything else either pass observed
  (cleaner-could-be, naming, structure, architecture observations). → carried
  verbatim into the PR for maintainer triage across PRs (L4).

No Medium-threshold panel, no re-review, no `DIFF_LINES`/`--quick` branching.

### 5.5 Fix — **REMOVED**
The loop never converged (§1). Objective failures escalate (§8); everything else
is a note.

### 5.6 Refactor — **REMOVED** (folded into Implement, §5.2)

### 5.7 Docs — input integrity first, output second
Two distinct jobs; the descriptor's `stages.docs` toggles the second:
- **Input docs** (the `docs:` corpus the agents read — standards, glossary,
  architecture): these are the factory's own inputs in a feedback loop (implement
  reads them → produces code → docs step keeps them current → next issue reads
  them). Keeping them **current and lean** is the docs stage's primary, real
  value. High priority.
- **Output docs** (README / user guides): **surface-triggered** — run only when
  the diff touches a documented surface (exported identifier, CLI flag, file
  referenced by README/config schema); otherwise skip without spawning an agent.

Runs on the descriptor's `models.docs` (default haiku).

### 5.8 Ship
PR body = AC verification table + gate verdict (security: clean / AC: all
covered) + **"Reviewer observations (not addressed — for maintainer triage)"**
(the NOTES bucket). Never auto-merged (L5).

---

## 6. State-machine changes (`internal/pipeline/pipeline.go`)

- Linear flow: `… → Implement → Review → [Docs] → Ship` (no Refactor).
- `Advance` for `StepReview`: **always → next (Docs/Ship)**, never → `StepFix`.
  The verdict (clean vs. blocking security/AC) is *recorded for the PR body and
  the ready/draft decision* (§8), not a branch in the state machine.
- Remove `ReviewCycle`, `MaxReviewCycles`, `checkReviewCycleLimit`,
  `Round3Trigger`, and the "continue to ship with unresolved findings" branch.
- Keep the Implement green-gate retry (`ImplementAttempts` /
  `maxGreenGateAttempts`).
- **Decision (§11.2):** keep `StepRefactor`/`StepFix` enum constants but make
  them unreachable (resume-compat), vs. delete them.

## 7. Runner / command / template changes

- **Honor the two-case review split:** `commands/review.md` (interactive full
  panel for humans) stays untouched; only `templates/review.md` (factory) is
  rewritten to the two fixed passes (§5.4).
- **Descriptor-driven, agnostic runner:** the Green Gate runs `verify` commands
  from the descriptor; the runner contains no `go test`/`gofmt` literals.
  `goTestRunner` becomes a generic "run the declared verify commands" runner.
- **T2 reversal:** revert this session's inline Go standards digest. Templates
  become stack-agnostic scaffolding that says *"follow this project's declared
  standards"* and read `docs.standards` fresh. (T2's token goal is met instead by
  keeping that doc lean — the docs stage's job — not by freezing a Go copy.)
- **Templates de-Go-ified:** remove Go idioms, `go test` phrasing, Go file-naming
  rules from the shared templates; push them to the descriptor / project docs.
- **Models from the descriptor**, not hardcoded per step.

## 8. Output & remediation

The factory **always emits a PR — never auto-merges, never auto-fixes anything**
(L5). The PR is the thing to work with. Two shapes:

- **Ready PR** — Green Gate passed and Review is clean (no blocking security/AC):
  carries the AC table + clean verdict + any minor NOTES. → merges with a glance;
  **autonomy is preserved for the clean majority.**
- **Flagged / draft PR** — either Review raised a blocking security/AC finding,
  or the Green Gate couldn't reach green after retries (draft, with the
  failing-gate verdict + breadcrumbs). → **interactive remediation.**

The factory **attempts zero fixes, even on must-fix findings.** It cannot
reliably tell must-fix from nice-to-have, and any in-scope fix attempt
re-introduces the non-convergent, sampler-driven churn this redesign removes
(L1, L4). Triage with judgment belongs in the interactive layer.

**Interactive remediation** (human-driven, uses the rich tooling kept *out* of
the factory):
1. Pull the PR + verdict/notes into a Claude Code session (`pr-review` / the full
   interactive `/review`).
2. **Sort each finding:** fix-now (needed for mergeability) vs. defer /
   follow-up issue (`review-walker`).
3. Apply **only the must-fixes** interactively, on the **same PR branch**, with
   the holistic cross-codebase view.
4. Re-run the gate, merge; file the deferred findings as issues.

There is **no dead-end "blocked issue, no PR" state** — a non-building attempt is
still a draft PR you can work from. Good breadcrumbs (verdict, notes, what was
built and why) make the session start informed, not from scratch.

## 9. Cost impact (estimate)

- Review: ~5 agents × ~3 cycles (~15 LLM calls) → **2 passes × 1**. ~85% off the
  dominant cost center.
- Refactor removed; Docs skipped on most issues.
- A trivial issue like #75 should drop from a multi-cycle, ~20-min, 22%-of-budget
  run to a single linear pass with one review.

## 10. Risks & assumptions

- **L5 depends on a real human PR gate.** Deferred NOTES rely on it; if PRs merge
  unread, they rot.
- **Linter availability** is per-descriptor; absent a linter, `verify` uses
  build + format + test (still most mechanical coverage).
- **Security single-pass** can miss things — acceptable because it escalates (not
  silently fixes) and the human reviews; not worse than today's expensive panel.
- **Descriptor is now load-bearing**: a wrong/missing `verify` command silently
  weakens the gate. Needs a sane default + a "no verify declared" warning.

## 11. Decisions (resolved)

1. **Failure handling:** the factory **always emits a PR** (ready or draft) and
   **attempts zero fixes** — even blocking security/AC findings ship as a flagged
   PR. All fixing is interactive (§8), sorted finding-by-finding into
   fix-now / defer / follow-up-issue. No "blocked issue, no PR" dead-end. The
   clean majority still merges with a glance, so autonomy holds.
2. **Enum values:** keep `StepRefactor`/`StepFix` constants but make them
   unreachable (resume compat); remove from `linearNext` / `agentSteps` /
   templates.
3. **Descriptor home:** a new `.themis/workflow.yaml`, distinct from the model
   profile.
4. **AC-satisfaction:** its own haiku pass, separate from the security pass.

## 12. Explicit non-goals (later conversations, not this pass)

These are parked deliberately until the agnostic core codes itself at sane token
cost:
- **UI via MCP** — the framework-agnostic design system and prototype platform
  (both MCP-accessible). A *project-declaration* problem when we get there.
- **Visual/interaction verification** for UI (preview/screenshot artifacts, the
  weaker-green-signal adaptation).
- **Stage-flow declared per project** beyond the simple toggles in §3 (e.g.
  projects defining custom steps). Get the fixed skeleton (L6) solid first.
- **Non-Go descriptors** (node, Julia, …) — prove the model on themis first, then
  the second descriptor validates agnosticism.
