# Telemetry (optional, recommended)

Telemetry is **optional**. It's **recommended** because it's how you — with the
[`diagnose-themis-run`](#diagnosing-a-run) skill — understand what a factory run
actually did when something goes sideways. You're combining two systems that are
each opaque from the outside (an AI and the factory); telemetry is the window into
the seam between them.

Themis emits **standard OTLP**. It is *not* coupled to any particular backend, and
Themis does **not** run or ship a collector. You run your own OTLP endpoint if you
want telemetry; nothing is sent anywhere otherwise.

> This page documents **one** working way to do it — a single-binary
> [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) container —
> because it needs no extra software (native OTLP ingest + a query language,
> LogsQL). Any OTLP backend works (Vector→Loki, an existing observability stack, …);
> point the endpoint wherever you like.

## What gets emitted

- **Claude Code's own telemetry** — the CC sessions the factory drives (and your
  interactive CC sessions, if configured), when `CLAUDE_CODE_ENABLE_TELEMETRY=1`.
- **Themis's factory narrative** — one structured record per pipeline step
  (stage, green-gate result, verify output, commits, outcome), which is the richest
  signal for diagnosing a blocked run.

Both are configured from the **same place** (below), so one setup captures
everything.

## 1. Run an OTLP endpoint (VictoriaLogs)

Run it as a **separate, long-lived container** — it persists across runs and can
capture any Claude Code session, Themis or not. Pin a specific version (don't use
`:latest`), and give it a volume so logs survive a restart.

**Podman:**
```bash
podman run -d --name victorialogs \
  -p 9428:9428 \
  -v victorialogs-data:/victoria-logs-data \
  docker.io/victoriametrics/victoria-logs:<version>
```

**Docker:**
```bash
docker run -d --name victorialogs \
  -p 9428:9428 \
  -v victorialogs-data:/victoria-logs-data \
  victoriametrics/victoria-logs:<version>
```

VictoriaLogs ingests OTLP logs at `…:9428/insert/opentelemetry/v1/logs` and serves
queries (LogsQL) from its UI/API on the same port. (See the VictoriaLogs docs for
the current image tag and any auth flags.)

## 2. Point Themis and Claude Code at it

Put the OTLP env in **`{project}/.claude/settings.json`** — the one file that
configures your interactive CC, the sandbox CC, **and** Themis's own emitter (the
factory reads this file's `env` for its process; an explicit launch env always
wins). Use **your** endpoint host — the examples use placeholders:

```jsonc
{
  "includeCoAuthoredBy": false,
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
    "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://<your-otlp-host>:9428/insert/opentelemetry/v1/logs",
    "OTEL_SERVICE_NAME": "claude-code"
  }
}
```

## 3. Reach it from the sandbox

The factory runs in a container, so `<your-otlp-host>` must be reachable **from
inside the sandbox**:

- **Endpoint on your host** (the VictoriaLogs container publishes `-p 9428:9428`):
  use the host-gateway name — `host.containers.internal` (Podman) or
  `host.docker.internal` (Docker) — as `<your-otlp-host>`. On Docker/Linux you may
  need `--add-host=host.docker.internal:host-gateway` on the sandbox run.
- **Endpoint elsewhere on your network** (a homelab box, another host): use its
  hostname/IP directly.

## Any OTLP backend

VictoriaLogs is documented here only because it's the least setup. Themis emits
standard OTLP — swap in Vector→Loki, an existing collector, or anything else by
pointing `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` at it. Themis has no dependency on
VictoriaLogs. (The `diagnose-themis-run` skill's queries are written for the
documented backend; on a different store you still have the data, just not the
skill's ready-made queries.)

## Diagnosing a run

Once telemetry is flowing, the `diagnose-themis-run` skill reads it to explain a
failed run in your terms — and to tell you whether it's *your* problem (a
mis-scoped issue, config, git state) or *Themis's*. Install the interactive skills
(`themis skills install`) and reload Claude Code to get it.

## What this is — and isn't (read before you rely on it)

- **No phone-home.** Themis never sends telemetry to its authors, and there is no
  collector we operate. You run the endpoint; the data is yours.
- **Third-party sink.** VictoriaLogs (or whatever you run) is third-party software —
  its data handling is governed by its own terms. Read them.
- **Not private from the AI.** The `diagnose-themis-run` skill puts your telemetry
  in front of the AI you're using — like any AI-assisted debugging, that is not
  private. Know that when you use it.
- **May contain sensitive data.** Telemetry can include source, file paths,
  internal hostnames/IPs, and secrets echoed in error output. There is **no
  reliable way to auto-classify what's sensitive**, so **review anything before you
  share it** anywhere (see the bug-report skill's warnings).
