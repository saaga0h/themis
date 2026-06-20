# Documentation Content Plan

<!-- @tier: 1 -->
<!-- @source: docs/ tree -->

Every documentation file in the repository, with its tier, a one-line
description, and grep-able tags. Commands resolve which docs are relevant to a
scope by matching tags here, then load only those paths.

| Path | Tier | Description | Tags |
|------|------|-------------|------|
| README.md | root | Project overview, philosophy, factory/container usage, command and agent catalog | overview,setup,onboarding,factory,workflow,commands,agents |
| CONCEPTS.md | root | Why the design works — deterministic shell, checkpoint contract, structured-result seam, bounded autonomy | concepts,design,philosophy,rationale,seam,autonomy |
| ARCHITECTURE.md | root | Component inventory, package roles, data flows, Go module structure | architecture,system,components,packages,structure |
| UBIQUITOUS_LANGUAGE.md | root | Canonical terminology (pipeline, step, runner, invoker, fetcher, checkpoint) | terminology,language,glossary,naming |
| CODING_STANDARDS.md | root | Go style, dependency rules, testing conventions, commit conventions, reviewer checklist | standards,style,conventions,testing,commits,lint |
| docs/development.md | 1 | Build/test/lint targets, env vars, commands, troubleshooting, secrets | dev,config,env,build,commands,make,troubleshooting,secrets |
| docs/datamodel.md | 1 | Persisted artifacts: state.json, review-results.json, profile.yaml schemas | datamodel,state,schema,json,yaml,persistence,review-results |
| docs/content-plan.md | 1 | This file — documentation index and tag map | docs,content-plan,index,tags |
| docs/subsystems/themis/README.md | 2 | `cmd/themis` CLI — subcommands (version/issue/run), flags, build | themis,cli,cmd,subcommands,issue,run,provider |
| docs/subsystems/runner/README.md | 2 | Pipeline orchestrator — Run loop, blocking, cycle limits, Ship guards, template args | runner,orchestration,pipeline,blocking,ship,review-cycle,templates |
| docs/subsystems/pipeline/README.md | 2 | Deterministic state machine — ten steps, Advance transitions, round-3 gate, test-fix limit | pipeline,state-machine,steps,advance,transitions,review-cycle,round3 |
| docs/subsystems/checkpoint/README.md | 2 | Per-step verification — commit prefix and clean-working-tree checks | checkpoint,verification,commit,prefix,working-tree |
| docs/subsystems/tracker/README.md | 2 | Issue tracker abstraction — Fetcher, GitHub/Gitea impls, AC parser, list-issue types | tracker,issue,fetcher,github,gitea,acceptance-criteria,checkboxes |
| docs/subsystems/agent/README.md | 2 | Agent invocation seam — Invoker interface, ClaudeCodeInvoker, OTEL tagging, commit detection | agent,invoker,claude,seam,otel,subprocess,model |
| docs/subsystems/profile/README.md | 2 | Per-project config — `.themis/profile.yaml` schema, defaults, validation | profile,config,yaml,model-assignment,defaults,validation |
| docs/subsystems/git/README.md | 2 | Context-aware git helpers and Gitea config inference from origin remote | git,subprocess,branch,commit,remote,gitea-infer,merge-base |
| docs/subsystems/prompt/README.md | 2 | `{{KEY}}` template substitution with bidirectional validation | prompt,template,substitution,placeholder,validation |
| docs/subsystems/review/README.md | 2 | Review-results domain types, blocking threshold, pure analysis functions, `ReadReviewResults` I/O | review,findings,blocking,severity,review-results,threshold |

## Notes

- **Root reference docs** (`UBIQUITOUS_LANGUAGE.md`, `CODING_STANDARDS.md`) sit
  beside the canonical root docs (README, CONCEPTS, ARCHITECTURE) and are loaded
  by the runner into agent prompts as `{{UBIQUITOUS_LANGUAGE}}` and
  `{{CODING_STANDARDS}}`.
- **No Tier 1 `api-reference.md` or `messaging.md`** — Themis exposes no
  HTTP/gRPC/GraphQL surface and uses no message broker. It is a CLI plus internal
  Go library.
- **No Tier 3 module docs** — the high-incident-surface behaviors (review-cycle
  gate, blocking determination, the ship-contested-work branch) are documented in
  the Tier 2 `runner` and `pipeline` READMEs and in `CONCEPTS.md`; isolating them
  at Tier 3 would duplicate.
- **`templates/`** (seven per-step prompt files) are documented as the source of
  the `prompt` subsystem rather than as standalone docs.
