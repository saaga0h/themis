---
name: hearth-sync
description: Use when planning work in a repo that consumes the Hearth design system (via the loom MCP server) and you want to see what changed in the DS since the last review and turn it into refactor/adoption issues — deliberately, never auto-editing code. Triggers on "sync with Hearth / the design system", "what changed in the DS", "plan DS refactoring", or at the planning stage of a Hearth-consuming repo. Reads HEARTH_REV, queries get_rev/get_changelog, triages by real usage, drafts factory issues, then re-pins HEARTH_REV.
---

# Hearth sync — plan against DS changes, deliberately

This produces a **plan and issues**, never code edits. Consumption is pull-based:
the app changes because *you planned it to* against a confirmed usage — not
because the DS moved. Works for any stack (custom framework, React, vanilla) — you
reason about tokens/components, then translate to the repo's idioms.

Requires the **loom MCP server** (`get_rev`, `get_changelog`, `search`, `get_spec`,
`get_tokens`). If MCP is unavailable, hit the HTTP API at `LOOM_API_URL`
(`/api/v1/rev`, `/api/v1/spec/{slug}`, …).

## `HEARTH_REV` is a watermark, not a version lock

`HEARTH_REV` (a single-line file at the repo root) records the Hearth `rev` this
repo was **last reviewed against**. It exists only to bound "what changed since."
You always build against the **current** Hearth — there is no fetching of old
versions, no pinning to a stale radio button. New work obeys the current DS.

## Steps

1. **Baseline.** Read `HEARTH_REV` at the repo root.
   - Missing → first sync. Call `get_rev`, write it to `HEARTH_REV`, and report
     that there's no delta to triage yet (future runs diff from here). Stop unless
     the user explicitly wants a full current-surface audit.

2. **Detect drift.** Call `get_rev`. If it equals `HEARTH_REV`, the DS hasn't
   moved — report "in sync" and stop. Otherwise call `get_changelog` and take the
   entries newer than `HEARTH_REV` (the changelog is dated, newest-first, and
   tagged; the log is small — read it all and bound by your watermark).

3. **Triage each changed entry by tag — gated by real usage in THIS repo:**
   - **value** — a token value changed. It already reached the app through
     `var(--token)`; **no code change.** Note an optional visual re-check; create
     no issue.
   - **added** — a new token/component/kit. Optional. Create an *adoption* issue
     only if the app has a real place for it; otherwise just note it's available.
   - **breaking** — a rename/removal/structural change. **Grep the repo** for the
     affected token name or component usage. No hits → no work, skip. Hits → a
     refactor scoped to those exact files/usages.

4. **Resolve meaning before drafting.** For anything an entry names (e.g. "radio"),
   `search` it and `get_spec` its chapter (and `get_tokens` for values) so the
   issue says concretely *what* to change and *to what*, in current Hearth terms.
   A component may be a section within a chapter — `search` resolves the name.

5. **Draft issues for the factory.** For each confirmed refactor (and wanted
   adoption), create an issue using the repo's workflow — prefer `idd:create-feature`
   if available, else `gh`/`tea issue create`, else append to the planning doc.
   Each issue states: the Hearth change (cite the changelog entry + the new `rev`),
   where it's used here (the grep hits), and the migration (changelog note +
   `get_spec`). Label per the repo's "ready-for-agent" convention so the factory
   picks it up.

6. **Re-pin.** After issues are filed, write the current `get_rev` value to
   `HEARTH_REV`. That bounds the next sync.

## Definition of done

Drift detected from `HEARTH_REV` · every changed entry triaged · breaking changes
gated by real usage · meaning resolved via `get_spec` · issues filed for the
factory · `HEARTH_REV` re-pinned. **No app code edited by this skill.**
