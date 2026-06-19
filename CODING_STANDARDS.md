# Coding Standards — Themis

This file is loaded by the factory agent during code review.
All implementation must conform to these standards before a PR is considered complete.

Reference `UBIQUITOUS_LANGUAGE.md` for canonical terminology — terminology violations
in code, comments, or commit messages are review failures.

---

## Terminology

- Use terms from `UBIQUITOUS_LANGUAGE.md` exactly in code identifiers, comments, and commit messages
- Do not use aliased terms listed in the "Aliases to avoid" column
- `Runner` not `Executor` or `Processor` for pipeline execution
- `Pipeline` not `Workflow` — workflows were considered and rejected; the pipeline is fixed in code
- `Step` not `Stage` or `Phase`
- `Checkpoint` not `Gate` or `Guard` for step-completion verification
- `Template` not `Prompt` when referring to the parameterized file — `Prompt` is the rendered output
- `Profile` not `Config` or `Settings` for per-project YAML configuration
- `Ship` not `Deploy` or `Release` or `Publish` for PR creation
- `Invoker` not `Client` or `Caller` for the agent abstraction boundary

---

## Go Standards

### Style
- Follow standard Go conventions: `gofmt`, `goimports`
- Use `PascalCase` for exported identifiers, `camelCase` for unexported
- Error strings are lowercase and do not end with punctuation (Go convention)
- Prefer named return values only when they add clarity — not by default
- No `init()` functions unless absolutely necessary and justified in a comment

### Project layout
```
cmd/themis/           # Binary entry point (version, issue, run subcommands)
internal/
  agent/              # Invoker interface and ClaudeCodeInvoker
  checkpoint/         # Step completion verification (commit prefix, clean tree)
  git/                # Context-aware git subprocess helpers
  pipeline/           # Pipeline state machine, step definitions, state persistence
  profile/            # Per-project YAML configuration schema and loader
  prompt/             # {{KEY}} placeholder substitution for template rendering
  runner/             # Pipeline orchestration: loads profile, fetches issue, invokes agents
  tracker/            # Fetcher interface and implementations for GitHub/Gitea
agents/               # Review agent prompt definitions (markdown with YAML front matter)
templates/            # Per-step prompt templates with {{KEY}} placeholders
skills/               # Reusable skill prompts composed into agents and commands
commands/             # Slash command prompts for Claude Code sessions
```

### Dependency injection pattern

Internal packages fall into two categories:

**Domain type packages** provide types and functions the runner orchestrates:
`pipeline` (step types, state machine), `prompt` (template rendering), `tracker`
(issue data). These are imported directly by packages that need their types.

**Infrastructure operation packages** provide capabilities that touch external
systems: `git` (subprocess calls), `profile` (filesystem reads), `agent` (subprocess
spawning). These are NOT imported directly by consuming packages. Instead, their
operations are injected via interfaces or function fields on `Config` structs.

The rule: **import domain types, inject infrastructure operations.**

- **Grouped operations** (3+ related operations from one domain) → define a
  consumer-side interface. Example: git operations (branch, push, checkout,
  commit count, current branch, commit log) → a `GitOps` interface defined in
  `internal/runner/`.
- **Single operations** → inject as a function type field on `Config`. Example:
  `CheckpointFn func(ctx, dir, step) error` on `runner.Config`.
- **The Invoker interface** is the boundary between deterministic orchestration and
  LLM creative work. It is a domain boundary, not an infrastructure injection —
  importing the `Invoker` type is correct.

Concrete implementations live in `cmd/themis/`, which wires them into `Config`
at construction time. Tests substitute stubs or fakes for every injected dependency.

### Dependency direction

- `cmd/themis/` depends on `internal/` — never the reverse. `cmd/themis/` is
  the wiring layer: it imports concrete implementations from infrastructure
  packages (`git`, `agent`, `profile`) and injects them into `runner.Config`.
- `internal/agent/` has **no dependencies** on other internal packages. Commit
  snapshotting and other git operations are injected via function fields on
  `InvokeOptions`. The agent package is a pure subprocess spawner.
