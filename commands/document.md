---
description: Audit and update project documentation. Discovers project type, scans for drift between code and docs, then updates what needs updating. Covers root docs (README, CONCEPTS, ARCHITECTURE) + 3-tier docs/ structure. Works across any language/framework.
argument-hint: [--dry-run] [--full] [--tier N]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Document Command

You orchestrate documentation auditing and updates by delegating to specialized agents. You retain all judgment calls — what needs CONCEPTS.md, what earns a Tier 3 doc, how to synthesize the drift report.

## Documentation structure

**Root docs** (human-facing landing surface):
- `README.md` — practical: what is this, how to set it up, how to run it
- `CONCEPTS.md` — philosophical: why the design choices, core abstractions, novel mechanisms
- `ARCHITECTURE.md` — technical: system overview, components, data flows, invariants, constraints

**docs/ tier** (deeper reference, 3 tiers):
- **Tier 1** (`docs/`) — development guide, data model, API reference, messaging, content plan
- **Tier 2** (`docs/subsystems/<name>/`) — per-subsystem architecture, data flows, interfaces
- **Tier 3** (`docs/subsystems/<name>/modules/`) — module-level business rules, implicit behaviors

**Modes:**
- Default: update only what has drifted
- `--full`: rebuild from scratch, treating code as sole source of truth
- `--dry-run`: show what would change, write nothing
- `--tier N`: scope audit to a specific tier (0=root docs, 1, 2, or 3)

---

## Step 0: Parse arguments

Note whether `--dry-run`, `--full`, `--tier`, or none was passed.

---

## Step 1: Scan

Delegate to **doc-scanner** agent. It will:
- Identify project type(s) from signature files
- Map the codebase structure (entry points, packages, config vars, migrations, routes, build targets)
- Inventory all existing documentation (root docs + docs/ tree)
- Identify drift between code and docs
- Suggest subsystem candidates

Wait for the report before proceeding.

---

## Step 2: Review scan results

Read the doc-scanner report. Determine:

1. **What root docs exist** — which of README, CONCEPTS, ARCHITECTURE are present?
2. **What Tier 1 docs exist** — development.md, datamodel.md, api-reference.md, messaging.md, content-plan.md?
3. **What subsystems are documented** — does `docs/subsystems/` exist? Which subsystems?
4. **What has drifted** — entry points, config, data model, API, build targets, dead references?
5. **What is missing entirely** — docs that should exist based on what the code has?

### Subsystem confirmation

If `docs/subsystems/` exists, use the existing subsystem list as canonical. Check if the scanner found new code groupings that should be added.

If no subsystems are documented yet:
- Review the scanner's subsystem candidates
- If the groupings are clear, proceed
- If ambiguous, ask the user: "The codebase suggests these subsystems: [list]. Does this match how you think about the project?"

---

## Step 3: Report drift

Present findings to the user, grouped by document:

```
## Documentation Audit

### Root Docs
- README.md: <exists, up to date | exists, drifted: [specifics] | missing>
- ARCHITECTURE.md: <exists, up to date | exists, drifted | missing | not needed>
- CONCEPTS.md: <exists, up to date | exists, drifted | missing | not needed>

### Tier 1
- docs/development.md: <status + drift details>
- docs/datamodel.md: <status>
- docs/api-reference.md: <status>
- docs/messaging.md: <status>
- docs/content-plan.md: <status>

### Tier 2 (subsystems)
<per subsystem: documented | undocumented | drifted>

### Tier 3 (modules)
<per existing module doc: up to date | drifted | source file removed>

### Dead References
<docs that reference files/functions that no longer exist>
```

If `--dry-run` was passed, stop here.

If nothing has drifted and `--full` was not passed, say so and stop.

---

## Step 4: Update root docs

Root docs require the most judgment. Handle each:

### README.md

If missing: delegate to **doc-writer** with the assignment to create it. Specify the format:
- One-line tagline + brief description + tech stack
- Prerequisites, Setup, Configuration table
- Binaries/Commands/Scripts table
- Running in Dev, Inspecting State, Infrastructure (as applicable)

If exists and drifted: delegate to **doc-writer** with the specific sections that need updating and the drift details from the scan.

### ARCHITECTURE.md

