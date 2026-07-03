---
name: pr-review
description: "When the user says a PR is ready for review (e.g. \"#XX is ready for review\"), follow this process to audit a factory-generated PR and decide its route: merge, fix, or discard-and-replan. Use whenever auditing a factory PR's review notes, checking the commit pipeline, or deciding whether a PR is good to merge."
---

# PR Review

The factory ships PRs with self-classified review notes. Your job is not to redo its review — it is to **audit that classification** against the project contracts, surface the debt it waved through, and **decide the route**. A merge verdict is never the whole output.

## How you work — orchestrate, don't read

You (the orchestrator) hold the **judgement**; delegate the **reading** so it happens in cheaper, separate contexts and only distilled evidence returns to you. Do not scroll the raw diff or full files yourself — reason over the digest. This keeps your context lean and puts the token-heavy reading on cheaper models.

- **Reading repo files → `codebase-scanner` (Haiku).** Whenever repo files need reading for orientation (where a symbol is used, package layout, what a touched file contains), spawn the scanner; it returns a compact map, not dumps.
- **Diagnosis → `pr-diagnose` (Sonnet).** Hand it the PR (diff + Review Notes), the issue's ACs, the contracts, and any scanner map. It returns a **structured evidence digest** — per concern: source / claim / location / what-it-does / evidence / candidate-fix. It locates and summarizes; **it does not render verdicts** — that's yours.
- **Fixes → `pr-fix` (Sonnet).** Once you've decided a fix, hand it a precise spec; it implements + verifies one fix and returns a compact result.

Contracts: `CODING_STANDARDS.md`, `UBIQUITOUS_LANGUAGE.md`. The diagnose agent checks every finding against them; you judge against them.

## 1. Diagnose (delegate)

Spawn `pr-diagnose` (Sonnet), reusing `codebase-scanner` (Haiku) for repo reads, to produce the evidence digest. It must cover: **pipeline order** (`test → feat → (refactor) → (fix) → (docs)`; a `fix` with no preceding `test` is suspect); each **factory Review Note** re-located in the code and stated as what it *actually* is (every factory label is a claim to verify, not a decision); **AC → test mapping** (each AC's *correctness*-asserting test — not existence, not "no error", not a bare count; flag any AC without one and any untested error path); **omissions** the factory didn't flag; and **contract checks** (hardcoded infra, swallowed errors, unthreaded context, aliased terminology, credentials, unbounded reads, missing validation).

A sparse digest on a non-trivial diff is itself suspect — re-diagnose rather than accept "nothing found."

## 2. Judge (you + the human)

Over the digest — not the raw code — classify each concern against the contracts:
- **Blocking** — any contract violation (always blocking, never downgrade for convenience); any AC without a correctness-asserting test; any untested error path; any security issue.
- **Follow-up** — improvements beyond the contract floor; pre-existing debt the PR touched or exposed (the moment the PR brushes a latent violation it's visible — propose the issue now, don't let "already there" close it).
- **Observation** — style not in the standards; different-but-fine alternatives; things a known issue already owns.

If a load-bearing spot is thin in the digest, spawn a targeted deeper dive rather than reading it yourself. If a finding exposes a gap in the contracts, say so.

## 3. Gate — decide the route (you + the human)

**Before doing anything, decide which route this PR is on.** This is the bailout: a broken PR does not have one path.
- **(a) Ready** → merge. Zero blocking, follow-ups captured.
- **(b) Fixable** → the blocking findings are genuine, scoped code fixes: plan them, then §4.
- **(c) Wrong route** → **discard the PR and go upstream.** When a finding is a spec/test bug (an AC that can't hold, a test that contradicts the domain) or the whole approach is off (often an over-scoped issue), the fix is not in the code — amend the issue (a `## Factory Amend` comment), re-split it, or file a new issue, and throw the PR away. **Discarding + re-planning is a valid, valued outcome** — forcing a fix on a fundamentally-wrong PR under sunk-cost pressure is the trap this gate exists to prevent.

Routing is your + the human's call — **not automated**. A finding may be a code-fix, a spec-amend, or "this whole thing is wrong"; telling them apart is judgement.

## 4. Fix (delegate — route b only)

For each planned fix, spawn `pr-fix` (Sonnet) with a precise spec (what / where / why). It implements + verifies one fix and returns a compact result; you verify its small diff and integrate. Re-run §1 if a fix could ripple. Commit/push per the project's convention once the cycle is clean.

## 5. Present

```
**Route** — merge | fix (N blocking) | discard + re-plan
**Commits** — [pipeline assessment]
**Review-notes audit** — [which factory labels held, which you re-classified]
**[Area]** — [finding · which standard · what breaks if left]
...
**Verdict** — [0 blocking → merge / N blocking → fix / wrong-route → discard, and what to re-plan]
**Follow-ups** — [issues to file, or "none identified"]
```

Never report "good to merge" without the **Follow-ups** line — a clean merge with latent debt still owes the user the follow-up candidates. Never claim zero blocking without having (via the digest) mapped every AC to a correctness-asserting test and every changed line to the standards.
