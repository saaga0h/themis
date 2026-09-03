# Ubiquitous Language — Themis

This document defines the canonical terminology used across all Themis code, issues,
skills, agents, and documentation. When in doubt, use these terms exactly. Precision
here prevents ambiguity in agent-implemented code.

---

## The Product

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Themis** | A software factory system: AI agent definitions, slash command prompts, reusable skills, and a compiled Go binary that orchestrate autonomous implementation pipelines over a codebase. Themis processes issues into tested, reviewed, documented pull requests | "CI", "automation", "bot" |
| **Factory** | The overall system that picks up labeled issues and processes them autonomously through the pipeline. In v1, the factory is the `/factory` command. In v2.0, the factory is the `themis run` subcommand | "CI pipeline", "build system" |

---

## Pipeline and Execution

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Pipeline** | The fixed sequence of steps that transforms an issue into a PR: Fetch → Scan → Branch → TestRed → Implement → Review → Docs → Ship. Encoded in Go code in `internal/pipeline/`. The pipeline is deterministic — step transitions are computed, not prompted. The step enum retains `Refactor` and `Fix` slots for state-file/resume compatibility, but the linear pipeline no longer reaches them — Review is single-pass and never routes to a fix | "workflow" (rejected — workflows imply configurable DAGs; the pipeline is a fixed sequence), "process" |
| **Step** | A single stage in the pipeline. Each step has a name, a required commit prefix (or none), and a transition rule. Steps are the unit of state persistence — the pipeline resumes at the last completed step | "stage", "phase", "task" |
| **Runner** | The orchestrator that executes the pipeline for a single issue. `runner.Run` in `internal/runner/` loads or resumes state, advances through steps, invokes agents for creative steps, and enforces limits. The runner is deterministic — it does not make creative decisions | "executor", "processor", "handler" |
| **State** | The persisted pipeline state for a single issue run. Stored as `.themis/state.json`. Contains the current step, test-fix and implement attempt counts, issue number, and code version. Enables crash recovery — a killed run resumes at the last completed step | "progress", "status" |
| **Checkpoint** | Verification that an agent step completed successfully. Checks that the working tree is clean and that the expected conventional-commit prefix exists on the latest commit. Implemented in `internal/checkpoint/` | "gate", "guard", "validation" |
| **Loop** | The `themis run` outer loop that processes multiple issues sequentially. Lists `ready-for-agent` issues, sorts by number, runs the pipeline for each, logs failures and continues. Distinct from the pipeline — the loop manages issue selection, the pipeline manages step execution | "batch", "queue" |

---

## Issue Processing

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Issue** | A Gitea or GitHub issue with acceptance criteria. The atomic work unit for the factory. An issue becomes a single PR. Issues are labeled `ready-for-agent` when ready for autonomous processing | "ticket", "task", "story" |
| **Acceptance Criterion (AC)** | A testable criterion in an issue body, written as a markdown checkbox (`- [ ]`). Each AC becomes one or more tests. The factory implements exactly what the ACs say — nothing more | "requirement", "user story", "spec" |
| **Fetcher** | The interface for retrieving issue data from a tracker. `tracker.Fetcher` with `Fetch(ctx, number) (*IssueData, error)`. Implementations exist for GitHub (`gh` CLI) and Gitea (REST API) | "loader", "reader", "client" |
| **IssueQuerier** | The interface for listing ready issues and checking issue state. Used by the loop (`themis run`) to discover work. Distinct from Fetcher — the querier lists and filters, the fetcher retrieves full issue data for a known number | "issue lister", "scanner" |
| **IssueWriter** | The interface for mutating issue state: adding/removing labels, posting comments, creating PRs. Defined in `internal/runner/` as a consumer interface, implemented in `cmd/themis/` | "issue client", "tracker client" |
| **IssueData** | The parsed representation of an issue: `Number`, `Title`, `Body`, `Labels`, `URL`, `Ref`. Owned by `internal/tracker/`. The `Ref` field carries the target branch from the issue metadata (used as PR base branch) | "issue model", "issue struct" |
| **Tracker** | The package (`internal/tracker/`) that owns issue-tracker integration. Contains Fetcher, IssueData, and AC checkbox parsing | "issue service", "ticket system" |

---

