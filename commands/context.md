---
description: Regenerate CLAUDE.md from current repo state. Use before major refactoring, after large merges, or whenever CLAUDE.md has drifted. Rebuilds from scratch by auditing the actual codebase.
argument-hint: [--dry-run]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Context Command

You regenerate CLAUDE.md by auditing the current repository state. This is a **full rebuild**, not an incremental update. Use when the existing CLAUDE.md is stale, wrong, or about to become wrong due to major changes.

## When to use

- Before a major refactoring that will invalidate current instructions
- After a large merge or restructuring
- When CLAUDE.md has accumulated drift
- When starting fresh on an inherited or unfamiliar repo
- User just wants a clean slate

## Step 0: Parse arguments

If `--dry-run` is passed, show what CLAUDE.md would contain but don't write it. Ask user to confirm before writing.

## Step 1: Scan the repo

Delegate to **codebase-scanner** agent for fast structure mapping.

## Step 2: Audit operational facts

Discover each section's content from the **actual repo**, not from existing CLAUDE.md:

### Build & Test
```bash
# What build system?
ls Makefile* CMakeLists.txt build.gradle pom.xml package.json Cargo.toml go.mod pyproject.toml 2>/dev/null

# If Makefile, what targets?
grep -E '^[a-zA-Z_-]+:' Makefile 2>/dev/null | head -20

# What test command works?
# Try the obvious ones, note which succeeds
```

Capture the **exact commands** that work right now. Don't guess from config files alone — verify if practical.

### Conventions
```bash
# Dominant patterns — only note things that aren't language defaults
# Error handling style
grep -rn "if err != nil" --include="*.go" | head -5
grep -rn "fmt.Errorf\|errors.Wrap" --include="*.go" | head -5

# Naming patterns that deviate from language convention
# Import organization if non-standard
# Anything enforced by CI but not by standard linters
```

Only include conventions that are **non-obvious** — things a competent developer in this language wouldn't assume by default.

### Constraints
Look for things an agent would get wrong:
- Are there sibling repos or external services this depends on?
- Are there directories that look like they should contain something but intentionally don't?
- Multiple docker-compose files with different purposes?
- Services that must be running on the host?
- Specific hardware or OS requirements?

```bash
# External service references
grep -rn "localhost\|host.docker.internal\|127.0.0.1" --include="*.go" --include="*.yaml" --include="*.yml" --include="*.toml" | head -20

# Docker/compose files
ls docker-compose*.yaml docker-compose*.yml Dockerfile* 2>/dev/null

# Hardware or driver references
grep -rn "rocm\|cuda\|gpu\|ROCm\|CUDA" --include="*.go" --include="*.yaml" --include="*.hcl" 2>/dev/null | head -10
```

### Gotchas
Check for non-obvious things:
- Config files that reference host-specific paths
- Known workarounds in CI or build scripts
- Comments containing "HACK", "WORKAROUND", "NOTE", "XXX"

```bash
grep -rn "HACK\|WORKAROUND\|XXX\|FIXME\|NOTE:" --include="*.go" | head -10
```

### Protocol contracts
If the project uses messaging, APIs, or inter-service communication:
```bash
# MQTT topics
grep -rn "mqtt\|topic\|publish\|subscribe" --include="*.go" --include="*.yaml" | head -20

# API routes
grep -rn "HandleFunc\|Handle\|router\.\|mux\.\|gin\.\|echo\." --include="*.go" | head -20
```

## Step 3: Read existing CLAUDE.md

If one exists, check for **user-added sections** — anything that doesn't fit the standard template. Preserve these; the user put them there for a reason.

## Step 4: Compose new CLAUDE.md

Write using this structure. **Every line must pass the test: "Can an agent figure this out from the code alone?"** If yes, don't include it.

```markdown
# CLAUDE.md — <Project Name>

## Build & Test
<!-- exact commands, versions, only what's not obvious from Makefile/config -->

## Conventions
<!-- only non-obvious, non-linter-enforceable patterns -->

## Constraints
<!-- what an agent WILL get wrong without being told -->

## Gotchas
<!-- non-obvious operational issues -->
```

Omit sections entirely if they'd be empty. A 10-line CLAUDE.md is better than a 50-line one with padding.

## Step 5: Present to user

Show the composed CLAUDE.md and ask:
- "Does this capture everything an agent needs to know that it can't learn from the code?"
- "Anything to add or remove?"

Wait for confirmation before writing, even if `--dry-run` wasn't passed.

## Step 6: Write

Write to CLAUDE.md at the repo root. Report what changed versus the previous version (if one existed).

## Important

- This is a REBUILD, not an update. Start from the repo, not from the old file.
- The only thing preserved from the old CLAUDE.md is user-added custom sections.
- Shorter is better. The paper says so, experience says so.
- If the repo is well-structured with a good Makefile and clear naming, CLAUDE.md might be 10 lines. That's fine.
- Don't include project descriptions, architecture overviews, or directory trees.
- Verify commands actually work when practical — don't just read config files.

## Hard boundary with /document

`/document` produces `docs/development.md` which covers setup, all build commands, all env vars, make targets, workflow, and troubleshooting — the full developer reference.

CLAUDE.md is not a developer reference. It is a **last-resort safety net**: the minimum facts an agent will get wrong on first contact, before any docs are loaded.

**The test for CLAUDE.md inclusion:** "If an agent starts a task cold, with no docs loaded, and doesn't know this fact — will it cause a hard failure or silently wrong result?"
- Yes → CLAUDE.md
- No, it's in docs/development.md → don't duplicate it here

**Never put in CLAUDE.md:**
- Full setup or prerequisites → `docs/development.md`
- All env vars and configuration → `docs/development.md` Configuration table
- All make targets or scripts → `docs/development.md` Make Targets
- Architecture or system overviews → `ARCHITECTURE.md`
- Why design choices were made → `CONCEPTS.md`
- Subsystem explanations → `docs/subsystems/<name>/README.md`

**What still belongs in CLAUDE.md:** the one-liner that prevents a hard failure — "tests require Postgres on :5433", "don't run `go build` directly, codegen runs via `make build`", "this repo has two go.mod files, don't merge them". Facts so sharp and failure-critical they must always be in context, not looked up.

**If `docs/development.md` exists:** add a single reference line to CLAUDE.md:

```markdown
## Docs
See `docs/development.md` for build commands, setup, env vars, and troubleshooting.
```

This costs one line. It ensures any agent — even one not going through the doc resolver — knows where the full reference lives. Commands that don't need it won't read it. Commands that do will find it.
