# Refactor implementation for issue #{{ISSUE_NUMBER}}

## Project standards

Follow this project's coding standards, terminology, and architecture. They are
authoritative — projects differ in style, naming, error handling, and structure,
so read them before writing or changing code:

{{STANDARDS_DOCS}}

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