- `internal/checkpoint/` depends on `internal/git/` and `internal/pipeline/` —
  checkpoint needs git operations and step types. This is an accepted
  cross-internal dependency.
- `internal/git/` has no dependencies on other internal packages (leaf package)
- `internal/pipeline/` has no dependencies on other internal packages (leaf package,
  stdlib only: `errors`, `fmt`, `time`, `encoding/json`, `os`, `path/filepath`)
- `internal/profile/` has no dependencies on other internal packages (leaf package)
- `internal/prompt/` has no dependencies on other internal packages (leaf package)
- `internal/runner/` imports domain type packages: `internal/pipeline/`,
  `internal/prompt/`, `internal/tracker/`. It does NOT import infrastructure
  packages (`git`, `agent`, `profile`) directly — those capabilities are injected
  via interfaces and function fields on `runner.Config`.
- `internal/tracker/` has no dependencies on other internal packages (leaf package)

New cross-internal dependencies are a **blocking review finding**. If a package
needs a capability from an infrastructure package, inject it. If it needs a type
from a domain package, check whether the import is listed above. Unlisted imports
require justification and a CODING_STANDARDS update.

### Interfaces
- Define interfaces at the point of use (consumer), not the point of implementation
- Keep interfaces small — prefer single-method interfaces where possible
- For grouped infrastructure operations, define an interface at the consumer:
  the interface lists what the consumer needs, not everything the provider offers
- `Invoker` is the abstraction boundary between deterministic pipeline control and
  LLM creative work — nothing in `internal/runner/` should know about Claude Code,
  subprocess flags, or model names
- `Fetcher` is the abstraction boundary for issue tracker access — nothing in
  `internal/runner/` should know about Gitea REST APIs or the `gh` CLI
- `IssueWriter` is the abstraction boundary for issue tracker mutations (labels,
  comments, PR creation) — the runner calls the interface, `cmd/themis/` provides the
  implementation
- `CheckpointFn` is a function type, not an interface — injectable via `Config`
  so the runner can be tested without a real git repository. Use function types
  for single operations; use interfaces for grouped operations (3+)

