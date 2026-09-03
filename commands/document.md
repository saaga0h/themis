---
description: Audit and update project documentation. By default it updates the human-facing narrative — the root docs (README, CONCEPTS, ARCHITECTURE) and Tier-1 developer guides. With --full or --tier 2+ it generates deeper subsystem/module views from the code, for onboarding or a brownfield→factory transition — generated on demand, not a maintained tier. Works across any language/framework.
argument-hint: [--dry-run] [--full] [--tier N]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Document Command

You orchestrate documentation auditing and updates by delegating to specialized agents. You retain all judgment calls — whether the project has conceptual depth worth a CONCEPTS.md, which module views earn generation, how to synthesize the drift report.

## What this owns — and what it does not

Facts about the code live **in the code**, as package and symbol doc comments, read with `go doc` (or the language's equivalent). That is the fact layer, and the build pipeline maintains it. `/document` does **not** maintain a parallel subsystem-doc tier, and must never re-narrate code into prose files that then drift.

`/document` owns the **human-facing narrative**, in two regimes:

**Maintained (the default):**
- **Root narrative** — `README.md` (what it is, how to run it), `CONCEPTS.md` (why the design), `ARCHITECTURE.md` (system structure, data flows, invariants).
- **Tier-1 developer guides** (`docs/*.md`) — development guide, data model, API reference, and the like. Human references kept current.

**Generated on demand (opt-in — `--full` / `--tier 2+`):**
- **Subsystem / module views** — a navigable structure of an unfamiliar codebase, generated **from the code as sole source of truth**. Their purpose is **onboarding** and **brownfield→factory transition** (a baseline so the factory can work an existing project). They are regenerated when wanted, never hand-maintained — so they cannot drift.

## Modes

- **Default (no flags):** audit and update the maintained human docs (root narrative + Tier-1 guides) that have drifted.
- **`--full`:** additionally (re)generate the on-demand deep views (subsystems, modules) from the code. Use for transition/onboarding.
- **`--tier N`:** scope to a tier — `0` root narrative, `1` developer guides, `2` subsystems, `3` modules. Tiers 2–3 are the on-demand deep views.
- **`--dry-run`:** show what would change, write nothing.

---

## Step 0 — Parse arguments

Note whether `--dry-run`, `--full`, `--tier`, or none was passed. **Tiers 2–3 run only under `--full` or an explicit `--tier 2`/`--tier 3`.** Default and `--tier 0/1` never touch subsystem/module views.

## Step 1 — Scan

Delegate to the **doc-scanner** agent: identify the project type, map the codebase structure (entry points, packages, config, routes, build targets), inventory the existing narrative docs, and identify drift between the code and the maintained docs. Wait for the report.

## Step 2 — Report drift

Present findings grouped by document — root narrative and Tier-1 guides always; subsystems/modules only when the deep views are in scope. Note dead references. If `--dry-run`, stop. If nothing drifted and no `--full`/`--tier 2+` was requested, say so and stop.

## Step 3 — Update root narrative (default)

Root docs require the most judgment.

- **README.md** — if missing or drifted, delegate to **doc-writer** with the sections/format to produce (tagline + description + stack; prerequisites, setup, configuration table; commands; running/inspecting as applicable).
- **ARCHITECTURE.md** — if the project has more than one component or a non-trivial data flow, delegate to **doc-writer** (system-overview diagram, component inventory, data flows, invariants, constraints). When updating, change labels/edges in place — don't restructure.
- **CONCEPTS.md — write this yourself, never delegate.** It requires the deepest judgment about *why* the system works this way. Not every project needs one; if uncertain, ask. When it exists and drifted, update only the changed rationale, matching the existing style; flag rather than guess.

## Step 4 — Update Tier-1 developer guides (default)

Determine which Tier-1 guides the project needs — development guide (always), data model (when there's a database), API reference (when there are routes), and so on. Delegate each to **doc-writer** in parallel, with the target path, source files, and drift details. `docs/development.md` must include a "When Things Look Wrong" troubleshooting table and a "Secrets & Deployment" section (what env vars exist, never how they're injected).

## Step 5 — Generate deep views (opt-in: `--full` / `--tier 2+`)

Only when requested. These are generated from the code as the sole source of truth, for onboarding / brownfield transition — not maintained afterward.

- **Subsystems (Tier 2):** for each code grouping the scanner identifies, delegate to **doc-writer** to produce a `docs/subsystems/<name>/README.md` that answers: how do I extend this? how do I diagnose problems? what is the failure behavior? Launch in parallel.
- **Modules (Tier 3) — your judgment.** Only where a developer or LLM would make a wrong assumption that causes a bug (a silent failure mode; a non-obvious priority, threshold, or invariant). Delegate each to **doc-writer** with the specific non-obvious behavior to capture. Never for straightforward CRUD or wrappers.

## Step 6 — Verify

Delegate to **codebase-scanner**: do the root docs exist with their expected sections; do `@parent`/`@source` references resolve; do cross-doc links resolve; is content duplicated between root and deeper docs? Report broken references or gaps.

## Step 7 — Report

Summarize per document — created / updated / unchanged — plus coverage of the root narrative and Tier-1 guides, the generated deep views (if any), and the verification results.

---

## Writing style rules (pass to every doc-writer)

- Code is the source of truth. If code and docs disagree, code wins.
- Do not invent content; leave a TODO marker when the code doesn't show it.
- Do not duplicate between layers — README says what to run, ARCHITECTURE how it's structured, CONCEPTS why. **Never re-narrate code that `go doc` already documents.**
- Match existing tone; no padding; update content within sections rather than removing them.
- Mermaid diagrams: update labels/edges, don't restructure.
- Secrets: document what env vars exist, never how they're injected.

## Important

- **You are the orchestrator.** Agents do the mechanical work; you make the judgment calls.
- **CONCEPTS.md is yours** — never delegate it.
- **Tier-3 decisions are yours** — "would someone be surprised by this?" is a judgment.
- **doc-scanner runs first, always.**
- **Deep views are opt-in and on-demand.** Never generate or maintain a subsystem/module tier by default; the code (via `go doc`) is the fact layer.
- **Do not invent content** — flag and ask rather than guess.

## Hard boundary with /context

`/document` owns the developer reference and narrative. `/context` owns only the failure-critical minimum that must be present in every agent context — facts so sharp that not knowing them causes a hard failure or silently wrong result on first contact (e.g. "tests need an external service running at a specific address", "use this build command due to codegen"). Those belong in CLAUDE.md, not here.

After creating or updating `docs/development.md`, check whether CLAUDE.md references it; if not, flag it in the report. Do not write to CLAUDE.md yourself — that is `/context`'s territory.
