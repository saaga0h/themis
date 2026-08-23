# 6 · Review & aftercare

The factory opens a PR and hands it back to you. **The factory never merges — you do.** This is the judgment step; the factory did the building.

## Review the PR

When a run ships, it removes `ready-for-agent` from the issue and adds **`needs-review`** — the issue won't be re-processed unless someone re-adds `ready-for-agent`. Then review the PR:

- **`/review`** runs the specialized reviewer battery (correctness, conventions, coverage, security, architecture) over the diff, and — with a flag — an individual perspective.
- **`pr-review`** (skill) is the guided "a PR is ready" workflow: read the diff + review notes against the issue's ACs, and decide **merge / send back / fix**.

You're judging two things: does it meet the ACs, and does it hold up. If a review finding is actually a **spec** problem (the AC was wrong), fix the issue — the issue is the source of truth, not the PR.

- **Merge** → the issue closes.
- **Send back** → re-add `ready-for-agent` (optionally with an amend note) to re-run, or hand-finish a small fix on the branch.

## Aftercare — docs

Themis keeps **facts in the code**: the factory writes package and symbol doc comments as it builds, so `go doc` (or your language's equivalent) is the live reference. You don't maintain a separate docs tier.

- **`/document`** updates the **human-facing narrative** — README, ARCHITECTURE, CONCEPTS — deliberately, when you choose (not per-issue). With `--full` it generates deeper subsystem/structure views on demand (handy for onboarding or a brownfield conversion), from the code — never a maintained, drift-prone tier.
- **`/context`** keeps `CLAUDE.md` down to the failure-critical minimum.

See [`SKILLS.md`](../../SKILLS.md) for the full set of skills, agents, and commands — and which are core to running the factory vs optional add-ons.

That's the loop. Resolve → slice → author → run → review → merge, and repeat.
