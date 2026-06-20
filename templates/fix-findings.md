# Fix blocking review findings for issue #{{ISSUE_NUMBER}} — cycle {{REVIEW_CYCLE}}

## Files changed in this PR

{{CHANGED_FILES}}

## Blocking findings to fix

{{BLOCKING_FINDINGS}}

Fix ONLY the specific issues listed above. Do not refactor surrounding code.
Do not address non-blocking findings. Change only the files and lines named
in the findings.

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Project standards

Follow this project's coding standards, terminology, and architecture. They are
authoritative — projects differ in style, naming, error handling, and structure,
so read them before writing or changing code:

{{STANDARDS_DOCS}}

## Instructions

Fix each blocking finding listed above. For each fix:
1. Make the minimal change that resolves the finding
2. Run the test suite to confirm GREEN — do not proceed to the next fix if tests are red
3. Do not expand scope beyond what the finding requires

Do not fix non-blocking findings. Do not modify test files unless the tests
themselves contain a blocking defect.

## Commit

```
fix(<scope>): resolve review findings cycle {{REVIEW_CYCLE}} for issue #{{ISSUE_NUMBER}}
```

## Completion

When all blocking findings are addressed and committed, output:

STEP COMPLETE

Fix only what the findings specify. Do not refactor surrounding code.
Do not address non-blocking findings.
