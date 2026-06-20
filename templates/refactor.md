# Refactor implementation for issue #{{ISSUE_NUMBER}}

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

One pass only. Review the code just written for: duplication, naming,
readability, adherence to the ubiquitous language and coding standards.

Make improvements. Run the test suite after each change.
If any test turns red: revert the change immediately — do not fix forward.
Do not loop — one refactor pass, then stop.

## Commit

If changes were made:
```
refactor(<scope>): clean up implementation for issue #{{ISSUE_NUMBER}}
```

If no refactoring was needed, do not create an empty commit — skip this step.

## Completion

First decide in one pass whether anything qualifies for refactoring.
If nothing qualifies, output immediately:

STEP COMPLETE — no changes

Do not re-scan, do not re-read files, do not look for more opportunities.

If refactoring was needed, commit the changes and output:

STEP COMPLETE
