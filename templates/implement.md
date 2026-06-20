# Implement issue #{{ISSUE_NUMBER}} until tests pass

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Standards digest (consult full CODING_STANDARDS.md only if needed)

- Error strings lowercase, no punctuation
- Wrap errors: `fmt.Errorf("doing X: %w", err)`
- HTTP clients must have Timeout set
- Context as first param on I/O functions
- Interfaces defined at consumer, not provider
- Import domain types, inject infrastructure operations
- Test files named by behaviour, not issue number
- Shared stubs, no per-issue duplicates
- No hardcoded hostnames/URLs/ports

## Instructions

Implement the code needed to make the failing tests pass.

When reading source files, use `grep -n` to find the relevant sections first,
then read only those sections. Do not read entire 1000+ line files — read the
specific functions or types you need to modify.

Work AC by AC — implement the minimum to pass each test, then move to the next.
Do not implement anything not required by an AC.

**Do NOT modify test files during implementation.** If a test is wrong, that
is a signal the AC needs clarification — comment on the issue and stop;
do not silently fix the test to match your implementation.

After each AC's tests pass, run the full test suite to confirm nothing regressed.

**Test-fix limit: 3 attempts per AC.** If a test won't pass after 3 distinct
implementation attempts, stop: comment on the issue explaining which AC is
failing and what was tried, add the `blocked` label, stop entirely.

When all ACs pass, run the full test suite. If anything fails, apply the same
3-attempt limit per failing test.

## Commit

```
feat(<scope>): implement issue #{{ISSUE_NUMBER}} — {{ISSUE_TITLE}}
```

## Completion

When the full test suite passes (GREEN) and changes are committed, output:

STEP COMPLETE

Do not refactor, do not improve code style, do not explore related files.
Commit the implementation and stop.
