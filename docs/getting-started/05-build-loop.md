# 5 · The build loop

This is the loop you run over and over: **idea → design → slices → issues → factory → PR.** Your work is the first four; the factory does the fifth.

## Resolve the design — `grill-me`

Start fuzzy, end decided. The `grill-me` skill interviews you until the design is resolved — the approach, what changes, and (for any move/rename/delete) *what must cease to exist*. This matters because **the factory's output quality is bounded by how precisely you specify** — resolve ambiguity here, not in a half-wrong PR.

## Cut vertical slices — `split-walker`

`split-walker` decomposes the resolved design into **vertical slices** (see [the model](00-the-model.md)) — each an independently testable behavior, each small enough to sit comfortably inside the factory's reliable envelope. If a slice can't be cut vertically (it's irreducibly horizontal), that's a signal it's *your* work, not the factory's.

## Author the issue — `issue-writer`

Each slice becomes an issue. A good factory issue has:

- **Precise acceptance criteria** — every AC must answer "what does the test assert?" Not "is tested", but "returns X for input Y".
- **A `footprint`** — the packages the change may touch, so a stray out-of-scope edit is caught. Declare it **permissively** (list every plausible home) or leave it to the implementer — an over-narrow footprint false-blocks correct work.
- **`check` blocks** for negative/structural ACs — "X no longer exists" isn't provable by a normal test, so declare the shell check that proves it (e.g. `! grep -rn 'type OldThing' ./...`).

`issue-writer` encodes these conventions. Then **label the issue `ready-for-agent`** — that's the signal that it's ready for the factory.

## Run the factory

`themis run` (all ready issues) or `themis issue <n>` (one). The factory runs a deterministic pipeline — **TestRed** (write failing tests from the ACs) → **Implement** (make them pass) → **Review** → **Docs** → **Ship** (open the PR). "Done" means your **green gate passes**: your `verify` commands *plus* the issue's declared `check`/`footprint` blocks for that run.

If a run blocks, it says why (a re-run, a config fix, or something a human must look at) — the categories are in the run output and telemetry.

→ Next: [Review & aftercare](06-review-and-aftercare.md)
