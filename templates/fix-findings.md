# Fix blocking review findings for issue #{{ISSUE_NUMBER}} — cycle {{REVIEW_CYCLE}}

## Blocking findings to fix

{{BLOCKING_FINDINGS}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Coding standards

{{CODING_STANDARDS}}

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
