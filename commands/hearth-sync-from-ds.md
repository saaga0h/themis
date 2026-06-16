---
description: Scan a Hearth-consuming application against the current design system revision. Writes a sync report and updates HEARTH_REV.
allowed-tools: Read, Write, Bash, mcp__loom-mcp__get_rev, mcp__loom-mcp__get_changelog, mcp__loom-mcp__lint_snippet
---

# Hearth Sync from DS

Scan the application against the Hearth design system. Write a report to `.claude/sync-reports/` and update `HEARTH_REV`.

## Step 1 — Read HEARTH_REV

Read `HEARTH_REV` from the repository root. Store its content as `previous_rev`.

If the file does not exist, create it with content `0`. Set `previous_rev` to `"0"`.

## Step 2 — Get current Hearth revision

Call `loom-mcp:get_rev`. Store the result as `target_rev`.

If `previous_rev` equals `target_rev`, print "Current with Hearth at `<target_rev>`." Stop.

## Step 3 — Scan the codebase

Delegate to **codebase-scanner** to map the project (runs on Haiku).

Identify all `.css`, `.ts`, `.tsx`, `.html`, and `.tui` files that may reference Hearth tokens. Note which directories contain source files.

## Step 4 — Get changelog

Call `loom-mcp:get_changelog`.

Extract entries tagged `breaking`, `value`, or `added`. From `breaking` and `value` entries, extract CSS custom property names (starting with `--`). Store as `changed_tokens`.

If `previous_rev` is `"0"`, all entries are relevant.

## Step 5 — Grep for affected tokens

Skip if `changed_tokens` is empty.

For each token, grep source directories from step 3:

```bash
grep -rn "<token>" <source-dirs> --include="*.ts" --include="*.tsx" --include="*.css" --include="*.html" --exclude-dir=node_modules --exclude-dir=dist
```

Also grep without the `--` prefix to catch utility class references.

## Step 6 — Lint style files

Select CSS files from step 3 that contain `var(--` or `@apply` (max 5, pick largest).

Run `loom-mcp:lint_snippet` on each with lang `css`.

## Step 7 — Write the report

Create `.claude/sync-reports/` if needed. Write to `.claude/sync-reports/<YYYY-MM-DD>.md`.

Report only facts. Do not assess importance. Do not recommend actions. Do not triage.

```
# Hearth Sync Report

**Previous rev:** <previous_rev>
**Target rev:** <target_rev>
**Date:** <today>

## Changelog entries

<Paste all relevant entries as-is from loom-mcp:get_changelog>

## Grep hits

<If no hits: "No changed tokens found in the codebase.">
<If hits, one subsection per token:>

### `--example-token`

| File | Line | Content |
|------|------|---------|
| `path/to/file.ts` | 42 | `the actual line` |

## Lint results

<If none: "No lint violations.">
<If violations:>

| File | Line | Issue | Suggestion |
|------|------|-------|------------|
| `path/to/styles.css` | 15 | Raw hex `#f54e7c` | `var(--accent)` |
```

## Step 8 — Update HEARTH_REV

Write `target_rev` to `HEARTH_REV`. Single line, no trailing newline.

## Step 9 — Print summary

```
Hearth sync complete.
  Previous: <previous_rev>
  Current:  <target_rev>
  Report:   .claude/sync-reports/<date>.md
```