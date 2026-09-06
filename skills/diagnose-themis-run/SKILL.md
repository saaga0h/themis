---
name: diagnose-themis-run
description: "Diagnose a failed or blocked Themis factory run from its telemetry: read the run's records, classify the failure, and say whether it's the user's problem (bad issue, config, git state) or a Themis defect — with an actionable fix. Use when a factory run blocked, was marked MANUAL/CONFIG, or produced a surprising result and telemetry is configured."
---

# Diagnose a Themis factory run

When a factory run blocks or misbehaves, the one-line block category (RE-RUN /
MANUAL / CONFIG) is triage, not root cause. This skill reads the run's telemetry
and explains, in the user's terms, **what happened and what to do** — and
crucially **whether it's the user's problem or Themis's**.

**Not private.** Running this skill puts the run's telemetry (which can include
code, paths, and error output) in front of the AI you're using. That is inherent
to AI-assisted debugging — tell the user so if it isn't obvious. Telemetry is only
available if the user set it up ([docs/telemetry.md](../../docs/telemetry.md)); if
it isn't, say so and fall back to reading the run log + the block message.

## 1. Find the telemetry endpoint and read the run

Telemetry is OTLP logs in the user's own store (the docs use VictoriaLogs). The
endpoint is in `{project}/.claude/settings.json` under `env` as
`OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` (the ingest URL, `…/insert/opentelemetry/v1/logs`).
Derive the **query** URL by replacing the ingest path with `/select/logsql/query`
on the same host:port, then query with LogsQL over HTTP.

The factory's own per-step records carry the instrumentation scope
**`scope.name:"themis/factory"`**; Claude Code's session telemetry is separate
(`scope.name:"com.anthropic.claude_code.events"`). Fields land flat.

```bash
# Derive the query base from the configured endpoint (host:port stays the same):
EP=$(python3 -c "import json;print(json.load(open('.claude/settings.json'))['env']['OTEL_EXPORTER_OTLP_LOGS_ENDPOINT'])")
BASE=$(printf '%s' "$EP" | sed -E 's#(https?://[^/]+).*#\1#')
# All factory step records for issue <N> in the last day, newest first:
curl -s "$BASE/select/logsql/query" \
  --data-urlencode 'query=_time:1d scope.name:"themis/factory" issue_number:<N>' \
  --data-urlencode 'limit=100'
```

Each record is one pipeline step, with these flat fields (present when relevant):
`stage` (Fetch/Scan/Branch/TestRed/Implement/Review/Docs/Ship), `outcome`,
`completed` (bool), `commits`, `duration_ms`, `green_gate`, `verify_output`,
`review_blocking`, `review_non_blocking`, `verdict`, `pr_url`, `detail`, plus
`run_id`. For agent-level context (tool counts, tokens, model) also read the
`com.anthropic.claude_code.events` records for the same time window.

*(Field names are how VictoriaLogs ingests OTLP attributes. On a different OTLP
backend the data is the same but the query syntax differs — this skill's queries
target the documented VictoriaLogs/LogsQL path.)*

## 2. Classify — signature → cause → fix

Match the run's records against these. Most failures are the **user's** to fix;
a genuine factory defect routes to the `themis-bug-report` skill.

- **Base mismatch** *(user/env)* — Implement hits the turn limit with ~0 commits;
  or a footprint check fails listing files that are actually the *base's* (from an
  unpushed local base commit), or `detail`/`verify_output` shows `current branch is
  the base branch`. **Fix:** push the base branch; launch the run from the default
  branch, not a leftover `issue/*` branch.
- **Footprint / gate false-block** *(usually user)* — `green_gate` fail with
  `verify_output` containing `footprint: files outside declared packages`. **Fix:**
  the issue's `footprint` is too narrow, or a manifest/lockfile isn't in
  `footprint_exempt`. Widen the footprint or add the exempt.
- **Check-block structural miss** *(user)* — tests pass but `green_gate` fails on a
  ```` ```check ```` command (visible in `verify_output`). **Fix:** the
  implementation didn't satisfy a declared structural AC; the retry usually fixes
  it, else the check is too strict/idiom-brittle (see issue-writer check-quality).
- **Convergence thrash** *(hard/flaky test)* — many test runs, timeout/panic in
  `verify_output`, edits but never green across attempts. **Fix:** the test is
  flaky or the issue too hard for one pass; make the test deterministic or split.
- **Scope ceiling** *(issue too big)* — Implement exhausts attempts across multiple
  components, `completed=false`, productive-but-unfinished. **Fix:** the issue is
  over-scoped; decompose it (split-walker).
- **Unsatisfied dependency** *(ordering)* — an issue run before its dependency
  merged: over-reach or an e2e that can't set up its precondition. **Fix:** merge
  the dependency first; the loop skips issues with open declared dependencies.
- **Dirty-tree checkpoint** *(env)* — a step commits but the tree stays dirty
  (a lockfile the toolchain rewrote, uncommitted). **Fix:** commit/ignore it.
- **Infeasible spec** *(design)* — the approach can't work as written (e.g. a test
  tool that cannot run the app). **Fix:** reshape the issue's approach.
- **Config / infra** *(user)* — identity/token/network in the block `detail`
  (CONFIG category). **Fix:** the named credential/endpoint.

## 3. Report

Give the user: which stage failed, the signature you matched, whether it's **their
problem or Themis's**, and the concrete fix. If it's their problem, stop there. If
the evidence points at a **Themis defect** (the factory itself behaved wrong, not
the issue/config/git), offer the `themis-bug-report` skill to produce a shareable
report — and remind them the report is theirs to review before sharing.

Anchor explanations to stable surfaces (the telemetry field names above, the
pipeline stage names, the block-category contract), not internal implementation.