### Error handling
- Never ignore errors — every `err` must be checked
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`
- Return errors to callers — do not log and swallow
- Fatal errors (startup failures) may call `fmt.Fprintf(os.Stderr, ...)` followed
  by `os.Exit(1)` — only in `cmd/themis/main.go`
- HTTP clients must have a `Timeout` set — `&http.Client{}` with no timeout is a
  blocking review finding (liveness risk under network partition)
- When a loop processes multiple items and one fails, log the failure and continue —
  do not abort the loop unless the failure makes all subsequent items impossible

### Context
- Every function that does I/O, blocks, or calls external systems must accept
  `context.Context` as its first parameter
- Respect context cancellation — check `ctx.Done()` in loops and long operations
- Never store a context in a struct

### Testing
- Unit tests must not require Gitea, GitHub, Claude Code, or any external process —
  use interfaces and hand-written stubs
- **Stubs over mocks** — write purpose-built stub structs (e.g., `fakeInvoker`,
  `stubQuerier`) rather than using code-generated mocks. Stubs are explicit about
  what they simulate and readable without framework knowledge
- Git helper tests (`internal/git/`) are the exception: they create temporary
  repositories and run real git commands — this is intentional (no mocking git)
- Test function names follow `TestFunctionName_ConditionDescription` pattern
- Table-driven tests for functions with multiple input/output combinations
- `go test -race ./...` must pass — no data races
- **Every AC in the issue must have a corresponding test that asserts correctness** —
  not mere existence ("no error"), not a bare count, but a check that the right thing
  happened. A test that calls the function and only checks `err == nil` is incomplete
- Every new error path must have a test that triggers it. If a function returns an
  error under a specific condition, a test must exercise that condition and verify
  the error
- **Test file naming**: test files are named after the code they test
  (`runner_test.go`, `pipeline_test.go`), not after the issue that created them.
  **Never create `<package>_issue<N>_test.go` files.** When an issue adds tests,
  they go into the existing test file for that package, alongside related tests.
  If a package's test file grows large enough to warrant splitting, split by
  behaviour area (`runner_review_test.go`, `runner_ship_test.go`), not by issue
  number. This is a **blocking review finding** — per-issue test files are rejected.
- **Test helpers are shared, not duplicated.** A package has one set of stub types
  and helper functions used across all its tests. Do not create per-issue copies
  of the same stub (`issue22StubFetcher`, `issue48StubFetcher`). If an existing
  stub doesn't support a new test's needs, extend the existing stub — do not
  clone it with a different prefix.

### Configuration and hardcoded values
- **No hardcoded hostnames, URLs, ports, or infrastructure-specific values in source
  code.** Every infrastructure-specific value must be configurable via environment
  variable or constructor parameter.
- Hardcoded infrastructure values are a **blocking review finding**.
- Default values are acceptable ONLY when they are:
  - Branch names with clear semantic meaning (e.g., `"main"` as fallback base branch)
  - Well-known protocol defaults
  - Documented in the relevant subsystem README
- The correct pattern: accept as a constructor parameter, fall back to an environment
  variable, fall back to a sensible default. Document the variable in the subsystem README.

### Security
- No credentials, tokens, or API keys in source code or committed files
- `GITEA_TOKEN` and `GITHUB_TOKEN` are read from the environment — never from files
  in the repository

---

## Commit Conventions

### Pipeline shape

Every factory-processed issue produces commits in this order. Stages must appear
in sequence; a `fix` with no preceding `test` is suspect.

```
test(<scope>):     add failing tests for issue #N
feat(<scope>):     implement issue #N — <title>
refactor(<scope>): clean up implementation               (if needed)
fix(<scope>):      resolve review findings cycle M        (if needed, repeatable)
docs(<scope>):     update documentation                   (if docs changed)
```

### Rules
- Scope is the package or subsystem touched (e.g., `cmd`, `runner`, `tracker`, `git`)
- Use comma-separated scopes when a commit spans packages: `feat(tracker,runner):`
- The issue number appears in the first `test` and `feat` commit messages
- Fix commits reference the review cycle number: `fix(<scope>): resolve review findings cycle 2`
- Refactor and docs commits are optional — only required when meaningful changes were made
- The Ship step does not produce a commit — it creates the PR

### What the reviewer checks
- Pipeline order is maintained (test before feat, feat before refactor, etc.)
- No `fix` commit without a preceding `test` commit
- Scope tags are consistent across commits in the same PR
- Issue number is referenced in test and feat commits

---

## Prompt Templates

### Format
- Templates live in `templates/` as markdown files
- Placeholders use `{{KEY}}` format — uppercase letters, digits, underscores only
- `internal/prompt.Substitute` errors on unmatched placeholders in either direction:
  a key without a placeholder or a placeholder without a key
- The runner filters placeholder args to only those present in the template before
  calling `Substitute` — this prevents spurious errors from runtime-only keys

### Runtime placeholders
| Placeholder | Source | Available in |
|-------------|--------|-------------|
| `{{ISSUE_NUMBER}}` | Issue metadata | All templates |
| `{{ISSUE_TITLE}}` | Issue metadata | All templates |
| `{{ACCEPTANCE_CRITERIA}}` | Parsed AC checkboxes | Agent-step templates |
| `{{AC_STATUS}}` | Same as ACCEPTANCE_CRITERIA | `ship.md` only |
| `{{REVIEW_OUTPUT}}` | Last Review step stdout | `ship.md` |
| `{{BLOCKING_FINDINGS}}` | Blocking findings from `.themis/review-results.json`, formatted as severity-ordered human-readable list (critical → high → medium, low omitted) | `fix-findings.md` |
| `{{REVIEW_CYCLE}}` | Current review cycle number (1-indexed) | `fix-findings.md` |
| `{{PIPELINE_SHAPE}}` | Distinct commit prefixes | `ship.md` |
| `{{COMMIT_LOG}}` | Branch commit log | `ship.md` |
| `{{CHANGED_FILES}}` | Files changed on branch | Templates that use it |

---

## Architecture

### The deterministic/creative boundary

The fundamental architectural property of Themis v2.0: **deterministic orchestration
in Go, creative work in the LLM agent.**

- The Go binary (`cmd/themis`) handles: step sequencing, cycle counting, checkpoint
  verification, state persistence, branch management, PR creation, turn budgeting
- The LLM agent handles: writing tests, implementing code, reviewing code, composing
  PR descriptions
- The `Invoker` interface is the boundary. Nothing in the Go code reasons about code
  quality, test strategy, or implementation approach. Nothing in the agent prompts
  manages pipeline state, cycle limits, or step transitions.

Do not move orchestration logic into prompts. Do not move creative judgments into Go code.

### What belongs where
- Pipeline state transitions: only in `internal/pipeline/`
- Git subprocess calls: only via `internal/git/` — never raw `exec.Command("git", ...)`
  from other packages
- Issue tracker API calls: only in `internal/tracker/` (for reads) and `cmd/themis/`
  (for writes via `IssueWriter`)
- Template rendering: only via `internal/prompt/`
- Agent invocation: only via `internal/agent/` through the `Invoker` interface
- Profile loading: only via `internal/profile/`
- Production wiring (connecting interfaces to implementations): only in `cmd/themis/`

### Ignorance is load-bearing

Each internal package's inability to see the others is a design property.
`internal/pipeline/` does not know about git. `internal/agent/` does not know about
issues. `internal/tracker/` does not know about pipelines. If a package needs
information from another layer, that information flows through `cmd/themis/`'s wiring
or through the `runner.Config` struct — not through a direct import.

---

## Review Classification

### Blocking (fix before merge)
- Any terminology violation against `UBIQUITOUS_LANGUAGE.md` in code, comments, or commits
- Any AC without a correctness-asserting test
- Any untested error path
- Any hardcoded infrastructure value
- Any `&http.Client{}` without a `Timeout`
- Any swallowed error (checked but not returned or logged)
- Any unthreaded context (I/O function without `context.Context` parameter)
- Any credentials in source code
- Any cross-internal dependency not listed in the dependency direction section above
- Any per-issue test file (`<package>_issue<N>_test.go`)
- Any duplicated test stub that clones an existing stub with a different prefix
- Any direct import of an infrastructure package (`git`, `agent`, `profile`) from
  a package other than `cmd/themis/` or `internal/checkpoint/` — use interface
  injection instead

Contract violations are always blocking — never downgrade one for convenience.

### Follow-up (file an issue)
- Improvements beyond the contract floor
- Patterns that would prevent a class of future bugs
- Pre-existing debt the PR touched or exposed — pre-existing is not a free pass:
  the moment the PR brushes a file with a latent violation, that debt is visible,
  so propose the issue now

### Observation (mention only)
- Style not covered by these standards
- Merely-different alternatives
- Things a known future issue already owns

---

## Review Checklist

The reviewer must verify all of the following before approving:

- [ ] All terms match `UBIQUITOUS_LANGUAGE.md` — no aliased terms in code, comments, or commits
- [ ] Commit pipeline follows the required order (test → feat → refactor → fix → docs)
- [ ] Every acceptance criterion in the issue has a corresponding correctness-asserting test
- [ ] Every new error path has a test that triggers it
- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes
- [ ] No credentials or API keys in any committed file
- [ ] No hardcoded infrastructure values — all must be configurable
- [ ] No ignored or swallowed errors
- [ ] Context is threaded through all I/O-performing functions
- [ ] HTTP clients have a `Timeout` set
- [ ] Interfaces are used at abstraction boundaries — no concrete type leakage across packages
- [ ] No direct imports of infrastructure packages outside `cmd/themis/` — use injection
- [ ] No new cross-internal dependencies beyond those listed in the dependency direction section
- [ ] Pipeline step templates use only documented `{{KEY}}` placeholders
- [ ] PR review notes document all non-blocking findings — sparse notes on a non-trivial diff are suspect
- [ ] Test files are named by behaviour, not by issue number — no `_issue<N>_test.go` files
- [ ] Test stubs are shared — no per-issue copies of the same stub type