If missing and the project has more than one component or non-trivial data flow: delegate to **doc-writer**. Specify the format:
- Mermaid system overview diagram
- Numbered sections with Table of Contents
- Component inventory table, data flow sequence diagrams
- Invariants section, Known Constraints section

If exists and drifted: delegate to **doc-writer** with drift details. Instruct it to update component inventory, data flows, and Mermaid diagrams in place — change labels/edges, don't restructure layout.

### CONCEPTS.md

**Write this yourself** — do not delegate. This requires the deepest judgment about *why* the system works the way it does.

If missing: assess whether the project has genuine conceptual depth — novel algorithms, non-obvious design philosophy, domain abstractions worth explaining. Not every project needs CONCEPTS.md. If uncertain, ask the user.

If it should exist, write it with:
- Numbered sections, Table of Contents
- Problem Statement (what this solves, why the approach is non-obvious)
- One section per core concept (definition, motivation, implementation sketch, what is novel)
- Design Decisions and Roads Not Taken
- Relationships Between Concepts (dependency diagram)

If exists and drifted: update only the sections where algorithm descriptions or design decisions have changed. Match the existing style exactly. If uncertain whether a conceptual claim is still accurate, flag it rather than guessing.

---

## Step 5: Update Tier 1 docs

Determine which Tier 1 docs are needed based on the scan:

| Document | Create when |
|---|---|
| `docs/development.md` | Always |
| `docs/datamodel.md` | Database exists (migrations, ORM models, schema files) |
| `docs/api-reference.md` | HTTP/gRPC/GraphQL routes exist |
| `docs/messaging.md` | Message broker used (MQTT, Kafka, RabbitMQ, etc.) |
| `docs/state-management.md` | Frontend with non-trivial state (Redux, Zustand, etc.) |
| `docs/content-plan.md` | Always |

