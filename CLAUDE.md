# CLAUDE.md — Themis

Self-contained safety net for autonomous runs: the factory executes inside a
podman sandbox with `--dangerously-skip-permissions` and may not load the global
`~/.claude/CLAUDE.md`. These rules stay here even where they also exist globally —
do not "deduplicate" them away.

## Secrets — never read, print, or include
- Never read secret material: `.env`, `~/.vault-token`, `~/.ssh/`, `~/.gnupg/`,
  or any credential dotfile. The running process reads tokens (`GITEA_TOKEN`,
  `CLAUDE_CODE_OAUTH_TOKEN`) from the environment — you never need their values.
- Reference secrets by location only (env var name, Vault path), never by value.
- If a task seems to require a secret's value, stop and report — do not read it.

## Sandbox safety
- Dependency fetches go through the configured GOPROXY (Athens). Never add
  `,direct`, never set `GONOSUMCHECK`. Missing from cache → stop and report, don't bypass.
- No `curl | sh`; pin container images by digest; commit `go.sum`.
- Stay within the project root. No `rm -rf` outside it. Use git commands; never
  edit `.git/` internals directly.

## Committed output
- Never write local infrastructure values into committed files — IPs, hostnames,
  ports, DNS names, proxy/registry/Vault/Nomad/Consul addresses, WireGuard
  endpoints. Use role names ("the Go proxy") or placeholders. Source-code
  specifics and the blocking-finding policy live in `CODING_STANDARDS.md`.

## Project rules
- This repo lives on Gitea, but the `themis` binary defaults to `--provider github`
  (`cmd/themis/issue.go`, `run.go`). Always pass `--provider gitea` for `themis issue`,
  `themis run`, and any issue/PR operation.
- Only implement issues labeled `ready-for-agent`; stay within the issue's
  Acceptance Criteria — the issue is the spec.

## Docs
See `docs/development.md` for build/test commands, env vars, and troubleshooting.
Conventions are authoritative in `UBIQUITOUS_LANGUAGE.md` (terminology) and
`CODING_STANDARDS.md` (Go style, architecture rules, review checklist).
