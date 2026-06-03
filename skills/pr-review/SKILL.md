---
name: pr-review
description: "When the user says a PR is ready for review (e.g. \\\"#XX is ready for review\\\"), follow this process. Use whenever auditing a factory-generated PR's review notes, checking a commit pipeline, or deciding whether a PR is good to merge."
---
 
# PR Review
 
The factory ships PRs with self-classified review notes. The job is not to redo its review — it is to **audit that classification** against the project contracts, and to surface the technical debt it waved through. A merge verdict is never the whole output: every review ends with explicit follow-up candidates.
 
Contracts: `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md`. Read both. Every finding is checked against them.
 
## 1. Read
 
Via the Gitea/GitHub MCP: PR metadata (title, body, labels, mergeable), commit history, full diff, and the PR's own review notes.
 
## 2. Verify the commit pipeline
 
Stages must appear in order; flag missing or reordered ones. A `fix` with no preceding `test` is suspect.
 
```
test(<scope>):     add failing tests for issue #N
feat(<scope>):     implement issue #N — <title>
refactor(<scope>): clean up implementation         (if needed)
fix(<scope>):      resolve review findings cycle N  (if needed)
docs(<scope>):     update documentation             (if docs changed)
```
 
## 3. Audit the review notes
 
The factory labels its own findings blocking / non-blocking. **Treat every label as a claim, not a decision.** For each note:
 
- Re-derive it against the contracts. A "non-blocking" note that is actually a contract violation is blocking — that is what the fix-commit stage exists for.
- A "non-blocking" note that is a real-but-minor improvement is a **follow-up candidate**, not a dismissal. Capture it for §5.
Then check what the notes *omit* — the factory rarely flags its own gaps. Every AC in the issue must map to a test that asserts correctness, not mere existence (not "no error", not a bare count). Every new error path needs a test that triggers it. Missing coverage is blocking.
 
## 4. Classify findings
 
**Blocking** (fix before merge) — any contract violation (hardcoded infra, swallowed errors, unthreaded context, aliased terminology in code/comments/commits); any AC without a correctness-asserting test; any untested error path; any security issue (credentials in code, unbounded reads, missing input validation). Contract violations are *always* blocking — never downgrade one for convenience.
 
**Follow-up** (file an issue) — improvements beyond the contract floor; patterns that would kill a class of future bugs; and **pre-existing debt the PR touched or exposed**. Pre-existing is not a free pass: the moment the PR brushes a file with a latent violation, that debt is visible, so propose the issue now rather than letting "the problem was already there" close the conversation. The standards are the floor, not the ceiling — if something small could be better, raise it as "minor, but worth tracking" and let the user decide.
 
**Observation** (mention only) — style not in the standards; merely-different alternatives; things a known future issue already owns.
 
If a finding exposes a gap in the contracts themselves, say so — the documents should evolve from what reviews surface.
 
## 5. Present
 
```
**Commits** — [pipeline assessment]
**Review-notes audit** — [which factory labels held, which you re-classified]
**[Area]** — [finding: what's wrong · which standard · what breaks if left]
...
**Verdict** — [N blocking → fix cycle / 0 blocking → good to merge]
**Follow-ups** — [issues to file, or "none identified"]
```
 
Never report "good to merge" without the **Follow-ups** line — a clean merge with latent debt still owes the user the follow-up candidates. Do not claim zero blocking findings without having mapped every AC to a test and every changed line to the standards.
 
Then wait. The user may accept, dispute (accept a sound reason; note if the contract should change instead), promote an observation, or override a blocking finding (note the override, proceed).
 
## 6. After merge
 
On the user's confirmation: file any agreed follow-up issues, then label the next queued issue `ready-for-agent` (ask which if not obvious).