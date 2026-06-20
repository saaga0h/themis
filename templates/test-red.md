# Write failing tests for issue #{{ISSUE_NUMBER}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

Read `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md` from the workspace root
for coding standards and terminology rules.

## Instructions

If the acceptance criteria describe specific types/functions to move, extract, or
delete (refactoring), write the failing tests directly — do NOT delegate to
test-architect or test-writer. The targets are already enumerated in the ACs.
Skip to the Verification phase once the tests are written.

If the acceptance criteria describe new behaviour that requires target discovery
(feature work with "all", "every", "each" language), follow the test-red skill
and delegate to test-architect → test-writer as described below.

When reading source files, use `grep -n` to find the relevant sections first,
then read only those sections. Do not read entire 1000+ line files — read the
specific functions or types you need to modify.

Follow the **test-red** skill (`skills/test-red/SKILL.md`). The skill defines
the three-phase methodology: target enumeration → test implementation →
verification.

**Phase 1 — Target Enumeration:**
Delegate to **test-architect** via Task. Pass the Acceptance Criteria above
and issue number {{ISSUE_NUMBER}}.

Running in autonomous mode. Include this instruction in the delegation:
> Running in autonomous mode. Skip human confirmation gates and proceed directly.
> Do not wait for review — produce the AC-to-targets mapping and return it.

The test-architect must return an AC-to-targets mapping with explicit counts.
For exhaustive ACs ("all", "every", "each"), it must grep the codebase and list
every matching instance. Verify the mapping is complete before proceeding.

**Phase 2 — Test Implementation:**
Delegate to **test-writer** via Task. Pass the AC-to-targets mapping from
Phase 1, the issue number, and the coding standards.

Include this instruction in the delegation:
> Running in autonomous mode. Skip human confirmation gates and proceed directly.
> Write tests for every target in the mapping below. Do not skip any.

**Phase 3 — Verification (do not delegate):**
1. Count test functions written vs targets enumerated — if mismatch, send
   test-writer back with the missing targets
2. Verify no implementation files: `git diff --name-only | grep -v _test.go | grep -v .claude/ | grep -v doc.go`
3. Run tests — they MUST fail or fail to compile
4. If any test passes without implementation, the test is wrong — fix it

## Commit

Only after all verifications pass:

```
test(<scope>): add failing tests for issue #{{ISSUE_NUMBER}}
```

## Completion

When all tests are committed and the suite fails (RED state confirmed), output:

STEP COMPLETE

Do not re-verify, re-read files, or explore further. One commit, one confirmation, done.
If using the refactoring fast-path (no Task delegation), the same rule applies:
commit the tests and output STEP COMPLETE.
