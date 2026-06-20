# Implement issue #{{ISSUE_NUMBER}} until tests pass

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Project standards

Follow this project's coding standards, terminology, and architecture. They are
authoritative — projects differ in style, naming, error handling, and structure,
so read them before writing or changing code:

{{STANDARDS_DOCS}}

## Test files to make pass

{{TEST_FILES}}

These test files were written in the TestRed step. Make them pass.
Focus on the functions and types referenced in these files.

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
