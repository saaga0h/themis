# PR Composition

A skill for composing pull request descriptions that serve as decision documents. The PR tells the reviewer what the implementation revealed, what the review found, and what needs attention — so they can make an informed merge decision without reading every line of code first.

## Philosophy

A PR description is the implementation's report back to the human. Its job is to surface information that changes decisions — not to prove work was done. "All ACs passed" is one line. The rest of the PR is about risk, findings, and what comes next.

The review command is a nit-picker by design. That's a feature. The PR takes those findings and translates them into actionable intelligence: what's safe, what's concerning, what needs investigation, what's been deferred.

## Structure

### Status line

One line. Either "All ACs passed" or "N of M ACs passed — see below." If all passed, move on. If not, explain which failed and why — that's the most important information in the PR.

### Implementation narrative

What happened during implementation that's worth knowing. Not a changelog (the commits tell that story) but the things commits don't capture:

- Assumptions the issue didn't mention that the code revealed
- Parts of the codebase that were harder than expected and why
- Patterns discovered that affect other code (good or bad)
- Dependencies or interfaces that were surprising

Keep this short. Two to four sentences for a clean implementation. Longer only if the implementation fought the codebase.

### Pipeline shape

The commit pipeline tells a story. Report it as a signal, not a list:

- `test → feat → docs` — clean run, no review findings. Low-risk merge.
- `test → feat → refactor → fix → fix → docs` — two review fix cycles. Read the findings below.
- `test → feat → fix → fix → fix → BLOCKED` — three fix attempts failed. Something structural is wrong.

One sentence mapping the pipeline to a confidence level.

### Review findings

This is the core of the PR. The review battery produces findings classified as blocking or non-blocking. The PR translates these into decisions:

**For each finding that matters, answer:**
1. What was found (one sentence)
2. Why it matters beyond this PR — does this pattern exist elsewhere? Will the next issue that touches this code hit the same problem?
3. What the recommended action is: merge as-is, merge and file follow-up, or investigate before merging

**Group by action, not by reviewer:**
- "Merge — these are fine" (findings that were correctly non-blocking, no broader implications)
- "Merge and track" (findings that are safe for this PR but indicate something to address — file as follow-up issues)
- "Review before merging" (findings where the human should look at the specific code before deciding)

Do not list findings without connecting them to a decision. "Security reviewer found X" without "and here's what that means for you" is noise.

**For clean reviews (no findings):** Say so in one line. Don't inflate the section to look thorough.

### Out-of-scope discoveries

Things found during implementation that aren't in the issue's ACs but matter:

- Bugs in adjacent code that the new tests exposed
- Missing test coverage in related modules
- Stale documentation that contradicts current code
- Dependency issues (outdated versions, unused imports, version conflicts)

For each: what it is, where it is, and whether it's urgent or can wait. Don't bury important caveats — if something is load-bearing for the next issue, say so at the top.

### Follow-ups

Concrete next actions with enough context to file an issue from. Not "consider refactoring X" but "X uses pattern Y which breaks when Z — file issue to migrate to W before the next change to this package."

If there are no follow-ups, don't include this section.

## Avoid

- **AC recitation** — don't reproduce AC text from the issue. "All passed" or "AC 3 failed because..." is sufficient. The issue has the full list.
- **Review findings without decisions** — "non-blocking" is a classification, not an action. What should the reviewer DO with this information?
- **Hiding problems in long lists** — if finding #3 out of 12 is the one that matters, lead with it. Don't bury it.
- **False confidence** — "all checks pass" when the review battery flagged 6 non-blocking items means "all checks pass but here are 6 things to be aware of."
- **Changelog format** — "added file X, modified file Y, deleted file Z" is what `git diff --stat` shows. Don't duplicate it.
- **Apologetic hedging** — "this might not be ideal but..." — state what was done, what the trade-off was, and let the reviewer decide.

## Input context

When composing a PR, you receive:
- Issue number and title
- Acceptance criteria (pass/fail status)
- Commit history (the pipeline shape)
- Review output (all findings from all reviewers, with classifications)
- Changed files list
- Any blocking findings that required fix cycles

Use all of these to compose the PR. Don't ask for more context — work with what's available.
