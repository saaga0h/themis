# Write failing tests for issue #{{ISSUE_NUMBER}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Coding standards

{{CODING_STANDARDS}}

## Instructions

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
