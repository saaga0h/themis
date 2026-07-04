---
name: pr-review
description: "When the user says a PR is ready for review (e.g. \"#XX is ready for review\"), follow this process to audit a factory-generated PR and decide its route: merge, fix, or discard-and-replan. Use whenever auditing a factory PR's review notes, checking the commit pipeline, or deciding whether a PR is good to merge."
---

# PR Review

The factory ships PRs with self-classified review notes. Your job is not to redo its review — it is to **audit that classification** against the project contracts, surface the debt it waved through, and **decide the route**. A merge verdict is never the whole output.

## How you work — orchestrate, delegate when it pays

You hold the **judgement**. The reading can happen in your own context (**direct**) or in cheaper delegated contexts (**delegate**) — and which one is a cost decision you make first (§0), not a default. Delegation keeps your context lean and puts token-heavy reading on cheaper models, but it carries fixed overhead: each subagent re-reads base context, and summaries round-trip back. That overhead only pays off on large PRs; on small ones, reading directly is cheaper *and* higher-fidelity.

The delegated toolset (used only when §0 routes to delegate):
- **`codebase-scanner` (Haiku)** — compact repo maps (symbol use, package layout, file contents); never dumps.
- **`pr-diagnose` (Sonnet)** — the evidence digest: per concern source / claim / location / what-it-does / evidence / candidate-fix. It locates and summarizes; **it does not render verdicts** — that's yours.
- **`pr-fix` (Sonnet)** — implements one scoped fix from a precise spec; returns a compact result.

Contracts: `CODING_STANDARDS.md`, `UBIQUITOUS_LANGUAGE.md` — every finding is checked against them; you judge against them.

## 0. Size gate — direct or delegate? (do this first, cheaply)

Decide the review's mode **before ingesting any raw content** — once you've read the diff and files to decide, the cost is already sunk and there's nothing left to save. Gate on metadata only:

1. **Free:** pull the diff stat (`pull_request_read` → `get_files`) — file count, ±lines, package spread, file kinds. No content.
2. **Route:**
   - **Direct** — localized: one/few packages, ≲6 files *and* ≲~500 changed lines, or test/docs-only. **You** read the diff, touched files, and contracts and diagnose in your own context. This is most PRs.
   - **Delegate** — broad: many packages, ≳~800 changed lines, or several large files — a surface big enough that carrying it across the whole review would crowd your context or hit the window. Use the agents in §1/§4.
   - **Scout first** — small diff, unknown blast radius (a few lines that need reading half the codebase to judge): spend **one** `codebase-scanner` (Haiku) pass to size the impl surface + call sites the review must read, then pick.
3. **Budget:** the gate spends at most one metadata call + one Haiku scan. Never read raw content into your context just to decide.

Rule of thumb: delegation's fixed overhead is only repaid when you'd otherwise carry **~80k+ tokens of raw material across the review** — every turn re-reads your context, and that amplification is the real cost, not the one-time read. Below that, direct wins. On a mid-size PR when unsure, prefer direct; delegate when breadth would crowd your judgement, not merely to offload reading.

## 1. Diagnose

Produce the evidence digest per the §0 route — **direct** (you read the diff, touched files, and contracts in your own context) or **delegated** (spawn `pr-diagnose` Sonnet, reusing `codebase-scanner` Haiku for repo reads). Either way it must cover: **pipeline order** (`test → feat → (refactor) → (fix) → (docs)`; a `fix` with no preceding `test` is suspect); each **factory Review Note** re-located in the code and stated as what it *actually* is (every factory label is a claim to verify, not a decision); **AC → test mapping** (each AC's *correctness*-asserting test — not existence, not "no error", not a bare count; flag any AC without one and any untested error path); **omissions** the factory didn't flag; and **contract checks** (hardcoded infra, swallowed errors, unthreaded context, aliased terminology, credentials, unbounded reads, missing validation).

A sparse digest on a non-trivial diff is itself suspect — re-diagnose rather than accept "nothing found."

## 2. Judge (you + the human)

Over the evidence — the digest on a delegated review, or what you read on a direct one — classify each concern against the contracts:
- **Blocking** — any contract violation (always blocking, never downgrade for convenience); any AC without a correctness-asserting test; any untested error path; any security issue.
- **Follow-up** — improvements beyond the contract floor; pre-existing debt the PR touched or exposed (the moment the PR brushes a latent violation it's visible — propose the issue now, don't let "already there" close it).
- **Observation** — style not in the standards; different-but-fine alternatives; things a known issue already owns.

If a load-bearing spot is thin, dig deeper — a targeted scan on a delegated review, or read it yourself on a direct one. If a finding exposes a gap in the contracts, say so.

## 3. Gate — decide the route (you + the human)

**Before doing anything, decide which route this PR is on.** This is the bailout: a broken PR does not have one path.
- **(a) Ready** → merge. Zero blocking, follow-ups captured.
- **(b) Fixable** → the blocking findings are genuine, scoped code fixes: plan them, then §4.
- **(c) Wrong route** → **discard the PR and go upstream.** When a finding is a spec/test bug (an AC that can't hold, a test that contradicts the domain) or the whole approach is off (often an over-scoped issue), the fix is not in the code — amend the issue (a `## Factory Amend` comment), re-split it, or file a new issue, and throw the PR away. **Discarding + re-planning is a valid, valued outcome** — forcing a fix on a fundamentally-wrong PR under sunk-cost pressure is the trap this gate exists to prevent.

Routing is your + the human's call — **not automated**. A finding may be a code-fix, a spec-amend, or "this whole thing is wrong"; telling them apart is judgement.

## 4. Fix (route b only)

For each planned fix: on a **direct** review make the minimal fix yourself; on a **delegated** review spawn `pr-fix` (Sonnet) with a precise spec (what / where / why) — it implements + verifies one fix and returns a compact result you verify and integrate. Re-run §1 if a fix could ripple. Commit/push per the project's convention once the cycle is clean.

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
