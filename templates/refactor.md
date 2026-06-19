# Refactor implementation for issue #{{ISSUE_NUMBER}}

Read `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md` from the workspace root
for coding standards and terminology rules.

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
