---
name: design-it-twice
description: Generate 3+ radically different designs for an interface, each optimized for a different design constraint (minimize interface, maximize flexibility, optimize common caller, ports & adapters). Compare them by depth, locality, and seam placement; produce an opinionated recommendation. Use when the right interface shape isn't obvious — inside /deepen for alternative interface exploration, or anywhere else committing to one design without seeing alternatives would be premature.
---

# Design It Twice

From Ousterhout: your first design is unlikely to be the best. This skill operationalizes that. Given an interface design problem, generate at least three **radically different** designs, compare them on the framework's axes, and recommend.

The framework — depth, leverage, locality, seam, adapter — lives in the `deepening` skill. Load it. This skill applies the framework to a specific design fork.

## When to use this

Not by default. The discipline is expensive — three designs cost three times one design. Use it when:

- The right interface shape isn't obvious from the problem statement
- Multiple plausible shapes have come up in conversation and you can't articulate which is best
- The interface will have many callers, or will be hard to change later
- The caller explicitly asks for alternatives

Don't use it when:

- The interface is constrained by an external contract you don't control
- One design is obviously right and you can articulate why
- The cost of revisiting later is low (small surface, few callers)

If you're in `/deepen` branch 2 and considering this skill, the rule of thumb is: fork when you'd struggle to defend the first design against a thoughtful objection. Don't fork when the first design is obvious.

## Input: the design brief

The caller provides:

1. **What's being designed.** The module name, what it does, what sits behind it.
2. **The dependency category** (from the `deepening` skill taxonomy): in-process, local-substitutable, remote-owned, or true external. Determines which constraint axes are relevant.
3. **The constraints.** What the interface must do — operations, invariants, ordering, error modes, performance characteristics.
4. **Project vocabulary.** If the project uses specific domain terms (e.g., "Order intake," "ingest pipeline"), use them. Designs that name things consistently with the existing codebase compare better than designs that invent parallel vocabularies.
5. **Custom constraint axes (optional).** The four standard axes below cover most interface design problems. If the caller has a domain-specific axis (e.g., "stateless vs stateful," "push vs pull"), accept it.

If the brief is missing any of (1)–(3), ask the caller. Don't proceed with hand-waved constraints — the divergence between designs only matters if each is anchored to a real constraint.

## Process

### 1. Frame the problem space

Write a short (3–6 sentence) user-facing explanation:

- What the interface is for, in the project's vocabulary
- The hard constraints (what every design must satisfy)
- The dependency category and what that implies
- A small code sketch of how a caller might use *some* version of this interface — not a proposal, just a way to make the constraints concrete

Show this to the user. Then immediately move to step 2 — the user reads while you generate.

### 2. Generate designs

The number of designs is at least 3, with 4 reasonable when ports & adapters is genuinely a separate concern.

Default axes:

- **Axis A — Minimize interface.** 1–3 entry points maximum. Maximize leverage per entry point. Hide configuration, branching, and edge cases behind defaults. The interface a new caller has to learn is small.
- **Axis B — Maximize flexibility.** Support many use cases and extension points. The interface exposes the structural variation. Configuration is explicit, parameters are fine-grained, the caller has full control.
- **Axis C — Optimize for the common caller.** The 99% case is trivial — one call with no parameters or one parameter. Escape hatches exist for the remaining cases but stay out of the default path.
- **Axis D (conditional) — Ports & adapters.** Only when the dependency category is remote-owned or true external. The interface is shaped around the port abstraction, with adapter substitution as a first-class concern. Production and test adapters are equally legible.

If the caller passed custom axes, replace one or more of the defaults with them. Keep axes orthogonal — three axes that all push toward "smaller interface" produce three similar designs and waste the exercise.

**In Claude Code**: spawn one Task per axis in parallel. Each Task receives a technical brief with the framework vocabulary, the project context, the dependency category, and *only the constraint for that axis*. Each must not see the other axes' constraints — the divergence depends on each generator being committed to its constraint.

