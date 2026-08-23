# 4 · Contract docs

Two files are the factory's **boundary** — the rules it works within:

- **`CODING_STANDARDS.md`** — your conventions and blocking rules (naming, error handling, structure, "what gets a PR rejected").
- **`UBIQUITOUS_LANGUAGE.md`** — your domain terms with their specific meanings, and the aliases to avoid.

These are the *only* docs the factory pushes into **every agent's context**, on every step. They're how you get the factory to build *your* way instead of a generic way. Point `.themis/workflow.yaml`'s `docs` block at them.

## Keep them minimal and real

They are pushed into context on every run, so **a bloated, generic contract makes things worse** — more cost, and the agent anchored on rules that don't matter. Write a *lean, real* contract:

- A handful of conventions you'd actually reject a PR over — not a style-guide dump.
- The domain terms that carry specific meaning here — not a dictionary.

You can't write the perfect contract up front, and you shouldn't try. Start small.

## Grow them as you go

Add a rule or a term **when a review or a bad PR reveals the gap** — that's the signal that a convention is real and load-bearing. The contract ratchets up from experience, staying lean and true.

Themis's own [`CODING_STANDARDS.md`](../../CODING_STANDARDS.md) and [`UBIQUITOUS_LANGUAGE.md`](../../UBIQUITOUS_LANGUAGE.md) are working examples. (A helper skill to draft a first contract from a conversation, or extract candidates from an existing codebase, is planned.)

→ Next: [The build loop](05-build-loop.md)
