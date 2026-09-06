# Beta readiness — notes for a first packable Themis

Working notes from a planning discussion (2026-08-15). Not a spec — a captured
direction to come back to. The premise: **the core is beta-quality; the gap is the
"usable by someone who isn't us" layer** (packaging, onboarding, docs, telemetry +
diagnosis), not the pipeline.

## Framing

- The v2 pipeline works and is deterministic; the gate tier (check-blocks, footprint,
  export budgets, AC-coverage, per-issue-test lint, destructive-AC meta-check) is real
  and dogfood-validated. Delivery (prebuilt binary + podman sandbox) and telemetry work.
- What's missing is tractable engineering (packaging, setup, container build, telemetry
  collection) — *not* a non-functioning core. That's the whole point: the frontier is
  "make the existing factory legible and survivable for a stranger," a more finishable
  kind of work than "build more factory."

## GitHub is the minimum viable provider — and must be *proven*

- People host on GitHub. Self-hosted Gitea is too high a bar for a beta tester.
  (Codeberg — a free public Gitea — is a bridge: the *proven* Gitea path without
  self-hosting. But GitHub must work.)
- GitHub is the **least-exercised** path — all dogfooding to date is Go + Gitea. Concrete
  unknowns to verify before any volunteer:
  - GitHub issues carry no `ref` field → base resolution falls to the originating-branch
    fallback (recently added, **untested on GitHub**).
  - `gh` CLI auth is separate from the Claude token.
  - PR creation via `gh pr create` (`ghIssueWriter.CreatePR`) is unexercised recently.
- **Step one of any beta: a GitHub dogfood** — run the factory on a throwaway GitHub repo
  end-to-end. Bounded, de-risks the whole beta.

## The volunteer-on-own-project strategy

Ask a handful of Claude-using people to run Themis on their **own side projects**. Why
this is strong, beyond recruitment:

- **It inverts the unknown.** The scary beta unknowns were (a) their stack, (b) their
  repo, (c) the factory. On their own code, *they* hold (a) and (b) — they know the
  build/test commands, can write a sensible issue, and can judge whether the output is
  right. The factory only has to prove *itself*, against code the human understands.
