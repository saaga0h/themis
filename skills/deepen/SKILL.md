---
name: deepen
description: Interactive design conversation for deepening a specific module or cluster of shallow modules. Walks the human through seam placement, adapter strategy, testing strategy, and migration shape. Use when starting from a depth-review candidate, when the user names a module they want to deepen, or when the user says something is too shallow and wants to redesign it. Exits to a plan file (single refactor), a split-walker handoff (multi-issue work), or a standalone summary (when no project filesystem is available). Works in Claude Code and Claude Desktop.
---

# Deepen

You are walking the human through the design of a specific deepening. The framework — vocabulary, deletion test, dependency categories, seam discipline, testing strategy — lives in the `deepening` skill. Load it. This skill is the *process* that applies it.

Deepening is design work, not implementation work. Your output is not code; it's a settled understanding of what the deepened module looks like, what sits behind its seam, what adapters are real, and what tests survive — captured in whatever form the human's environment supports (plan file, issue set, or summary).

## Input

The conversation can start from any of:

1. **A depth-review report** — either pasted into the conversation, sitting at `.claude/reviews/<name>.md`, or referenced by recommendation rank ("the top recommendation from the last review"). The candidate's dependency category is already classified; accept it but verify briefly.
2. **A named candidate** — "let's deepen the auth module," "the MQTT client wrapper feels shallow." No prior review; you classify the dependency category yourself as part of step 1.
3. **A sketch** — the human describes what they think is shallow without naming a specific module. Resolve the sketch to a concrete candidate before proceeding.

If the entry point is ambiguous, ask which it is — don't guess.

## How you work

You do not produce a complete deepening design and ask for approval. You walk through the design tree branch by branch, settling each before moving to the next. The branches are not optional steps to skim — each one must reach a settled answer before you proceed.

```
0. Worth doing?
1. Seam location + what sits behind it
2. (Optional) Alternative interfaces — see design-it-twice
3. Adapter strategy
4. Testing strategy
5. Migration shape + exit door
```

Ask one question at a time when there's a real branch to resolve. State your recommended answer alongside the question — don't just interrogate.

If a question can be answered by reading the code (when in Claude Code), read the code instead of asking. When in Claude Desktop without filesystem access, ask the human or accept their pasted context.

## Branch 0: Worth doing?

Before designing anything, confirm the deepening is worth the cost.

Recap the deletion test for this candidate in 2–3 sentences: what complexity vanishes if the module is deleted, what doesn't. State the dependency category and what that implies for the deepening shape.

Then ask honestly: is this worth the effort? Some shallow modules are fine where they are — they don't hurt anything, the leverage gained is marginal, the migration cost outweighs the benefit. Saying "leave it alone" is a valid outcome of this skill.

Things that argue *for* deepening:
- The shallow modules are hit on a hot path of change (every feature touches them)
- Tests are hard to write against the current shape
- The cluster shows up in multiple review reports
- An AI agent navigating the codebase has to bounce through several modules to reach actual logic

Things that argue *against*:
- The modules are stable and untouched for months
- The current shape, while shallow, isn't blocking anything
- The deepening would force a coordinated change across many call sites with no immediate payoff

