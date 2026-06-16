# Write failing tests for issue #{{ISSUE_NUMBER}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Coding standards

{{CODING_STANDARDS}}

## Instructions

**This step writes ONLY test files. Do NOT create or modify any implementation files.**
**Do NOT think about the implementation yet. Focus only on what the ACs require.**

Each Acceptance Criteria item becomes one or more tests. For each AC:
- Determine the appropriate test type (unit, integration — refer to the coding standards)
- Write a test that asserts the exact behaviour the AC describes — not a proxy for it
- If the test needs minimal type stubs to compile, create the absolute minimum
  (empty struct, interface with no methods) — not the real implementation

## Verification — do not skip

After writing all test files, verify no implementation files were created:

```bash
git diff --name-only | grep -v _test.go | grep -v .claude/ | grep -v doc.go
```

If this command produces ANY output, you have created implementation files.
**Delete them now.** Only `*_test.go` files and minimal stubs (empty types in
existing files) are permitted at this point.

Run the tests — they MUST fail or fail to compile.
If any test passes without real implementation, the test is wrong — fix it.

## Commit

```
test(<scope>): add failing tests for issue #{{ISSUE_NUMBER}}
```
