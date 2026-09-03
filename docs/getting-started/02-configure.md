# 2 · Configure a project

Every project the factory works on needs one file: **`.themis/workflow.yaml`** — the *green gate*. It tells the stack-agnostic factory how to prove your code is correct. Themis hardcodes no language; this file is where *your* project declares its stack and its checks.

## Scaffold it

From your project root:

```bash
themis init          # writes .themis/workflow.yaml (skips if it exists; --force to overwrite)
```

This drops a **stack-neutral skeleton** you fill in. It bakes in no language — the examples are just comments.

## Fill it in

```yaml
stack: "unconfigured"          # informational label: go, rust, node, python, ...

verify:
  # The Green Gate: shell commands run in order via `bash -c`; each must exit 0
  # for a run to ship. Replace this with YOUR project's real build/lint/test.
  # The default below fails on purpose so an unconfigured project can't green-gate.
  - "echo 'themis: configure verify' && exit 1"

docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md
```

- **`verify`** is the heart of it — the commands that decide "done". A Go project might use `go build ./...`, `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test ./...`; a Node project `npm run build`, `npm run lint`, `npm test`; anything else, whatever proves *your* code. Until you replace the default, the gate fails deliberately — the factory won't run on an unconfigured project.
- **`docs`** points at your [contract docs](04-contracts.md) — the authoritative standards the review reads.

Themis's own [`.themis/workflow.yaml`](../../.themis/workflow.yaml) is a worked example.

## The sandbox image

`themis init` scaffolds the `workflow.yaml`; the container image (your toolchain + Claude Code) is set up separately for now — Themis's [`Containerfile`](../../Containerfile) is a Go-based starting point to adapt to your stack. (A generated starter `Containerfile` is a planned `themis init` follow-up.)

→ Next: [A green baseline](03-project-baseline.md)
