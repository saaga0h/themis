# 0 · The model

Before anything else, understand what Themis is and what your job becomes — the setup steps only make sense on top of this.

## The role inversion

With Themis, **you don't write code — you specify it and judge it.**

1. You resolve an idea into a precise, decided design.
2. You cut it into **vertical slices** and write each as an issue with acceptance criteria.
3. You label the issue `ready-for-agent`.
4. The factory — the `themis` binary driving Claude Code inside a sandbox — builds it **test-first** and opens a pull request.
5. You **review** the PR and merge (or send it back).

Your leverage is entirely **upstream**, in how precisely you specify the work. A vague issue produces a confident, polished, wrong PR. Themis is a tool for people willing to think; it is not click-and-run magic.

## What a vertical slice is

A **vertical slice** is *one end-to-end testable behavior* — something you can write a real test for.

- ✅ "`themis init` writes `.themis/workflow.yaml`, non-destructively" — a behavior with a test.
- ✅ "Un-export the internal-only `ACTargets` type" — a precise, checkable change.
- ❌ "Add a database layer" — horizontal; the only 'test' is "the table exists".
- ❌ "Build the todo app" — huge; no single test defines done.

The factory succeeds on well-specified vertical slices and **thrashes** on width. Slicing well is the single most important skill (see [the build loop](05-build-loop.md)).

## What Themis is *not* for

- **Horizontal scaffolding** — a project's bare skeleton (dirs, build files, a hello-world) is horizontal work. Do it yourself (see [a green baseline](03-project-baseline.md)).
- **Fuzzy specs** — resolve the design first (that's what `grill-me` is for).
- **Huge scope** — decompose until each slice is small and precise.

## The three dimensions you work in

Themis writes the code — it doesn't do the thinking. That stays with you, and it
lives in three places along the flow.

- **Resolving the design** — with `grill-me` (see [the build loop](05-build-loop.md)).
  Before anything is sliced or built, the idea has to become a *decided* design — the
  questions answered, the approach settled. `grill-me` interrogates the plan until the
  fuzziness is gone. Resolve it here, or the factory builds confidently on sand — this
  is where the work starts.
- **Guiding the implementation** — with your [contract docs](04-contracts.md).
  `CODING_STANDARDS.md` sits *between* the two ends: it doesn't specify a feature or
  judge a result, it directs *how* the factory implements. It's a small, living guide —
  grown gradually, kept focused, trimmed as often as it's added to, never "done."
  Knowing what belongs in it is a skill you build by watching the factory work. (It's
  why starting nearly empty is right.)
- **Judging the result** — the PR (see [review & aftercare](06-review-and-aftercare.md)).
  The code is already there; your job is to judge whether it does what was asked *and*
  fits the architecture intended, and to keep successive PRs aligned into one coherent
  whole. For that moment, you're the lead developer. Expect a PR larger than the small
  commits you may be used to — a slice lands as one whole behavior — and how large it
  is traces straight back to how you sliced it.

## One dependency; everything else is yours

Themis needs **Claude**. Beyond that it is agnostic — language, conventions, and host are all declared by *your* project, never baked into the tool. The factory enforces *your* rules (your [contract docs](04-contracts.md), your [workflow.yaml](02-configure.md)), not its opinions.

→ Next: [Install & run](01-install-and-run.md)
