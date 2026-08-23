# Skills, Agents & Commands — needed vs optional

Themis ships three kinds of prompt asset alongside the Go binary: **skills** (`skills/`), **agents** (`agents/`), and **commands** (`commands/`). This is the map of what the v2 factory **needs to run** versus what's **optional tooling** around it — so you can tell at a glance.

## Core — needed to run the factory

### Autonomous run

Loaded into the sandbox container. The authoritative list is [`factory/manifest.txt`](factory/manifest.txt), enforced by `internal/runner/factory_assets_test.go`:

- **Skills:** `test-red` (TestRed orchestration), `pr-composition` (Ship PR body).
- **Agents:** `test-architect`, `test-writer`, `test-runner` (the TDD step), `security-reviewer` (the Review step).

The pipeline is the `themis` Go binary + `templates/`; it invokes only the manifest assets. The guard test keeps this honest — a template that delegates to an asset missing from the manifest fails the build, and every manifest entry must exist. So the core set is small and self-checking.

### Intake (interactive)

How you turn a concept into the `ready-for-agent` issues the factory consumes:

- **Skills:** `grill-me` (resolve a design), `split-walker` (decompose into vertical-slice issues), `issue-writer` (author the issue — ACs, `check`/`footprint`/`exports` directives).

Intake runs at the keyboard, not in the container, but it's core to *working with* the factory: the factory produces code; intake produces the specs it works from.

### Setup (interactive)

One-time, when a project first meets the factory:

- **Skills:** `contract-drafter` (draft the first minimal `CODING_STANDARDS.md` + `UBIQUITOUS_LANGUAGE.md` — the contract docs the factory pushes into every agent's context).

Optional to run, but the contracts it produces are core: the factory reads them on every step. Hand-writing them works too — the skill just turns the blank page into confirm/prune. See [`docs/getting-started/04-contracts.md`](docs/getting-started/04-contracts.md).

## Add-ons — optional tooling around the factory

Useful, not required to run an autonomous issue:

- **Review** (interactive review of factory PRs): `/review` · skills `pr-review`, `review-walker` · agents `architecture`/`complexity`/`convention`/`coverage`/`depth`/`numerical`-reviewer, `pr-diagnose`, `pr-fix`, `pr-composer`. (`security-reviewer` doubles here and in the autonomous Review step.)
- **Documentation** (per the doc strategy, #125 — root narrative + on-demand transition views): `/document`, `/context` · agents `doc-scanner`, `doc-writer`, `doc-updater`, `codebase-scanner`, `context-updater`.
- **Design-thinking** (general, project-agnostic; may move to the loom design projects later): skills `deepen`, `deepening`, `design-it-twice`.

## Archived out of the repo

Retrievable via git history:

- **v1 slash-command pipeline** → `../themis-v1-legacy/` — the pre-binary interactive pipeline (`/architect`, `/implement`, `/issue`, `/ship`, `/feature`, `/concept`, `/factory`, `interview`, `ac-drafter`, `plan-reader`, …), superseded by the Go binary.
- **Unrelated skills** → `../optional-skills/` — Renovate PR management and Hearth design-system skills, not part of the factory.