## Agent and Prompts

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Agent** | In `agents/`: a markdown file with YAML front matter defining a specialised subagent (model, tools, prompt). Invoked via `Task` delegation during pipeline runs. In code: the entity that does creative work (writes tests, implements code, reviews) — always on the other side of the `Invoker` boundary | "LLM", "model", "AI" (in code identifiers) |
| **Invoker** | The interface between deterministic pipeline control and LLM creative work. `agent.Invoker` with `Invoke(ctx, InvokeOptions) (*InvokeResult, error)`. `ClaudeCodeInvoker` is the production implementation. Tests use `fakeInvoker` | "caller", "client", "executor" |
| **Template** | A markdown file in `templates/` with `{{KEY}}` placeholders. One per pipeline step. The template is the parameterised instruction; the rendered output (after substitution) is the prompt sent to the agent | "prompt" (when referring to the file) |
| **Prompt** | The rendered output of a template after `{{KEY}}` substitution. What the agent actually receives. Templates are authored; prompts are computed at runtime | "template" (when referring to the rendered output) |
| **Profile** | Per-project YAML configuration at `.themis/profile.yaml`. Controls per-step model assignments, test-fix attempt limits, and optional step enablement. Loaded by `internal/profile/` with sensible defaults when absent | "config", "settings", "preferences" |
| **Skill** | A reusable prompt fragment in `skills/<name>/SKILL.md`. Composed into agents and commands. Skills provide domain expertise (pr-review, issue-writer, deepening) without pipeline awareness | "plugin", "extension", "module" |
| **Command** | A slash command prompt in `commands/<name>.md`. Executed directly by the user in a Claude Code session. Commands are user-facing entry points; the factory pipeline is one command among many | "action", "task" |

---

## Review

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Review Step** | The factory pipeline's single-pass safety gate (`templates/review.md`): one security-reviewer delegation plus an inline AC-coverage check, writing findings to `.themis/review-results.json` (severity `critical`/`high`/`low`; never `medium`). It runs **once**, edits nothing, never loops, and never routes to a fix — findings inform the PR verdict at Ship, not control flow | "review battery" (that is the interactive tier) |
| **Review Battery** | The full set of specialised review agents (architecture, convention, coverage, complexity, depth, security) available to the **interactive** `/review` command and human PR review. The autonomous factory does **not** run the battery — its Review Step is the single-pass gate above | "review pipeline", "review chain" |
| **Finding** | Something a reviewer identified in the code. Every finding is classified as blocking, non-blocking, or observation. The PR body's Review Notes section must document all non-blocking findings | "issue" (ambiguous — means a tracker issue), "comment" |
| **Blocking Finding** | A finding that must be resolved before merge. Contract violations (CODING_STANDARDS.md), an AC with no test, swallowed errors, hardcoded infrastructure. In the factory these are the `critical`/`high` findings in `review-results.json`; they gate the PR verdict at Ship — the autonomous pipeline does **not** auto-fix them (there is no Fix step), so resolution is the human's at PR review | "critical", "P0" |
| **Non-blocking Finding** | A finding that is real but can be deferred. Becomes a follow-up candidate. The factory labels its own findings blocking/non-blocking — the reviewer audits these labels as claims, not decisions | "minor", "nice-to-have" |
| **Follow-up** | A non-blocking finding worth tracking as a future issue. Includes improvements beyond the contract floor and pre-existing debt the PR exposed. Captured in the review's Follow-ups section | "TODO", "tech debt" (too vague) |
| **Observation** | A finding that is style-only, merely-different, or already owned by a known future issue. Mentioned but not actionable | "nit" |
| **Review Notes** | The section of a PR body where the factory documents every non-blocking finding it waved through. Sparse review notes on a non-trivial diff are suspect — the factory found nothing, or it under-reported | "review comments" |
| **Fix Cycle** / **Review Cycle** | **v1 constructs, removed in v2.** The factory no longer loops the Review Step or runs a Fix step (the `Refactor`/`Fix` enum slots persist only for state-file/resume compatibility). Blocking findings gate the PR verdict rather than triggering an automated fix round; iterative fixing is the human/interactive tier's job | "review round", "iteration" |

---

## Code Delivery

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Ship** | The final pipeline step. Pushes the issue branch, invokes the PR composition agent, and creates the PR via IssueWriter. Ship does not produce a commit — it produces a PR. The Ship step has pre-push guards: it refuses to create a PR if the current branch equals the base branch or if no commits exist on the branch | "deploy", "release", "publish", "merge" |
| **PR** | A pull request created by the Ship step. Contains the AC verification table, pipeline shape, and review notes composed by the pr-composer agent | "merge request" |
| **Branch** | The issue branch created during the Branch step. Named `issue/<N>-<slugified-title>`. The base branch comes from `IssueData.Ref` (falling back to `"main"`) | "feature branch" |
| **Commit Pipeline** | The ordered sequence of conventional commits produced by a factory run: `test → feat → refactor → fix → docs`. Each commit has a scope and references the issue number. The commit pipeline is the reviewable evidence of the factory's work | "commit history", "git log" |

---

## External Systems

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Gitea** | Self-hosted Git forge. Provides the issue tracker API and Git hosting. Themis accesses Gitea via REST API for issue fetching, issue querying, label management, and PR creation. Connection details are inferred from the `origin` git remote (overridable via `GITEA_OWNER`, `GITEA_REPO`, `GITEA_API_URL`). `GITEA_TOKEN` is always from the environment | "GitHub" (different system), "git server" |
| **GitHub** | Cloud Git forge. Themis accesses GitHub via the `gh` CLI for issue fetching and querying. Used when `--provider github` is specified | "Gitea" (different system) |
| **Claude Code** | The LLM agent runtime. Invoked via `claude --print --dangerously-skip-permissions` with prompts on stdin. The `ClaudeCodeInvoker` wraps this as a subprocess. Claude Code does the creative work; the Go binary does the orchestration | "Claude", "the LLM", "the model" (in code — use Invoker) |

