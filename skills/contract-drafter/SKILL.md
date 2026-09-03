---
name: contract-drafter
description: "Draft a minimal, real first CODING_STANDARDS.md and UBIQUITOUS_LANGUAGE.md for a project — the contract docs the factory pushes into every agent's context. Interactive confirm/prune, not a template dump. Use when a project has no contract docs yet, or mentions \"contract docs\", \"coding standards\", \"ubiquitous language\"."
---

# Contract Drafter

You are helping a human produce their project's two **contract docs**:

- **`CODING_STANDARDS.md`** — the conventions and blocking rules the factory works within (naming, error handling, structure, "what gets a PR rejected").
- **`UBIQUITOUS_LANGUAGE.md`** — the domain terms that carry specific meaning here, and the aliases to avoid.

These are the *only* docs the factory pushes into **every agent's context, on every step**. They are how the human gets the factory to build *their* way. See `docs/getting-started/04-contracts.md` for the concept — this skill produces the first real version of it.

## The one principle: minimal and real

A blank page is impossible; a bloated, generic contract is *worse than nothing* — it inflates every agent's context and railroads the builder with rules that don't matter here. Your entire job is to turn blank-page authoring into **confirm/prune**, and to bias hard toward **cutting**.

The output is a **starting boundary**, not a finished style guide. It will be grown later — a rule or term is added when a review or a bad PR reveals the gap (that's `04-contracts.md`'s grow-as-you-go). So:

- Prefer 5 rules the human would actually reject a PR over, to 50 generic ones.
- Prefer the 8 domain terms that carry specific meaning, to a dictionary.
- When in doubt, **leave it out** and say so — an empty contract that grows is healthier than a bloated one that misleads.

## Stack-agnostic — never impose

Themis builds any language. **You do not carry in a language's conventions.** Do not seed the contract with generic best-practices, a framework's idioms, or "industry standard" rules the human never asked for. Every rule must come from *this project* — either the human stated it, or you extracted it from *their* code and they confirmed it. If you catch yourself writing a rule that would be true of any project in this language, cut it.

## Step 1 — detect mode

Look at the working directory.

- **Brownfield** (code already exists) → **extract**, then confirm/prune.
- **Greenfield** (little or no code) → **elicit** with a few sharp questions.

You may mix: extract what you can, ask about the rest. State which mode you're in before you start.

## Step 2 — draft `CODING_STANDARDS.md` (the rules)

### Brownfield: extract candidates

Scan the code (the `/document --full` muscle) for *observed, load-bearing* conventions — how errors are actually handled, how things are named, how the code is structured, what layering exists. Present each as a candidate the human confirms or cuts:

```
Candidate rule (observed in <file>): "Errors are wrapped with %w and a call-site prefix; never discarded."
Keep / cut / reword?
```

Extract only what's *consistent and intentional* in the code. If a pattern appears twice and contradicts itself, that's not a rule — ask which one.

### Greenfield: elicit

Ask a few sharp questions, one at a time (not a survey). Aim for the rules that carry weight:

- "Name one thing in a PR you'd reject on sight." → the highest-value rule.
- "How should errors/failures be handled here?" (propagate, wrap, log-and-continue?)
- "Any structural boundary that must hold?" (e.g. this layer never imports that one)
- "Any naming convention you care about?"

Stop when you have a handful of real rules. Do not pad to look complete.

### Output shape

A short markdown doc: a one-line purpose, then grouped rules stated as **checkable assertions** ("X, never Y"), each rejectable at review. No prose essays. The project's own existing standards doc (if the human has a model to point at) is a fine format reference — but content comes only from this project.

## Step 3 — draft `UBIQUITOUS_LANGUAGE.md` (the terms)

Keep this **distinct** from the rules doc — terms, not conventions.

### Brownfield: extract candidates

Pull the domain nouns that recur in type names, package names, and identifiers, especially ones with a *specific, non-obvious* meaning or a tempting-but-wrong synonym. Present each:

```
Candidate term: "Run" = one full pipeline execution over a single issue (NOT a CLI invocation, which may process many). Alias to avoid: "job".
Keep / cut / reword?
```

### Greenfield: elicit

- "What are the core nouns of this domain?"
- "For each — is there a word people wrongly use as a synonym?" (the alias-to-avoid is often the highest-value part)
- "Any word that means something *different* here than in general use?"

### Output shape

A short glossary: **Term — precise definition — aliases to avoid.** Only terms that carry specific meaning; skip anything a reader would guess correctly.

## Step 4 — confirm and write

Show the human both drafts together. Invite cuts before additions ("what here is noise?" before "what's missing?"). When they confirm, write `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md` to the project root, and remind them to point `.themis/workflow.yaml`'s `docs` block at them and to grow both as reviews reveal gaps.

## What not to do

- **Do not emit a template.** A doc full of `<placeholder>` rules the human never confirmed is exactly the bloat this skill exists to prevent.
- **Do not import generic best-practices** or a language's idioms. Content comes from this project only.
- **Do not merge the two docs.** Rules and terms are separate files with separate jobs.
- **Do not pad for completeness.** A short, true contract is the goal — "we deliberately kept this minimal" is a feature, not a gap.
- **Do not claim the contract is finished.** It's a starting boundary; say so, and point at grow-as-you-go.