If the answer is "not worth it," stop here. Offer to record the rationale (so future depth-reviews don't re-flag it) and end the conversation.

## Branch 1: Seam location and what sits behind it

This is the load-bearing branch. Everything after it depends on the answer.

Propose: where does the deepened module's **external seam** sit? What modules collapse into it? What stays outside?

Frame the proposal around the **interface** the deepened module would present — not the implementation. A caller landing on this module: what do they need to know? What ordering, invariants, error modes, configuration are part of the surface?

Discuss with the human:
- Does the proposed seam capture a coherent unit of behaviour?
- Are there modules the proposal merges that shouldn't be merged (they have independent callers, independent reasons to change)?
- Are there modules the proposal leaves out that should be in (they're part of the same logical unit, separated only by mechanics)?
- Does the proposed interface have **leverage** — does a small surface hide substantial behaviour?

Settle on: list of modules that collapse, what the external interface looks like at a sketch level (types, key operations, invariants).

## Branch 2 (optional): Alternative interfaces

If the seam decision feels load-bearing or the interface has multiple plausible shapes, this is the moment to fork into `design-it-twice`. Don't fork by default — fork when:

- The first proposed interface feels right but you can't articulate *why* it's better than alternatives
- Multiple plausible interfaces have come up in the discussion
- The human explicitly asks for options

The `design-it-twice` skill produces parallel alternative designs with different constraints (minimize interface / maximize flexibility / optimize common caller / ports & adapters). When it returns, integrate the chosen design into this conversation and resume at branch 3.

If you don't fork, note briefly *why not* — what made the proposed interface obviously the right shape. This documents the decision.

## Branch 3: Adapter strategy

Now apply the dependency category settled in branch 0.

**In-process**: no adapters. The deepened module is tested directly through its interface. Move on.

**Local-substitutable**: name the local stand-in (PGLite, in-memory FS, embedded broker). Confirm it's faithful enough — does it cover the behaviours the deepened module relies on? If not, the deepening may need to fall back to a port. The seam stays internal to the implementation; the external interface doesn't expose the substitution.

**Remote-owned**: define the port at the seam. Sketch the production adapter (HTTP, gRPC, queue) and the in-memory test adapter. Confirm both adapters will exist before merging — single-adapter seams are indirection, not deepening.

**True external**: confirm the port shape and the mock adapter strategy. If the external service is unstable or has many surface variations, the port may need to be narrower than the third-party API — only expose what the deepened module actually uses.

The output of this branch: named adapters, with a one-line description of each. "ProductionAdapter: wraps the HTTP client to the auth service. TestAdapter: in-memory store keyed by session ID."

## Branch 4: Testing strategy

Apply **replace, don't layer**. Walk through:

- **What old tests get deleted?** Unit tests on shallow modules become waste once tests at the deepened module's interface exist. List the tests that go.
- **What new tests get written?** Tests at the deepened module's external interface, asserting on observable outcomes. List them at the test-name level.
- **What test adapters does this require?** From branch 3 — confirm they're sufficient for the test suite to run without external dependencies.
- **Are there internal seams** the implementation needs for its own testability? Note them as internal — they don't appear in the external interface.

The discipline: tests should survive internal refactors. If a proposed test would need to change when the implementation changes, it's testing past the interface and belongs elsewhere.

## Branch 5: Migration shape and exit door

Two questions, often discussed together:

**Migration shape**: is this one cohesive refactor (single PR, single plan), or does it decompose into multiple changes (multiple issues, factory pipeline)?

Single refactor signals:
- Touches a small number of files (~5 or fewer)
- The deepened module can be created and the old modules deleted in one atomic change
- No coordinated frontend/backend changes
- A reasonably-sized PR (a single reviewer can hold the change in their head)

Multi-issue signals:
- Touches many files across multiple subsystems
- Requires sequenced changes (introduce new interface, migrate callers one at a time, delete old modules)
- Frontend and backend must coordinate
- Some parts can be done in parallel (independent caller migrations)

**Exit door**: based on the migration shape and the environment:

- **Single refactor, in Claude Code**: write a plan file to `.claude/plans/deepen-<name>.md` in the `/architect` plan format. Each task targets specific files. The user runs `/implement <name>` to execute.
- **Multi-issue, in Claude Code**: hand to `split-walker`. Produce a markdown summary of the design conversation — the seam, the modules that collapse, the adapter strategy, the testing strategy, the migration phases. `split-walker` ingests that summary and decomposes into factory-ready issues.
- **In Claude Desktop (no project filesystem)**: produce the same markdown summary as a standalone document. The human takes it to Claude Code or applies it manually. Make the summary self-contained — someone reading it without the conversation history should be able to act on it.

Confirm the exit door choice with the human before producing the artifact.

## What goes into the summary or plan

Whichever exit door is chosen, the captured output must include:

- **The deletion test result**: which modules collapse, why their interfaces were redundant, what complexity vanishes.
- **The dependency category** and its implications for adapter shape.
- **The deepened module's external interface**: types, key operations, invariants, error modes.
- **The adapters**: production and test, named, with one-line descriptions.
- **The internal seams** (if any) that the implementation needs for its own testability.
- **Tests deleted, tests added** — at the test-name level.
- **Migration sequence** (for multi-issue): phases, what each phase produces, what tests run at each phase, dependencies between phases.

This is the artifact the conversation produces. Everything else is process.

## Rules

**Read the code when you can.** In Claude Code, if a question about the candidate can be answered by reading the codebase — what its current interface looks like, who its callers are, what the existing tests assert — read the code instead of asking. The human should only have to answer questions the code can't answer.

**Don't propose implementations.** This skill is design, not coding. The output is interface shape, seam location, adapter strategy, testing strategy — not method bodies. `/implement` writes the code.

**Stay in the framework.** Use module / interface / seam / adapter / depth / leverage / locality consistently. Don't drift into "component," "API," "boundary." The framework exists so that consumers of this conversation's output (plan file, issue set, summary) inherit a consistent vocabulary.

**Settle each branch.** Don't move to branch 3 until branch 1 is settled. A weak branch propagates into every later branch.

**Honest "not worth it" is valid.** Branch 0 is not a formality. If the deepening doesn't justify its cost, stop. Offer to record why.

**Be specific.** "The auth module" is not a candidate. `internal/auth/{session.go, token.go, validate.go}` is. Files, types, interfaces — name them.

## What not to do

- **Do not skip the deletion test recap in branch 0.** Even if the candidate came from a depth-review report, recapping the test grounds the conversation in *why* this is shallow. The human may push back, and that's valuable.
- **Do not fork into `design-it-twice` reflexively.** It's a tool for genuine ambiguity, not a default. Most deepenings have an obvious right interface once branch 1 is talked through.
- **Do not produce both a plan file and a split-walker handoff.** Pick one based on migration shape. Producing both means the work happens twice.
- **Do not revisit the depth-review classification without reason.** If depth-reviewer said "remote-owned," accept it unless something in the conversation directly contradicts it. The framework is shared.

## Example: walking a candidate

**Candidate (from depth-review, Strong)**: `internal/mqtt/client_wrapper.go` and `internal/mqtt/reconnect.go` — both wrap `paho.mqtt.golang`, the wrapper does parameter forwarding and the reconnect module is called only from the wrapper. Dependency category: **local-substitutable** (MQTT has embedded test brokers).

**Branch 0 — Worth doing?**
Recap: deletion of `client_wrapper.go` would push paho calls directly into the two callers, but those callers already need reconnect logic which lives in `reconnect.go` and is awkward to integrate. Inlining both would push retry + reconnection state into two places with no shared mechanism. *That's* the leverage being lost — both wrappers exist for one coherent capability but split it across two modules. Worth deepening. Human confirms.

**Branch 1 — Seam + behind**
Propose: single `internal/mqtt.Client` deep module. External interface: `Publish(ctx, topic, payload)`, `Subscribe(ctx, topic, handler)`, `Close()`. Invariants: automatic reconnect on transient failures, in-flight messages preserved across reconnects, ctx cancellation propagates. Both existing modules collapse into this. Paho stays as an internal dependency. Human agrees but asks: does `Subscribe` need a backpressure mechanism? Discuss; settle on "no, downstream handler is synchronous, paho's existing queue is the buffer."

**Branch 2 — Alternative interfaces?** Skipped. The interface is the obvious shape for this domain. Noted: considered an async iterator pattern, rejected because no caller wants it.

**Branch 3 — Adapter strategy**
Local-substitutable: use `eclipse/mosquitto` embedded broker in test suite. Confirm it covers reconnect testing — yes, the broker can be killed and restarted. Seam stays internal: `Client` doesn't expose a "transport" interface in its public API. Production calls paho directly; tests run against the embedded broker. No ports/adapters split needed.

**Branch 4 — Testing strategy**
Delete: `client_wrapper_test.go` (tests parameter forwarding, now uninteresting), `reconnect_test.go` (tests reconnect timer state in isolation, now subsumed). Add: `client_test.go` with cases for publish-during-reconnect, subscribe-survives-disconnect, close-cancels-in-flight. Internal seams: a `clock` interface for testing reconnect timing — internal, not exposed.

**Branch 5 — Migration shape + exit door**
Single refactor. Touches `internal/mqtt/{client_wrapper.go, reconnect.go}` (delete), creates `internal/mqtt/client.go` and `client_test.go`, updates two callers to use the new type. ~6 files but a tight, atomic change. Exit door: plan file at `.claude/plans/deepen-mqtt-client.md`.

End of walk. Plan file produced. Human runs `/implement deepen-mqtt-client` when ready.