For each needed doc, delegate to **doc-writer** (parallel — one agent per doc). Pass:
- The target file path
- The template to use (from the agent's template library)
- The source files to read (from the scan report)
- The drift details (what specifically needs creating or updating)

**Special instructions for development.md:** must include a "When Things Look Wrong" troubleshooting table and a "Secrets & Deployment" section. The troubleshooting table maps symptoms → checks → fixes. Secrets section documents what env vars are needed without prescribing how they're injected.

**content-plan.md**: generate from the actual docs/ tree. List every doc with path, tier, one-line description, and comma-separated tags. Tags are the subsystem names, concern areas, or keywords that a command can grep to find this doc — e.g. `mqtt,messaging,broker` or `auth,sessions,tokens` or `datamodel,schema,migrations`. Tags should match the vocabulary a developer would use when naming a feature or plan.

```markdown
# Documentation Content Plan

| Path | Tier | Description | Tags |
|---|---|---|---|
| README.md | root | Project overview, setup, running | setup,onboarding,install |
| ARCHITECTURE.md | root | System structure, data flows, invariants | architecture,system,components |
| docs/development.md | 1 | Config reference, all commands, troubleshooting | dev,config,env,build,commands |
| docs/subsystems/mqtt/README.md | 2 | MQTT subsystem | mqtt,messaging,broker,pubsub |
...
```

---

## Step 6: Update Tier 2 docs

For each subsystem (confirmed in Step 2), delegate to **doc-writer** with:
- Target: `docs/subsystems/<name>/README.md`
- Template: Tier 2 subsystem README
- Source files: the key files identified by the scanner for that subsystem
- Instruction: the README must answer these three questions somewhere in its content:
  1. **How do I add X?** — the common extension path
  2. **How do I diagnose problems?** — first 2-3 checks
  3. **What is the failure behavior?** — drop and retry? crash? silently skip?

Launch subsystem agents in parallel — they are independent of each other.

---

## Step 7: Decide Tier 3

**This is your decision, not an agent's.** Review the scan results and Tier 2 docs. Ask for each module: would a developer or LLM make a wrong assumption that leads to a bug if this module has no doc?

The bar is **incident surface area**:
- A function has a silent failure mode (wrong result, not error)
- A priority order, threshold, or exclusion exists whose reason is not in the code
- An invariant is enforced non-obviously
- A bug was caused (or nearly caused) by misunderstanding this module

For each module that earns a Tier 3 doc, delegate to **doc-writer** with:
- Target: `docs/subsystems/<name>/modules/<module>.md`
- Template: Tier 3 module doc
- Source file path
- Specific instruction about what non-obvious behavior to document

Do NOT create Tier 3 for straightforward CRUD, wrapper functions, or anything fully covered by the Tier 2 README.

---

## Step 8: Verify

Delegate to **codebase-scanner** agent with this specific task:

```
Check the docs/ directory for consistency:
- Does README.md exist with prerequisites + setup sections?
- Does ARCHITECTURE.md exist with component inventory? (if project is non-trivial)
- Do all files listed in docs/content-plan.md actually exist?
- Do all @parent references in doc metadata resolve to existing files?
- Do all @source references resolve to existing files?
- Do markdown links between docs resolve (no dead cross-references)?
- Is there content duplicated between root docs and docs/ tier?
```

Report any broken references or gaps.

---

## Step 9: Report

Present the final summary:

```
## Documentation Update Report

### Root Docs
- README.md: [created | updated section X | no change]
- ARCHITECTURE.md: [created | updated section X | not needed | no change]
- CONCEPTS.md: [created | updated section X | not needed | no change]

### Tier 1
- docs/development.md: [created | updated | no change]
- docs/datamodel.md: [created | updated | skipped — no database | no change]
- docs/content-plan.md: [created | updated]
...

### Tier 2
- docs/subsystems/<name>/README.md: [created | updated | no change]
...

### Tier 3
- docs/subsystems/<name>/modules/<module>.md: [created | no change]
...

### Verification
- Cross-references: [all valid | N broken links]
- Dead references: [none | N found]

### Coverage
- Root docs: [X/3] present
- Tier 1: [X] docs covering [Y] project areas
- Tier 2: [X/Y] subsystems documented
- Tier 3: [X] module docs where [Y] were identified as needing them
```

---

## Writing style rules (for all delegated writing)

Pass these to every doc-writer agent:

- Code is the source of truth. If code and docs disagree, code wins.
- Do not invent content. If you can't determine something from code alone, leave a TODO marker.
- Do not duplicate between layers — README says what to run, ARCHITECTURE says how it's structured, CONCEPTS says why.
- Match existing tone when updating.
- Do not add padding or filler.
- Do not remove sections — only update content within them.
- Cross-reference: if adding to one doc, check if another needs a mention.
- Mermaid diagrams: update labels and edges, don't restructure layout.
- Tier 3: lead with non-obvious behavior, not function signature restatements.
- Secrets: document what env vars exist, never prescribe how they're injected.

---

## Important

- **You are the orchestrator.** Agents do the mechanical work. You make the judgment calls.
- **CONCEPTS.md is yours.** Never delegate it. It requires understanding *why*, not just *what*.
- **Tier 3 decisions are yours.** Agents can't judge "would someone be surprised by this?" — you can.
- **doc-scanner runs first, always.** Everything else depends on its report.
- **Parallel when independent.** Tier 1 docs, Tier 2 subsystems, and Tier 3 modules are independent — launch their doc-writer agents in parallel.
- **Do not invent content.** If the code doesn't clearly show something, flag it and ask the user rather than guessing.

## Hard boundary with /context

`/document` owns the full developer reference: setup, build commands, env vars, make targets, workflow, troubleshooting, architecture, subsystems, data model, API surface, design rationale, module business rules.

`/context` owns **only the failure-critical minimum** that must be present in every agent context before any docs are loaded — facts so sharp that not knowing them causes a hard failure or silently wrong result on first contact.

**Never put in docs/development.md what belongs in CLAUDE.md:**
- "Tests require an external service running at a specific address" → CLAUDE.md
- "Don't use the standard build command, use this one instead due to codegen" → CLAUDE.md
- Non-obvious hard constraints an agent will violate immediately without being told → CLAUDE.md

Everything else — the full setup guide, all env vars, all targets, all workflow — belongs here in docs/development.md, not in CLAUDE.md.

If you find failure-critical facts buried in CLAUDE.md that are actually covered by docs/development.md with no hard-failure consequence, flag it for the user — CLAUDE.md may be longer than it needs to be.

**After creating or updating `docs/development.md`:** check whether CLAUDE.md contains a `## Docs` section referencing it. If not, flag it in the final report:

```
Note: CLAUDE.md has no reference to docs/development.md. Consider adding:

## Docs
See `docs/development.md` for build commands, setup, env vars, and troubleshooting.
```

Do not write to CLAUDE.md yourself — that is `/context`'s territory.