---

## Design Tooling

| Term | Definition | Aliases to avoid |
|------|------------|-----------------|
| **Grill-me** | A skill that interviews the user about a plan or design until reaching shared understanding. The input to split-walker | "interview", "Q&A" |
| **Split-walker** | A skill that decomposes a resolved design into vertical slices (issues). Each slice becomes an issue via issue-writer. The factory's issue-creation pipeline | "decomposer", "issue generator" |
| **Review-walker** | A skill that walks through a code review report, triages findings by severity, and creates issues for follow-ups | "review processor" |
| **Deepening** | A framework for evaluating and improving module depth — seam placement, adapter strategy, dependency taxonomy. Used by deepen and design-it-twice skills | "refactoring" (too broad) |
| **Intent Doc** | A formalised intent document captured from a webUI conversation. The bridge between conversational design and the factory pipeline. Created by the intent-doc skill, consumed by intent-bridge → architect → factory | "spec", "requirements doc" |
| **Hearth** | The design system. Themis's compose-prototype skill builds prototypes against Hearth tokens. Hearth is external to Themis but referenced in UI-related workflows | "design tokens", "style guide" |

---

## Labels

| Label | Meaning |
|-------|---------|
| `ready-for-agent` | The factory picks this up for autonomous processing |
| `needs-review` | PR is ready for human review |
| `blocked` | Waiting on a dependency or human attention |
| `foundation` | Core infrastructure or foundational work |

---

## Relationships

- The **Human** writes **Issues** with **ACs**, or uses **Grill-me** → **Split-walker** to generate them
- The **Factory** (**Loop**) picks up `ready-for-agent` **Issues** and runs the **Pipeline** for each
- The **Runner** executes the **Pipeline**: **Fetch** → **Scan** → **Branch** → creative **Steps** → **Ship**
- Creative **Steps** render **Templates** into **Prompts** and pass them to the **Invoker**
- The **Invoker** delegates to **Claude Code**, which does the creative work
- After each creative **Step**, the **Checkpoint** verifies the **Agent** committed correctly
- The single-pass **Review Step** evaluates the code and records **Findings** to `review-results.json`
- **Blocking Findings** gate the PR verdict at Ship (no automated fix round); **Non-blocking Findings** become **Follow-ups**
- The **Ship** step creates a **PR** with **Review Notes** documenting all findings
- The **Human** reviews the PR using the **pr-review** skill, auditing finding classifications
- After merge, agreed **Follow-ups** become new **Issues**

---

## Flagged Ambiguities

- **"Agent"** — overloaded. In `agents/`, an agent is a markdown prompt file defining a
  specialised subagent. In the architecture, "the agent" means the LLM doing creative work
  on the other side of the Invoker boundary. In Moira, agents are deployed entities with
  credentials. Qualify when crossing project boundaries.
- **"Config"** — overloaded. `runner.Config` is the struct that wires production dependencies
  into the runner. `Profile` is the per-project YAML configuration. `.env` is runtime
  environment configuration. Do not use "config" unqualified — say which one.
- **"Template" vs "Prompt"** — the file in `templates/` is a template. After `{{KEY}}`
  substitution, the rendered text is a prompt. The template is authored and committed;
  the prompt is computed at runtime and ephemeral.
- **"Issue" vs "Finding"** — an issue is a work unit in the tracker (Gitea/GitHub). A finding
  is something a reviewer identified in code. Findings may become issues (as follow-ups),
  but they are different things at different lifecycle stages.
- **"Pipeline" vs "Commit Pipeline"** — the pipeline is the state machine (Fetch through Ship).
  The commit pipeline is the ordered sequence of conventional commits the pipeline produces.
  The pipeline has 8 reachable steps (Fetch through Ship, past the retained Refactor/Fix slots); the commit pipeline typically has 2–3 commits (test, feat, and docs when docs changed).
- **"Factory" vs "Runner" vs "Loop"** — the factory is the overall system. The loop (`themis run`)
  selects and sequences issues. The runner (`runner.Run`) executes the pipeline for a single
  issue. Factory ⊃ Loop ⊃ Runner.
- **"Checkpoint" vs "State"** — a checkpoint is a verification (did the agent commit correctly?).
  State is the persisted pipeline progress. Checkpoints verify; state persists. A checkpoint
  failure does not corrupt state — the pipeline can retry the step.
- **"Skill" vs "Command" vs "Agent"** — a skill is a reusable prompt fragment (composed into
  other things). A command is a user-facing slash command. An agent is a specialised subagent
  definition. Skills are building blocks; commands are entry points; agents are delegates.