**In Claude Desktop** (no Task tool): generate each design sequentially in the conversation. Frame each generation explicitly: "Now designing under Axis A — minimize interface." Reset framing between designs so each one is committed to its own constraint, not influenced by what the prior design produced.

Both modes produce the same artifact. The parallel mode is faster; the sequential mode requires self-discipline to not collapse the designs toward each other.

### 3. What each design must produce

Every design, regardless of axis, produces exactly these five pieces:

1. **Interface.** Types, operations, parameters. Plus the parts of "interface" that aren't in the type signature: invariants, ordering constraints, error modes, required configuration, performance characteristics.
2. **Usage example.** A short snippet showing how a caller actually uses this interface for a representative case. Real code shape, not pseudocode.
3. **What sits behind the seam.** What the implementation hides — composition, internal modules, state, dependencies.
4. **Dependency strategy and adapters.** Which adapters exist, what shape they take, what tests use. Apply the dependency category from the brief.
5. **Trade-offs.** Where leverage is high. Where it's thin. What the design is bad at. Be honest — every design has a worst case.

Do not let any design skip pieces. Comparison only works when all designs are described on the same axes.

### 4. Present and compare

Present the designs sequentially so the user can absorb each one fully before seeing the next. Don't interleave.

After all designs are out, write a comparison in prose. The framework's axes are the right ones to compare on:

- **Depth** — leverage at the interface. Which design hides the most behaviour behind the smallest surface?
- **Locality** — where change concentrates. If the underlying logic shifts, which design contains the blast radius?
- **Seam placement** — where the interface sits relative to the dependency boundary. Which design puts the seam in the cleanest place?

Also compare on the practical axes that matter for the project:

- How does each design handle the project's likely evolutions? (New use cases, new dependencies, new callers.)
- What does each design's test suite look like in practice?
- Which design's failure modes are easiest to debug?

### 5. Recommend

Be opinionated. The user wants a strong read, not a menu. Pick one design and say why. If elements from different designs would combine well, propose a hybrid explicitly — name which design contributes which element and why the combination works.

A recommendation has three parts:

1. **The pick** — which design, named clearly.
2. **The reason** — what made this design stronger than the alternatives, in terms of the framework's axes.
3. **What it costs** — what the picked design is *worst* at. Every design has a worst case; naming it makes the recommendation honest.

If the comparison genuinely doesn't have a clear winner — two designs are strong on different axes and the project's future evolution will determine which wins — say so. But default to opinionated. Indecision is the worse failure mode.

## Rules

**Designs must be radically different.** Three designs that vary in naming and parameter order are not three designs. The constraint axes exist to force divergence — if your designs are converging, your axes aren't orthogonal.

**Stay in the framework.** Use module, interface, seam, adapter, depth, leverage, locality consistently across all designs. Drift into "component" or "API" makes comparison harder, not easier.

**Each design is committed to its axis.** Axis A is not "a smaller version of Axis B." Each design embraces its constraint as the primary value, and accepts the trade-offs that come with it. A timid Axis A design that hedges toward flexibility is worse than a confident minimal one — it neither minimizes nor flexes.

**Use the project's vocabulary.** If the project has established names for domain concepts, use them. Don't invent parallel vocabulary in the designs — it makes the recommendation harder to act on.

**Be specific.** "An Order intake interface" is not a design. `Intake.Submit(ctx, OrderDraft) (OrderID, error)` with explicit invariants and error modes is.

**Honest trade-offs.** Each design's trade-offs section must name a real weakness. "This design is great at everything" is a failed design description — every constraint costs something.

## What not to do

