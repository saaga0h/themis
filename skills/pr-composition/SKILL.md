---
name: pr-composition
description: Compose a PR description that surfaces what the implementation and review found — status line, implementation narrative, pipeline shape, review findings, follow-ups. Used by the ship step of the factory pipeline.
---

# PR Composition

A skill for composing pull request descriptions that surface what the implementation and review found. The PR gives the reviewer enough information to make an informed merge decision without reading every line of code first.

## Philosophy

A PR description is the implementation's report back to the human. Its job is to surface what was found — not to prove work was done, and not to prescribe what to do about it. The human connects findings to the broader context, decides priorities, and determines next steps.

The review command is a nit-picker by design. That's a feature. The PR translates those findings into readable, honest reporting: what's there, what was surprising, what was deferred. The human decides what matters.

## Structure

### Status line

One line. Either "All ACs passed" or "N of M ACs passed — see below." If all passed, move on. If not, explain which failed and why — that's the most important information in the PR.

### Implementation narrative

What happened during implementation that's worth knowing. Not a changelog (the commits tell that story) but the things commits don't capture:

- Assumptions the issue didn't mention that the code revealed
- Parts of the codebase that were harder than expected and why
- Patterns encountered in existing code that affected the implementation
- Dependencies or interfaces that were surprising

Keep this short. Two to four sentences for a clean implementation. Longer only if the implementation fought the codebase — and then say what it fought and where.

### Pipeline shape

The commit pipeline tells a story. Report it as a signal:

- `test → feat → docs` — clean run, no review findings.
- `test → feat → refactor → fix → fix → docs` — two review fix cycles.
- `test → feat → fix → fix → fix → BLOCKED` — three fix attempts didn't resolve the problem.

One sentence mapping the pipeline shape to what it means.

### Review findings

This is the core of the PR. The review battery produces findings classified as blocking or non-blocking. The PR surfaces these honestly.

**For each finding, state:**
1. What was found (one sentence)
2. Where it is (file and line, or package)
3. How it was classified (blocking — fixed, or non-blocking — deferred)

**Group by what happened to them:**
- "Fixed during review cycle" — findings that were blocking and resolved. State what the fix was.
- "Deferred" — findings classified as non-blocking. State what they are and why they were deferred. Don't minimize them — the reviewer decides if the deferral is acceptable.
- "Observations" — things the reviewers noted that aren't findings per se but are worth knowing.

**For clean reviews (no findings):** Say so in one line. Don't inflate the section.

Do not hide findings. Do not editorialize about whether they matter — report them and let the reviewer judge. If a finding was deferred because it's out of scope, say that, but don't bury it.

### Out-of-scope discoveries

Things found during implementation that aren't in the issue's ACs but exist in the code:

- Bugs in adjacent code that new tests exposed
- Missing test coverage noticed in related modules
- Stale documentation that contradicts current code
- Dependency issues encountered

For each: what it is and where it is. Don't assess urgency or prescribe fixes — the reviewer has context the agent doesn't. Surface the finding; the human triages.

### Follow-ups

Things that were explicitly deferred during implementation or review. State what they are, where they are, and why they were deferred (out of scope, non-blocking classification, time constraint).

Don't prescribe solutions — the agent sees this issue through a keyhole and can't reason about the broader codebase or roadmap. Surface what was found with enough context for the human to decide what to do with it.

If there are no follow-ups, don't include this section.

## Avoid

- **AC recitation** — don't reproduce AC text from the issue. "All passed" or "AC 3 failed because..." is sufficient.
- **Prescribing solutions** — the agent sees one issue's changes. Don't suggest fixes for broader codebase problems or predict implications. Surface the finding; the human decides the approach.
- **Predicting broader implications** — "this will break when..." requires holistic context the agent doesn't have. State what was found. The human connects it to what they know.
- **Minimizing deferred findings** — "non-blocking" is a classification from the review step. Report it as-is. Don't add "and this is fine" — the reviewer decides if it's fine.
- **Hiding problems in long lists** — if one finding out of twelve looks concerning, put it where it's visible.
- **False confidence** — "all checks pass" when the review flagged 6 non-blocking items. Report both facts.
- **Changelog format** — "added file X, modified file Y" is what `git diff --stat` shows. Don't duplicate it.

## Input context

When composing a PR, you receive:
- Issue number and title
- Acceptance criteria (pass/fail status)
- Commit history (the pipeline shape)
- Review output (all findings from all reviewers, with classifications)
- Changed files list
- Any blocking findings that required fix cycles

Use all of these to compose the PR. Work with what's available — surface it clearly, honestly, and let the human decide what to do with it.
