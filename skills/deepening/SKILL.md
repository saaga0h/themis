---
name: deepening
description: Framework and vocabulary for thinking about module depth, seams, adapters, and dependency taxonomy. Load when reviewing for shallow modules, designing a deepened module's interface, or classifying dependencies to choose an adapter strategy. Provides shared language across depth-reviewer, /deepen, and design-it-twice.
---

# Deepening

This skill is a reference, not a process. It defines the vocabulary and frameworks that depth-reviewer (the agent that scans for shallow modules), `/deepen` (the conversation that designs a specific deepening), and design-it-twice (the parallel-design pattern) all share. Load it whenever a Themis flow is reasoning about module depth.

The goal of deepening is testability and AI-navigability: turn shallow modules — where the interface is nearly as complex as the implementation — into deep ones, where a small interface hides a large amount of behaviour. Deep modules are easier to test, easier to change, and easier for an agent to navigate without bouncing between many small pieces.

## Language

Use these terms exactly. Consistent vocabulary is the point — drift into "component," "service," "API," or "boundary" makes the framework unusable across consumers.

**Module.** Anything with an interface and an implementation. Deliberately scale-agnostic: a function, class, package, or tier-spanning slice are all modules. _Avoid:_ unit, component, service.

**Interface.** Everything a caller must know to use the module correctly. Type signature, yes, but also invariants, ordering constraints, error modes, required configuration, and performance characteristics. _Avoid:_ API, signature — both are too narrow.

**Implementation.** The code inside the module. Distinct from adapter: a thing can be a small adapter with a large implementation (a Postgres repo) or a large adapter with a small implementation (an in-memory fake). Reach for "adapter" when the seam is the topic; "implementation" otherwise.

**Depth.** Leverage at the interface — how much behaviour a caller or test can exercise per unit of interface they have to learn. A module is **deep** when a large amount of behaviour sits behind a small interface. A module is **shallow** when the interface is nearly as complex as the implementation.

**Seam** (Feathers). A place where you can alter behaviour without editing in that place. The *location* at which a module's interface lives. Choosing where to put the seam is its own design decision, distinct from what goes behind it. _Avoid:_ boundary — overloaded with DDD's bounded context.

**Adapter.** A concrete thing that satisfies an interface at a seam. Names a *role* (what slot it fills), not substance (what's inside).

**Leverage.** What callers get from depth. More capability per unit of interface they have to learn. One implementation pays back across N call sites and M tests.

**Locality.** What maintainers get from depth. Change, bugs, knowledge, and verification concentrate at one place rather than spreading across callers. Fix once, fixed everywhere.

## Principles

**Depth is a property of the interface, not the implementation.** A deep module can be internally composed of small, mockable, swappable parts — they just aren't part of the interface. A module can have **internal seams** (private to its implementation, used by its own tests) as well as the **external seam** at its interface. Don't expose internal seams through the external interface just because tests use them.

**The deletion test.** Imagine deleting the module. If complexity vanishes, it was a pass-through — it wasn't hiding anything. If complexity reappears across N callers, the module was earning its keep. The deletion test is the primary tool for spotting shallow modules in review.

**The interface is the test surface.** Callers and tests cross the same seam. If you want to test *past* the interface — reach into internal state, assert on private helpers — the module is probably the wrong shape. Tests at the external interface should survive internal refactors. If a test has to change when the implementation changes, it's testing past the interface.

**One adapter means a hypothetical seam. Two adapters means a real one.** Don't introduce a port unless at least two adapters justify it (typically production + test). A single-adapter seam is just indirection.

## Rejected framings

These are tempting but wrong, and named explicitly to prevent drift:

- **Depth as ratio of implementation-lines to interface-lines** (Ousterhout's original definition): rewards padding the implementation. We use depth-as-leverage instead — a module is deep because of how much behaviour it concentrates per unit of interface, not because of its line ratio.
- **"Interface" as the TypeScript `interface` keyword, or a class's public methods**: far too narrow. Interface here includes every fact a caller must know to use the module correctly.
- **"Boundary"**: overloaded with DDD's bounded context. Always say *seam* or *interface*.

## Dependency categories

When assessing a candidate for deepening, classify its dependencies. The category determines whether the candidate is deepenable, what shape the seam takes, and what kind of adapter the tests need.

### 1. In-process

Pure computation, in-memory state, no I/O. Always deepenable — merge the modules and test through the new interface directly. No adapter needed. The seam is just a function or method call.

### 2. Local-substitutable

Dependencies that have local test stand-ins: PGLite for Postgres, in-memory filesystem for disk, embedded broker for MQTT. Deepenable if the stand-in exists and is faithful enough. The deepened module is tested with the stand-in running in the test suite. The seam is internal to the module's implementation; no port at the external interface.

### 3. Remote but owned (Ports & Adapters)

Your own services across a network boundary — microservices, internal APIs, queues you publish to. Define a **port** (interface) at the seam. The deep module owns the logic; the transport is injected as an **adapter**. Tests use an in-memory adapter; production uses an HTTP/gRPC/queue adapter.

Recommendation shape: *"Define a port at the seam, implement an HTTP adapter for production and an in-memory adapter for testing, so the logic sits in one deep module even though it's deployed across a network."*

### 4. True external

Third-party services you don't control — Stripe, Twilio, OpenAI, weather APIs. The deepened module takes the external dependency as an injected port; tests provide a mock adapter that returns canned responses.

## Seam discipline

**One adapter is hypothetical, two is real.** Stated as a principle above; concretely: if you only ever have a production adapter, you don't have a seam, you have indirection. The seam pays for itself when there's a test adapter that exercises the same interface.

**Internal seams vs external seams.** A deep module can have internal seams that its own tests use without those seams becoming part of the module's public interface. The discipline: an internal seam exists for the module's *own* implementation flexibility; an external seam exists for the *caller's* flexibility. Don't expose internal seams through the external interface just because they were convenient for testing.

## Testing strategy

**Replace, don't layer.** Old unit tests on shallow modules become waste once tests at the deepened module's interface exist. Delete them. Keeping both means the test suite asserts on internals that no longer have a stable shape.

**Test at the deepened module's external interface.** Tests assert on observable outcomes through the interface, not internal state. If an internal refactor breaks a test, the test was reaching past the interface.

**Match test adapter to dependency category.** In-process: no adapter, direct calls. Local-substitutable: stand-in running in the suite. Remote-owned: in-memory port adapter. True external: mock adapter.

## How consumers use this skill

- **depth-reviewer** applies the deletion test and dependency categories to find and classify shallow candidates. Reports include the classification ("remote-owned, requires ports & adapters") so downstream consumers don't re-derive it.
- **`/deepen`** uses the full framework to structure the design conversation: where the seam goes, which adapter shapes are real, what tests survive, what gets deleted.
- **design-it-twice** uses the vocabulary so that parallel interface designs can be compared on the same axes — depth, locality, seam placement — rather than on superficial style.
