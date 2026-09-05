# Themis

Themis is an **autonomous software factory**. You author precise, vertically-sliced issues; Themis drives Claude Code through a deterministic, test-first pipeline **inside a sandboxed container** and opens a pull request. You review and merge. You don't write the code — you specify it and judge it.

Themis is a **meta-tool for programming**. Its one hard dependency is Claude. It is **agnostic**:

- **Language** — written in Go, but it builds anything: C, Rust, Julia, TypeScript, Fortran, …
- **Conventions** — what "good" means is *yours*, declared per-project in your contract docs. Themis enforces *your* rules, not its opinions.
- **Host** — GitHub or a self-managed Gitea.

It works best on **well-specified vertical slices** — one end-to-end testable behavior. It is *not* for horizontal scaffolding, fuzzy specs, or huge scope; those stay with you.

## Getting started

New to Themis? Walk these in order — each is short:

0. [The model](docs/getting-started/00-the-model.md) — what Themis is, your role, and what a *vertical slice* is
1. [Install](docs/getting-started/01-install-and-run.md) — build the binary, prerequisites, the sandbox
2. [Configure a project](docs/getting-started/02-configure.md) — `themis init` and `.themis/workflow.yaml`
3. [A green baseline](docs/getting-started/03-project-baseline.md) — get your project to a floor the factory can build on
4. [Contract docs](docs/getting-started/04-contracts.md) — the boundary the factory works within
5. [The build loop](docs/getting-started/05-build-loop.md) — from an idea to a merged PR
6. [Review & aftercare](docs/getting-started/06-review-and-aftercare.md) — `/review`, `/document`

## Reference

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — how the pipeline is built
- [`CONCEPTS.md`](CONCEPTS.md) — why it's designed this way
- [`SKILLS.md`](SKILLS.md) — the skills/agents/commands: what's needed to run the factory vs optional add-ons
- [`docs/configuration-reference.md`](docs/configuration-reference.md) — every `themis init` flag, `workflow.yaml` field, and Containerfile stanza; how to configure a language without a preset
- [`docs/providers.md`](docs/providers.md) — GitHub vs Gitea: the provider contract and per-host setup
- [`docs/development.md`](docs/development.md) — build/test/run reference, environment variables, troubleshooting
- [`CODING_STANDARDS.md`](CODING_STANDARDS.md), [`UBIQUITOUS_LANGUAGE.md`](UBIQUITOUS_LANGUAGE.md) — Themis's own contract docs (and a model for yours)

## Status

Pre-beta. Dogfooded on itself (Go, via Gitea). GitHub support exists but is not yet proven end-to-end, and the packaged run-against-your-own-project flow is being finalized — see [`docs/beta-readiness.md`](docs/beta-readiness.md).