- **Do not present designs as a ranked list.** Present them as equals; the ranking comes in the comparison step. Pre-ranking biases the user.
- **Do not produce a recommendation that hedges.** "Both designs have merit" is not a recommendation. Pick one.
- **Do not skip the framing step.** Without a shared problem space, the user can't compare the designs against the same constraints.
- **Do not generate four designs when three suffice.** Adding a fourth axis just to fill space dilutes attention without adding signal.
- **Do not let the sequential mode (Desktop) collapse into convergence.** Each design must commit to its axis even when generated by the same conversation; if you find yourself reaching for the same shape twice, stop and re-anchor on the constraint.

## Example: a retry mechanism

**Brief.** A retry-with-backoff primitive for Go services. Dependency category: in-process. Constraints: must support context cancellation, must support different backoff strategies (constant, exponential, full jitter), must distinguish retryable from non-retryable errors. Project uses standard Go context patterns.

**Frame.** "Retry wraps a fallible operation, re-invoking it on retryable failures with configurable backoff until success, exhaustion, or context cancellation. Used across all RPC clients and queue consumers." Code sketch:

```go
err := retry.Do(ctx, func() error {
    return client.Send(req)
})
```

**Design A — Minimize interface.**

Interface: `retry.Do(ctx, fn func() error) error`. Backoff is exponential with full jitter, fixed defaults (100ms base, 30s cap, max 5 attempts). Retryable errors are determined by an internal `IsRetryable(err)` function checking for `net.Error`, `context.DeadlineExceeded`, and a sentinel `retry.Retryable` wrapper.

Behind the seam: the backoff schedule, the timer management, the error classification, the attempt counter.

Adapters: none (in-process). Tests use `retry.Do` directly with a fake clock injected via `package-private` setter.

Trade-offs: high leverage for the common case — every retry site is one line. Cost: callers that need a non-default backoff have no way to express it without reaching past the interface. If the defaults are wrong, fixing them is a global change.

**Design B — Maximize flexibility.**

Interface: `retry.New(BackoffStrategy).WithMaxAttempts(n).WithClassifier(fn).Do(ctx, fn)`. `BackoffStrategy` is an interface; `Constant`, `Exponential`, `FullJitter` implement it. `Classifier` is `func(error) bool`. Builder is reusable across call sites.

Behind the seam: the loop, the clock, the cancellation propagation.

Adapters: none (in-process). Tests can either use the builder with a fake clock strategy, or wire mock classifiers.

Trade-offs: every site can tune every parameter. Cost: every site *must* tune every parameter (or accept verbose defaults). The 99% case is now five lines, not one. New developers learn the entire builder before they can retry anything.

**Design C — Optimize for the common caller.**

Interface: `retry.Do(ctx, fn)` for the common case (exponential full jitter, 5 attempts, standard error classification). `retry.DoWith(ctx, fn, opts...)` with functional options (`retry.WithBackoff`, `retry.WithMaxAttempts`, `retry.WithClassifier`) for callers that need to tune.

Behind the seam: same as A, plus the options machinery.

Adapters: none. Tests use either form with an injected clock.

Trade-offs: 99% case is one line, escape hatch is a few more. Cost: two entry points to learn instead of one, and the options machinery is slightly more code than either A or B.

**Comparison.**

Depth: A is deepest — one function, large behaviour. C is slightly less deep (two entry points). B is shallowest — the builder is exposed structure.

Locality: A wins. Changing the default backoff shape happens in one place, every call site benefits. B disperses the choice across every call site; changing defaults requires touching all of them. C centralizes the common path while letting the rare path opt in.

Seam placement: all three put the seam at the call site; B additionally exposes the strategy interface, which is a seam in its own right (good for testing strategies, but a real surface that has to stay stable).

**Recommendation: Design C.**

The reason: most retry sites in this project are RPC clients and queue consumers with similar needs — the common case is genuinely common. Design A is tempting for its depth, but the project already has cases that need non-default classifiers (some errors are app-specific), and Design A would force those cases to bypass the seam. Design B's flexibility is paid for at every call site, which is a high tax on the 99% case.

The cost: two entry points, and a slightly more complex implementation than Design A. Worth it.