- **It's the engine that closes gaps we can't close ourselves:** each volunteer's stack
  is free cross-stack coverage; their environment surfaces robustness surprises we'd never
  hit; their issue-writing is the real test of whether the authoring skills work for a
  stranger; and it feeds the scope-ceiling calibration (#87) with real diverse data.

## Telemetry-on + a diagnosis skill = the survivability tool (make-or-break)

A volunteer's first run *will* hit something (ours did, repeatedly). Retention hinges on
that first failure being **survivable**.

- The categorized block diagnosis (RE-RUN / MANUAL / CONFIG + one action line) is
  **triage** — enough to say "retry" vs "a human must look" without any infra. Necessary
  but not sufficient: it doesn't give *root cause*, and a stranger doesn't know how Themis
  thinks, so "MANUAL: inspect the branch" is a dead end for them.
- So for beta: **telemetry ON**, plus a **`diagnose-themis-run` skill** that reads it. The
  AI is the right interface *because the user doesn't need to understand the logs* — the
  skill encodes "what to look for and why." (We've effectively been the prototype of this
  skill in every "pull Loki and diagnose" during dogfooding — the expertise is
  extractable, and the accumulated real runs are its training set.)

### What the diagnosis skill must encode (seed taxonomy from dogfood experience)

Failure classes, each with a distinct telemetry signature:

- **Base mismatch** — turn-limit, ~0 commits, *few* tool calls, whole-repo/history greps
  (the agent can't find its code). *(e.g. the loop-hardcoded-`main` bug.)*
- **Convergence thrash** (flaky/hard test) — many `go test` runs + timeout/panic keywords,
  edits but never green. *(the MQTT-reconnect class.)*
- **Footprint / gate false-block** — `gate=fail` with the footprint/export check named +
  `verify_output` listing out-of-scope files → check `$BASE` / the declared footprint.
- **Scope ceiling** — Implement exhausts attempts across *multiple* components,
  productive-but-unfinished.
- **Dirty-tree checkpoint** — a step commits but leaves the tree dirty (e.g. a `go.sum`
  the toolchain rewrote and the agent didn't commit) → checkpoint fails.
- **Config / infra** — identity/token/network in the block detail (CONFIG category).

Plus the **Loki query patterns** (`{job="claude-code-factory", issue_number="N"}` → step
records with `green_gate`/`verify_output`/`commits`/`outcome`; tool-counts per step; the
categorized block detail), and **Themis's load-bearing assumptions a stranger won't have**
(TDD-shaped: TestRed writes the spec, green gate = done; gates are issue-*declared*; the
base branch matters; "sandbox git reality" is a whole failure family). The skill turns a
signature into *why*, in the user's terms, then a fix — and reconciles with the
block-category the user already saw.

### Cautions

- **Drift:** the skill encodes assumptions — anchor it to *stable* surfaces (telemetry
  schema, pipeline stage names, the block-category contract), not volatile internals, or
  it rots like the doc drift we keep fixing. It *reads* run data — no need to materialize
  it into factory context (the #110 read/write-doc discipline).
- **Packaging dependency:** it needs telemetry reachable + a Loki query path (the loki MCP
  or a thin wrapper) shipped in the beta bundle. "Telemetry-on for beta" ⟹ bundle the
  collection stack + the query path + the skill.

## Minimum viable beta package (rough order)

1. **Prove GitHub** — the end-to-end GitHub dogfood. Non-negotiable.
2. **Package** — clean container build + setup script + telemetry stack (Vector/Loki or a
   lite equivalent) + the loki query path.
3. **`diagnose-themis-run` skill** — the survivability tool (author it by distilling the
   diagnosis pattern we keep applying to real runs).
4. **Accurate GitHub-first getting-started** — the current README teaches the *removed* v1
   slash-command flow; that front-door lie is the first doc to fix (#110 / #23).
5. **"Writing factory issues" guide** — the authoring discipline distilled: one component
   per issue, use footprint, mind the scope ceiling. Makes the *first* issue succeed.
6. **"When it blocks" troubleshooting page** — built on the block diagnosis + the skill.

## Trust + expectations to state loudly

- **Sandboxed and PR-gated** — nothing is auto-merged into their repo; the factory works
  in a podman sandbox and opens a PR for human review.
- **Cost** — each issue costs real Claude tokens (~$2–4 observed), so set expectations.

## Related issues

- #110 — documentation architecture (read/write doc discipline; the diagnosis skill fits here)
- #23 — v2 roadmap refresh (README still teaches v1)
- #58 — workflow-profile system (config scaffolding / `themis init`)
- #56 — crash-recovery robustness (resume-branch, dirty-tree, restart discipline)
- #88 / telemetry — the factory's own narrative to Loki (the diagnosis skill's data source)
- #87 — scope ceiling (volunteer runs feed the calibration)

---

## Assessment — 2026-09-05 (post-GitHub-dogfood)

A verified audit against the "Minimum viable beta package" list above, done after the
first full GitHub end-to-end dogfood (greenfield Node repo `saaga0h/todo`). Evidence is
from real runs and repo inspection, not memory.

**Headline:** the doc's premise — *"the core is beta-quality; the gap is the
usable-by-a-stranger layer"* — is now **evidenced, not asserted**. Step 1 (prove GitHub)
is done, and the base-resolution unknown it flagged turned out to be a real bug, now
fixed. The remaining blockers cluster exactly where the doc predicted: **first-failure
survivability** (telemetry bundle + diagnosis skill + troubleshooting).

### Scorecard

| MVP-beta item | Verdict | Evidence |
|---|---|---|
| 1. Prove GitHub e2e | ✅ **done** | Issues #2/#3/#4 → PRs #7/#8/#9; both ready + draft(blocking-finding) routes; dependency-DAG skip (#5/#6 correctly deferred on open #4); `gh pr create` + label swap (ready-for-agent→needs-review) working. The flagged base-resolution/originating-branch unknown had a real bug (`$BASE` vs unpushed local base; base==issue-branch) — fixed in `bbe6d9a`. |
| 2. Package (build + telemetry stack + query path) | ◑ **partial** | Build-from-source Containerfile + `themis init` scaffolding done. Telemetry **emission** exists (`cmd/themis/emitter_otel.go`) but is **env-gated** (`OTEL_EXPORTER_OTLP_ENDPOINT`) and the **sink is not bundled**; no query path shipped → the diagnosis data source is absent out-of-the-box. **Simplification (2026-09-05): use VictoriaLogs as the sink** — it ingests OTLP natively (no Vector) and exposes LogsQL for queries, so the bundle is a *single binary*, not a Loki+Vector stack. The emitter is OTLP-generic, so this needs **no code change** — just point `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` at VictoriaLogs (operator has validated VM-logs OTLP on another repo). |
| 3. `diagnose-themis-run` skill | ✗ **not done** | No such skill in `skills/` (only `pr-diagnose`, a different thing). The doc calls this *make-or-break for retention*. Seed taxonomy exists below; this session added real evidence (base-resolution class, footprint false-blocks, check-block retry). |
| 4. GitHub-first getting-started | ✅ **done** (1 stale line) | No v1 slash-command lies in README/getting-started/providers (rewritten this session: getting-started, providers.md, new configuration-reference.md). **But the README `## Status` line is stale** — still says "GitHub support… not yet proven end-to-end." |
| 5. Writing-issues guide | ✅ **mostly** | `getting-started/05-build-loop.md` teaches issue authoring (precise ACs, footprint, ready-for-agent) + the `issue-writer` skill (check-block-quality guidance added `3b55445`). |
| 6. "When it blocks" troubleshooting page | ✗ **not done** | No troubleshooting page in `docs/`. |

**Gaps beyond the doc's list, surfaced this session:**
- **Non-Node presets unproven e2e** — the `go`/`python`/`rust` presets (`internal/preset`) are unit-tested but never run through a real image build + factory cycle. A preset that doesn't actually build would bounce a volunteer.
- **Onboarding not cold-read by an external user; Windows host `init` untested** on real Windows.
- **Cost/trust not stated loudly** — sandbox/PR-gated is implied in `00-the-model.md`; the ~$2–4/issue cost expectation isn't prominent.

### Update (2026-09-06): a hidden safety gap, found and fixed

While validating telemetry, reading the run path turned up something the earlier
scorecard missed: **`themis run`/`themis issue` had no sandbox enforcement.** They
ran the pipeline in-process wherever themis ran — so on the host, the factory drove
Claude Code with `--dangerously-skip-permissions` **unsandboxed on the user's
machine**. The "host-mode launcher" was referenced in comments but never
implemented; the only containerizing path was `make factory`. That is a hard
NO-GO for any beta.

Fixed this session (commit `7819422`, #140): a host-mode launcher gates `run`/`issue`
— on the host it launches the sandbox container or **refuses**, never falling back
to in-process; sandbox detection is corroborated (marker `== "1"` AND an
independent container signal) so a spoofed/inherited env var can't disarm it;
validated by a zero-context adversarial audit. **Rebuild the host binary to make it
live.** Remaining cleanup: `make factory` still bind-mounts `factory-cc/.claude`
over the workspace (shadowing the project's `.claude`) — finish #139 (retire that
mount; the launcher supersedes it).

### Go / no-go

- **Supervised beta (a few volunteers we actively watch + diagnose for): GO.** Core, GitHub, and onboarding are proven; *we* are the diagnosis skill for a small, watched cohort. This is also how the diagnosis-skill training set grows.
- **Open / unsupervised beta: NO-GO** until the survivability cluster lands. A stranger's first failure *will* happen (ours did, repeatedly) and must be survivable without us — the doc's own make-or-break.

### Ranked remaining before unsupervised beta

1. **Telemetry config + docs** (completes item 2) — the sink is **VictoriaLogs**, user-run (native OTLP + LogsQL), *not* scaffolded: telemetry is a separate service. Config unit is `{project}/.claude/settings.json` `env` (opt-in per project). Filed as **#149** (factory honors that `env` — the one code slice) + **#150** (docs: self-hosted VM-logs examples + honest boundaries). No emitter change; Themis stays OTLP-agnostic.
2. **Author `diagnose-themis-run` skill** — **#151**, the make-or-break survivability tool (read telemetry → classify → your-problem-vs-Themis's + fix); seed taxonomy above + this session's real runs. Plus **#152** (optional `themis-bug-report`, the feedback loop). The diagnose skill also serves item 6 ("when it blocks"); a thin troubleshooting page can point at it later.
4. **Prove `go`/`python`/`rust` presets e2e** — build one image + run one issue per language.
5. **External cold-read of onboarding + Windows `init` check.**
6. **Polish:** refresh the stale README `## Status` line; state cost/trust loudly.
