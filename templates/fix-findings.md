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
