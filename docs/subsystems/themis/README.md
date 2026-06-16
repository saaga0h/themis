# Themis CLI

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: cmd/themis/ -->

## Overview

`themis` is the CLI entry point for the Themis v2.0 deterministic orchestration system. It dispatches subcommands and exits non-zero on any error.

## Key Files & Entry Points

| File | Role |
|------|------|
| `cmd/themis/main.go` | `package main`; argument parsing and subcommand dispatch |
| `cmd/themis/main_test.go` | Integration tests that compile the binary and exec it |

## Architecture

All logic lives in `main()`. `os.Args[1]` selects the subcommand via a `switch` statement. No flags or external packages are used; stdlib only (`fmt`, `os`).

Unknown subcommands print `unknown command: <name>` to stderr and exit 1. Invocation with no arguments prints a usage line to stderr and exits 1.

## Subcommands

| Subcommand | Status | Behavior |
|------------|--------|----------|
| `version` | Implemented | Prints `0.1.0` to stdout, exits 0 |
| `run` | Planned (issue #7) | <!-- TODO: document when implemented --> |
| `issue` | Planned (issue #10) | <!-- TODO: document when implemented --> |

## Build, Test, and Lint

```
make build   # go build ./cmd/themis/
make test    # go test ./...
make lint    # go vet ./...
```

Module: `git.home.federation.fi/lavernea/themis`, Go 1.22.

## Dependencies

None beyond the Go standard library.

## Related Documents

- `ARCHITECTURE.md` — system-level design
- `docs/subsystems/` — other subsystem READMEs
