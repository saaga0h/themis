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

## One dependency; everything else is yours

Themis needs **Claude**. Beyond that it is agnostic — language, conventions, and host are all declared by *your* project, never baked into the tool. The factory enforces *your* rules (your [contract docs](04-contracts.md), your [workflow.yaml](02-configure.md)), not its opinions.

→ Next: [Install & run](01-install-and-run.md)
