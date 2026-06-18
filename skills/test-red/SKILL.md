---
name: test-red
description: "Orchestrates the test-writing step of the pipeline. Delegates to test-architect for AC-to-targets enumeration, then to test-writer for test implementation. Ensures exhaustive ACs get grep-backed target lists before any tests are written, preventing the narrowed-search failure mode where targets found during exploration are lost during implementation."
---

# Test-Red Skill

Write failing tests for an issue's Acceptance Criteria. This skill orchestrates
two agents — **test-architect** for target enumeration and **test-writer** for
test implementation — to ensure every AC is covered before any implementation
code exists.

The test files produced by this skill ARE the specification for the implement
step. If a target has no test, the implement step will not fix it. Completeness
here determines correctness downstream.

---

## When to use

This skill is invoked during the test-red step of the pipeline. The caller
provides the issue number, the Acceptance Criteria, and the coding standards.

---

## Phase 1 — Target Enumeration

Delegate to **test-architect** via Task. Pass the Acceptance Criteria and the
issue number. In autonomous mode (factory pipeline), include this line in the
delegation prompt:

> Running in autonomous mode. Skip human confirmation gates and proceed directly.
> Do not wait for review — produce the AC-to-targets mapping and return it.

### What test-architect must produce

An **AC-to-targets mapping**: for each AC, the list of code targets that need
tests, with a count.

**Singular ACs** — an AC that describes one behaviour ("the function returns an
error when X", "the subcommand accepts `--provider gitea`") produces one or a
small number of targets. The test-architect identifies the entry point and
assertion intent.

**Exhaustive ACs** — an AC containing "all", "every", "each", "no X anywhere",
or implying completeness ("standardize X across Y") requires codebase
enumeration. The test-architect must:

1. Grep the codebase for every instance matching the AC's scope
2. List each instance explicitly: file path, line, function/call site name
3. State the count: "AC1 requires N tests — one per call site"

This enumeration is the critical step. The failure mode this skill exists to
prevent is: the agent finds N targets during exploration, then writes tests for
fewer than N. The explicit mapping with counts makes the gap visible.

### What test-architect returns

A structured mapping in its response (not a file). Example:

```
AC-to-Targets Mapping

AC1: "All git init calls use --initial-branch=main" — EXHAUSTIVE
  Targets (6):
  1. initTestRepo — internal/git/git_test.go:31
  2. initGitRepo — internal/checkpoint/checkpoint_test.go:26
  3. initIssue22Repo — internal/runner/runner_issue22_test.go:36
  4. initIssue22RepoWithRemote (bare) — internal/runner/runner_issue22_test.go:53
  5. initGitRepoForRunner — internal/runner/runner_checkpoint_test.go:32
  6. newTestGitRepo — cmd/themis/gitea_config_test.go:23

AC2: "Test helpers reference 'main' consistently" — SINGULAR
  Targets (1):
  1. Verify branch name in all helpers listed in AC1

...

Total: 5 ACs → 12 targets
```

### Before proceeding to Phase 2

Read the test-architect's mapping. Verify:

- Every AC has at least one target
- Exhaustive ACs have an explicit instance list from a codebase grep, not an
  assumption about what exists
- The total count is stated

If any AC has zero targets or an exhaustive AC lacks a grep-backed enumeration,
send the test-architect back with a specific correction: "AC3 says 'all X' but
you listed no targets — grep the codebase for X and enumerate every instance."

---

## Phase 2 — Test Implementation

Delegate to **test-writer** via Task. Pass:

- The AC-to-targets mapping from Phase 1 (paste it into the delegation prompt)
- The issue number
- The coding standards

In autonomous mode, include this line:

> Running in autonomous mode. Skip human confirmation gates and proceed directly.
> Write tests for every target in the mapping below. Do not skip any.

### What test-writer must produce

- One or more test files containing failing tests for every target in the mapping
- Tests must fail because the implementation does not exist yet — not because of
  syntax errors, missing imports, or misconfiguration
- No implementation files — only `*_test.go` files and minimal type stubs
  (empty struct, interface with no methods) needed for compilation

### What test-writer must NOT do

- Do not write implementation code
- Do not create intermediate files (skeletons, discovery documents, plans)
- Do not modify existing tests unrelated to this issue
- Do not reduce the target count from the mapping — every target gets a test

---

## Phase 3 — Verification

After test-writer returns, verify completeness yourself. Do not delegate this.

### 3a: Count check

Count the test functions written across all new test files. Compare against the
total target count from the Phase 1 mapping.

If the counts do not match: identify which targets from the mapping have no
corresponding test. Delegate back to test-writer with the specific missing
targets: "These targets from the mapping have no tests: [list]. Write tests for
them."

### 3b: No implementation files

```bash
git diff --name-only | grep -v _test.go | grep -v .claude/ | grep -v doc.go
```

If this produces output, implementation files were created. Delete them.

### 3c: RED confirmation

Run the tests. They must all fail or fail to compile. If any test passes without
implementation, the test is wrong — it does not specify anything. Fix it or flag
it.

### 3d: Commit

Only after all verifications pass:

```
test(<scope>): add failing tests for issue #<N>
```

---

## Autonomous mode

When invoked by the factory pipeline (`themis issue` or `themis run`), this skill
runs without human interaction. Both agents receive the autonomous mode
instruction in their delegation prompts. The gates they normally present to
humans (Specification Review, skeleton approval) are skipped.

When invoked interactively (e.g., via `/feature`), omit the autonomous mode
instruction. The agents will present their normal human gates.

The skill's methodology is identical in both modes. The only difference is
whether the agents pause for human confirmation.

---

## Anti-patterns this skill prevents

**Narrowed search** — the agent greps for targets during exploration, finds N,
then writes tests for fewer than N because it searched again with a narrower
query during implementation. The mapping from Phase 1 is the authoritative
target list. Phase 2 implements against the mapping, not against a fresh search.

**Implicit enumeration** — the agent "knows" there are several call sites but
does not list them. Exhaustive ACs require explicit, grep-backed enumeration
with file paths and line numbers. An unlisted target will not get a test.

**Skeleton files as ceremony** — the mapping stays in-context between Phase 1
and Phase 2. No intermediate files are written. The test files are the only
artifact that gets committed.

**Test-implementation coupling** — the test step must not think about
implementation. The test-architect decides what to test; the test-writer writes
tests that fail. Neither reasons about how to make the tests pass. That is the
implement step's job.
